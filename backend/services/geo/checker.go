package geo

import (
	"context"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai"
	"geo-tool/services/points"
)

// maxResponseLen 入库时回答截断长度（前 N 字符）。
// 为支撑"原始答案留存 + 引用溯源"的商用要求，需要足够长以保留完整回答。
const maxResponseLen = 6000

// citationURLRe 提取回答文本中的引用 URL。
// 仅匹配 ASCII URL 合法字符（RFC 3986），遇到中文/全角标点/空格即停止，
// 避免把「https://x.com）发布的公告为准」这类后续中文误并入 URL。
var citationURLRe = regexp.MustCompile(`https?://[A-Za-z0-9\-._~:/?#\[\]@!$&*+,;=%]+`)

// platformGate 同平台请求节流门：同一平台的请求完全串行（一次只有一个在途），
// 且两次请求之间至少间隔 interval（默认 800ms），从根上规避各平台 RPM 限流(429)。
// 典型配置：Kimi 免费档建议 20000ms，其余平台 0（走默认 800ms）即可。
type platformGate struct {
	mu       sync.Mutex
	lastAt   time.Time
	interval time.Duration
}

// do 在门内执行 f：拿锁排队 → 补足间隔 → 记录时间 → 执行请求
func (g *platformGate) do(f func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.lastAt.IsZero() && g.interval > 0 {
		if wait := time.Until(g.lastAt.Add(g.interval)); wait > 0 {
			time.Sleep(wait)
		}
	}
	g.lastAt = time.Now()
	f()
}

// minPlatformInterval 平台未配置 interval_ms 时的默认同平台间隔
const minPlatformInterval = 800 * time.Millisecond

// RecoverStaleTasks 服务启动时回收僵尸任务：
// 上一次进程退出/崩溃时仍处于 running 的任务永远不会结束，会永久阻塞该租户发起新巡检。
// 启动时统一标记为 failed 并计入 error_count，保证可立即重新发起巡检。
func RecoverStaleTasks() {
	now := time.Now()
	res := database.DB.Model(&models.CheckTask{}).
		Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status":      "failed",
			"error_count": 1,
			"finished_at": &now,
		})
	if res.RowsAffected > 0 {
		log.Printf("[geo] 已回收 %d 个僵尸巡检任务（running -> failed）", res.RowsAffected)
	}
}

// RunTenantTask 执行指定租户的一次完整巡检：启用平台 × 启用关键词 全组合查询
// 返回是否启动成功（若该租户已有 running 任务则返回 false）
func RunTenantTask(tenantID uint, mode string) bool {
	db := database.DB
	var running int64
	db.Model(&models.CheckTask{}).Where("tenant_id = ? AND status = ?", tenantID, "running").Count(&running)
	if running > 0 {
		log.Printf("[geo] 租户#%d 已有巡检任务运行中，跳过", tenantID)
		return false
	}
	now := time.Now()
	task := models.CheckTask{TenantID: tenantID, Mode: mode, Status: "running", StartedAt: &now}
	if err := db.Create(&task).Error; err != nil {
		log.Printf("[geo] 创建任务失败: %v", err)
		return false
	}

	go execute(tenantID, &task)
	return true
}

func execute(tenantID uint, task *models.CheckTask) {
	db := database.DB
	start := time.Now()

	// 平台 = 全局平台(tenant_id=0) + 分站覆盖层合并，仅取启用平台
	var platforms []models.AiPlatform
	db.Where("tenant_id = ?", 0).Order("sort_order asc, id asc").Find(&platforms)
	if tenantID != 0 {
		var ovs []models.TenantPlatformOverride
		db.Where("tenant_id = ?", tenantID).Find(&ovs)
		ovMap := map[uint]models.TenantPlatformOverride{}
		for _, o := range ovs {
			ovMap[o.PlatformID] = o
		}
		for i := range platforms {
			if o, ok := ovMap[platforms[i].ID]; ok {
				if o.APIKey != "" {
					platforms[i].APIKey = o.APIKey
				}
				if o.Enabled != nil {
					platforms[i].Enabled = *o.Enabled
				}
			}
		}
	}
	enabledPlats := platforms[:0]
	for _, p := range platforms {
		if p.Enabled {
			enabledPlats = append(enabledPlats, p)
		}
	}
	platforms = enabledPlats

	var keywords []models.GeoKeyword
	db.Where("tenant_id = ? AND enabled = ?", tenantID, true).Find(&keywords)

	// 默认品牌词：分站设置 settings.default_brand 优先，回退全局 GEO_DEFAULT_BRAND。
	// 关键词未单独配置品牌词时，用默认品牌词做命中检测（与前端提示「留空则使用系统默认品牌词」一致）。
	defaultBrand := tenantBrand(tenantID)

	total := len(platforms) * len(keywords)
	db.Model(task).Updates(map[string]interface{}{
		"total_queries": total, "status": "running",
	})

	if total == 0 {
		fin := time.Now()
		db.Model(task).Updates(map[string]interface{}{
			"status": "failed", "finished_at": &fin, "error_count": 1,
		})
		log.Printf("[geo] 租户#%d 任务#%d 无可用配置，标记失败", tenantID, task.ID)
		return
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	hitCount, missCount, errCount := 0, 0, 0
	hitPlatforms := map[string]bool{}

	// 每个平台一个节流门：同平台串行 + 最小间隔，避免并发轰炸触发 429
	gates := make(map[uint]*platformGate, len(platforms))
	for _, p := range platforms {
		interval := time.Duration(p.IntervalMs) * time.Millisecond
		if interval <= 0 {
			interval = minPlatformInterval
		}
		gates[p.ID] = &platformGate{interval: interval}
	}

	sem := make(chan struct{}, 8)

	for _, p := range platforms {
		for _, k := range keywords {
			wg.Add(1)
			sem <- struct{}{}
			go func(p models.AiPlatform, k models.GeoKeyword) {
				defer wg.Done()
				defer func() { <-sem }()

				res := models.CheckResult{
					TenantID: tenantID, TaskID: task.ID, PlatformID: p.ID, PlatformName: p.Name,
					KeywordID: k.ID, Question: k.Question, BrandKeywords: k.BrandKeywords,
				}

				qStart := time.Now()
				// 每次 AI 调用前扣 1 点点卡，余额不足则记为失败
				if derr := points.DeductOne(tenantID, "GEO 智能巡检"); derr != nil {
					res.ErrorMsg = derr.Error()
					res.Hit = false
					res.HitPosition = -1
					res.CostMs = time.Since(qStart).Milliseconds()
					mu.Lock()
					errCount++
					mu.Unlock()
					mu.Lock()
					db.Create(&res)
					mu.Unlock()
					return
				}
			client := ai.NewClient(p.BaseURL, p.APIKey, p.Model)
			// 429 限流退避重试：最多重试 2 次，间隔 5s / 15s；整个重试序列在平台节流门内执行，
			// 保证同一平台任何时刻只有一个请求在途（彻底消除并发 429）
			var answer string
			var err error
			backoffs := []time.Duration{5 * time.Second, 15 * time.Second}
			gates[p.ID].do(func() {
				for attempt := 0; ; attempt++ {
					ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
					answer, err = client.Ask(ctx, k.Question)
					cancel()
					if err == nil || attempt >= len(backoffs) || !strings.Contains(err.Error(), "429") {
						break
					}
					time.Sleep(backoffs[attempt])
				}
			})
				res.CostMs = time.Since(qStart).Milliseconds()

				if err != nil {
					res.ErrorMsg = err.Error()
					res.Hit = false
					res.HitPosition = -1
					mu.Lock()
					errCount++
					mu.Unlock()
			} else {
				q := strings.ToLower(strings.TrimSpace(k.BrandKeywords))
				if q == "" {
					q = strings.ToLower(strings.TrimSpace(defaultBrand))
				}
				low := strings.ToLower(answer)
				hit, pos, mentions := detectBrand(q, low)
					res.Hit = hit
					res.HitPosition = pos
					res.MentionCount = mentions
					res.Response = truncate(answer, maxResponseLen)
					mu.Lock()
					if hit {
						hitCount++
						hitPlatforms[p.Name] = true
					} else {
						missCount++
					}
					mu.Unlock()
				}

				mu.Lock()
				db.Create(&res)
				mu.Unlock()

				// 引用溯源：从完整原始回答中提取 URL 落库（供溯源分析与引用率统计）
				if err == nil && res.ID > 0 && strings.Contains(answer, "http") {
					saveCitations(res, answer)
				}
			}(p, k)
		}
	}
	wg.Wait()

	fin := time.Now()
	status := "success"
	if errCount > 0 {
		status = "partial"
	}
	if errCount == total {
		status = "failed"
	}
	db.Model(task).Updates(map[string]interface{}{
		"status": status, "hit_count": hitCount, "miss_count": missCount,
		"error_count": errCount, "coverage": len(hitPlatforms), "finished_at": &fin,
	})
	log.Printf("[geo] 租户#%d 任务#%d 完成 命中=%d 未命中=%d 错误=%d 覆盖平台=%d 耗时=%v",
		tenantID, task.ID, hitCount, missCount, errCount, len(hitPlatforms), time.Since(start))
}

// tenantBrand 返回分站默认品牌词：settings.default_brand 优先，回退全局 GEO_DEFAULT_BRAND。
func tenantBrand(tenantID uint) string {
	if tenantID > 0 {
		var s models.Setting
		if err := database.DB.Where("tenant_id = ? AND key = ?", tenantID, "default_brand").First(&s).Error; err == nil {
			if v := strings.TrimSpace(s.Value); v != "" {
				return v
			}
		}
	}
	return config.Load().DefaultBrand
}

// detectBrand 检测品牌词（逗号分隔）在回答中的命中情况
func detectBrand(brandKeywords, lowerAnswer string) (bool, int, int) {
	if strings.TrimSpace(brandKeywords) == "" {
		return false, -1, 0
	}
	// 按字符（rune）处理：HitPosition 语义为"品牌词在回答中第几个字符位置"（1 起始），
	// 与仪表盘 top1/top3/top10 分档口径一致；此前按字节偏移会导致中文场景位置虚高且出现 0 值。
	runes := []rune(lowerAnswer)
	words := strings.Split(brandKeywords, ",")
	bestPos := -1
	mentions := 0
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		wr := []rune(strings.ToLower(w))
		idx := 0
		for {
			pos := indexRunes(runes[idx:], wr)
			if pos < 0 {
				break
			}
			abs := idx + pos
			mentions++
			if bestPos < 0 || abs < bestPos {
				bestPos = abs
			}
			idx = abs + len(wr)
		}
	}
	if bestPos < 0 {
		return false, -1, mentions
	}
	return true, bestPos + 1, mentions
}

// indexRunes 在 rune 切片 haystack 中查找 needle 首次出现的位置（按 rune 计），未找到返回 -1。
func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// saveCitations 从完整回答中提取引用 URL 写入溯源表（URL 去重，最多 20 条）
func saveCitations(res models.CheckResult, answer string) {
	urls := citationURLRe.FindAllString(answer, -1)
	if len(urls) == 0 {
		return
	}
	seen := map[string]bool{}
	citations := make([]models.Citation, 0, len(urls))
	pos := 0
	for _, u := range urls {
		u = strings.TrimRight(u, ".,;:!?)]}）】》\"'")
		if u == "" {
			continue
		}
		// 过滤本地/示例地址（AI 回答中的代码示例，非真实引用来源）
		low := strings.ToLower(u)
		if strings.HasPrefix(low, "http://127.0.0.1") || strings.HasPrefix(low, "http://localhost") || strings.HasPrefix(low, "https://localhost") || strings.HasPrefix(low, "http://0.0.0.0") {
			continue
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		pos++
		citations = append(citations, models.Citation{
			TenantID:     res.TenantID,
			ResultID:     res.ID,
			TaskID:       res.TaskID,
			PlatformName: res.PlatformName,
			Question:     res.Question,
			URL:          u,
			Domain:       extractDomain(u),
			Position:     pos,
		})
		if pos >= 20 {
			break
		}
	}
	if len(citations) > 0 {
		database.DB.Create(&citations)
	}
}

// extractDomain 提取 URL 的主域名（去除 www. 前缀），便于按来源聚合
func extractDomain(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := u.Hostname()
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	return host
}
