package baidu

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ============ 百度 SERP 解析（广告位与自然排名分离识别） ============

type RankItem struct {
	Rank         int    `json:"rank"`
	Type         string `json:"type"` // ad=广告 / organic=自然
	Peer         bool   `json:"peer"` // 是否识别为同行（自然排名统计口径，广告不计）
	Name         string `json:"name"`
	Domain       string `json:"domain"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	Snippet      string `json:"snippet"`
	Suspected    bool   `json:"suspected"`     // 疑似同行（命中行业词，待人工确认）
	SuspectHints string `json:"suspectHints"` // 命中的行业词
}

type PageResult struct {
	Page          int        `json:"page"`
	Items         []RankItem `json:"items"`
	AdCount       int        `json:"adCount"`
	OrganicCount  int        `json:"organicCount"`
	OrganicPeers  int        `json:"organicPeers"`
	ParseErr      string     `json:"parseErr,omitempty"`
}

// ParseSERP 解析一页结果 HTML。heuristic:
//   广告：容器 tpl 以 ad_ 开头，或 data-tools 内 is_ad=true，或无 h3 的 result-op 块
//   自然：h3 a 标题 + cite/URL 提取域名
func ParseSERP(html string, keyword string) PageResult {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return PageResult{ParseErr: "HTML 解析失败: " + err.Error()}
	}
	res := PageResult{}
	// 遍历 #content_left 直接子结果块
	content := doc.Find("#content_left")
	if content.Length() == 0 {
		content = doc.Find("#results, .result-op-parent, #content_left, body")
	}

	seen := map[string]bool{}
	process := func(sel *goquery.Selection) {
		sel.Each(func(i int, block *goquery.Selection) {
			if len(res.Items) >= 10 {
				return
			}
			// 跳过总体容器/别的噪声块
			cls := block.AttrOr("class", "")
			if cls == "" {
				return
			}
			// 跳过"大家还在搜"等推荐区块（既非广告也非结果）
			if strings.Contains(block.Text()[:min(len(block.Text()), 60)], "大家还在搜") {
				return
			}
			isAd := isAdBlock(block)
			h3 := block.Find("h3 a").First()
			title := strings.TrimSpace(h3.Text())
			var link string
			if len(title) > 0 {
				if href, ok := h3.Attr("href"); ok {
					link = href
				}
			}
			if title == "" {
				// 无 h3 的块：若不是广告则跳过（多为推荐/相关词）
				if !isAd {
					return
				}
				title = block.Find(".c-title, .c-color-link").First().Text()
				if title == "" {
					title = block.Text()[:min(len(block.Text()), 40)]
				}
				if href, ok := block.Find("a").First().Attr("href"); ok {
					link = href
				}
			}
			domain := extractDomain(link, h3, block)
			if domain == "" {
				domain = "unknown"
			}
			if seen[domain+title] {
				return
			}
			seen[domain+title] = true
			// 摘要
			snippet := block.Find(".c-abstract, .c-span-last, .content-right_8Zs40, .cos-text").First().Text()
			snippet = strings.TrimSpace(snippet)

			item := RankItem{
				Rank:    len(res.Items) + 1,
				Type:    "organic",
				Domain:  domain,
				Title:   strings.TrimSpace(title),
				URL:     strings.TrimSpace(link),
				Snippet: snippet,
			}
			if isAd {
				item.Type = "ad"
				res.AdCount++
			} else {
				res.OrganicCount++
			}
			res.Items = append(res.Items, item)
		})
	}

	content.Children().Each(func(_ int, block *goquery.Selection) {
		process(block)
	})
	if len(res.Items) == 0 {
		// 兜底：全页 h3 a
		doc.Find("#content_left h3 a").Each(func(i int, a *goquery.Selection) {
			if len(res.Items) >= 10 {
				return
			}
			title := strings.TrimSpace(a.Text())
			if title == "" {
				return
			}
			link, _ := a.Attr("href")
			domain := extractDomain(link, a, a.ParentsUntil("#content_left"))
			item := RankItem{
				Rank: len(res.Items) + 1, Type: "organic", Title: title, URL: link, Domain: domain,
				Snippet: strings.TrimSpace(a.Parent().Find(".c-abstract").Text()),
			}
			res.Items = append(res.Items, item)
			res.OrganicCount++
		})
	}
	return res
}

// isAdBlock 判定结果块是否为广告位
func isAdBlock(block *goquery.Selection) bool {
	tpl := block.AttrOr("tpl", "")
	if strings.HasPrefix(tpl, "ad_") {
		return true
	}
	cls := block.AttrOr("class", "")
	if strings.Contains(cls, "ec_") || strings.Contains(cls, "result-op") && block.Find("h3 a").Eq(0).Length() < 1 {
		// result-op 大量广告承载；无 h3 的 result-op 视为广告
		if block.Find("h3 a").Length() == 0 {
			return true
		}
	}
	if tools, ok := block.Attr("data-tools"); ok && tools != "" {
		var m map[string]interface{}
		if json.Unmarshal([]byte(tools), &m) == nil {
			if v, exists := m["is_ad"]; exists {
				if b, _ := v.(bool); b {
					return true
				}
				if s, _ := v.(string); strings.EqualFold(s, "true") {
					return true
				}
			}
		}
	}
	if v, ok := block.Attr("data-mark"); ok {
		if strings.Contains(v, "ad") {
			return true
		}
	}
	return false
}

// extractDomain 从跳转链接/mu/cite 提取真实域名（注册主域，去 www）
// 真实百度 SERP 结果容器带 mu 属性（明文目标 URL），优先级最高
func extractDomain(link string, h3a *goquery.Selection, block *goquery.Selection) string {
	// 1) mu 属性（新版百度明文目标地址，最可靠）
	if block != nil {
		if v, ok := block.Attr("mu"); ok && v != "" {
			if d := hostOf(v); d != "" {
				return d
			}
		}
	}
	// 2) data-landurl 属性（部分自然结果带真实 URL）
	if block != nil {
		if v, ok := block.Attr("data-landurl"); ok && v != "" {
			if d := hostOf(v); d != "" {
				return d
			}
		}
	}
	// 3) cite 文本（百度展示的域名字样）
	if block != nil {
		if cite := strings.TrimSpace(block.Find("cite").Eq(0).Text()); cite != "" {
			if d := hostOf(cite); d != "" {
				return d
			}
			return cleanDomain(cite)
		}
	}
	// 4) 解析 href（baidu jump link 则取 query url 参数）
	if u, err := url.Parse(link); err == nil {
		if strings.Contains(u.Host, "baidu.com") {
			if q := u.Query().Get("url"); q != "" {
				if d := hostOf(q); d != "" {
					return d
				}
			}
			return ""
		}
		if d := hostOf(link); d != "" {
			return d
		}
	}
	return ""
}

func hostOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "//") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return cleanDomain(u.Host)
}

// cleanDomain 去端口、去 www.、小写
func cleanDomain(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	if strings.HasPrefix(h, "www.") {
		h = h[4:]
	}
	return h
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
