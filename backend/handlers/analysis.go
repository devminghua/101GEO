package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// Analysis 专业分析数据结构
type Analysis struct {
	Days      int              `json:"days"`
	Period    string           `json:"period"`
	Totals    AnalysisTotals   `json:"totals"`
	RankDist  []LabelValue      `json:"rank_dist"`
	Platforms []PlatformStat   `json:"platforms"`
	Keywords  []KeywordStat    `json:"keywords"`
	Trend     []TrendPoint     `json:"trend"`
	WeakWords []KeywordStat    `json:"weak_keywords"` // 多次未命中的关键词
	Insights  []string         `json:"insights"`
}

type AnalysisTotals struct {
	Total       int     `json:"total"`
	Success     int     `json:"success"`
	Hit         int     `json:"hit"`
	Miss        int     `json:"miss"`
	Errors      int     `json:"errors"`
	BrandRate   float64 `json:"brand_rate"`
	AvgMention  float64 `json:"avg_mention"`
	Top3Count   int     `json:"top3_count"`
	Top3Rate    float64 `json:"top3_rate"`
	PlatformNum int     `json:"platform_num"`
	KeywordNum  int     `json:"keyword_num"`
}

type LabelValue struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type PlatformStat struct {
	Name       string  `json:"name"`
	Queries    int     `json:"queries"`
	Hit        int     `json:"hit"`
	Rate       float64 `json:"rate"`
	Top3       int     `json:"top3"`
	AvgMention float64 `json:"avg_mention"`
	Errors     int     `json:"errors"`
}

type KeywordStat struct {
	Question     string  `json:"question"`
	Queries      int     `json:"queries"`
	Hit          int     `json:"hit"`
	Rate         float64 `json:"rate"`
	BestPlatform string  `json:"best_platform"`
}

type TrendPoint struct {
	Day     string  `json:"day"`
	Queries int     `json:"queries"`
	Hit     int     `json:"hit"`
	Rate    float64 `json:"rate"`
}

// buildAnalysis 对查询结果做多维聚合分析
func buildAnalysis(results []models.CheckResult, days int, since time.Time) *Analysis {
	a := &Analysis{Days: days, Period: fmt.Sprintf("%s ~ %s", since.Format("2006-01-02"), time.Now().Format("2006-01-02"))}

	totals := AnalysisTotals{}
	platformMap := map[string]*PlatformStat{}
	kwMap := map[string]*KeywordStat{}
	kwMiss := map[string]int{}
	rankCnt := map[string]int{}
	dayMap := map[string]*TrendPoint{}
	mentionSum, mentionN := 0, 0
	pMention := map[string]int{}
	pMentionN := map[string]int{}

	for _, r := range results {
		totals.Total++
		day := r.CreatedAt.Format("01-02")
		if _, ok := dayMap[day]; !ok {
			dayMap[day] = &TrendPoint{Day: day}
		}
		dayMap[day].Queries++

		if _, ok := platformMap[r.PlatformName]; !ok {
			platformMap[r.PlatformName] = &PlatformStat{Name: r.PlatformName}
		}
		ps := platformMap[r.PlatformName]
		ps.Queries++

		if _, ok := kwMap[r.Question]; !ok {
			kwMap[r.Question] = &KeywordStat{Question: r.Question}
		}
		ks := kwMap[r.Question]
		ks.Queries++

		if r.ErrorMsg != "" {
			totals.Errors++
			ps.Errors++
			rankCnt["请求错误"]++
			continue
		}
		totals.Success++

		if r.Hit {
			totals.Hit++
			ps.Hit++
			ks.Hit++
			mentionSum += r.MentionCount
			mentionN++
			pMention[r.PlatformName] += r.MentionCount
			pMentionN[r.PlatformName]++
			if r.HitPosition <= 3 {
				ps.Top3++
			}
			if r.HitPosition <= 1 {
				rankCnt["第 1 位"]++
			} else if r.HitPosition <= 3 {
				rankCnt["第 2-3 位"]++
			} else if r.HitPosition <= 10 {
				rankCnt["第 4-10 位"]++
			} else {
				rankCnt["10 名以外"]++
			}
		} else {
			totals.Miss++
			kwMiss[r.Question]++
			rankCnt["未命中"]++
		}
		dayMap[day].Hit += boolInt(r.Hit)
	}

	if totals.Success > 0 {
		totals.BrandRate = round1(float64(totals.Hit) / float64(totals.Success) * 100)
	}
	if mentionN > 0 {
		totals.AvgMention = round1(float64(mentionSum) / float64(mentionN))
	}
	totals.Top3Count = rankCnt["第 1 位"] + rankCnt["第 2-3 位"]
	if totals.Success > 0 {
		totals.Top3Rate = round1(float64(totals.Top3Count) / float64(totals.Success) * 100)
	}
	totals.PlatformNum = len(platformMap)
	totals.KeywordNum = len(kwMap)
	a.Totals = totals

	// 排名分布（固定顺序）
	for _, label := range []string{"第 1 位", "第 2-3 位", "第 4-10 位", "10 名以外", "未命中", "请求错误"} {
		a.RankDist = append(a.RankDist, LabelValue{Label: label, Value: rankCnt[label]})
	}

	// 平台统计
	for _, ps := range platformMap {
		if ps.Queries > 0 {
			ps.Rate = round1(float64(ps.Hit) / float64(ps.Queries) * 100)
		}
		if n := pMentionN[ps.Name]; n > 0 {
			ps.AvgMention = round1(float64(pMention[ps.Name]) / float64(n))
		}
		a.Platforms = append(a.Platforms, *ps)
	}
	sort.Slice(a.Platforms, func(i, j int) bool {
		if a.Platforms[i].Rate != a.Platforms[j].Rate {
			return a.Platforms[i].Rate > a.Platforms[j].Rate
		}
		return a.Platforms[i].Queries > a.Platforms[j].Queries
	})

	// 关键词统计（含最佳平台）
	platformOfMsg := map[string]string{}
	for _, r := range results {
		if r.Hit {
			platformOfMsg[r.Question] = r.PlatformName
		}
	}
	for _, ks := range kwMap {
		if ks.Queries > 0 {
			ks.Rate = round1(float64(ks.Hit) / float64(ks.Queries) * 100)
		}
		ks.BestPlatform = platformOfMsg[ks.Question]
		a.Keywords = append(a.Keywords, *ks)
	}
	sort.Slice(a.Keywords, func(i, j int) bool {
		if a.Keywords[i].Queries != a.Keywords[j].Queries {
			return a.Keywords[i].Queries > a.Keywords[j].Queries
		}
		return a.Keywords[i].Rate > a.Keywords[j].Rate
	})
	// 薄弱关键词：未命中次数 >= 2 且 查询数 >= 2，按未命中次数降序，最多 5 个
	for _, ks := range a.Keywords {
		if kwMiss[ks.Question] >= 2 && ks.Queries >= 2 {
			a.WeakWords = append(a.WeakWords, ks)
		}
		if len(a.WeakWords) >= 5 {
			break
		}
	}

	// 趋势（按日期升序）
	trendDays := make([]string, 0, len(dayMap))
	for d := range dayMap {
		trendDays = append(trendDays, d)
	}
	sort.Strings(trendDays)
	for _, d := range trendDays {
		tp := dayMap[d]
		if tp.Queries > 0 {
			tp.Rate = round1(float64(tp.Hit) / float64(tp.Queries) * 100)
		}
		a.Trend = append(a.Trend, *tp)
	}

	a.Insights = genInsights(a)
	return a
}

func genInsights(a *Analysis) []string {
	var s []string
	rate := a.Totals.BrandRate
	switch {
	case a.Totals.Total == 0:
		s = append(s, "本周期暂无巡检数据，请先完成平台与关键词配置后运行巡检。")
	case rate >= 80:
		s = append(s, fmt.Sprintf("品牌出现率 %.1f%%，整体表现优秀，建议保持内容更新频率并继续巩固各平台引用。", rate))
	case rate >= 50:
		s = append(s, fmt.Sprintf("品牌出现率 %.1f%%，处于良好水平，仍有提升空间。", rate))
	default:
		s = append(s, fmt.Sprintf("品牌出现率 %.1f%%，整体偏低，建议优先补齐薄弱关键词的内容铺设。", rate))
	}
	if len(a.Platforms) > 0 && a.Platforms[0].Queries >= 3 && a.Platforms[0].Rate > 0 {
		s = append(s, fmt.Sprintf("表现最好的平台是「%s」（出现率 %.1f%%），建议作为重点投放阵地。", a.Platforms[0].Name, a.Platforms[0].Rate))
	}
	if len(a.Platforms) > 2 {
		worst := a.Platforms[len(a.Platforms)-1]
		if worst.Queries >= 1 && worst.Rate < a.Totals.BrandRate {
			s = append(s, fmt.Sprintf("「%s」出现率仅 %.1f%%，低于整体水平，建议检查该平台内容覆盖与引用密度。", worst.Name, worst.Rate))
		}
	}
	if a.Totals.Success > 0 {
		s = append(s, fmt.Sprintf("品牌进入 TOP3 的占比为 %.1f%%，是决定被 AI 回答引用的关键指标。", a.Totals.Top3Rate))
	}
	if len(a.WeakWords) > 0 {
		names := make([]string, 0, len(a.WeakWords))
		for _, w := range a.WeakWords {
			names = append(names, w.Question)
		}
		s = append(s, fmt.Sprintf("以下问题多次未命中品牌词，建议优先铺设含品牌词的自然回答：%s。", strings.Join(names, "；")))
	}
	if a.Totals.Errors > 0 {
		s = append(s, fmt.Sprintf("有 %d 条请求失败，请检查对应平台的 API Key 与计费状态。", a.Totals.Errors))
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// queryResultsInRange 读取最近 N 天结果（可带与 ListResults 一致的筛选）
func queryResultsInRange(c *gin.Context, defaultDays int) ([]models.CheckResult, int, time.Time) {
	tid := TenantID(c)
	days := defaultDays
	if v := c.DefaultQuery("days", fmt.Sprint(defaultDays)); v != "" {
		if n := parseDay(v); n > 0 {
			days = n
		}
	}
	since := time.Now().AddDate(0, 0, -days)
	q := database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since)
	if v := c.Query("platform_name"); v != "" {
		q = q.Where("platform_name = ?", v)
	}
	if v := c.Query("hit"); v == "1" {
		q = q.Where("hit = ?", true)
	} else if v == "0" {
		q = q.Where("hit = ?", false).Where("error_msg = '' OR error_msg IS NULL")
	}
	if v := c.Query("keyword"); v != "" {
		q = q.Where("question LIKE ?", "%"+v+"%")
	}
	var results []models.CheckResult
	q.Order("created_at desc").Find(&results)
	return results, days, since
}

// ExportReport 一键导出报告文档（md / html / docx）
func ExportReport(c *gin.Context) {
	results, days, since := queryResultsInRange(c, 7)
	a := buildAnalysis(results, days, since)
	format := c.DefaultQuery("format", "md")
	date := time.Now().Format("20060102")
	var ctype, fname string
	var data []byte
	switch format {
	case "docx":
		ctype = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		fname = fmt.Sprintf("geo-report-%s.docx", date)
		data = buildDocx(a)
	case "html":
		ctype = "text/html; charset=utf-8"
		fname = fmt.Sprintf("geo-report-%s.html", date)
		data = []byte(buildHTML(a))
	default:
		ctype = "text/markdown; charset=utf-8"
		fname = fmt.Sprintf("geo-report-%s.md", date)
		data = []byte(buildMarkdown(a))
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))
	c.Data(http.StatusOK, ctype, data)
}

// ---------- 文档生成 ----------

func buildMarkdown(a *Analysis) string {
	var b strings.Builder
	b.WriteString("# GEO 优化报告\n\n")
	b.WriteString(fmt.Sprintf("> 生成时间：%s\n\n", time.Now().Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("> 统计周期：%s\n\n", a.Period))

	t := a.Totals
	b.WriteString("## 一、核心指标\n\n")
	b.WriteString(fmt.Sprintf("- 查询总数：%d\n", t.Total))
	b.WriteString(fmt.Sprintf("- 成功请求：%d\n", t.Success))
	b.WriteString(fmt.Sprintf("- 命中品牌：%d\n", t.Hit))
	b.WriteString(fmt.Sprintf("- 未命中：%d\n", t.Miss))
	b.WriteString(fmt.Sprintf("- 请求错误：%d\n", t.Errors))
	b.WriteString(fmt.Sprintf("- **品牌出现率：%.1f%%**\n", t.BrandRate))
	b.WriteString(fmt.Sprintf("- TOP3 覆盖率：%.1f%%\n", t.Top3Rate))
	b.WriteString(fmt.Sprintf("- 平均提及次数：%.1f\n", t.AvgMention))

	b.WriteString("\n## 二、各平台表现\n\n")
	if len(a.Platforms) == 0 {
		b.WriteString("- 暂无数据\n")
	} else {
		b.WriteString("| 平台 | 查询数 | 命中 | 出现率 | TOP3 | 错误 |\n|---|---|---|---|---|---|\n")
		for _, p := range a.Platforms {
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% | %d | %d |\n", p.Name, p.Queries, p.Hit, p.Rate, p.Top3, p.Errors))
		}
	}

	b.WriteString("\n## 三、查询问题表现\n\n")
	if len(a.Keywords) == 0 {
		b.WriteString("- 暂无数据\n")
	} else {
		b.WriteString("| 问题 | 查询数 | 命中 | 出现率 | 最佳平台 |\n|---|---|---|---|---|\n")
		for _, k := range a.Keywords {
			best := k.BestPlatform
			if best == "" {
				best = "-"
			}
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% | %s |\n", k.Question, k.Queries, k.Hit, k.Rate, best))
		}
	}

	b.WriteString("\n## 四、智能洞察\n\n")
	for _, s := range a.Insights {
		b.WriteString(fmt.Sprintf("- %s\n", s))
	}
	return b.String()
}

func buildHTML(a *Analysis) string {
	t := a.Totals
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>GEO 优化报告</title><style>
body{font-family:-apple-system,"PingFang SC","Microsoft YaHei",sans-serif;max-width:900px;margin:32px auto;padding:0 24px;color:#1d2129;line-height:1.7}
h1{font-size:26px;border-bottom:3px solid #165DFF;padding-bottom:12px}
h2{font-size:19px;margin-top:28px;color:#165DFF}
.kpis{display:flex;flex-wrap:wrap;gap:16px;margin:20px 0}
.kpi{flex:1;min-width:140px;background:#f7f8fa;border:1px solid #e5e6eb;border-radius:10px;padding:14px 18px}
.kpi .v{font-size:24px;font-weight:700;color:#165DFF}
.kpi .l{font-size:12px;color:#86909c;margin-top:2px}
table{border-collapse:collapse;width:100%;margin:12px 0;font-size:14px}
th,td{border:1px solid #e5e6eb;padding:8px 12px;text-align:left}
th{background:#f2f3f5}
.meta{color:#86909c;font-size:13px;margin-bottom:4px}
ul{padding-left:20px}
.footer{margin-top:40px;color:#c9cdd4;font-size:12px;text-align:center}
</style></head><body>`)
	b.WriteString(fmt.Sprintf("<h1>GEO 优化报告</h1><p class=\"meta\">生成时间：%s</p><p class=\"meta\">统计周期：%s</p>",
		time.Now().Format("2006-01-02 15:04"), a.Period))
	b.WriteString(`<div class="kpis">`)
	kpis := []struct {
		v string
		l string
	}{
		{fmt.Sprintf("%.1f%%", t.BrandRate), "品牌出现率"},
		{fmt.Sprintf("%d", t.Total), "查询总数"},
		{fmt.Sprintf("%d", t.Hit), "命中品牌"},
		{fmt.Sprintf("%.1f%%", t.Top3Rate), "TOP3 覆盖率"},
		{fmt.Sprintf("%.1f", t.AvgMention), "平均提及次数"},
	}
	for _, k := range kpis {
		b.WriteString(fmt.Sprintf(`<div class="kpi"><div class="v">%s</div><div class="l">%s</div></div>`, k.v, k.l))
	}
	b.WriteString("</div>")

	b.WriteString("<h2>二、各平台表现</h2>")
	b.WriteString("<table><tr><th>平台</th><th>查询数</th><th>命中</th><th>出现率</th><th>TOP3</th><th>错误</th></tr>")
	for _, p := range a.Platforms {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%.1f%%</td><td>%d</td><td>%d</td></tr>", p.Name, p.Queries, p.Hit, p.Rate, p.Top3, p.Errors))
	}
	b.WriteString("</table>")

	b.WriteString("<h2>三、查询问题表现</h2>")
	b.WriteString("<table><tr><th>问题</th><th>查询数</th><th>命中</th><th>出现率</th><th>最佳平台</th></tr>")
	for _, k := range a.Keywords {
		best := k.BestPlatform
		if best == "" {
			best = "-"
		}
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%.1f%%</td><td>%s</td></tr>", k.Question, k.Queries, k.Hit, k.Rate, best))
	}
	b.WriteString("</table>")

	b.WriteString("<h2>四、智能洞察</h2><ul>")
	for _, s := range a.Insights {
		b.WriteString(fmt.Sprintf("<li>%s</li>", s))
	}
	b.WriteString("</ul>")
	b.WriteString(`<div class="footer">由 LinkGeo · 生成式引擎优化平台自动生成</div></body></html>`)
	return b.String()
}

func buildDocx(a *Analysis) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZip := func(name, content string) {
		f, err := zw.Create(name)
		if err != nil {
			return
		}
		_, _ = f.Write([]byte(content))
	}
	writeZip("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	writeZip("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	// 标题
	body.WriteString(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="36"/></w:rPr><w:t xml:space="preserve">GEO 优化报告</w:t></w:r></w:p>`)
	body.WriteString(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:sz w:val="20"/><w:color w:val="888888"/></w:rPr><w:t xml:space="preserve">` + xmlEscape(a.Period+"　|　生成时间 "+time.Now().Format("2006-01-02 15:04")) + `</w:t></w:r></w:p>`)

	addHeading(&body, "一、核心指标")
	t := a.Totals
	lines := []string{
		fmt.Sprintf("查询总数：%d", t.Total),
		fmt.Sprintf("成功请求：%d", t.Success),
		fmt.Sprintf("命中品牌：%d", t.Hit),
		fmt.Sprintf("未命中：%d", t.Miss),
		fmt.Sprintf("请求错误：%d", t.Errors),
		fmt.Sprintf("品牌出现率：%.1f%%", t.BrandRate),
		fmt.Sprintf("TOP3 覆盖率：%.1f%%", t.Top3Rate),
		fmt.Sprintf("平均提及次数：%.1f", t.AvgMention),
	}
	for _, l := range lines {
		addPara(&body, l)
	}

	addHeading(&body, "二、各平台表现")
	if len(a.Platforms) == 0 {
		addPara(&body, "暂无数据")
	} else {
		for _, p := range a.Platforms {
			addPara(&body, fmt.Sprintf("• %s：%d 次查询，命中 %d，出现率 %.1f%%，TOP3 %d 次，错误 %d", p.Name, p.Queries, p.Hit, p.Rate, p.Top3, p.Errors))
		}
	}

	addHeading(&body, "三、查询问题表现")
	if len(a.Keywords) == 0 {
		addPara(&body, "暂无数据")
	} else {
		for _, k := range a.Keywords {
			best := k.BestPlatform
			if best == "" {
				best = "-"
			}
			addPara(&body, fmt.Sprintf("• %s：%d 次查询，命中 %d，出现率 %.1f%%，最佳平台 %s", k.Question, k.Queries, k.Hit, k.Rate, best))
		}
	}

	addHeading(&body, "四、智能洞察")
	for _, s := range a.Insights {
		addPara(&body, "• "+s)
	}

	body.WriteString(`<w:sectPr/></w:body></w:document>`)
	writeZip("word/document.xml", body.String())
	_ = zw.Close()
	return buf.Bytes()
}

func addHeading(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="28"/><w:color w:val="2F54EB"/></w:rPr><w:t xml:space="preserve">` + xmlEscape(text) + `</w:t></w:r></w:p>`)
}

func addPara(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + xmlEscape(text) + `</w:t></w:r></w:p>`)
}

func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}
