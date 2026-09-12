package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai_platform"
	"geo-tool/services/biztime"
)

/* ============================================================================
 * 工作日志（GET /api/geo/worklog）
 *
 * 解决什么问题：
 *   GEO 系统的价值大部分发生在「客户没看着的时候」——定时巡检、生成行动清单、
 *   复测闭环、内容生成……这些动作散落在 check_tasks / opt_tasks / creative_records
 *   / audit_results 等 6 张表里。客户想知道「这软件到底帮我干了什么、多久干一次」，
 *   得自己在 11 个 Tab 里拼，拼不出来就会觉得「花了钱买了个摆设」。
 *
 * 做什么：
 *   把上述动作归一成**同一种事件结构**（时间 + 做了什么 + 什么结果），
 *   按北京时间倒序排成一条人能读懂的时间线；再补一段「工作节奏」
 *   （上次/下次自动巡检、今天干了几件事），让客户一眼看到系统在持续运转。
 *
 * 为什么事件在 Go 侧拼、而不是 SQL UNION：
 *   六张表字段口径完全不同（巡检有 coverage、创作有 kind、审计有 score），
 *   UNION 出来的列无法解释含义；且需按租户 + 时间窗过滤后统一排序。
 *   Go 侧拼装更直观，也便于每种类型单独写文案。数据量已按 days 限制，
 *   单租户单类型远期量级 10^3~10^4，内存排序无压力。
 * ========================================================================= */

const workLogMaxEvents = 400 // 事件上限：防止恶意 days=90 把大租户全量拉进内存

// workLogEvent 一条工作日志：时间 + 谁干的 + 做了什么 + 结果
type workLogEvent struct {
	Time   string `json:"time"`   // 北京时间 "2006-01-02 15:04"
	Date   string `json:"date"`   // 北京时间日期，前端按天分组用
	Kind   string `json:"kind"`   // patrol巡检 / action行动 / content内容 / audit审计 / verify复测 / cluster聚类 / citation引用
	Icon   string `json:"icon"`   // 展示图标 key（前端映射 Arco 图标）
	Auto   bool   `json:"auto"`   // true=系统自动完成（客户没操作）
	Title  string `json:"title"`  // 一句话主标题
	Detail string `json:"detail"` // 结果补充
	Cost   int64  `json:"cost"`   // 消耗点卡（0=不消耗）
	Status string `json:"status"` // success / partial / failed
}

// workLogPace 工作节奏：告诉客户「系统什么时候干活、还干不干得动」
type workLogPace struct {
	AutoEnabled bool   `json:"auto_enabled"` // 是否开启自动巡检
	IntervalMin int    `json:"interval_min"` // 巡检间隔（分钟）
	WindowText  string `json:"window_text"`  // 时段说明，如「每天 8:00-22:00」
	LastAutoAt  string `json:"last_auto_at"` // 上次自动巡检时间（空=从未）
	NextAutoAt  string `json:"next_auto_at"` // 预计下次自动巡检（空=未开启或时段外）
	TodayCount  int    `json:"today_count"`  // 今天系统完成的工作项数
	WeekCount   int    `json:"week_count"`   // 近 7 天工作项数
	Running     bool   `json:"running"`      // 当前是否有巡检在跑
	Platforms   int    `json:"platforms"`    // 可用 AI 平台数（巡检依赖）
	Keywords    int    `json:"keywords"`     // 启用关键词数（巡检依赖）
	Points      int64  `json:"points"`       // 当前点卡余额（每次 AI 调用扣 1）
	Ready       bool   `json:"ready"`        // 自动巡检是否具备运行条件（有平台+有词）
	ReadyHint   string `json:"ready_hint"`   // 不具备条件时的原因（给客户的自查提示）
}

// WorkLog GET /api/geo/worklog?days=7
func WorkLog(c *gin.Context) {
	tid := TenantID(c)
	days := atoiDefault(c.Query("days"), 7)
	if days < 1 {
		days = 1
	}
	if days > 90 {
		days = 90
	}
	since := biztime.Since(days)

	var events []workLogEvent
	events = append(events, workLogPatrols(tid, since)...)
	events = append(events, workLogActions(tid, since)...)
	events = append(events, workLogContents(tid, since)...)
	events = append(events, workLogAudits(tid, since)...)
	events = append(events, workLogClusters(tid, since)...)
	events = append(events, workLogCitations(tid, since)...)

	// 统一按时间倒序：最新发生的排最前（客户最关心「刚刚干了什么」）
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time > events[j].Time })
	if len(events) > workLogMaxEvents {
		events = events[:workLogMaxEvents]
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"events": events,
		"pace":   workLogPaceOf(tid, events),
		"days":   days,
	}})
}

/* ---------------------------------------------------------------------------
 * ① 巡检：一次巡检 = 一条事件
 * ------------------------------------------------------------------------- */

func workLogPatrols(tid uint, since time.Time) []workLogEvent {
	var tasks []models.CheckTask
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).
		Order("created_at DESC").Limit(workLogMaxEvents).Find(&tasks)

	// 实际扣点：巡检按「平台 × 关键词」组合调用，但开启多采样后每个组合会问
	// SampleCount 次、每次都扣 1 点。因此不能拿 total_queries（组合数）当消耗，
	// 否则展示的消耗会低于客户账单。一次 GROUP BY 取回各任务真实采样次数总和。
	costByTask := map[uint]int64{}
	if len(tasks) > 0 {
		ids := make([]uint, 0, len(tasks))
		for _, t := range tasks {
			ids = append(ids, t.ID)
		}
		type row struct {
			TaskID uint
			Cost   int64
		}
		var rows []row
		database.DB.Model(&models.CheckResult{}).
			Select("task_id, SUM(sample_count) AS cost").
			Where("tenant_id = ? AND task_id IN ?", tid, ids).
			Group("task_id").Scan(&rows)
		for _, r := range rows {
			costByTask[r.TaskID] = r.Cost
		}
	}

	out := make([]workLogEvent, 0, len(tasks))
	for _, t := range tasks {
		// 以「结束时间」为准（客户关心的是活儿什么时候干完的）；
		// 未结束的（running/failed 无 finished_at）回落到创建时间。
		at := t.CreatedAt
		if t.FinishedAt != nil {
			at = *t.FinishedAt
		}
		ev := workLogEvent{
			Time:   biztime.Format(at, "2006-01-02 15:04"),
			Date:   biztime.Format(at, "2006-01-02"),
			Kind:   "patrol",
			Icon:   "radar",
			Auto:   t.Mode == "auto",
			Status: t.Status,
		}
		who := "手动触发巡检"
		if t.Mode == "auto" {
			who = "自动巡检"
		}

		switch t.Status {
		case "running":
			ev.Title = who + "进行中"
			ev.Detail = "正在向 AI 平台提问，请稍候"
			ev.Status = "partial"
		case "failed":
			ev.Title = who + "未完成"
			// 最常见原因是「没有可用平台 / 没有启用关键词 / 点卡不足」，值得点出来
			if t.TotalQueries == 0 {
				ev.Detail = "本轮没有可执行的项目（未配置可用 AI 平台或未启用关键词）"
			} else {
				ev.Detail = "AI 平台返回异常或点卡不足，本轮未取得有效结果"
			}
			ev.Status = "failed"
		default: // success / partial
			ev.Title = who + " " + strconv.Itoa(int(t.TotalQueries)) + " 次提问"
			ev.Detail = "命中 " + strconv.Itoa(int(t.HitCount)) + " 次，覆盖率 " +
				strconv.Itoa(int(t.Coverage)) + "%"
			if t.ErrorCount > 0 {
				ev.Detail += "（" + strconv.Itoa(int(t.ErrorCount)) + " 次调用失败）"
			}
			if t.Status == "partial" {
				ev.Status = "partial"
			} else {
				ev.Status = "success"
			}
		}
		// 优先用实测采样次数（等于真实扣点数）；查不到（如结果被清理）再回落到组合数
		if c, ok := costByTask[t.ID]; ok {
			ev.Cost = c
		} else {
			ev.Cost = int64(t.TotalQueries)
		}
		out = append(out, ev)
	}
	return out
}

/* ---------------------------------------------------------------------------
 * ② 优化行动：生成清单（系统推导）+ 完成（人做）+ 复测（系统验证）
 * ------------------------------------------------------------------------- */

// optTaskTypeText 行动类型中文名
var optTaskTypeText = map[string]string{
	"gap":        "内容缺口",
	"risk":       "风险修复",
	"audit":      "网站优化",
	"citation":   "引用提升",
	"competitor": "竞品应对",
}

func workLogActions(tid uint, since time.Time) []workLogEvent {
	out := []workLogEvent{}

	// 2.1 完成行动项：以 DoneAt 为事件时间
	var doneTasks []models.OptTask
	database.DB.Where("tenant_id = ? AND done_at IS NOT NULL AND done_at >= ?", tid, since).
		Order("done_at DESC").Limit(workLogMaxEvents).Find(&doneTasks)
	for _, t := range doneTasks {
		at := *t.DoneAt
		ev := workLogEvent{
			Time: biztime.Format(at, "2006-01-02 15:04"), Date: biztime.Format(at, "2006-01-02"),
			Kind: "action", Icon: "check", Auto: false, Status: "success",
			Title:  "完成优化项：" + t.Title,
			Detail: "类型：" + optTaskTypeTextOrRaw(t.Type),
		}
		if t.VerifyStatus != "" {
			ev.Detail += "；已完成复测（" + verifyStatusText(t.VerifyStatus) + "）"
		} else {
			ev.Detail += "；等待复测验证效果"
		}
		out = append(out, ev)
	}

	// 2.2 复测：以 VerifiedAt 为事件时间（系统自动跑对比）
	var verifiedTasks []models.OptTask
	database.DB.Where("tenant_id = ? AND verified_at IS NOT NULL AND verified_at >= ?", tid, since).
		Order("verified_at DESC").Limit(workLogMaxEvents).Find(&verifiedTasks)
	for _, t := range verifiedTasks {
		at := *t.VerifiedAt
		ev := workLogEvent{
			Time: biztime.Format(at, "2006-01-02 15:04"), Date: biztime.Format(at, "2006-01-02"),
			Kind: "verify", Icon: "sync", Auto: true, Status: verifyStatusLevel(t.VerifyStatus),
			Title:  "复测优化效果：" + t.Title,
			Detail: verifyStatusText(t.VerifyStatus),
		}
		if strings.TrimSpace(t.VerifyNote) != "" {
			ev.Detail += "｜" + t.VerifyNote
		}
		out = append(out, ev)
	}

	return out
}

func optTaskTypeTextOrRaw(t string) string {
	if v, ok := optTaskTypeText[t]; ok {
		return v
	}
	if t == "" {
		return "综合优化"
	}
	return t
}

// verifyStatusText 复测结论的中文解释（口径与「行动清单」页保持一致）
func verifyStatusText(s string) string {
	switch s {
	case "improved":
		return "指标已改善"
	case "unchanged":
		return "指标暂无明显变化"
	case "worse":
		return "指标出现下滑，建议调整方案"
	case "pending":
		return "已排期待复测"
	}
	return "复测完成"
}

func verifyStatusLevel(s string) string {
	switch s {
	case "improved":
		return "success"
	case "worse":
		return "failed"
	}
	return "partial"
}

/* ---------------------------------------------------------------------------
 * ③ 内容生成：客户点一次生成 = 一条事件（AI 创作，消耗点卡）
 * ------------------------------------------------------------------------- */

// creativeKindText 创作类型中文名（与创作中心一致）
var creativeKindText = map[string]string{
	"copy": "营销文案", "script": "抖音脚本", "xhs": "小红书文案",
	"learn": "深度学习", "xiegou": "洗稿改写", "image": "图片生成",
	"video": "视频生成", "imitate": "拆解模仿", "article": "文章生成",
}

func workLogContents(tid uint, since time.Time) []workLogEvent {
	var recs []models.CreativeRecord
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).
		Order("created_at DESC").Limit(workLogMaxEvents).Find(&recs)

	out := make([]workLogEvent, 0, len(recs))
	for _, r := range recs {
		kind := creativeKindText[r.Kind]
		if kind == "" {
			kind = "内容生成"
		}
		// 创作记录的 Title 有时本身就带类型前缀（如「小红书文案：相亲平台种草」），
		// 直接拼接会得到「生成小红书文案：小红书文案：…」的重复文案，先剥掉再拼。
		title := stripKindPrefix(r.Title, kind)
		ev := workLogEvent{
			Time: biztime.Format(r.CreatedAt, "2006-01-02 15:04"),
			Date: biztime.Format(r.CreatedAt, "2006-01-02"),
			Kind: "content", Icon: "file", Auto: false,
			Title: "生成" + kind + "：" + truncateRunes(title, 40),
			Cost:  1,
		}
		switch r.Status {
		case "failed":
			ev.Status = "failed"
			ev.Detail = "生成失败"
			if strings.TrimSpace(r.Error) != "" {
				ev.Detail += "：" + truncateRunes(r.Error, 80)
			}
		case "pending", "processing":
			ev.Status = "partial"
			ev.Detail = "生成中"
		default:
			ev.Status = "success"
			ev.Detail = "已生成"
			if r.Platform != "" {
				ev.Detail += "，使用 " + r.Platform
				if r.Model != "" {
					ev.Detail += "（" + r.Model + "）"
				}
			}
		}
		out = append(out, ev)
	}
	return out
}

// stripKindPrefix 去掉标题开头重复的类型前缀。
//
// 实测标题前缀有两种写法，都要能认出来：
//   全称 —— kind=「小红书文案」，标题「小红书文案：相亲平台种草」
//   简称 —— kind=「营销文案」，  标题「文案：婚恋品牌 GEO 优化」（作者只写了「文案」）
// 因此判定条件放宽为：分隔符前那一段是 kind 本身的子串（≥2 字）即可。
// 仅在没有分隔符、或剥完为空时放弃，避免把「文案优化技巧」这类正常标题误剥。
func stripKindPrefix(title, kind string) string {
	t := strings.TrimSpace(title)
	for _, sep := range []string{"：", ":", "—", "-"} {
		idx := strings.Index(t, sep)
		if idx <= 0 {
			continue
		}
		head := strings.TrimSpace(t[:idx])
		if head == kind || (len([]rune(head)) >= 2 && strings.Contains(kind, head)) {
			if rest := strings.TrimSpace(t[idx+len(sep):]); rest != "" {
				return rest
			}
		}
	}
	return t
}

/* ---------------------------------------------------------------------------
 * ④ 网站审计：系统抓取站点并按四层打分
 * ------------------------------------------------------------------------- */

var auditLevelText = map[string]string{
	"excellent": "优秀", "good": "良好", "medium": "中等", "poor": "待改进",
}

func workLogAudits(tid uint, since time.Time) []workLogEvent {
	var audits []models.AuditResult
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).
		Order("created_at DESC").Limit(workLogMaxEvents).Find(&audits)

	out := make([]workLogEvent, 0, len(audits))
	for _, a := range audits {
		lv := auditLevelText[a.Level]
		if lv == "" {
			lv = a.Level
		}
		status := "success"
		if a.Score < 50 {
			status = "failed"
		} else if a.Score < 70 {
			status = "partial"
		}
		out = append(out, workLogEvent{
			Time: biztime.Format(a.CreatedAt, "2006-01-02 15:04"),
			Date: biztime.Format(a.CreatedAt, "2006-01-02"),
			Kind: "audit", Icon: "safe", Auto: false, Status: status,
			Title:  "网站 GEO 体检：" + truncateRunes(a.URL, 48),
			Detail: "得分 " + strconv.Itoa(a.Score) + " 分（" + lv + "）",
		})
	}
	return out
}

/* ---------------------------------------------------------------------------
 * ⑤ 话题簇聚类：AI 把关键词按搜索意图分组（消耗点卡）
 * ------------------------------------------------------------------------- */

func workLogClusters(tid uint, since time.Time) []workLogEvent {
	var clusters []models.KeywordCluster
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).
		Order("created_at DESC").Limit(workLogMaxEvents).Find(&clusters)

	out := make([]workLogEvent, 0, len(clusters))
	for _, cl := range clusters {
		out = append(out, workLogEvent{
			Time: biztime.Format(cl.CreatedAt, "2006-01-02 15:04"),
			Date: biztime.Format(cl.CreatedAt, "2006-01-02"),
			Kind: "cluster", Icon: "thunder", Auto: false, Status: "success", Cost: 1,
			Title:  "AI 新建话题簇：" + cl.Name,
			Detail: "搜索意图：" + intentText(cl.Intent) + "｜" + truncateRunes(cl.Description, 60),
		})
	}
	return out
}

func intentText(s string) string {
	switch s {
	case models.IntentInformational:
		return "认知型（还在了解行业）"
	case models.IntentCommercial:
		return "对比型（正在选型比较）"
	case models.IntentTransactional:
		return "决策型（准备联系/购买）"
	case models.IntentNavigational:
		return "品牌型（已在找特定品牌）"
	}
	return "未标注"
}

/* ---------------------------------------------------------------------------
 * ⑥ 引用溯源：从 AI 回答里提取到的引用来源（系统自动采集，不额外扣点）
 * ------------------------------------------------------------------------- */

// workLogCitations 引用记录按「天 + 平台」聚合展示：
// 单条引用太碎（一次巡检可能产出几十条），客户要看的是
// 「系统今天从哪些 AI 那里抓到了我们的引用」。
func workLogCitations(tid uint, since time.Time) []workLogEvent {
	var cits []models.Citation
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).
		Order("created_at DESC").Limit(workLogMaxEvents).Find(&cits)
	if len(cits) == 0 {
		return nil
	}

	type key struct{ day, platform string }
	agg := map[key]int{}
	latest := map[key]time.Time{}
	order := []key{}
	for _, ci := range cits {
		k := key{biztime.Format(ci.CreatedAt, "2006-01-02"), ci.PlatformName}
		if _, ok := agg[k]; !ok {
			order = append(order, k)
		}
		agg[k]++
		if ci.CreatedAt.After(latest[k]) {
			latest[k] = ci.CreatedAt
		}
	}

	out := make([]workLogEvent, 0, len(order))
	for _, k := range order {
		at := latest[k]
		plat := k.platform
		if plat == "" {
			plat = "AI 平台"
		}
		out = append(out, workLogEvent{
			Time: biztime.Format(at, "2006-01-02 15:04"),
			Date: k.day,
			Kind: "citation", Icon: "link", Auto: true, Status: "success",
			Title:  "采集到 " + strconv.Itoa(agg[k]) + " 条 AI 引用来源",
			Detail: "来源平台：" + plat,
		})
	}
	return out
}

/* ---------------------------------------------------------------------------
 * 工作节奏
 * ------------------------------------------------------------------------- */

func workLogPaceOf(tid uint, events []workLogEvent) workLogPace {
	p := workLogPace{
		AutoEnabled: config.AutoCheckEnabled(),
		IntervalMin: config.AutoCheckMinutes(),
		WindowText: "每天 " + strconv.Itoa(config.AutoCheckStartHour) + ":00-" +
			strconv.Itoa(config.AutoCheckEndHour) + ":00",
	}

	// 可用平台数：口径与巡检完全一致——都走 ai_platform.Usable（唯一权威判定）
	for _, pl := range ai_platform.OwnPlatforms(tid) {
		if ai_platform.Usable(&pl) {
			p.Platforms++
		}
	}
	var kwCnt int64
	database.DB.Model(&models.GeoKeyword{}).
		Where("tenant_id = ? AND enabled = ?", tid, true).Count(&kwCnt)
	p.Keywords = int(kwCnt)

	// 点卡余额：巡检每次 AI 调用扣 1 点，余额为 0 时自动巡检会静默失败
	// （checker 记 error_count），所以这里必须让客户看到。
	// tenant_id=0 为总后台，不扣费，余额无意义。
	if tid > 0 {
		var t models.Tenant
		if err := database.DB.Select("points").First(&t, tid).Error; err == nil {
			p.Points = t.Points
		}
	} else {
		p.Points = -1 // -1 = 不限量（前端不渲染余额提示）
	}

	// 当前是否有巡检在跑
	var running int64
	database.DB.Model(&models.CheckTask{}).
		Where("tenant_id = ? AND status = ?", tid, "running").Count(&running)
	p.Running = running > 0

	// 上次自动巡检（不限于查询窗口，用全量最近一条，才能反映「真实上次」
	var lastAuto models.CheckTask
	if err := database.DB.Where("tenant_id = ? AND mode = ?", tid, "auto").
		Order("created_at DESC").First(&lastAuto).Error; err == nil {
		at := lastAuto.CreatedAt
		if lastAuto.FinishedAt != nil {
			at = *lastAuto.FinishedAt
		}
		p.LastAutoAt = biztime.Format(at, "2006-01-02 15:04")
	}

	// 下次预计自动巡检：以上次完成为基准 + 间隔，若已落在时段外则推到次日开始
	if p.AutoEnabled {
		created := lastAuto.CreatedAt
		p.NextAutoAt = nextAutoCheckText(lastAuto.FinishedAt, &created)
	}

	// 今天 / 近 7 天工作项数（今天口径 = 北京时间自然日）
	today := biztime.Today()
	weekSince := biztime.Since(7)
	for _, ev := range events {
		if ev.Date == today {
			p.TodayCount++
		}
		if ev.Time >= biztime.Format(weekSince, "2006-01-02 15:04") {
			p.WeekCount++
		}
	}

	// 自动巡检是否具备运行条件（有平台 + 有词 + 有余额）
	switch {
	case p.Platforms == 0:
		p.ReadyHint = "尚未配置可用的 AI 平台，自动巡检无法执行，请在「系统设置 → AI 平台」中添加"
	case p.Keywords == 0:
		p.ReadyHint = "尚未启用关键词，自动巡检没有可提问的内容，请先在「关键词管理」中添加"
	case tid > 0 && p.Points <= 0:
		p.ReadyHint = "点卡余额不足，自动巡检会因扣费失败而中断，请及时充值"
	default:
		p.Ready = true
	}
	return p
}

// nextAutoCheckText 计算「预计下次自动巡检」的展示文案。
//
// 口径说明：巡检由 ticker 每 IntervalMin 分钟触发一次，且仅在北京时间
// [8,22) 点内真正执行。因此下次时间 = 上次完成时间 + 间隔；
// 若结果落在 22:00 之后或 8:00 之前，则顺延到次日 8:00。
// 这是「预计」而非承诺——实际触发还取决于进程是否存活、前一轮是否跑完。
func nextAutoCheckText(finished, created *time.Time) string {
	base := time.Time{}
	if finished != nil {
		base = *finished
	} else if created != nil {
		base = *created
	}
	interval := time.Duration(config.AutoCheckMinutes()) * time.Minute
	if base.IsZero() {
		// 从未巡检过：给出下一个整点时段起点（今天 8 点后就是「马上」，否则次日 8 点）
		now := biztime.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(),
			config.AutoCheckStartHour, 0, 0, 0, biztime.Zone())
		if !now.Before(next) {
			if now.Hour() < config.AutoCheckEndHour {
				return "随时（自动巡检已开启，将按周期自动执行）"
			}
			next = next.AddDate(0, 0, 1)
		}
		return biztime.Format(next, "01-02 15:04")
	}

	next := base.Add(interval)
	day := biztime.In(next)
	start := time.Date(day.Year(), day.Month(), day.Day(),
		config.AutoCheckStartHour, 0, 0, 0, biztime.Zone())
	end := time.Date(day.Year(), day.Month(), day.Day(),
		config.AutoCheckEndHour, 0, 0, 0, biztime.Zone())
	switch {
	case day.Before(start):
		// 早于当天时段起点（如凌晨补跑完）：提前到**当天** 8:00，
		// 而不是次日 —— ticker 到点后判断在时段内即会执行。
		next = start
	case !day.Before(end):
		// 已越过当天时段终点（如 21:30 + 60min = 22:30）→ 顺延**次日** 8:00
		next = start.AddDate(0, 0, 1)
	}
	// 已过期（例如昨天跑完就一直没开）→ 回落到「随时」
	if biztime.In(next).Before(biztime.Now()) {
		return "随时（自动巡检已开启，将按周期自动执行）"
	}
	return biztime.Format(next, "01-02 15:04")
}

// truncateRunes 按「字符」截断（中文按 1 个字算），超长补省略号。
// 不能用 s[:n] —— 那样会把 UTF-8 汉字切成半个字节产生乱码。
func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

// atoiDefault 解析非负整型查询参数，非法输入回落默认值
func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return def
	}
	return n
}
