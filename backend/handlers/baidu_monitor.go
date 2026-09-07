package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/baidu"
)

/* ================================================================
 * 百度关键词分析 · 配置与排名历史 HTTP 处理层
 *
 * 接口（全部按租户 TenantID 隔离，总后台 tid=0 不可见）：
 *  1. 客户网站配置 CRUD：/api/baidu/sites
 *  2. 监控关键词配置 CRUD：/api/baidu/monitor-keywords
 *  3. 排名历史查询：/api/baidu/rank-history?keyword=&days=
 * ================================================================ */

// ---------- 域名归一化 ----------

// normalizeDomain 去除协议 / www / 尾部斜杠等，保留 host 部分用于匹配
func normalizeDomain(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return ""
	}
	d = strings.ToLower(d)
	if i := strings.Index(d, "://"); i >= 0 {
		d = d[i+3:]
	}
	if i := strings.IndexByte(d, '/'); i >= 0 {
		d = d[:i]
	}
	if i := strings.IndexByte(d, '?'); i >= 0 {
		d = d[:i]
	}
	d = strings.TrimPrefix(d, "www.")
	return d
}

// ---------- 1. 客户网站配置 BaiduSite ----------

// BaiduListSites 客户网站列表
func BaiduListSites(c *gin.Context) {
	tid := TenantID(c)
	var list []models.BaiduSite
	database.DB.Where("tenant_id = ?", tid).Order("id asc").Find(&list)
	dyOK(c, list)
}

// BaiduCreateSite 新增客户网站
func BaiduCreateSite(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Domain  string `json:"domain"`
		Name    string `json:"name"`
		Enabled *bool  `json:"enabled"`
	}
	if !jsonBody(c, &req) {
		return
	}
	domain := normalizeDomain(req.Domain)
	if domain == "" {
		dyErr(c, http.StatusBadRequest, "请填写网站域名")
		return
	}
	// 同租户域名去重
	var dup int64
	database.DB.Model(&models.BaiduSite{}).Where("tenant_id = ? AND domain = ?", tid, domain).Count(&dup)
	if dup > 0 {
		dyErr(c, http.StatusBadRequest, "该网站域名已存在")
		return
	}
	site := models.BaiduSite{
		TenantID: tid, Domain: domain,
		Name:    strings.TrimSpace(req.Name),
		Enabled: true,
	}
	if req.Enabled != nil {
		site.Enabled = *req.Enabled
	}
	database.DB.Create(&site)
	dyOK(c, site)
}

// BaiduUpdateSite 更新客户网站（域名/名称/是否启用）
func BaiduUpdateSite(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var req struct {
		Domain  *string `json:"domain"`
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
	}
	if !jsonBody(c, &req) {
		return
	}
	db := database.DB
	var site models.BaiduSite
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&site).Error; err != nil {
		dyErr(c, http.StatusNotFound, "网站不存在")
		return
	}
	if req.Domain != nil {
		nd := normalizeDomain(*req.Domain)
		if nd == "" {
			dyErr(c, http.StatusBadRequest, "请填写网站域名")
			return
		}
		if nd != site.Domain {
			var dup int64
			db.Model(&models.BaiduSite{}).Where("tenant_id = ? AND domain = ? AND id <> ?", tid, nd, id).Count(&dup)
			if dup > 0 {
				dyErr(c, http.StatusBadRequest, "该网站域名已存在")
				return
			}
			site.Domain = nd
			// 同步冗余域名到关联的监控关键词
			db.Model(&models.BaiduMonitorKeyword{}).
				Where("tenant_id = ? AND site_id = ?", tid, id).
				Update("site_domain", nd)
		}
	}
	if req.Name != nil {
		site.Name = strings.TrimSpace(*req.Name)
	}
	if req.Enabled != nil {
		site.Enabled = *req.Enabled
	}
	db.Save(&site)
	dyOK(c, site)
}

// BaiduDeleteSite 删除客户网站（同时删除其关联监控关键词，保留历史快照）
func BaiduDeleteSite(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	res := db.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.BaiduSite{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "网站不存在")
		return
	}
	db.Where("tenant_id = ? AND site_id = ?", tid, id).Delete(&models.BaiduMonitorKeyword{})
	dyOK(c, gin.H{"deleted": true})
}

// ---------- 2. 监控关键词配置 BaiduMonitorKeyword ----------

// BaiduListMonitorKeywords 监控关键词列表（可选 site_id 过滤）
func BaiduListMonitorKeywords(c *gin.Context) {
	tid := TenantID(c)
	var list []models.BaiduMonitorKeyword
	q := database.DB.Where("tenant_id = ?", tid)
	if sid := queryUint(c, "site_id"); sid > 0 {
		q = q.Where("site_id = ?", sid)
	}
	q.Order("id asc").Find(&list)
	// 关联网站名称（冗余展示）
	siteNames := map[uint]string{}
	var sites []models.BaiduSite
	database.DB.Where("tenant_id = ?", tid).Find(&sites)
	for _, s := range sites {
		siteNames[s.ID] = s.Name
	}
	type kwItem struct {
		models.BaiduMonitorKeyword
		SiteName string `json:"site_name"`
	}
	out := make([]kwItem, 0, len(list))
	for _, k := range list {
		out = append(out, kwItem{BaiduMonitorKeyword: k, SiteName: siteNames[k.SiteID]})
	}
	dyOK(c, out)
}

// BaiduCreateMonitorKeyword 新增监控关键词（可关联网站）
func BaiduCreateMonitorKeyword(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Keyword string `json:"keyword"`
		SiteID  uint   `json:"site_id"`
		Enabled *bool  `json:"enabled"`
	}
	if !jsonBody(c, &req) {
		return
	}
	kw := strings.TrimSpace(req.Keyword)
	if kw == "" {
		dyErr(c, http.StatusBadRequest, "请填写要监控的关键词")
		return
	}
	domain := ""
	if req.SiteID > 0 {
		var site models.BaiduSite
		if err := database.DB.Where("id = ? AND tenant_id = ?", req.SiteID, tid).First(&site).Error; err != nil {
			dyErr(c, http.StatusBadRequest, "关联网站不存在")
			return
		}
		domain = site.Domain
	}
	// 同租户同词同站去重
	var dup int64
	database.DB.Model(&models.BaiduMonitorKeyword{}).
		Where("tenant_id = ? AND keyword = ? AND site_id = ?", tid, kw, req.SiteID).Count(&dup)
	if dup > 0 {
		dyErr(c, http.StatusBadRequest, "该关键词已存在（同网站下不能重复）")
		return
	}
	item := models.BaiduMonitorKeyword{
		TenantID: tid, Keyword: kw, SiteID: req.SiteID, SiteDomain: domain, Enabled: true,
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	database.DB.Create(&item)
	dyOK(c, item)
}

// BaiduUpdateMonitorKeyword 更新监控关键词
func BaiduUpdateMonitorKeyword(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var req struct {
		Keyword *string `json:"keyword"`
		SiteID  *uint   `json:"site_id"`
		Enabled *bool   `json:"enabled"`
	}
	if !jsonBody(c, &req) {
		return
	}
	db := database.DB
	var item models.BaiduMonitorKeyword
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&item).Error; err != nil {
		dyErr(c, http.StatusNotFound, "监控关键词不存在")
		return
	}
	if req.Keyword != nil {
		k := strings.TrimSpace(*req.Keyword)
		if k == "" {
			dyErr(c, http.StatusBadRequest, "请填写要监控的关键词")
			return
		}
		item.Keyword = k
	}
	if req.SiteID != nil {
		sid := *req.SiteID
		if sid > 0 {
			var site models.BaiduSite
			if err := db.Where("id = ? AND tenant_id = ?", sid, tid).First(&site).Error; err != nil {
				dyErr(c, http.StatusBadRequest, "关联网站不存在")
				return
			}
			item.SiteID = sid
			item.SiteDomain = site.Domain
		} else {
			item.SiteID = 0
			item.SiteDomain = ""
		}
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	// 同租户同词同站去重（排除自身）
	var dup int64
	db.Model(&models.BaiduMonitorKeyword{}).
		Where("tenant_id = ? AND keyword = ? AND site_id = ? AND id <> ?", tid, item.Keyword, item.SiteID, id).Count(&dup)
	if dup > 0 {
		dyErr(c, http.StatusBadRequest, "该关键词已存在（同网站下不能重复）")
		return
	}
	db.Save(&item)
	dyOK(c, item)
}

// BaiduDeleteMonitorKeyword 删除监控关键词
func BaiduDeleteMonitorKeyword(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.BaiduMonitorKeyword{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "监控关键词不存在")
		return
	}
	dyOK(c, gin.H{"deleted": true})
}

// ---------- 3. 排名历史查询 BaiduRankSnapshot ----------

// BaiduRankHistory 查询某关键词近 N 天按日聚合的排名序列（供前端趋势折线）
func BaiduRankHistory(c *gin.Context) {
	tid := TenantID(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		dyErr(c, http.StatusBadRequest, "请填写要查询的关键词")
		return
	}
	days := 30
	if v := c.Query("days"); v != "" {
		d := queryUint(c, "days")
		if d > 0 {
			days = int(d)
		}
	}
	points := baidu.QueryRankHistory(tid, keyword, days)
	dyOK(c, gin.H{
		"keyword": keyword,
		"days":    days,
		"points":  points,
	})
}
