package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
	"geo-tool/services/pay"
)

// 敏感密钥配置 Key：查询接口必须脱敏，绝不下发明文。
const (
	KeySmsAccessKeySecret = "aliyun_sms_access_key_secret"
	KeyOssAccessKeySecret = "aliyun_oss_access_key_secret"
	KeyDoubaoImageAPIKey  = "doubao_image_api_key" // 豆包文生图（Seedream）API Key
)

// secretMasked 已设置占位标记：仅告知已配置，不下发任何真实密钥内容。
const secretMasked = "******"

// isSecretKey 判断是否为需脱敏的密钥配置项（短信 / OSS 的 AccessKey Secret、豆包 API Key、支付密钥）
func isSecretKey(key string) bool {
	return key == KeySmsAccessKeySecret || key == KeyOssAccessKeySecret ||
		key == KeyDoubaoImageAPIKey || pay.IsSecretKey(key)
}

// GetSettings 读取租户级配置。短信/OSS 的 AccessKey Secret 一律脱敏返回，
// 仅以占位符标识「已设置」，绝不返回明文。
func GetSettings(c *gin.Context) {
	tid := TenantID(c)
	var settings []models.Setting
	database.DB.Where("tenant_id = ?", tid).Find(&settings)
	m := map[string]string{}
	for _, s := range settings {
		if isSecretKey(s.Key) && s.Value != "" {
			m[s.Key] = secretMasked
		} else {
			m[s.Key] = s.Value
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": m})
}

// SaveSettings 保存租户级配置。
// 对短信/OSS 的 AccessKey Secret：提交为空（前端留空=保留原密钥）或仍为脱敏占位符
// （前端未修改原样回传）时跳过不写，仅当填写了新值才覆盖。
func SaveSettings(c *gin.Context) {
	tid := TenantID(c)
	var body map[string]string
	if !jsonBody(c, &body) {
		return
	}
	db := database.DB
	for k, v := range body {
		if strings.TrimSpace(k) == "" {
			continue
		}
		// 敏感密钥：留空或为占位符 → 保留原密钥，不覆盖
		if isSecretKey(k) {
			v = strings.TrimSpace(v)
			if v == "" || v == secretMasked {
				continue
			}
		}
		var s models.Setting
		if err := db.First(&s, "tenant_id = ? AND key = ?", tid, k).Error; err == nil {
			s.Value = strings.TrimSpace(v)
			db.Save(&s)
		} else {
			db.Create(&models.Setting{TenantID: tid, Key: k, Value: strings.TrimSpace(v)})
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存"})
}

// GenerateReport 生成 GEO 优化报告（Markdown，按租户）
func GenerateReport(c *gin.Context) {
	tid := TenantID(c)
	days := 7
	if v := c.DefaultQuery("days", "7"); v != "" {
		if n := parseDay(v); n > 0 {
			days = n
		}
	}
	since := biztime.Since(days)

	var results []models.CheckResult
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).Find(&results)

	var b strings.Builder
	b.WriteString("# GEO 优化报告\n\n")
	b.WriteString(fmt.Sprintf("> 统计周期：%s ~ %s\n\n", since.Format("2006-01-02"), biztime.Today()))

	total, hit, miss, errs := len(results), 0, 0, 0
	platformHits := map[string]int{}
	platformTotals := map[string]int{}
	kwHits := map[string]int{}
	kwTotals := map[string]int{}
	for _, r := range results {
		platformTotals[r.PlatformName]++
		kwTotals[r.Question]++
		if r.ErrorMsg != "" {
			errs++
			continue
		}
		if r.Hit {
			hit++
			platformHits[r.PlatformName]++
			kwHits[r.Question]++
		} else {
			miss++
		}
	}
	rate := 0.0
	if total > 0 {
		rate = float64(hit) / float64(total) * 100
	}
	b.WriteString(fmt.Sprintf("## 一、总览\n\n- 查询总数：%d\n- 命中品牌：%d\n- 未命中：%d\n- 请求错误：%d\n- **品牌出现率：%.1f%%**\n\n", total, hit, miss, errs, rate))

	b.WriteString("## 二、各平台表现\n\n| 平台 | 查询数 | 命中 | 出现率 |\n|---|---|---|---|\n")
	for name, t := range platformTotals {
		h := platformHits[name]
		r := float64(h) / float64(t) * 100
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", name, t, h, r))
	}

	b.WriteString("\n## 三、各问题表现\n\n| 问题 | 查询数 | 命中 | 出现率 |\n|---|---|---|---|\n")
	for q, t := range kwTotals {
		h := kwHits[q]
		r := float64(h) / float64(t) * 100
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", q, t, h, r))
	}

	b.WriteString("\n## 四、优化建议\n\n")
	if rate >= 50 {
		b.WriteString("- 品牌出现率良好，建议保持内容更新频率，巩固各平台引用\n")
	}
	if rate < 50 && total > 0 {
		b.WriteString("- 品牌出现率偏低，建议：\n")
		b.WriteString("  1. 在权威平台（知乎/公众号/36氪）增加行业观点与案例内容\n")
		b.WriteString("  2. 让官网内容结构化（FAQ/表格/数据）便于 AI 摘录\n")
		b.WriteString("  3. 对照未命中的问题，针对性铺设含品牌词的自然回答\n")
	}
	if errs > 0 {
		b.WriteString(fmt.Sprintf("- 注意：有 %d 条请求失败，请检查对应平台的 API Key 与计费状态\n", errs))
	}
	if total == 0 {
		b.WriteString("- 本周期暂无巡检数据，请先完成平台与关键词配置后运行巡检\n")
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"report": b.String(), "days": days}})
}

// AdviceItem 结构化优化建议
type AdviceItem struct {
	Level string `json:"level"` // good / warn / bad
	Title string `json:"title"`
	Desc  string `json:"desc"`
}

// ReportData GEO 可视化报告聚合数据：KPI + 趋势 + 占比 + 平台表 + 关键词 + 洞察 + 建议
func ReportData(c *gin.Context) {
	results, days, since := queryResultsInRange(c, 7)
	a := buildAnalysis(results, days, since)
	tid := TenantID(c)

	// 场景覆盖分布（按关键词分类）
	type sceneRow struct {
		Category string
		Cnt      int64
	}
	var scenes []sceneRow
	database.DB.Model(&models.GeoKeyword{}).
		Select("coalesce(nullif(category,''),'未分类') as category, count(*) as cnt").
		Where("tenant_id = ?", tid).
		Group("category").Scan(&scenes)
	sceneDist := make([]gin.H, 0, len(scenes))
	for _, s := range scenes {
		sceneDist = append(sceneDist, gin.H{"name": s.Category, "value": s.Cnt})
	}

	// 关键词 TOP10
	kwTop := a.Keywords
	if len(kwTop) > 10 {
		kwTop = kwTop[:10]
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"days":         a.Days,
		"period":       a.Period,
		"generated_at": biztime.DateTimeSec(),
		"kpi":          a.Totals,
		"trend":        a.Trend,
		"rank_dist":    a.RankDist,
		"platforms":    a.Platforms,
		"keywords_top": kwTop,
		"weak_words":   a.WeakWords,
		"scene_dist":   sceneDist,
		"insights":     a.Insights,
		"advice":       genAdvice(a, sceneDist),
		"has_data":     a.Totals.Total > 0,
	}})
}

// genAdvice 基于分析结果生成分级优化建议
func genAdvice(a *Analysis, scenes []gin.H) []AdviceItem {
	var list []AdviceItem
	t := a.Totals

	if t.Total == 0 {
		list = append(list, AdviceItem{Level: "warn", Title: "暂无巡检数据", Desc: "本周期还没有巡检记录。请先配置 AI 平台与监测关键词，点击「开始巡检查询」后回来生成报告。"})
		return list
	}

	// 1) 品牌出现率
	switch {
	case t.BrandRate >= 80:
		list = append(list, AdviceItem{Level: "good", Title: "品牌可见度表现优秀", Desc: fmt.Sprintf("品牌出现率 %.1f%%，各平台引用稳定。建议保持内容更新频率，持续巩固现有阵地。", t.BrandRate)})
	case t.BrandRate >= 50:
		list = append(list, AdviceItem{Level: "good", Title: "品牌可见度处于良好水平", Desc: fmt.Sprintf("品牌出现率 %.1f%%，总体达标。建议针对未命中的问题补充含品牌词的自然回答，冲击更高覆盖率。", t.BrandRate)})
	default:
		list = append(list, AdviceItem{Level: "bad", Title: "品牌可见度偏低", Desc: fmt.Sprintf("品牌出现率仅 %.1f%%，需要重点整改。建议优先在权威平台铺设含品牌词的内容，并对薄弱关键词逐条优化。", t.BrandRate)})
	}

	// 2) TOP3 覆盖率
	if t.Top3Rate < 30 && t.Success > 0 {
		list = append(list, AdviceItem{Level: "warn", Title: "TOP3 覆盖待提升", Desc: fmt.Sprintf("品牌进入推荐前 3 位的比例仅 %.1f%%。AI 优先引用前三位内容，建议用结构化内容（FAQ/数据表/对比清单）提升被摘录的概率。", t.Top3Rate)})
	} else if t.Success > 0 {
		list = append(list, AdviceItem{Level: "good", Title: "TOP3 覆盖良好", Desc: fmt.Sprintf("品牌进入推荐前 3 位的比例达 %.1f%%，是被 AI 引用的高优位置，保持并微调内容即可。", t.Top3Rate)})
	}

	// 3) 平台：最佳 / 薄弱
	if len(a.Platforms) > 0 {
		best := a.Platforms[0]
		list = append(list, AdviceItem{Level: "good", Title: fmt.Sprintf("「%s」表现最佳", best.Name), Desc: fmt.Sprintf("出现率 %.1f%%，可加大在该平台的优质内容投放，作为重点阵地。", best.Rate)})
		if len(a.Platforms) > 1 {
			worst := a.Platforms[len(a.Platforms)-1]
			if worst.Queries > 0 && worst.Rate < t.BrandRate {
				list = append(list, AdviceItem{Level: "warn", Title: fmt.Sprintf("「%s」覆盖偏弱", worst.Name), Desc: fmt.Sprintf("出现率仅 %.1f%%，低于整体水平。建议增加该平台的内容引用密度与权威信源背书。", worst.Rate)})
			}
		}
	}

	// 4) 薄弱关键词
	if len(a.WeakWords) > 0 {
		names := make([]string, 0, len(a.WeakWords))
		for _, w := range a.WeakWords {
			names = append(names, w.Question)
		}
		list = append(list, AdviceItem{Level: "bad", Title: fmt.Sprintf("%d 个薄弱问题需优先处理", len(a.WeakWords)), Desc: fmt.Sprintf("以下问题多次未命中品牌词：%s。可在各平台铺设含品牌名称与业务关键词的自然问答、案例与数据段落。", strings.Join(names, "；"))})
	}

	// 5) 场景覆盖
	if len(scenes) < 3 {
		list = append(list, AdviceItem{Level: "warn", Title: "关键词场景覆盖较单一", Desc: fmt.Sprintf("当前仅覆盖 %d 个场景分类。建议按用户高频咨询场景（如服务介绍、价格、案例、对比选型等）扩充关键词，形成场景矩阵。", len(scenes))})
	} else {
		list = append(list, AdviceItem{Level: "good", Title: "关键词场景矩阵完整", Desc: fmt.Sprintf("已覆盖 %d 个场景分类，结构完整。建议定期根据新业务或热门话题补充场景关键词，保持与时俱进。", len(scenes))})
	}

	// 6) 请求错误
	if t.Errors > 0 {
		list = append(list, AdviceItem{Level: "bad", Title: fmt.Sprintf("%d 条请求失败", t.Errors), Desc: "部分查询出现错误，请检查对应 AI 平台的 API Key 是否有效、余额是否充足，并及时修正避免数据残缺。"})
	}

	return list
}

func parseDay(v string) int {
	n := 0
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 || n > 366 {
		return 0
	}
	return n
}
