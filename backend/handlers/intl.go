package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/services/baidu"
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
