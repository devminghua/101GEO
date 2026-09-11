package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// ============ 站点审计统一核心（v1.0.33） ============
//
// 历史背景：系统里曾并存两套站点审计口径 ——
//   ① handlers.RunAudit        （POST /api/geo/audit）      10 个维度各 10 分制、简单加和
//   ② handlers.SiteAuditDetail （POST /api/baidu/site-audit）四层体检（访问/定向/理解/可引用）
// 两套算法不同，同一站点会得到两个不同分数，却写同一张 audit_results 表：
// 客户在「GEO 智能 → 网站审计」和「百度分析 → 站点体检」看到两套结论，互相打架。
//
// v1.0.33 起统一为「四层加权口径」：
//   · 四层体检是唯一骨架：访问 30 / 定向 18 / 理解 32 / 可引用 20，合计 100 权重
//   · 每个检查项带权重，得分率 ok = 100% / warn = 50% / fail = 0%
//   · 总分 = Σ(权重 × 得分率)，归一到 100 分制；等级 90 / 70 / 50 三档
//   · 前端「网站审计」用的 10 维视图由同一份检查项派生，不再独立打分
//   · 两个入口落同一张 audit_results 表、同一套评分、同一套行动工单逻辑
const (
	auditOK   = "ok"
	auditWarn = "warn"
	auditFail = "fail"
)

type auditCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok / warn / fail
	Note   string `json:"note"`
	Key    string `json:"key"`    // 维度标识（如 access_http）
	Weight int    `json:"weight"` // 该检查项权重（满分权重）
	Score  int    `json:"score"`  // 实得分 = weight × 得分率
}

type auditLayer struct {
	Key    string       `json:"key"`
	Label  string       `json:"label"`
	Desc   string       `json:"desc"`
	Status string       `json:"status"` // ok / warn / fail
	Weight int          `json:"weight"` // 该层总权重
	Score  int          `json:"score"`  // 该层实得分
	Checks []auditCheck `json:"checks"`
}

type siteAuditDetail struct {
	URL         string         `json:"url"`
	Host        string         `json:"host"`
	Score       int            `json:"score"`
	Level       string         `json:"level"`
	Layers      []auditLayer   `json:"layers"`
	Dimensions  []auditDims    `json:"dimensions"` // 10 维视图（由四层检查项派生，兼容旧前端）
	Findings    []string       `json:"findings"`   // 待优化项
	GradeDist   map[string]int `json:"grade_dist"` // A/B/C/D 抽取块分级
	OverallNote string         `json:"overall_note"`
}

// auditReport 统一审计结果（两个入口共用）。
type auditReport struct {
	URL         string
	Host        string
	Score       int
	Level       string
	Layers      []auditLayer
	Dimensions  []auditDims
	Findings    []string
	GradeDist   map[string]int
	OverallNote string
}

func auditLevel(score int) string {
	switch {
	case score >= 90:
		return "excellent"
	case score >= 70:
		return "good"
	case score >= 50:
		return "medium"
	default:
		return "poor"
	}
}

// auditRate 状态 → 得分率。
func auditRate(status string) float64 {
	switch status {
	case auditOK:
		return 1
	case auditWarn:
		return 0.5
	default:
		return 0
	}
}

// mkAuditCheck 依据状态构造检查项并算出实得分。
func mkAuditCheck(key, name string, weight int, status, note string) auditCheck {
	return auditCheck{
		Key: key, Name: name, Weight: weight, Status: status, Note: note,
		Score: int(math.Round(float64(weight) * auditRate(status))),
	}
}

// finalizeLayer 汇总层状态（fail > warn > ok）与层得分。
func finalizeLayer(l auditLayer) auditLayer {
	status := auditOK
	got, total := 0, 0
	for _, c := range l.Checks {
		got += c.Score
		total += c.Weight
		if c.Status == auditFail {
			status = auditFail
		} else if c.Status == auditWarn && status != auditFail {
			status = auditWarn
		}
	}
	l.Status = status
	l.Score = got
	l.Weight = total
	return l
}

// normalizeAuditURL 补全协议并校验 URL。
func normalizeAuditURL(raw string) string {
	target := strings.TrimSpace(raw)
	if target == "" {
		return ""
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "https://" + target
	}
	if _, err := url.Parse(target); err != nil {
		return ""
	}
	return target
}

// saveAuditResult 统一落库：Dimensions 存「四层快照 + 10 维视图 + 抽取块分布」，
// Findings 存待优化项。两个入口写同一张表、同一结构，历史记录因此完全通用。
func saveAuditResult(tid uint, target string, rep *auditReport) uint {
	snapshot := gin.H{
		"layers":     rep.Layers,
		"dimensions": rep.Dimensions,
		"grade_dist": rep.GradeDist,
	}
	dimJSON, _ := json.Marshal(snapshot)
	findJSON, _ := json.Marshal(rep.Findings)
	rec := models.AuditResult{
		TenantID: tid, URL: target, Score: rep.Score, Level: rep.Level,
		Dimensions: string(dimJSON), Findings: string(findJSON),
	}
	database.DB.Create(&rec)
	return rec.ID
}

// spawnAuditTasks 低分（<70）自动生成行动工单；两个入口共用，保证结论一致。
func spawnAuditTasks(tid uint, target string, rep *auditReport) int {
	if rep.Score >= 70 {
		return 0
	}
	created := 0
	for _, f := range rep.Findings {
		title := "网站优化：" + f
		if r := []rune(title); len(r) > 90 {
			title = string(r[:90]) + "…"
		}
		detail := "站点 " + target + " 四层体检得分 " + itoaInt(rep.Score) + "/100。" + f + "。修复后重新体检可自动复测。"
		if createOptTaskIfMissing(tid, "audit", title, detail, 2, "audit:"+target+":"+f) {
			created++
		}
	}
	return created
}

// runSiteAuditCore 站点审计统一核心：抓取一次，跑完四层 14 项加权检查，产出统一报告。
// 两个 HTTP 入口（/geo/audit 与 /baidu/site-audit）都必须走这里，禁止各自实现。
func runSiteAuditCore(client *http.Client, target, brand string) *auditReport {
	host := ""
	if u, err := url.Parse(target); err == nil {
		host = u.Hostname()
	}
	base := strings.TrimRight(target, "/")

	resp, err := client.Get(target)
	statusOK := false
	statusCode := 0
	html := ""
	contentType := ""
	if err == nil && resp != nil {
		statusCode = resp.StatusCode
		statusOK = statusCode == 200
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 3*1024*1024))
		resp.Body.Close()
		html = string(raw)
		contentType = resp.Header.Get("Content-Type")
	}
	isHTML := strings.Contains(contentType, "text/html") || strings.Contains(html, "<html")
	textLen := len(stripTags(html))

	// ---------- ① 访问层（权重 30）：抓取器能拿到内容吗 ----------
	access := auditLayer{Key: "access", Label: "访问", Desc: "抓取器能拿到内容吗"}
	if statusOK {
		access.Checks = append(access.Checks, mkAuditCheck("access_http", "HTTP 状态", 8, auditOK, "HTTP 200"))
	} else {
		note := "无法访问站点（连接失败或超时）"
		if statusCode > 0 {
			note = "HTTP " + itoaInt(statusCode) + "，抓取器拿不到页面"
		}
		access.Checks = append(access.Checks, mkAuditCheck("access_http", "HTTP 状态", 8, auditFail, note))
	}

	// 正文可读性：JS 渲染空壳是 AI 时代最常见的致命伤
	if textLen < 300 {
		access.Checks = append(access.Checks, mkAuditCheck("access_render", "正文可读性", 8, auditFail,
			"抓取器读不到正文（疑似 JS 渲染，正文不足 300 字），AI 引擎看到的是一片空白"))
	} else if textLen <= 500 || !isHTML {
		access.Checks = append(access.Checks, mkAuditCheck("access_render", "正文可读性", 8, auditWarn,
			"提取正文约 "+itoaInt(textLen)+" 字，偏少，建议补充可被摘录的正文"))
	} else {
		access.Checks = append(access.Checks, mkAuditCheck("access_render", "正文可读性", 8, auditOK,
			"提取正文约 "+itoaInt(textLen)+" 字"))
	}

	// AI 爬虫实测：以各主流 AI 引擎真实爬虫 UA 请求首页。
	// 很多站点会按 UA 做差异化响应（甚至直接 403 挡掉 AI 爬虫），
	// 用浏览器 UA 测出来的「可访问」并不代表 AI 能抓到 —— 必须逐个 UA 实测。
	aiBots := []struct{ Name, UA string }{
		{"GPTBot", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; GPTBot/1.1; +https://openai.com/gptbot"},
		{"Google-Extended", "Mozilla/5.0 (compatible; Google-Extended/1.0; +http://www.google.com/bot.html)"},
		{"PerplexityBot", "Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)"},
		{"ClaudeBot", "Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)"},
		{"Bytespider", "Mozilla/5.0 (compatible; Bytespider; spider-feedback@bytedance.com)"},
		{"Other-AI", "Mozilla/5.0 (compatible; AI-Crawler/1.0)"},
	}
	blockedBots := make([]string, 0)
	allowedBots := make([]string, 0)
	for _, b := range aiBots {
		st, ok := fetchAsBot(client, base, b.UA)
		// 200/3xx 视为可抓取；403/404/5xx/超时视为被拦
		if ok && st >= 200 && st < 400 {
			allowedBots = append(allowedBots, b.Name)
		} else {
			blockedBots = append(blockedBots, b.Name)
		}
	}
	switch {
	case len(blockedBots) == 0:
		access.Checks = append(access.Checks, mkAuditCheck("access_bot", "AI 爬虫 UA 实测", 8, auditOK,
			"6 类 AI 爬虫 UA 均可正常抓取（"+strings.Join(allowedBots, "、")+"）"))
	case len(allowedBots) == 0:
		access.Checks = append(access.Checks, mkAuditCheck("access_bot", "AI 爬虫 UA 实测", 8, auditFail,
			"全部 AI 爬虫被拦（"+strings.Join(blockedBots, "、")+"）—— 站点对 AI 引擎等于不可见，必须放行"))
	default:
		access.Checks = append(access.Checks, mkAuditCheck("access_bot", "AI 爬虫 UA 实测", 8, auditWarn,
			"以下 AI 爬虫被拦或异常："+strings.Join(blockedBots, "、")+"；建议检查 CDN/WAF 的爬虫放行规则"))
	}

	// 页面体积
	if len(html) < 2*1024*1024 {
		access.Checks = append(access.Checks, mkAuditCheck("access_size", "页面体积", 6, auditOK,
			fmt.Sprintf("%.1f KB", float64(len(html))/1024)))
	} else {
		access.Checks = append(access.Checks, mkAuditCheck("access_size", "页面体积", 6, auditWarn,
			fmt.Sprintf("%.1f MB，建议压缩（体积过大拖慢抓取与解析）", float64(len(html))/1024/1024)))
	}
	access = finalizeLayer(access)

	// ---------- ② 定向层（权重 18）：抓取器找得到、认得清每个 URL 吗 ----------
	direct := auditLayer{Key: "direct", Label: "定向", Desc: "抓取器找得到、认得清每个 URL 吗"}
	canonical := firstMatch(html, `<link[^>]+rel=["']canonical["'][^>]+href=["']([^"']+)["']`)
	if canonical == "" {
		canonical = firstMatch(html, `<link[^>]+href=["']([^"']+)["'][^>]+rel=["']canonical["']`)
	}
	switch {
	case canonical != "" && (host == "" || strings.Contains(canonical, host)):
		direct.Checks = append(direct.Checks, mkAuditCheck("direct_canonical", "Canonical 指向", 6, auditOK, "canonical 指向本站"))
	case canonical != "":
		direct.Checks = append(direct.Checks, mkAuditCheck("direct_canonical", "Canonical 指向", 6, auditWarn,
			"canonical 指向别处："+truncateCN(canonical, 60)))
	default:
		direct.Checks = append(direct.Checks, mkAuditCheck("direct_canonical", "Canonical 指向", 6, auditWarn,
			"未设置 canonical，同一内容多 URL 时易被判为重复页"))
	}
	if fetchHeadOK(client, base+"/robots.txt") {
		direct.Checks = append(direct.Checks, mkAuditCheck("direct_robots", "robots.txt", 6, auditOK, "存在 robots.txt"))
	} else {
		direct.Checks = append(direct.Checks, mkAuditCheck("direct_robots", "robots.txt", 6, auditWarn,
			"未检测到 robots.txt，爬虫无法确认抓取边界"))
	}
	if fetchHeadOK(client, base+"/sitemap.xml") {
		direct.Checks = append(direct.Checks, mkAuditCheck("direct_sitemap", "sitemap.xml", 6, auditOK, "存在 sitemap.xml"))
	} else {
		direct.Checks = append(direct.Checks, mkAuditCheck("direct_sitemap", "sitemap.xml", 6, auditWarn,
			"未检测到 sitemap.xml，站点结构对爬虫不透明"))
	}
	direct = finalizeLayer(direct)

	// ---------- ③ 理解层（权重 32）：机器读得懂这是什么实体吗 ----------
	understand := auditLayer{Key: "understand", Label: "理解", Desc: "机器读得懂这是什么实体吗"}
	title := firstMatch(html, `<title[^>]*>([^<]{1,200})</title>`)
	switch {
	case title == "":
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_title", "标题与品牌词", 8, auditFail,
			"缺少 <title>，AI 无法判定页面主体是谁"))
	case brand == "" || strings.Contains(title, brand):
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_title", "标题与品牌词", 8, auditOK, title))
	default:
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_title", "标题与品牌词", 8, auditWarn,
			"标题未含品牌词："+truncateCN(title, 40)))
	}

	hasJSONLD := strings.Contains(html, "application/ld+json")
	hasMicro := strings.Contains(html, "itemscope") || strings.Contains(html, "itemtype=")
	if hasJSONLD || hasMicro {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_schema", "Schema 结构化数据", 8, auditOK,
			"检测到 JSON-LD / Microdata"))
	} else {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_schema", "Schema 结构化数据", 8, auditWarn,
			"缺少 Schema，机器难以理解实体关系（建议补 Organization / Product / FAQ）"))
	}

	h1s := regexp.MustCompile(`<h1[^>]*>`).FindAllString(html, -1)
	if len(h1s) >= 1 && len(h1s) <= 3 {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_h1", "H1 标题结构", 6, auditOK,
			itoaInt(len(h1s))+" 个 H1"))
	} else {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_h1", "H1 标题结构", 6, auditWarn,
			itoaInt(len(h1s))+" 个 H1（建议 1 个）"))
	}

	desc := firstMatch(html, `<meta[^>]+name=["']description["'][^>]+content=["']([^"']{1,300})["']`)
	if desc == "" {
		desc = firstMatch(html, `<meta[^>]+content=["']([^"']{1,300})["'][^>]+name=["']description["']`)
	}
	if desc != "" {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_meta", "Meta 描述", 6, auditOK, truncateCN(desc, 60)))
	} else {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_meta", "Meta 描述", 6, auditWarn,
			"缺少 meta description，摘要由引擎自行猜测"))
	}

	imgN := len(regexp.MustCompile(`<img[^>]*>`).FindAllString(html, -1))
	altN := len(regexp.MustCompile(`<img[^>]*alt=["'][^"']+["']`).FindAllString(html, -1))
	if imgN == 0 || altN == imgN {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_alt", "图片 Alt", 4, auditOK,
			itoaInt(imgN)+" 张图全部带 alt"))
	} else {
		understand.Checks = append(understand.Checks, mkAuditCheck("understand_alt", "图片 Alt", 4, auditWarn,
			itoaInt(imgN)+" 张图中 "+itoaInt(imgN-altN)+" 张缺 alt"))
	}
	understand = finalizeLayer(understand)

	// ---------- ④ 可引用层（权重 20）：内容能被摘出来直接引用吗 ----------
	citable := auditLayer{Key: "citable", Label: "可引用", Desc: "内容能被摘出来直接引用吗"}
	if fetchHeadOK(client, base+"/llms.txt") {
		citable.Checks = append(citable.Checks, mkAuditCheck("citable_llms", "llms.txt", 6, auditOK,
			"已上线 llms.txt，可直接喂给大模型"))
	} else {
		citable.Checks = append(citable.Checks, mkAuditCheck("citable_llms", "llms.txt", 6, auditWarn,
			"缺少 llms.txt，建议生成供大模型友好读取"))
	}

	// 抽取块分级：检索按段落选材，整页无一段自包含可引的内容会被点名 —— GEO 最大的单项杠杆。
	gradeA, gradeB, gradeC, gradeD := 0, 0, 0, 0
	blocks := regexp.MustCompile(`(?s)<(p|li|h[1-3])[^>]*>.*?</(p|li|h[1-3])>`).FindAllString(html, -1)
	for _, b := range blocks {
		bt := stripTags(b)
		if len(bt) < 20 {
			gradeD++
			continue
		}
		hasList := regexp.MustCompile(`(?s)<(ul|ol)[^>]*>`).MatchString(b)
		hasDef := strings.Contains(bt, "是") || strings.Contains(bt, "定义") || strings.Contains(bt, "：")
		hasData := regexp.MustCompile(`\d+%|\d+\.\d+|\d+ 年|\d+ 个|\d+ 家`).MatchString(bt)
		switch {
		case hasList && hasDef && hasData:
			gradeA++
		case hasList || hasDef:
			gradeB++
		default:
			gradeC++
		}
	}
	if len(blocks) == 0 && textLen < 300 {
		gradeD++
	}
	gradeDist := map[string]int{"A": gradeA, "B": gradeB, "C": gradeC, "D": gradeD}
	blockTotal := gradeA + gradeB + gradeC + gradeD
	citableRate := 0.0
	if blockTotal > 0 {
		citableRate = float64(gradeA+gradeB) / float64(blockTotal)
	}
	blockNote := fmt.Sprintf("可引用段落 %d/%d（A 类 %d · B 类 %d · C 类 %d · D 类 %d）",
		gradeA+gradeB, blockTotal, gradeA, gradeB, gradeC, gradeD)
	switch {
	case blockTotal == 0:
		citable.Checks = append(citable.Checks, mkAuditCheck("citable_blocks", "可引用段落", 14, auditFail,
			"整页无自包含段落，检索无材可选"))
	case citableRate >= 0.6:
		citable.Checks = append(citable.Checks, mkAuditCheck("citable_blocks", "可引用段落", 14, auditOK, blockNote))
	case citableRate >= 0.3:
		citable.Checks = append(citable.Checks, mkAuditCheck("citable_blocks", "可引用段落", 14, auditWarn,
			blockNote+"；建议把关键结论改写成「结论 + 依据 + 数据」的自包含段落"))
	default:
		citable.Checks = append(citable.Checks, mkAuditCheck("citable_blocks", "可引用段落", 14, auditFail,
			blockNote+"；GEO 最大的单项杠杆 —— 检索按段落选材，无自包含段落等于无法被引用"))
	}
	citable = finalizeLayer(citable)

	// ---------- 综合分（唯一算法） ----------
	layers := []auditLayer{access, direct, understand, citable}
	got, weightTotal := 0, 0
	for _, l := range layers {
		got += l.Score
		weightTotal += l.Weight
	}
	score := 0
	if weightTotal > 0 {
		score = int(math.Round(float64(got) * 100 / float64(weightTotal)))
	}
	level := auditLevel(score)

	// ---------- 10 维视图（由四层检查项派生，不再独立打分） ----------
	dimensions := []auditDims{}
	findings := []string{}
	for _, l := range layers {
		for _, c := range l.Checks {
			s10 := 0
			if c.Weight > 0 {
				s10 = int(math.Round(float64(c.Score) * 10 / float64(c.Weight)))
			}
			dimensions = append(dimensions, auditDims{
				Label: l.Label + " · " + c.Name,
				Score: s10,
				Pass:  c.Status == auditOK,
				Note:  c.Note,
			})
			if c.Status == auditOK {
				continue
			}
			tag := "待优化"
			if c.Status == auditFail {
				tag = "不达标"
			}
			findings = append(findings, "["+l.Label+"/"+tag+"] "+c.Name+"："+c.Note)
		}
	}

	// ---------- 总评 ----------
	overall := ""
	for _, l := range layers {
		if l.Status != auditFail {
			continue
		}
		if l.Key == "access" {
			overall = "先修「访问」层 —— 访问层失败时，下游一切优化在引擎侧都不可见。"
		} else {
			overall = "优先修「" + l.Label + "」层 —— 该层存在硬性缺陷，会直接阻断 AI 引擎的抓取或理解。"
		}
		break
	}
	if overall == "" {
		warns := 0
		for _, l := range layers {
			for _, c := range l.Checks {
				if c.Status == auditWarn {
					warns++
				}
			}
		}
		if warns > 0 {
			overall = "四层均可访问，仍有 " + itoaInt(warns) + " 处隐患；按清单逐项修复可持续抬高被引用概率。"
		} else {
			overall = "四层体检全部通过，站点对 AI 引擎友好。"
		}
	}

	return &auditReport{
		URL: target, Host: host, Score: score, Level: level,
		Layers: layers, Dimensions: dimensions, Findings: findings,
		GradeDist: gradeDist, OverallNote: overall,
	}
}

// SiteAuditDetail 站点体检（四层）—— 与 /api/geo/audit 共用同一套评分核心。
func SiteAuditDetail(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "请提供要体检的站点 URL"})
		return
	}
	target := normalizeAuditURL(body.URL)
	if target == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "URL 格式错误"})
		return
	}

	tid := TenantID(c)
	client := &http.Client{Timeout: 20 * time.Second}
	rep := runSiteAuditCore(client, target, BrandOf(c))
	saveAuditResult(tid, target, rep)
	spawnAuditTasks(tid, target, rep)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": siteAuditDetail{
		URL: rep.URL, Host: rep.Host, Score: rep.Score, Level: rep.Level,
		Layers: rep.Layers, Dimensions: rep.Dimensions, Findings: rep.Findings,
		GradeDist: rep.GradeDist, OverallNote: rep.OverallNote,
	}})
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ============ 差距诊断（三缺口：内容 → 阵地 → 事实） ============

type gapDiagnose struct {
	ContentGap       int      `json:"content_gap"`
	ContentQuestions []string `json:"content_questions"`
	ChannelGap       int      `json:"channel_gap"`
	ChannelList      []string `json:"channel_list"`
	FactGap          int      `json:"fact_gap"`
	FactList         []string `json:"fact_list"`
}

// GapDiagnose 差距诊断：分数低只有三个原因，按顺序修（内容 → 阵地 → 事实）。
func GapDiagnose(c *gin.Context) {
	tid := TenantID(c)

	// 1) 内容缺口：近 30 天品牌缺席 >= 2 次的问题
	results, _, _ := queryResultsInRange(c, 30)
	qMiss := map[string]int{}
	for _, r := range results {
		if r.ErrorMsg == "" && !r.Hit {
			qMiss[r.Question]++
		}
	}
	contentQ := []string{}
	for q, n := range qMiss {
		if n >= 2 {
			contentQ = append(contentQ, q)
		}
	}
	sort.Strings(contentQ)

	// 2) 阵地缺口：主要阵地中未被 AI 引用的
	var cites []models.Citation
	database.DB.Where("tenant_id = ? AND domain != ''", tid).Find(&cites)
	citedDomains := map[string]bool{}
	for _, ct := range cites {
		citedDomains[ct.Domain] = true
	}
	channelKw := map[string]string{
		"百度百科": "baike.baidu.com", "知乎": "zhihu.com", "微信公众号": "mp.weixin.qq.com",
		"今日头条": "toutiao.com", "百家号": "baijiahao.baidu.com", "小红书": "xiaohongshu.com",
		"抖音": "douyin.com", "B站": "bilibili.com", "搜狐号": "sohu.com", "网易号": "163.com",
		"腾讯新闻": "news.qq.com", "36氪": "36kr.com", "虎嗅": "huxiu.com",
		"Wikipedia": "wikipedia.org", "G2": "g2.com", "Reddit": "reddit.com", "YouTube": "youtube.com",
	}
	channelList := []string{}
	for name, kw := range channelKw {
		covered := false
		for d := range citedDomains {
			if strings.Contains(d, kw) {
				covered = true
				break
			}
		}
		if !covered {
			channelList = append(channelList, name)
		}
	}
	sort.Strings(channelList)

	// 3) 事实偏差：事实库 NotFact 被 AI 回答命中
	var facts []models.FactItem
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&facts)
	factList := []string{}
	for _, f := range facts {
		if strings.TrimSpace(f.NotFact) == "" {
			continue
		}
		hit := false
		for _, r := range results {
			if r.ErrorMsg == "" && strings.Contains(strings.ToLower(r.Response), strings.ToLower(f.NotFact)) {
				hit = true
				break
			}
		}
		if hit {
			factList = append(factList, f.NotFact)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gapDiagnose{
		ContentGap: len(contentQ), ContentQuestions: contentQ,
		ChannelGap: len(channelList), ChannelList: channelList,
		FactGap: len(factList), FactList: factList,
	}})
}
