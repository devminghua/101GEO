package baidu

import (
	"fmt"
	"strings"
	"time"
)

// ============ 关键词分析任务编排（抓取 → 解析 → 同行识别 → 归因 → 建议） ============

type TaskConfig struct {
	Keyword  string
	// 抓取几页，固定 3。
	// 深度是百度反爬命中率的主要变量：每多 1 页就多 1 次 HTTP 请求，被风控识别的机会线性增加。
	// 已按老板 2026-09-03 拍板去掉前端「抓取页数」控件，深度固定为 3，不再由调用方指定。
	Depth    int
	MyDomain string   // 我方主域（用于识别我方排名）；可空
	MyDomains []string // 多我方域
	PeerLib  map[string]string
}

// maxDepth 抓取深度上限：固定 3，防止调用方传入更大值把反爬风险拉回去。
const maxDepth = 3

func (c *TaskConfig) Normalize() {
	// 无论调用方传什么，一律收敛到 maxDepth（<=0 视为未传，取默认值 3）
	if c.Depth <= 0 {
		c.Depth = maxDepth
	}
	if c.Depth > maxDepth {
		c.Depth = maxDepth
	}
	if c.PeerLib == nil {
		c.PeerLib = DefaultPeerLib
	}
	if c.MyDomain != "" {
		c.MyDomains = append(c.MyDomains, c.MyDomain)
	}
}

// RunKeyword 同步执行一次关键词分析，返回完整 Result（百度引擎）
func RunKeyword(cfg TaskConfig) (*Result, error) {
	return RunKeywordEngine(cfg, httpEngine{}, SuggestThrottle)
}

// Engine 抓取并解析一页搜索结果的引擎（引擎差异点收敛于此，管线共用）。
// 百度走「HTTP 抓取 HTML + goquery 解析」；Google/Naver 走第三方 SERP API。
type Engine interface {
	FetchParse(keyword string, page int) (PageResult, error)
}

// httpEngine 百度引擎：抓 HTML 后解析
type httpEngine struct{}

func (httpEngine) FetchParse(keyword string, page int) (PageResult, error) {
	html, err := FetchPage(keyword, page)
	if err != nil {
		return PageResult{}, err
	}
	return ParseSERP(html, keyword), nil
}

// RunKeywordEngine 引擎可注入的关键词分析管线（抓取 → 解析 → 同行识别 → 归因 → 建议）。
// 口径唯一：百度/Google/Naver 共用此管线，仅引擎实现不同。
func RunKeywordEngine(cfg TaskConfig, eng Engine, throttle func()) (*Result, error) {
	cfg.Normalize()
	start := time.Now()

	var pages []PageInfo
	var suspicious []SuspiciousPeer
	errors := 0

	// 页间节流串行执行更稳妥（避免风控）；解析在内存中完成，无需并发。
	for p := 1; p <= cfg.Depth; p++ {
		pr, err := eng.FetchParse(cfg.Keyword, p)
		if err != nil {
			errors++
			pages = append(pages, PageInfo{Page: p, ParseErrTag: err.Error()})
			continue
		}
		analyzePage(&pr, cfg)
		pages = append(pages, PageInfo{
			Page: p, Ads: pr.AdCount, OrganicPeers: pr.OrganicPeers, OrganicOther: countOrganicOther(pr),
			Density: densityOf(pr), Items: pr.Items,
		})
		// 疑似同行收集
		for i := range pr.Items {
			it := &pr.Items[i]
			if it.Suspected {
				suspicious = append(suspicious, SuspiciousPeer{
					Domain: it.Domain, Title: it.Title, URL: it.URL,
					Pages: []int{p}, Hints: it.SuspectHints,
				})
			}
		}
		// 页面间节流（nil = 引擎自带限流，无需额外节流）
		if p < cfg.Depth && throttle != nil {
			throttle()
		}
	}

	// 去重（按 domain）
	susDedup := map[string]SuspiciousPeer{}
	for _, s := range suspicious {
		k := s.Domain
		if exist, ok := susDedup[k]; ok {
			exist.Pages = append(exist.Pages, s.Pages...)
			exist.Pages = sortedInts(exist.Pages)
			susDedup[k] = exist
		} else {
			susDedup[k] = s
		}
	}
	susList := make([]SuspiciousPeer, 0, len(susDedup))
	for _, s := range susDedup {
		susList = append(susList, s)
	}

	opts := AnalyzeOptions{MyDomains: cfg.MyDomains, PeerLib: cfg.PeerLib}
	res := BuildResult(cfg.Keyword, cfg.Depth, pages, susList, opts)
	res.CostMs = int(time.Since(start).Milliseconds())
	if errors > 0 {
		// 区分「真·反爬」和「网络/转发服务故障」，避免把 connection refused 误报成反爬，
		// 否则排查方向会完全跑偏（2026-09-03 实测踩过：转发服务没起，3/3 页全报反爬）。
		antiCrawl, netErr := 0, 0
		for _, p := range pages {
			if p.ParseErrTag == "" {
				continue
			}
			if strings.Contains(p.ParseErrTag, "安全验证") || strings.Contains(p.ParseErrTag, "反爬") {
				antiCrawl++
			} else {
				netErr++
			}
		}
		var reason string
		switch {
		case antiCrawl > 0 && netErr > 0:
			reason = fmt.Sprintf("（%d 页触发百度安全验证，%d 页网络或转发服务故障）", antiCrawl, netErr)
		case antiCrawl > 0:
			reason = "（触发百度安全验证，建议降低调用频率或稍后重试）"
		default:
			reason = "（网络或抓取转发服务不可用，请检查 GEO_BAIDU_PROXY 指向的转发服务是否已启动）"
		}
		res.PartialFail = fmt.Sprintf("%d/%d 页抓取失败%s，以下为部分结果", errors, cfg.Depth, reason)
	}
	return res, nil
}

// analyzePage 对单页结果做同行识别（就地修改 item.Peer / Suspected / Name）
func analyzePage(pr *PageResult, cfg TaskConfig) {
	for i := range pr.Items {
		it := &pr.Items[i]
		// 广告位不作同行统计（广告与自然分离识别）
		if it.Type == "ad" {
			continue
		}
		it.Name = it.Domain
		if name, ok := cfg.PeerLib[it.Domain]; ok {
			it.Peer = true
			it.Name = name
			pr.OrganicPeers++
			continue
		}
		if KnownNonPeer[it.Domain] {
			continue
		}
		// 疑似同行：标题/摘要命中行业词
		blob := it.Title + " " + it.Snippet
		hints := matchIndustryHints(blob)
		if len(hints) > 0 {
			it.Suspected = true
			it.SuspectHints = strings.Join(hints, "、")
		}
	}
}

func matchIndustryHints(blob string) []string {
	var out []string
	for _, h := range IndustryHints {
		if strings.Contains(blob, h) && !containsString(out, h) {
			out = append(out, h)
		}
	}
	return out
}

func countOrganicOther(pr PageResult) int {
	n := 0
	for _, it := range pr.Items {
		if it.Type == "organic" && !it.Peer && !it.Suspected {
			n++
		}
	}
	return n
}

func densityOf(pr PageResult) float64 {
	if pr.OrganicCount == 0 {
		return 0
	}
	return float64(pr.OrganicPeers) / float64(pr.OrganicCount)
}

func containsString(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
