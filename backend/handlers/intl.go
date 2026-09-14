package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/services/baidu"
	"geo-tool/services/biztime"
	"geo-tool/services/datalab"
	"geo-tool/services/serp"
)

/* ================================================================
 * 国际搜索优化（Google/Naver）—— P1：Google 关键词分析
 *  - 与百度共用一条分析管线（baidu.RunKeywordEngine），引擎按参数路由
 *  - 配额：与百度共用每日查询配额池（QuotaGuard 同口径）
 *  - Serper Key：env GEO_SERPER_KEY 优先，其次 settings（serper_api_key）
 * ================================================================ */

const keySerperAPIKey = "serper_api_key"
const keySerpAPIKey = "serpapi_key"
const keyDatalabClientID = "datalab_client_id"
const keyDatalabClientSecret = "datalab_client_secret"

// SerperKey 读取 Serper API Key（env 优先，其次总后台 settings）
func SerperKey() string {
	if v := strings.TrimSpace(os.Getenv("GEO_SERPER_KEY")); v != "" {
		return v
	}
	return strings.TrimSpace(readSetting(0, keySerperAPIKey))
}

// SerpAPIKey 读取 SerpAPI Key（Naver SERP 数据源）
func SerpAPIKey() string {
	if v := strings.TrimSpace(os.Getenv("GEO_SERPAPI_KEY")); v != "" {
		return v
	}
	return strings.TrimSpace(readSetting(0, keySerpAPIKey))
}

// DatalabClientID / DatalabClientSecret Naver Datalab 凭据（行业排行数据源）
func DatalabClientID() string {
	if v := strings.TrimSpace(os.Getenv("GEO_DATALAB_CLIENT_ID")); v != "" {
		return v
	}
	return strings.TrimSpace(readSetting(0, keyDatalabClientID))
}

func DatalabClientSecret() string {
	if v := strings.TrimSpace(os.Getenv("GEO_DATALAB_CLIENT_SECRET")); v != "" {
		return v
	}
	return strings.TrimSpace(readSetting(0, keyDatalabClientSecret))
}

// AnalyzeIntlKeyword POST /api/intl/analyze —— 国际关键词分析（engine=google）
func AnalyzeIntlKeyword(c *gin.Context) {
	var req struct {
		Keyword   string   `json:"keyword"`
		Engine    string   `json:"engine"` // google（P1）；naver 后续
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
	switch req.Engine {
	case "google", "naver":
	case "":
		req.Engine = "google"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "暂不支持该搜索引擎（当前支持 google / naver）"})
		return
	}
	// API Key 前置校验：未配置直接报错（先校验依赖 → 再扣费铁律）
	if req.Engine == "naver" {
		if SerpAPIKey() == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "未配置 SerpAPI Key，请联系服务商（总后台「数据 API」页配置）"})
			return
		}
	} else if SerperKey() == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "未配置 Serper API Key，请联系服务商（总后台「数据 API」页配置）"})
		return
	}
	// 每日查询配额（与百度同一配额池，口径一致）
	if !QuotaGuard(c) {
		return
	}
	// 客户域名归一化（与百度同口径）
	myDomains := normalizeDomains(append(req.MyDomains, req.MyDomain))
	cfg := baidu.TaskConfig{
		Keyword:    req.Keyword,
		MyDomain:   strings.TrimSpace(req.MyDomain),
		MyDomains:  myDomains,
		Depth:      3, // 与百度一致：固定 3 页，防风控
	}
	var res *baidu.Result
	var err error
	if req.Engine == "naver" {
		res, err = serp.RunNaver(cfg, SerpAPIKey())
	} else {
		res, err = serp.RunGoogle(cfg, SerperKey())
	}
	if err != nil && res == nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "分析失败：" + err.Error()})
		return
	}
	// 排名快照：国际关键词同词同域同日去重写入快照表（复用百度口径，engine 维度暂并入）
	tid := TenantID(c)
	if tid > 0 {
		best := bestRankPerDomain(res, myDomains)
		for domain, br := range best {
			baidu.SaveSnapshotEngine(tid, req.Keyword, domain, req.Engine, br.BestRank, br.Occurrences)
		}
		res.TrendSummary = baidu.TrendSummary(tid, req.Keyword, 30)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": res})
}

// IntlIndexCount POST /api/intl/index-count —— 国际收录查询（site: 走 SERP 引擎）
func IntlIndexCount(c *gin.Context) {
	var req struct {
		Domain string `json:"domain"`
		Engine string `json:"engine"`
	}
	if !jsonBody(c, &req) {
		return
	}
	domain := normalizeDomain(strings.TrimSpace(req.Domain))
	if domain == "" {
		dyErr(c, http.StatusBadRequest, "请填写要查询的网站域名")
		return
	}
	switch req.Engine {
	case "google", "naver":
	default:
		dyErr(c, http.StatusBadRequest, "暂不支持该搜索引擎（当前支持 google / naver）")
		return
	}
	// 先校验依赖 → 再扣费
	if req.Engine == "naver" {
		if SerpAPIKey() == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "未配置 SerpAPI Key，请联系服务商（总后台「数据 API」页配置）"})
			return
		}
	} else if SerperKey() == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "未配置 Serper API Key，请联系服务商（总后台「数据 API」页配置）"})
		return
	}
	if !QuotaGuard(c) {
		return
	}
	var res *serp.IndexResult
	var err error
	if req.Engine == "naver" {
		res, err = serp.IndexCountNaver(domain, SerpAPIKey())
	} else {
		res, err = serp.IndexCountGoogle(domain, SerperKey())
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "收录查询失败：" + err.Error()})
		return
	}
	dyOK(c, res)
}

// IntlTrends POST /api/intl/trends —— 国际行业排行/趋势（naver→Datalab 官方，google→SerpAPI Trends）
func IntlTrends(c *gin.Context) {
	var req struct {
		Engine   string   `json:"engine"`
		Keywords []string `json:"keywords"`
		Range    string   `json:"range"` // 1m/3m/6m/12m（naver 仅支持日期区间，google 用 date 参数）
	}
	if !jsonBody(c, &req) {
		return
	}
	// 关键词去重、去空、截断（最多 5 组各 1 词；P4 首版每组 1 词，后续可扩展组内多词）
	seen := map[string]bool{}
	kws := make([]string, 0, 5)
	for _, k := range req.Keywords {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		kws = append(kws, k)
		if len(kws) >= 5 {
			break
		}
	}
	if len(kws) == 0 {
		dyErr(c, http.StatusBadRequest, "请至少填写一个关键词")
		return
	}
	// 依赖前置校验（先校验 → 再扣费）
	if req.Engine == "naver" {
		if DatalabClientID() == "" || DatalabClientSecret() == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "未配置 Naver Datalab 凭据（总后台「数据 API」页配置）"})
			return
		}
	} else if req.Engine == "google" {
		if SerpAPIKey() == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "未配置 SerpAPI Key（Google Trends 数据源，总后台「数据 API」页配置）"})
			return
		}
	} else {
		dyErr(c, http.StatusBadRequest, "暂不支持该搜索引擎（当前支持 google / naver）")
		return
	}
	if !QuotaGuard(c) {
		return
	}

	if req.Engine == "naver" {
		// 近 N 月日期区间（北京时间）
		months := 3
		switch req.Range {
		case "1m": months = 1
		case "6m": months = 6
		case "12m": months = 12
		default: months = 3
		}
		end := biztime.Today()
		start := biztime.Day(-months * 30)
		groups := make([]struct {
			Name     string   `json:"name"`
			Keywords []string `json:"keywords"`
		}, 0, len(kws))
		for _, k := range kws {
			groups = append(groups, struct {
				Name     string   `json:"name"`
				Keywords []string `json:"keywords"`
			}{Name: k, Keywords: []string{k}})
		}
		series, err := datalab.Query(DatalabClientID(), DatalabClientSecret(), start, end, "date", groups)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "趋势查询失败：" + err.Error()})
			return
		}
		dyOK(c, gin.H{"engine": "naver", "series": series, "source": "naver_datalab"})
		return
	}
	// google：SerpAPI google_trends
	date := "today 3-m"
	switch req.Range {
	case "1m": date = "today 1-m"
	case "6m": date = "today 6-m"
	case "12m": date = "today 12-m"
	}
	series, err := serp.TrendsGoogle(SerpAPIKey(), date, kws)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "趋势查询失败：" + err.Error()})
		return
	}
	dyOK(c, gin.H{"engine": "google", "series": series, "source": "google_trends"})
}

// IntlDataSourceStatus GET /api/intl/data-source-status —— 分站可读的数据源配置状态（不含任何密钥）
func IntlDataSourceStatus(c *gin.Context) {
	dyOK(c, gin.H{
		"serper_enabled":  SerperKey() != "",
		"serper_masked":   maskToken(SerperKey()),
		"serpapi_enabled": SerpAPIKey() != "",
		"serpapi_masked":  maskToken(SerpAPIKey()),
		"datalab_enabled": DatalabClientID() != "" && DatalabClientSecret() != "",
		"datalab_masked":  maskToken(DatalabClientID()),
	})
}
