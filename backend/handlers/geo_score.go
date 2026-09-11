package handlers

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"geo-tool/models"
)

/*
AI 可见度评分（AI Visibility Score，0~100）
=================================================
对标 GEO 行业 2026 主流产品（Profound / Otterly.AI / CiteLens / Peec AI）的旗舰指标。
这些平台的核心交付物不是一堆散指标，而是「一个客户能看懂的总分 + 拆解 + 改进方向」。

LinkGeo 原有六项指标（出现率 / 推荐率 / 引用率 / 事实一致率 / 竞品声量 / 风险率）
是「分子分母」级别的原始值，缺少：
  1) 加权合成 —— 客户无法一眼判断"我现在到底行不行"
  2) 行业基准 —— 没有参照，80% 到底算好还是差
  3) 分层归因 —— 分数低是内容问题还是信源问题

本文件实现三层评分体系：

  Layer 1  总分 AIVS（0~100）—— 四维加权
  Layer 2  四维分项（各 0~100）——
             · 曝光 Exposure    品牌是否出现在回答里
             · 位次 Prominence  出现得多靠前、提及密度
             · 可信 Trust       事实一致率 + 引用率（被 AI 当作可信信源）
             · 风险 Risk        风险回答率反向扣分
  Layer 3  等级 + 行业基准 + 改进建议

权重设计依据（对齐行业共识）：
  Trust 权重最高（0.35）—— GEO 的本质是「语义信任」，AI 是否引用你取决于可信度；
  Exposure 0.30 —— 先能被提到，才有后面一切；
  Prominence 0.20 —— 在候选集中排第几，决定推荐概率；
  Risk 0.15 —— 合规底线，出现风险表述直接拖垮整体可信度。
*/

// dimensionScore 单个维度得分
type dimensionScore struct {
	Key    string  `json:"key"`
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Weight float64 `json:"weight"`
	Detail string  `json:"detail"`
}

// benchmarkItem 对标行业基准
type benchmarkItem struct {
	Key       string  `json:"key"`
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	Benchmark float64 `json:"benchmark"`
	Gap       float64 `json:"gap"`
	Level     string  `json:"level"` // excellent / good / fair / poor
}

// visibilityScore AI 可见度评分结果
type visibilityScore struct {
	Score       float64          `json:"score"`
	Grade       string           `json:"grade"`
	GradeLabel  string           `json:"grade_label"`
	GradeColor  string           `json:"grade_color"`
	Dimensions  []dimensionScore `json:"dimensions"`
	Benchmarks  []benchmarkItem  `json:"benchmarks"`
	Suggestions []string         `json:"suggestions"`
	Delta30     float64          `json:"delta_30"`   // 与上一周期相比的分数变化
	Confidence  string           `json:"confidence"` // 置信度提示（样本量）
}

// 行业基准值（基于 GEO 行业公开数据的经验阈值，用于「你处在什么水位」参照）
const (
	benchExposure   = 60.0
	benchProminence = 50.0
	benchTrust      = 45.0
	benchRisk       = 90.0 // 风险维度：越干净越高，基准 90 表示允许多数回答无风险词
)

// 权重（和为 1）
const (
	wExposure   = 0.30
	wProminence = 0.20
	wTrust      = 0.35
	wRisk       = 0.15
)

// buildVisibilityScore 由六项原始指标合成 AI 可见度评分。
// 输入均为 0~100 的百分比值。
func buildVisibilityScore(
	brandRate, top3Rate, citationRate, accuracyRate, riskRate float64,
	avgMention float64,
	successCount int,
	scoreDelta float64,
) visibilityScore {
	// ---- 维度 1：曝光 Exposure ----
	// 品牌出现率是主项；提及密度（单次回答提及品牌次数）作为加成，饱和在 3 次。
	mentionBonus := clamp01(avgMention/3.0) * 100
	exposure := clamp01((brandRate*0.8+mentionBonus*0.2)/100) * 100

	// ---- 维度 2：位次 Prominence ----
	// 推荐率（TOP3）= 出现在 AI 推荐清单靠前位置的占比。
	// 用 top3/brand 的比值衡量"一旦出现，是否靠前"，再与绝对推荐率加权。
	ratio := 0.0
	if brandRate > 0 {
		ratio = top3Rate / brandRate
	}
	prominence := clamp01((top3Rate*0.6+ratio*100*0.4)/100) * 100

	// ---- 维度 3：可信 Trust ----
	// 事实一致率 + 引用率。引用率代表"AI 愿意把你当信源"，是 GEO 最核心的信任信号。
	trust := clamp01((accuracyRate*0.45+citationRate*0.55)/100) * 100

	// ---- 维度 4：风险 Risk（反向）----
	// riskRate 越低越好：riskRate=0 → 100 分；riskRate>=20 → 0 分（线性衰减）
	risk := clamp01(1-riskRate/20.0) * 100

	total := exposure*wExposure + prominence*wProminence + trust*wTrust + risk*wRisk
	total = round1(clamp(0, 100, total))

	dims := []dimensionScore{
		{Key: "exposure", Name: "曝光度", Score: round1(exposure), Weight: wExposure,
			Detail: dimDetail(exposure, "品牌出现率", brandRate, "AI 回答中提到品牌的频率")},
		{Key: "prominence", Name: "推荐位次", Score: round1(prominence), Weight: wProminence,
			Detail: dimDetail(prominence, "推荐率", top3Rate, "出现在 AI 推荐前列的频率")},
		{Key: "trust", Name: "可信度", Score: round1(trust), Weight: wTrust,
			Detail: dimDetail(trust, "引用率", citationRate, "AI 把你作为信源引用的频率")},
		{Key: "risk", Name: "合规安全", Score: round1(risk), Weight: wRisk,
			Detail: dimDetail(risk, "风险回答率", riskRate, "回答中出现风险表述的频率（越低越好）")},
	}

	grade, label, color := gradeOf(total)

	// 基准对标
	bms := []benchmarkItem{
		mkBench("exposure", "曝光度", exposure, benchExposure),
		mkBench("prominence", "推荐位次", prominence, benchProminence),
		mkBench("trust", "可信度", trust, benchTrust),
		mkBench("risk", "合规安全", risk, benchRisk),
	}

	// 改进建议：挑得分最低、且权重不低的维度优先给建议
	sugs := buildSuggestions(exposure, prominence, trust, risk, citationRate, accuracyRate, brandRate, riskRate)

	// 置信度：LLM 输出有概率性，样本太少时评分波动大，必须提示客户
	conf := "高"
	switch {
	case successCount < 30:
		conf = "低"
	case successCount < 100:
		conf = "中"
	}

	return visibilityScore{
		Score: total, Grade: grade, GradeLabel: label, GradeColor: color,
		Dimensions: dims, Benchmarks: bms, Suggestions: sugs,
		Delta30: round1(scoreDelta), Confidence: conf,
	}
}

func dimDetail(_ float64, metricName string, metricVal float64, desc string) string {
	return metricName + " " + fmtPct(metricVal) + "，" + desc
}

// gradeOf 分数 → 等级（行业通用 A~E 五档）
func gradeOf(score float64) (string, string, string) {
	switch {
	case score >= 80:
		return "A", "领先", "green"
	case score >= 65:
		return "B", "良好", "arcoblue"
	case score >= 50:
		return "C", "及格", "orange"
	case score >= 35:
		return "D", "偏弱", "red"
	default:
		return "E", "待建设", "red"
	}
}

func mkBench(key, name string, score, benchmark float64) benchmarkItem {
	gap := round1(score - benchmark)
	lv := "fair"
	switch {
	case score >= benchmark+15:
		lv = "excellent"
	case score >= benchmark:
		lv = "good"
	case score >= benchmark-15:
		lv = "fair"
	default:
		lv = "poor"
	}
	return benchmarkItem{Key: key, Name: name, Score: round1(score), Benchmark: benchmark, Gap: gap, Level: lv}
}

// buildSuggestions 依据短板生成可执行建议（按影响面排序）
func buildSuggestions(exposure, prominence, trust, risk, citationRate, accuracyRate, brandRate, riskRate float64) []string {
	type sug struct {
		impact  float64 // 加权影响分：越低说明该维度拖后腿越多
		overall float64 // 该维度自身得分，用于判断是否真的需要改进
		text    string
	}
	list := []sug{
		{exposure * wExposure, exposure, "曝光度偏低：优先补充「品牌事实库」条目并检查关键词覆盖，让 AI 在回答时有据可依。"},
		{prominence * wProminence, prominence, "推荐位次靠后：在内容中强化品牌在品类中的差异化定位与对比信息，争取进入 AI 推荐清单前排。"},
		{trust * wTrust, trust, "可信度不足：提升引用率与事实一致率 —— 补充第三方权威信源（媒体/百科/行业站），并清理与事实库冲突的表述。"},
		{risk * wRisk, risk, "存在合规风险：回答中命中风险词，建议尽快处理「竞品与风险词」中的高风险项。"},
	}
	// 维度已达标（≥70 分）的不再列为「优先改进项」
	filtered := make([]sug, 0, len(list))
	for _, s := range list {
		if s.overall < 70 {
			filtered = append(filtered, s)
		}
	}

	// 依据具体数值做更精准的定向建议（这些是「点名式」建议，优先级最高）
	targeted := make([]sug, 0, 4)
	if citationRate < 20 {
		targeted = append(targeted, sug{0, 0, "引用率仅 " + fmtPct(citationRate) + "：AI 很少把你当信源。重点建设「阵地地图」中竞品有、你没有的信源域名。"})
	}
	if accuracyRate < 70 {
		targeted = append(targeted, sug{0, 0, "事实一致率 " + fmtPct(accuracyRate) + "：AI 回答与你的品牌事实库存在冲突，请核对事实库并同步到官网。"})
	}
	if brandRate < 30 {
		targeted = append(targeted, sug{0, 0, "品牌出现率仅 " + fmtPct(brandRate) + "：AI 基本不提及你的品牌，需从内容投放与信源建设两侧同时发力。"})
	}
	if riskRate > 5 {
		targeted = append(targeted, sug{0, 0, "风险回答率 " + fmtPct(riskRate) + "：已超 5% 预警线，建议立即排查风险词库与相关投放内容。"})
	}

	// 排序：维度类按加权影响升序（拖后腿最多的排前面）
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].impact < filtered[j].impact })

	out := make([]string, 0, len(targeted)+len(filtered))
	for _, s := range targeted {
		out = append(out, s.text)
	}
	for _, s := range filtered {
		out = append(out, s.text)
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

// scoreOfResults 复算一组巡检结果的评分（用于计算环比 Delta）
func scoreOfResults(rs []models.CheckResult, citeCnt map[uint]int, facts []models.FactItem, risks []models.RiskWord, compWords []string) float64 {
	if len(rs) == 0 {
		return 0
	}
	success, hit, top3, cited, accurate, riskHit := 0, 0, 0, 0, 0, 0
	mentionSum := 0
	for _, r := range rs {
		if r.ErrorMsg != "" {
			continue
		}
		success++
		low := strings.ToLower(r.Response)
		if r.Hit {
			hit++
			mentionSum += r.MentionCount
			if r.HitPosition <= 3 {
				top3++
			}
		}
		if citeCnt[r.ID] > 0 {
			cited++
		}
		conflict := false
		for _, f := range facts {
			if strings.TrimSpace(f.NotFact) != "" && strings.Contains(low, strings.ToLower(f.NotFact)) {
				conflict = true
				break
			}
		}
		if !conflict {
			accurate++
		}
		for _, rw := range risks {
			if strings.Contains(low, strings.ToLower(rw.Word)) {
				riskHit++
				break
			}
		}
	}
	if success == 0 {
		return 0
	}
	pct := func(n int) float64 { return round1(float64(n) / float64(success) * 100) }
	avgM := 0.0
	if hit > 0 {
		avgM = round1(float64(mentionSum) / float64(hit))
	}
	vs := buildVisibilityScore(pct(hit), pct(top3), pct(cited), pct(accurate), pct(riskHit), avgM, success, 0)
	return vs.Score
}

// ---------- 数学小工具 ----------

func clamp(lo, hi, v float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clamp01 把 x 限制在 [0,1]
func clamp01(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	return clamp(0, 1, x)
}

// fmtPct 以「一位小数 + %」格式化百分比
func fmtPct(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) + "%"
}
