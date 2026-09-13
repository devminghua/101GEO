package handlers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/services/baidu"
)

// BaiduIndexCount 收录查询：POST /api/baidu/index-count
// 输入域名，抓取百度 site:domain 首页，返回：
//   - count 收录总数（解析「百度为您找到相关结果约 N 个」，抓不到时用首页条目数近似）
//   - items 首页收录条目（标题 / URL / 摘要）
// 复用 baidu.FetchPage + baidu.ParseSERP（口径唯一，不另写抓取实现）。
func BaiduIndexCount(c *gin.Context) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !jsonBody(c, &req) {
		return
	}
	domain := normalizeDomain(strings.TrimSpace(req.Domain))
	if domain == "" {
		dyErr(c, http.StatusBadRequest, "请填写要查询的网站域名")
		return
	}
	// 每日查询配额：与关键词分析共用同一配额
	if !QuotaGuard(c) {
		return
	}
	// site: 查询固定抓第 1 页（收录总数在首屏，抓更多页只会增加反爬风险）
	html, err := baidu.FetchPage("site:"+domain, 1)
	if err != nil {
		dyErr(c, http.StatusBadGateway, "抓取失败（百度反爬或网络波动），请稍后重试")
		return
	}
	// 复用统一解析器
	pageRes := baidu.ParseSERP(html, "site:"+domain)
	items := make([]gin.H, 0, len(pageRes.Items))
	for _, it := range pageRes.Items {
		items = append(items, gin.H{
			"title":   it.Title,
			"url":     it.URL,
			"domain":  it.Domain,
			"snippet": it.Snippet,
		})
	}
	dyOK(c, gin.H{
		"domain":      domain,
		"count":       parseIndexTotal(html, len(items)),
		"count_exact": parseIndexTotal(html, len(items)) > 0,
		"items":       items,
		"page_count":  len(items),
	})
}

// indexTotalRe 匹配百度 SERP 的「百度为您找到相关结果约 12,345 个」
var indexTotalRe = regexp.MustCompile(`相关结果约?[约\s]*([\d,]+)\s*个`)

// parseIndexTotal 从 HTML 中解析收录总数；解析失败返回 0（调用方用首页条数近似）。
func parseIndexTotal(html string, fallback int) int {
	m := indexTotalRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return fallback
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
