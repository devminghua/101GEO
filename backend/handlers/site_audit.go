package handlers

import (
	"encoding/json"
	"io"
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

// ============ 站点体检（四层：访问 → 定向 → 理解 → 可引用） ============

type auditCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok / warn / fail
	Note   string `json:"note"`
}

type auditLayer struct {
	Key    string       `json:"key"`
	Label  string       `json:"label"`
	Desc   string       `json:"desc"`
	Status string       `json:"status"` // ok / warn / fail
	Checks []auditCheck `json:"checks"`
}

type siteAuditDetail struct {
	URL         string         `json:"url"`
	Host        string         `json:"host"`
	Score       int            `json:"score"`
	Level       string         `json:"level"`
	Layers      []auditLayer   `json:"layers"`
	GradeDist   map[string]int `json:"grade_dist"` // A/B/C/D 抽取块分级
	OverallNote string         `json:"overall_note"`
}

// SiteAuditDetail 站点体检（四层）：按「访问→定向→理解→可引用」组织技术层检查。
func SiteAuditDetail(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "请提供要体检的站点 URL"})
		return
	}
	target := strings.TrimSpace(body.URL)
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "https://" + target
	}
	u, err := url.Parse(target)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "URL 格式错误"})
		return
	}
	host := u.Hostname()
	base := strings.TrimRight(target, "/")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(target)
	statusOK := err == nil && resp != nil && resp.StatusCode == 200
	html := ""
	contentType := ""
	if resp != nil {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 3*1024*1024))
		resp.Body.Close()
		html = string(raw)
		contentType = resp.Header.Get("Content-Type")
	}
	_ = contentType

	// ---------- 访问层 ----------
	access := auditLayer{Key: "access", Label: "访问", Desc: "抓取器能拿到内容吗", Status: "ok"}
	if statusOK {
		access.Checks = append(access.Checks, auditCheck{Name: "HTTP 状态", Status: "ok", Note: "HTTP 200"})
	} else {
		access.Status = "fail"
		access.Checks = append(access.Checks, auditCheck{Name: "HTTP 状态", Status: "fail", Note: "无法访问或非 200"})
	}
	textLen := len(stripTags(html))
	if textLen < 300 {
		access.Status = "fail"
		access.Checks = append(access.Checks, auditCheck{Name: "前端渲染空壳", Status: "fail", Note: "抓取器读不到正文（疑似 JS 渲染，正文不足 300 字）"})
	} else {
		access.Checks = append(access.Checks, auditCheck{Name: "正文可读性", Status: "ok", Note: "提取正文约 " + itoaInt(textLen) + " 字"})
	}

	// AI 爬虫实测：用各主流 AI 引擎的真实爬虫 UA 请求首页。
	// 很多站点会按 UA 做差异化响应（甚至直接 403 挡掉 AI 爬虫），
	// 用浏览器 UA 测出来的「可访问」并不代表 AI 能抓到 —— 必须逐个 UA 实测。
	// 覆盖：OpenAI GPTBot / Google-Extended / PerplexityBot / ClaudeBot /
	//       Bytespider(字节·豆包) / Kimi(YisouBot 近似) / 通义(TongyiBot)
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
	if len(blockedBots) == 0 {
		access.Checks = append(access.Checks, auditCheck{
			Name: "AI 爬虫 UA 实测", Status: "ok",
			Note: "6 类 AI 爬虫 UA 均可正常抓取（" + strings.Join(allowedBots, "、") + "）",
		})
	} else {
		access.Status = "warn"
		access.Checks = append(access.Checks, auditCheck{
			Name: "AI 爬虫 UA 实测", Status: "warn",
			Note: "以下 AI 爬虫被拦或异常：" + strings.Join(blockedBots, "、") + "；建议检查 CDN/WAF 的爬虫放行规则",
		})
		// 全被拦属于严重问题
		if len(allowedBots) == 0 {
			access.Status = "fail"
		}
	}
	// ---------- 定向层 ----------
	direct := auditLayer{Key: "direct", Label: "定向", Desc: "抓取器找得到、认得清每个 URL 吗", Status: "ok"}
	canonical := firstMatch(html, `<link[^>]+rel=["']canonical["'][^>]+href=["']([^"']+)["']`)
	if canonical == "" {
		canonical = firstMatch(html, `<link[^>]+href=["']([^"']+)["'][^>]+rel=["']canonical["']`)
	}
	if canonical != "" && !strings.Contains(canonical, host) {
		direct.Status = "warn"
		direct.Checks = append(direct.Checks, auditCheck{Name: "Canonical 指向", Status: "warn", Note: "canonical 指向别处：" + truncateCN(canonical, 60)})
	} else if canonical != "" {
		direct.Checks = append(direct.Checks, auditCheck{Name: "Canonical 指向", Status: "ok", Note: "canonical 指向本站"})
	} else {
		direct.Checks = append(direct.Checks, auditCheck{Name: "Canonical 指向", Status: "warn", Note: "未设置 canonical"})
	}

	// ---------- 理解层 ----------
	understand := auditLayer{Key: "understand", Label: "理解", Desc: "机器读得懂这是什么实体吗", Status: "ok"}
	brand := BrandOf(c)
	title := firstMatch(html, `<title[^>]*>([^<]{1,200})</title>`)
	if title != "" && (brand == "" || strings.Contains(title, brand)) {
		understand.Checks = append(understand.Checks, auditCheck{Name: "标题与品牌词", Status: "ok", Note: title})
	} else {
		understand.Status = "warn"
		understand.Checks = append(understand.Checks, auditCheck{Name: "标题与品牌词", Status: "warn", Note: "标题缺失或未含品牌词"})
	}
	hasJSONLD := strings.Contains(html, "application/ld+json")
	hasMicro := strings.Contains(html, "itemscope") || strings.Contains(html, "itemtype=")
	if hasJSONLD || hasMicro {
		understand.Checks = append(understand.Checks, auditCheck{Name: "Schema 结构化数据", Status: "ok", Note: "检测到 JSON-LD / Microdata"})
	} else {
		understand.Status = "warn"
		understand.Checks = append(understand.Checks, auditCheck{Name: "Schema 结构化数据", Status: "warn", Note: "缺少 Schema，机器难以理解实体关系"})
	}
	h1s := regexp.MustCompile(`<h1[^>]*>`).FindAllString(html, -1)
	if len(h1s) >= 1 && len(h1s) <= 3 {
		understand.Checks = append(understand.Checks, auditCheck{Name: "H1 标题结构", Status: "ok", Note: itoaInt(len(h1s)) + " 个 H1"})
	} else {
		understand.Checks = append(understand.Checks, auditCheck{Name: "H1 标题结构", Status: "warn", Note: itoaInt(len(h1s)) + " 个 H1（建议 1 个）"})
	}

	// ---------- 可引用层 ----------
	citable := auditLayer{Key: "citable", Label: "可引用", Desc: "robots / WAF / sitemap / llms.txt", Status: "ok"}
	robotsURL := base + "/robots.txt"
	if fetchHeadOK(client, robotsURL) {
		citable.Checks = append(citable.Checks, auditCheck{Name: "robots.txt", Status: "ok", Note: "存在 robots.txt"})
	} else {
		citable.Checks = append(citable.Checks, auditCheck{Name: "robots.txt", Status: "warn", Note: "未检测到 robots.txt"})
	}
	// （AI 爬虫 UA 实测已上移到访问层，用真实 UA 逐个探测）
	if fetchHeadOK(client, base+"/sitemap.xml") {
		citable.Checks = append(citable.Checks, auditCheck{Name: "sitemap.xml", Status: "ok", Note: "存在 sitemap.xml"})
	} else {
		citable.Checks = append(citable.Checks, auditCheck{Name: "sitemap.xml", Status: "warn", Note: "未检测到 sitemap.xml"})
	}
	if fetchHeadOK(client, base+"/llms.txt") {
		citable.Checks = append(citable.Checks, auditCheck{Name: "llms.txt", Status: "ok", Note: "已上线 llms.txt"})
	} else {
		citable.Checks = append(citable.Checks, auditCheck{Name: "llms.txt", Status: "warn", Note: "缺少 llms.txt，建议生成供大模型友好读取"})
	}

	// ---------- 抽取块分级（基于段落/列表/定义块粗判） ----------
	gradeA := 0 // 含列表 + 定义 + 数据
	gradeB := 0 // 含列表或定义
	gradeC := 0 // 有正文但无结构化要素
	gradeD := 0 // 无正文
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
		if hasList && hasDef && hasData {
			gradeA++
		} else if hasList || hasDef {
			gradeB++
		} else {
			gradeC++
		}
	}
	if len(blocks) == 0 && textLen < 300 {
		gradeD++
	}

	// ---------- 综合分 ----------
	score := 100
	layerScore := map[string]int{"access": 25, "direct": 15, "understand": 30, "citable": 30}
	for _, l := range []auditLayer{access, direct, understand, citable} {
		switch l.Status {
		case "fail":
			score -= layerScore[l.Key] / 2
		case "warn":
			score -= layerScore[l.Key] / 4
		}
	}
	if score < 0 {
		score = 0
	}
	level := "poor"
	if score >= 90 {
		level = "excellent"
	} else if score >= 70 {
		level = "good"
	} else if score >= 50 {
		level = "medium"
	}

	overall := "四层体检完成。"
	if access.Status == "fail" {
		overall = "先修「访问」层——访问层失败时，下游一切优化在引擎侧都不可见。"
	}

	// 落库（复用 AuditResult，附加四层快照）
	dimJSON, _ := json.Marshal([]auditLayer{access, direct, understand, citable})
	rec := models.AuditResult{
		TenantID: TenantID(c), URL: target, Score: score, Level: level,
		Dimensions: string(dimJSON), Findings: "[]",
	}
	database.DB.Create(&rec)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": siteAuditDetail{
		URL: target, Host: host, Score: score, Level: level,
		Layers:      []auditLayer{access, direct, understand, citable},
		GradeDist:   map[string]int{"A": gradeA, "B": gradeB, "C": gradeC, "D": gradeD},
		OverallNote: overall,
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
