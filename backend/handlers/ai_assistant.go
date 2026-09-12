package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai"
	"geo-tool/services/ai_platform"
	"geo-tool/services/points"
)

/*
AI 数据分析助手（右侧悬浮对话框）
=================================================
客户看得懂的数字已经有了（AIVS 评分 + 六项指标 + 四层审计），但「这个数字意味着什么、
我下一步该做什么」仍然需要人来解释——这是 GEO 产品最大的服务成本，也是客户流失点。

本文件把 LinkGeo 已积累的全部诊断数据（可见度评分 / 六项指标 / 内容缺口 / 竞品声量 /
待办行动 / 站点审计 / 关键词与话题簇规模）汇总成结构化上下文，注入 DeepSeek，
让客户用自然语言就能问出「我的优化效果怎么样」「为什么出现率上不去」「先做哪三件事」，
并得到**基于本租户真实数据**的回答，而不是泛泛的行业套话。

三个关键设计：

  1) 口径唯一 —— 指标完全复用 computeGeoIndicator（仪表盘同款算法），
     绝不另写一套，避免"仪表盘说 A、AI 说 B"的信任崩塌。

  2) 只读不写 —— 助手只分析与建议，不修改任何业务数据（老板 2026-09-12 确认）。
     想执行的动作由客户自己在对应页面点击，AI 负责告诉他「去哪点、点什么」。

  3) 数据裁剪 —— 上下文控制在 ~2000 字内：缺口只取 TOP5、竞品只取 TOP3、
     行动项只取未完成的前 5 条。既省 token，又让模型聚焦最要紧的问题。
*/

// ---------- 数据快照：喂给 AI 的上下文 + 前端首屏展示 ----------

// assistantContext AI 上下文与前端快照的统一载体
type assistantContext struct {
	Brand       string  `json:"brand"`
	Period      string  `json:"period"`
	Score       float64 `json:"score"`        // AIVS 总分 0~100
	Grade       string  `json:"grade"`        // A~E
	GradeLabel  string  `json:"grade_label"`  // 领先/良好/及格/偏弱/待建设
	GradeColor  string  `json:"grade_color"`  // 前端配色 key
	Delta       float64 `json:"delta"`        // 环比变化（百分点）
	Confidence  string  `json:"confidence"`   // 评分置信度 高/中/低
	SampleCount int     `json:"sample_count"` // 样本量

	// 四维分项（用于前端快照 + 提示模型优先级）
	Dims []assistantDim `json:"dims"`

	// 六项原始指标
	BrandRate     float64 `json:"brand_rate"`
	Top3Rate      float64 `json:"top3_rate"`
	CitationRate  float64 `json:"citation_rate"`
	AccuracyRate  float64 `json:"accuracy_rate"`
	RiskRate      float64 `json:"risk_rate"`
	CompetitorSov float64 `json:"competitor_sov"`

	// 结构性诊断
	TopGaps     []assistantGap `json:"top_gaps"`      // 未覆盖问题 TOP5
	Competitors []string       `json:"competitors"`   // 声量最高的竞品名
	OpenTasks   int            `json:"open_tasks"`    // 待办行动数
	DoneTasks   int            `json:"done_tasks"`    // 已完成数
	Verified    int            `json:"verified_tasks"`// 已复测数
	Improved    int            `json:"improved"`      // 复测确认改善数
	Keywords    int            `json:"keywords"`      // 启用关键词数
	Clusters    int            `json:"clusters"`      // 话题簇数
	Unclassified int           `json:"unclassified"`  // 未归类关键词数
	RecentResults int          `json:"recent_results"`// 近 7 天回答数

	// 站点审计（最近一次）
	AuditURL    string  `json:"audit_url"`
	AuditScore  int     `json:"audit_score"`
	AuditLevel  string  `json:"audit_level"`
	AuditLayers []string `json:"audit_layers"` // 形如 "访问 22/30"

	// 系统预置建议（来自 AIVS buildSuggestions，作为兜底与交叉验证）
	SystemSuggestions []string `json:"system_suggestions"`

	// 数据完备性提示：没数据时 AI 不该硬编结论
	HasData bool `json:"has_data"`
}

type assistantDim struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Weight float64 `json:"weight"`
}

type assistantGap struct {
	Question string `json:"question"`
	Misses   int    `json:"misses"`
	Level    string `json:"level"`
}

// buildAssistantContext 汇总本租户全部诊断数据。
// 指标口径严格复用 computeGeoIndicator（仪表盘同款），保证 AI 与页面数字一致。
func buildAssistantContext(tid uint) *assistantContext {
	brand := brandNameOf(tid)
	if brand == "" {
		brand = "本品牌"
	}

	// 近 7 天数据（与仪表盘默认口径一致）
	since := time.Now().AddDate(0, 0, -7)
	var results []models.CheckResult
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).
		Order("created_at desc").Find(&results)

	ind := computeGeoIndicator(tid, results, since, 7)

	ctx := &assistantContext{
		Brand:         brand,
		Period:        ind.Period,
		Score:         ind.Visibility.Score,
		Grade:         ind.Visibility.Grade,
		GradeLabel:    ind.Visibility.GradeLabel,
		GradeColor:    ind.Visibility.GradeColor,
		Delta:         ind.Visibility.Delta30,
		Confidence:    ind.Visibility.Confidence,
		SampleCount:   ind.SampleCount,
		BrandRate:     ind.BrandRate,
		Top3Rate:      ind.Top3Rate,
		CitationRate:  ind.CitationRate,
		AccuracyRate:  ind.AccuracyRate,
		RiskRate:      ind.RiskRate,
		CompetitorSov: ind.CompetitorSov,
		SystemSuggestions: ind.Visibility.Suggestions,
		HasData:       ind.SampleCount > 0,
	}
	for _, d := range ind.Visibility.Dimensions {
		ctx.Dims = append(ctx.Dims, assistantDim{Name: d.Name, Score: d.Score, Weight: d.Weight})
	}

	// ---- 内容缺口 TOP5（未覆盖/覆盖差的问题）----
	// 复用 computeGapResp，与「差距诊断」页同一套算法
	if gaps := computeGapResp(tid, results); len(gaps.Gaps) > 0 {
		n := len(gaps.Gaps)
		if n > 5 {
			n = 5
		}
		for i := 0; i < n; i++ {
			ctx.TopGaps = append(ctx.TopGaps, assistantGap{
				Question: gaps.Gaps[i].Question, Misses: gaps.Gaps[i].Misses, Level: gaps.Gaps[i].Level,
			})
		}
	}

	// ---- 竞品声量 TOP3 ----
	ctx.Competitors = topCompetitorNames(tid, results, 3)

	// ---- 行动清单闭环状态 ----
	var open, done, verified, improved int64
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND status IN ?", tid, []string{"open", "doing"}).Count(&open)
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND status = ?", tid, "done").Count(&done)
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND verify_status <> ''", tid).Count(&verified)
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND verify_status = ?", tid, "improved").Count(&improved)
	ctx.OpenTasks, ctx.DoneTasks, ctx.Verified, ctx.Improved = int(open), int(done), int(verified), int(improved)

	// ---- 关键词与话题簇规模 ----
	var kwCnt, clCnt, unclassified int64
	database.DB.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND enabled = ?", tid, true).Count(&kwCnt)
	database.DB.Model(&models.KeywordCluster{}).Where("tenant_id = ?", tid).Count(&clCnt)
	database.DB.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND enabled = ? AND cluster_id = 0", tid, true).Count(&unclassified)
	ctx.Keywords, ctx.Clusters, ctx.Unclassified = int(kwCnt), int(clCnt), int(unclassified)
	ctx.RecentResults = len(results)

	// ---- 最近一次站点审计（四层得分）----
	var audit models.AuditResult
	if database.DB.Where("tenant_id = ?", tid).Order("id desc").First(&audit).Error == nil {
		ctx.AuditURL, ctx.AuditScore, ctx.AuditLevel = audit.URL, audit.Score, audit.Level
		ctx.AuditLayers = parseAuditLayers(audit.Dimensions)
	}

	return ctx
}

// topCompetitorNames 竞品声量排行（按被提及的问题数）
func topCompetitorNames(tid uint, results []models.CheckResult, limit int) []string {
	var comps []models.Competitor
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&comps)
	cnt := map[string]int{}
	for _, r := range results {
		if r.ErrorMsg != "" {
			continue
		}
		low := strings.ToLower(r.Response)
		for _, cm := range comps {
			for _, w := range strings.Split(cm.Name, ",") {
				if w = strings.TrimSpace(w); w == "" {
					continue
				}
				if strings.Contains(low, strings.ToLower(w)) {
					cnt[w]++
				}
			}
		}
	}
	type kv struct {
		k string
		v int
	}
	list := make([]kv, 0, len(cnt))
	for k, v := range cnt {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].v != list[j].v {
			return list[i].v > list[j].v
		}
		return list[i].k < list[j].k
	})
	out := []string{}
	for i := 0; i < len(list) && i < limit; i++ {
		out = append(out, fmt.Sprintf("%s（被提及 %d 次）", list[i].k, list[i].v))
	}
	return out
}

// parseAuditLayers 从 AuditResult.Dimensions JSON 中提取四层得分摘要
func parseAuditLayers(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var wrap struct {
		Layers []struct {
			Label  string `json:"label"`
			Score  int    `json:"score"`
			Weight int    `json:"weight"`
		} `json:"layers"`
	}
	if err := json.Unmarshal([]byte(raw), &wrap); err != nil {
		return nil
	}
	out := make([]string, 0, len(wrap.Layers))
	for _, l := range wrap.Layers {
		out = append(out, fmt.Sprintf("%s %d/%d", l.Label, l.Score, l.Weight))
	}
	return out
}

// ---------- 上下文文本化（喂给模型） ----------

// renderContext 把快照渲染成模型易读的结构化文本。
// 刻意使用「指标名: 值」的平铺格式，比 JSON 更省 token 且模型理解更稳。
func renderContext(ctx *assistantContext) string {
	var b strings.Builder

	fmt.Fprintf(&b, "【品牌】%s\n", ctx.Brand)
	fmt.Fprintf(&b, "【统计周期】%s（近 7 天，共 %d 条 AI 回答样本）\n\n", ctx.Period, ctx.SampleCount)

	if !ctx.HasData {
		b.WriteString("【重要】该品牌近 7 天没有任何巡检数据，无法分析优化效果。\n")
		b.WriteString("请引导客户先完成：添加关键词 → 配置 AI 平台 → 执行巡检任务，再回来分析。\n")
		return b.String()
	}

	fmt.Fprintf(&b, "【AI 可见度总分 AIVS】%.1f / 100（等级 %s·%s，环比 %+.1f，评分置信度%s）\n",
		ctx.Score, ctx.Grade, ctx.GradeLabel, ctx.Delta, ctx.Confidence)
	b.WriteString("【四维分项（满分100，括号为权重）】\n")
	for _, d := range ctx.Dims {
		fmt.Fprintf(&b, "  - %s: %.1f（权重 %.0f%%）\n", d.Name, d.Score, d.Weight*100)
	}

	b.WriteString("\n【六项核心指标（行业口径）】\n")
	fmt.Fprintf(&b, "  - 品牌出现率: %.1f%%（AI 回答里提到本品牌的占比）\n", ctx.BrandRate)
	fmt.Fprintf(&b, "  - 推荐率TOP3: %.1f%%（本品牌出现在推荐前列的占比）\n", ctx.Top3Rate)
	fmt.Fprintf(&b, "  - 引用率: %.1f%%（AI 把本品牌当信源引用的占比，GEO 最核心指标）\n", ctx.CitationRate)
	fmt.Fprintf(&b, "  - 事实一致率: %.1f%%（与品牌事实库无冲突的占比）\n", ctx.AccuracyRate)
	fmt.Fprintf(&b, "  - 风险回答率: %.1f%%（命中风险词，越低越好）\n", ctx.RiskRate)
	fmt.Fprintf(&b, "  - 竞品声量: %.1f%%（回答中提到竞品的占比）\n", ctx.CompetitorSov)

	if len(ctx.TopGaps) > 0 {
		b.WriteString("\n【最严重的未覆盖问题 TOP5】\n")
		for i, g := range ctx.TopGaps {
			fmt.Fprintf(&b, "  %d. %s（缺失 %d 次，严重度 %s）\n", i+1, g.Question, g.Misses, g.Level)
		}
	}
	if len(ctx.Competitors) > 0 {
		fmt.Fprintf(&b, "\n【竞品声量排行】%s\n", strings.Join(ctx.Competitors, "、"))
	}

	b.WriteString("\n【优化闭环进度】\n")
	fmt.Fprintf(&b, "  - 待办行动项: %d 项\n", ctx.OpenTasks)
	fmt.Fprintf(&b, "  - 已完成: %d 项\n", ctx.DoneTasks)
	fmt.Fprintf(&b, "  - 已复测验证: %d 项（其中确认改善 %d 项）\n", ctx.Verified, ctx.Improved)

	b.WriteString("\n【内容资产规模】\n")
	fmt.Fprintf(&b, "  - 启用关键词: %d 个\n", ctx.Keywords)
	fmt.Fprintf(&b, "  - 话题簇: %d 个（未归类关键词 %d 个）\n", ctx.Clusters, ctx.Unclassified)

	if ctx.AuditURL != "" {
		fmt.Fprintf(&b, "\n【最近站点审计】%s → %d 分（%s）\n", ctx.AuditURL, ctx.AuditScore, ctx.AuditLevel)
		if len(ctx.AuditLayers) > 0 {
			fmt.Fprintf(&b, "  四层得分：%s\n", strings.Join(ctx.AuditLayers, " · "))
		}
	}

	if len(ctx.SystemSuggestions) > 0 {
		b.WriteString("\n【系统自动诊断的建议（供参考，可交叉验证）】\n")
		for i, s := range ctx.SystemSuggestions {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, s)
		}
	}

	return b.String()
}

// assistantSystemPrompt 系统提示词：锁定角色、口径与边界
const assistantSystemPrompt = `你是 LinkGeo 平台的 GEO（生成式引擎优化）数据分析顾问，服务的是不懂技术的企业客户。

你的职责是：解读平台已经采集好的数据，用客户听得懂的话说明「现在什么水平、问题出在哪、下一步做什么」。

严格遵守以下规则：
1. **只用给定数据说话**。所有结论必须能对应到上下文里的具体数字或条目，禁止编造数据、禁止补充上下文没有的指标。
2. **口语化、结论先行**。先给一句话结论，再展开原因。客户看不懂术语（如"引用率"），要顺手用一句人话解释。
3. **给可执行的下一步**。建议要具体到「做什么动作 + 在平台哪个功能里操作」，例如"去「百度分析 → 站点体检」看可引用段落得分"。不要只说"提升内容质量"这种空话。
4. **尊重样本量**。上下文里标注了样本量，样本少（<30）时要主动提醒"当前样本较少，结论仅供参考"。
5. **没数据就直说**。如果上下文提示没有数据，不要硬分析，直接告诉客户需要先完成哪几步。
6. **不承诺效果**。GEO 是概率性优化，禁止承诺"一定能提升到多少分""保证被 AI 推荐"。可以说"通常有助于""预计会有改善"。
7. 回答用中文，适度使用小标题或短列表，篇幅控制在 600 字以内。**必须写完**：宁可少讲一点也不要写到一半断掉，结尾要给出完整可执行的结论。
8. 你只有分析能力，不能修改系统数据。需要执行动作时，明确告诉客户去哪操作。`

// ---------- HTTP 接口 ----------

// AssistantSnapshot GET /api/assistant/snapshot
// 返回精简数据快照：前端首屏直接在对话头部展示当前分数与核心指标，
// 让客户打开对话框立刻知道"AI 看到的是什么"，建立数据可信感。
func AssistantSnapshot(c *gin.Context) {
	tid := TenantID(c)
	ctx := buildAssistantContext(tid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ctx})
}

// AssistantQuickAsk 预置快捷提问：客户不知道能问什么时，一键发起高价值问题
var assistantQuickAsks = []gin.H{
	{"key": "effect", "icon": "chart", "title": "优化效果如何", "question": "我最近的 GEO 优化效果怎么样？和前期相比是变好还是变差了？"},
	{"key": "reason", "icon": "bulb", "title": "为什么上不去", "question": "我的品牌出现率和引用率为什么不高？最根本的原因是什么？"},
	{"key": "next", "icon": "list", "title": "先做哪三件事", "question": "如果只让我做三件事来提升可见度，应该优先做哪三件？请给出具体操作步骤。"},
	{"key": "competitor", "icon": "trophy", "title": "竞品对比分析", "question": "和竞品相比我差在哪里？应该怎么应对？"},
	{"key": "citation", "icon": "link", "title": "怎么被 AI 引用", "question": "怎样才能让 AI 在回答里引用我的网站作为信源？我现在缺什么？"},
	{"key": "solve", "icon": "tool", "title": "解决未覆盖问题", "question": "我最严重的未覆盖问题是哪些？针对它们我该做什么内容？"},
}

// AssistantQuickAsks GET /api/assistant/quick-asks
func AssistantQuickAsks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": assistantQuickAsks})
}

// AssistantChat POST /api/assistant/chat
// body: { session_id?: uint, content: string }
// 未传 session_id 时自动新建会话；消息落库，客户下次登录可回看。
func AssistantChat(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		SessionID uint   `json:"session_id"`
		Content   string `json:"content"`
	}
	if !jsonBody(c, &body) {
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		dyErr(c, http.StatusOK, "请输入你想问的问题")
		return
	}
	if len([]rune(content)) > 1000 {
		dyErr(c, http.StatusOK, "问题太长了，请精简到 1000 字以内")
		return
	}

	// 1) 先解析 AI 平台，再扣点卡，最后才建会话。
	// 顺序很关键：本租户若没有可用平台（未配置/未启用/无 Key），必须**在扣点和建会话之前**返回，
	// 否则会同时留下两个副作用——「点了卡却拿不到分析结果」的亏空，以及历史列表里成片打不开的空会话。
	// 全站其他 AI 能力（如 ai_creation.mustPlatform）也是先校验平台再扣点，此处保持一致。
	client, perr := assistantClient(tid)
	if perr != nil {
		dyErr(c, http.StatusOK, perr.Error())
		return
	}

	// 2) 扣点卡（与全站 AI 能力一致，按次计费）
	if err := points.DeductOne(tid, "AI 数据分析助手"); err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}

	// 3) 会话：不存在则新建（标题取首个问题的前 14 字）
	sess, err := ensureAssistantSession(c, tid, body.SessionID, content)
	if err != nil {
		// 建会话失败同样退款：客户没拿到任何分析结果
		_ = points.Recharge(tid, 1, "AI 数据分析助手建会话失败退款")
		dyErr(c, http.StatusOK, err.Error())
		return
	}

	// 4) 汇总实时数据上下文（每次提问都重算，保证数字是最新的）
	snap := buildAssistantContext(tid)
	dataBlock := renderContext(snap)

	// 5) 组装消息：历史对话 + 本轮（数据上下文以 system 形式注入，避免污染对话历史）
	history := loadAssistantHistory(tid, sess.ID, 12)
	msgs := make([]ai.Message, 0, len(history)+2)

	// 把数据块单独作为一条 user 消息前置，模型对 user 角色的数据块遵循度更高
	msgs = append(msgs, ai.Message{
		Role: "user",
		Content: "以下是本品牌此刻在 LinkGeo 平台上的真实数据快照，请基于它回答我接下来的问题：\n\n" +
			dataBlock + "\n（数据快照结束）",
	})
	msgs = append(msgs, ai.Message{
		Role:    "assistant",
		Content: "好的，我已读取到该品牌的最新数据快照，请提问。",
	})
	// 再追加真实历史对话（最近若干轮，控制 token）
	msgs = append(msgs, history...)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	// max_tokens 给到 3600：DeepSeek-V4 等推理模型的思维链同样计入该预算，
	// 1600 时实测回答会在「下一步做什么」中途被截断（finish_reason=length）。
	// 这是输出上限而非实际用量，按真实输出计费，给足不影响成本。
	answer, err := client.Chat(ctx, assistantSystemPrompt, msgs, 3600, 0.6)
	if err != nil {
		// 调用失败即退款：客户没拿到分析结果，不该承担这次消耗（同 tool_parse 的失败退款策略）
		_ = points.Recharge(tid, 1, "AI 数据分析助手调用失败退款")
		dyErr(c, http.StatusOK, "AI 分析失败："+err.Error())
		return
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		_ = points.Recharge(tid, 1, "AI 数据分析助手空响应退款")
		dyErr(c, http.StatusOK, "AI 未返回内容，请重试")
		return
	}

	// 6) 落库（用户消息 + 助手回复）
	database.DB.Create(&models.ChatMessage{TenantID: tid, SessionID: sess.ID, Role: "user", Content: content})
	database.DB.Create(&models.ChatMessage{TenantID: tid, SessionID: sess.ID, Role: "assistant", Content: answer})
	database.DB.Model(sess).Updates(map[string]interface{}{
		"message_count": sess.MessageCount + 2,
		"updated_at":    time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"session_id": sess.ID,
		"reply":      answer,
		"platform":   client.PlatformName,
		"model":      client.Model,
		"snapshot":   snap,
	}})
}

// assistantClient 构造 AI 客户端：**优先 DeepSeek**（老板指定），
// 不可用时回落到本租户第一个可用平台，保证助手不会被单一平台故障卡死。
//
// 平台归属与筛选统一委托 services/ai_platform（唯一权威来源）：
// 只用分站自己的平台（不继承总后台全局平台），避免与创作中心/巡检出现两套取 Key 逻辑。
func assistantClient(tid uint) (*ai.Client, error) {
	chosen := ai_platform.PickPreferred(tid, "deepseek")
	if chosen == nil {
		return nil, fmt.Errorf("还没有可用的 AI 平台，请先在「AI 平台」中配置并启用")
	}
	return ai.NewClient(chosen.BaseURL, chosen.APIKey, chosen.Model).
		WithMeta(tid, chosen.Name, "AI 数据分析助手"), nil
}

// ensureAssistantSession 取或建会话（按租户隔离）
func ensureAssistantSession(c *gin.Context, tid uint, sid uint, firstContent string) (*models.ChatSession, error) {
	if sid > 0 {
		var s models.ChatSession
		if err := database.DB.Where("id = ? AND tenant_id = ?", sid, tid).First(&s).Error; err == nil {
			return &s, nil
		}
		// 指定的会话不存在（可能已被删除）：降级为新建，而不是直接报错
	}
	runes := []rune(firstContent)
	if len(runes) > 14 {
		runes = runes[:14]
	}
	s := models.ChatSession{
		TenantID: tid,
		Title:    "数据分析 · " + string(runes),
		RoleName: "GEO 数据分析顾问",
	}
	if err := database.DB.Create(&s).Error; err != nil {
		return nil, fmt.Errorf("创建会话失败")
	}
	return &s, nil
}

// loadAssistantHistory 读取最近 limit 条消息（正序返回），控制上下文长度
func loadAssistantHistory(tid, sid uint, limit int) []ai.Message {
	var msgs []models.ChatMessage
	database.DB.Where("tenant_id = ? AND session_id = ?", tid, sid).
		Order("id desc").Limit(limit).Find(&msgs)
	// 反转为时间正序
	out := make([]ai.Message, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		out = append(out, ai.Message{Role: msgs[i].Role, Content: msgs[i].Content})
	}
	return out
}

// AssistantSessions GET /api/assistant/sessions 历史会话列表（含消息数）
func AssistantSessions(c *gin.Context) {
	tid := TenantID(c)
	var list []models.ChatSession
	database.DB.Where("tenant_id = ? AND role_name = ?", tid, "GEO 数据分析顾问").
		Order("updated_at desc").Limit(50).Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// AssistantMessages GET /api/assistant/sessions/:id/messages 某会话的历史消息
func AssistantMessages(c *gin.Context) {
	tid := TenantID(c)
	sid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if sid == 0 {
		dyErr(c, http.StatusOK, "会话不存在")
		return
	}
	var sess models.ChatSession
	if err := database.DB.Where("id = ? AND tenant_id = ?", uint(sid), tid).First(&sess).Error; err != nil {
		dyErr(c, http.StatusOK, "会话不存在")
		return
	}
	var msgs []models.ChatMessage
	database.DB.Where("tenant_id = ? AND session_id = ?", tid, uint(sid)).Order("id asc").Find(&msgs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"session": sess, "messages": msgs}})
}

// AssistantDeleteSession DELETE /api/assistant/sessions/:id
func AssistantDeleteSession(c *gin.Context) {
	tid := TenantID(c)
	sid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if sid == 0 {
		dyErr(c, http.StatusOK, "会话不存在")
		return
	}
	database.DB.Where("tenant_id = ? AND session_id = ?", tid, uint(sid)).Delete(&models.ChatMessage{})
	database.DB.Where("id = ? AND tenant_id = ?", uint(sid), tid).Delete(&models.ChatSession{})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}
