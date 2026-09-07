package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/services/baidu"
)

// KeywordSuggest 拓词选题：拉取百度下拉 / Google 补全的候选搜索词。
// POST /api/baidu/suggest  body: { "keyword": "词根", "source": "baidu"|"google"|"both" }
func KeywordSuggest(c *gin.Context) {
	var req struct {
		Keyword string `json:"keyword"`
		Source  string `json:"source"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请输入拓词词根"})
		return
	}
	src := strings.TrimSpace(req.Source)
	if src == "" {
		src = "baidu"
	}
	words := []string{}
	var baiduWords, googleWords []string
	if src == "baidu" || src == "both" {
		if w, err := baidu.Suggest(req.Keyword); err == nil {
			baiduWords = w
		}
	}
	if src == "google" || src == "both" {
		if w, err := baidu.SuggestGoogle(req.Keyword); err == nil {
			googleWords = w
		}
	}
	// 合并去重，保持顺序
	seen := map[string]bool{}
	for _, w := range append(append([]string{}, baiduWords...), googleWords...) {
		if !seen[w] {
			seen[w] = true
			words = append(words, w)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"keyword":      req.Keyword,
		"words":        words,
		"baidu_count":  len(baiduWords),
		"google_count": len(googleWords),
	}})
}

// AnalyzeBaiduKeyword 百度关键词分析：同步抓取 SERP → 同行识别 → 归因 → 建议
// 增强：入参支持 my_domains（客户配置的网站域名）；分析完成后将各客户网站该词
// 当日最优排名写入排名历史快照表（按租户隔离），并在返回中附带该词我方近一个月趋势摘要。
func AnalyzeBaiduKeyword(c *gin.Context) {
	var req struct {
		Keyword   string   `json:"keyword"`
		Depth     int      `json:"depth"`
		MyDomain  string   `json:"my_domain"`
		MyDomains []string `json:"my_domains"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请填写要分析的关键词"})
		return
	}
	// 每日查询配额：跨百度/抖音/小红书统一计数
	if !QuotaGuard(c) {
		return
	}
	// 归一化客户域名（去协议/www/路径）
	myDomains := normalizeDomains(append(req.MyDomains, req.MyDomain))
	cfg := baidu.TaskConfig{
		Keyword:   req.Keyword,
		Depth:     req.Depth,
		MyDomain:  strings.TrimSpace(req.MyDomain),
		MyDomains: myDomains,
	}
	res, err := baidu.RunKeyword(cfg)
	if err != nil && res == nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "分析失败：" + err.Error()})
		return
	}
	// 分析完成后：将各客户网站该词当日最优排名写入快照表（同词+同域+同日去重）
	tid := TenantID(c)
	if tid > 0 {
		best := bestRankPerDomain(res, myDomains)
		for domain, br := range best {
			baidu.SaveSnapshot(tid, req.Keyword, domain, br.BestRank, br.Occurrences)
		}
		// 附上该词我方近一个月排名趋势摘要
		res.TrendSummary = baidu.TrendSummary(tid, req.Keyword, 30)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": res})
}

// myRankItem 某域名该词当日的统计
type myRankItem struct {
	BestRank    int
	Occurrences int
}

// bestRankPerDomain 从分析结果中统计各客户域名在自然结果中的最佳排名与出现次数
func bestRankPerDomain(res *baidu.Result, myDomains []string) map[string]myRankItem {
	mySet := map[string]bool{}
	for _, d := range myDomains {
		mySet[strings.TrimSpace(d)] = true
	}
	out := map[string]myRankItem{}
	for _, pg := range res.Pages {
		for _, it := range pg.Items {
			if it.Type == "ad" || !mySet[it.Domain] {
				continue
			}
			item := out[it.Domain]
			item.Occurrences++
			if item.BestRank == 0 || (it.Rank > 0 && it.Rank < item.BestRank) {
				item.BestRank = it.Rank
			}
			out[it.Domain] = item
		}
	}
	return out
}

// normalizeDomains 清洗并去重客户域名列表（复用 baidu_monitor.go 的 normalizeDomain）
func normalizeDomains(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range list {
		nd := normalizeDomain(d)
		if nd == "" || seen[nd] {
			continue
		}
		seen[nd] = true
		out = append(out, nd)
	}
	return out
}
