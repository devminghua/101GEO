package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

/* ================================================================
 * 成功案例：SaaS 端（super）上传/编辑/删除，客户端（分站）展示。
 * 客户端只读平台级已发布案例（tenant_id=0 且 status=1），
 * 不开放分站自建（TenantID 字段已预留）。
 * ================================================================ */

// CaseList GET /api/cases —— 客户端：平台级已发布案例列表（摘要字段，不返回全文）
func CaseList(c *gin.Context) {
	var list []models.CaseStudy
	database.DB.Where("tenant_id = 0 AND status = 1").
		Order("sort_order desc, id desc").Find(&list)
	if list == nil {
		list = []models.CaseStudy{}
	}
	type item struct {
		ID       uint   `json:"id"`
		Title    string `json:"title"`
		Summary  string `json:"summary"`
		CoverURL string `json:"cover_url"`
		Tags     string `json:"tags"`
	}
	out := make([]item, 0, len(list))
	for _, c := range list {
		out = append(out, item{ID: c.ID, Title: c.Title, Summary: c.Summary, CoverURL: c.CoverURL, Tags: c.Tags})
	}
	dyOK(c, out)
}

// CaseDetail GET /api/cases/:id —— 客户端：案例详情（含正文）
func CaseDetail(c *gin.Context) {
	id := parseID(c)
	var cs models.CaseStudy
	if err := database.DB.Where("id = ? AND tenant_id = 0 AND status = 1", id).First(&cs).Error; err != nil {
		dyErr(c, http.StatusNotFound, "案例不存在或未发布")
		return
	}
	dyOK(c, cs)
}

// ---------- SaaS 端管理 ----------

// SuperCaseList GET /api/super/cases —— 全部案例（含草稿）
func SuperCaseList(c *gin.Context) {
	var list []models.CaseStudy
	database.DB.Where("tenant_id = 0").Order("sort_order desc, id desc").Find(&list)
	if list == nil {
		list = []models.CaseStudy{}
	}
	dyOK(c, list)
}

// SuperCaseSave POST /api/super/cases —— 新建（PUT 走 SuperCaseUpdate）
func SuperCaseSave(c *gin.Context) {
	var req struct {
		Title     string `json:"title"`
		Summary   string `json:"summary"`
		Content   string `json:"content"`
		CoverURL  string `json:"cover_url"`
		Tags      string `json:"tags"`
		Status    int    `json:"status"`
		SortOrder int    `json:"sort_order"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		dyErr(c, http.StatusBadRequest, "请填写案例标题")
		return
	}
	if len([]rune(req.Title)) > 128 {
		dyErr(c, http.StatusBadRequest, "标题过长（最多 128 字）")
		return
	}
	cs := models.CaseStudy{
		TenantID: 0, Title: req.Title,
		Summary: strings.TrimSpace(req.Summary), Content: req.Content,
		CoverURL: strings.TrimSpace(req.CoverURL), Tags: strings.TrimSpace(req.Tags),
		Status: req.Status, SortOrder: req.SortOrder,
	}
	if cs.Status != 0 {
		cs.Status = 1
	}
	if err := database.DB.Create(&cs).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "保存失败："+err.Error())
		return
	}
	dyOK(c, cs)
}

// SuperCaseUpdate PUT /api/super/cases/:id —— 编辑
func SuperCaseUpdate(c *gin.Context) {
	id := parseID(c)
	var cs models.CaseStudy
	if err := database.DB.Where("id = ? AND tenant_id = 0", id).First(&cs).Error; err != nil {
		dyErr(c, http.StatusNotFound, "案例不存在")
		return
	}
	var req struct {
		Title     string `json:"title"`
		Summary   string `json:"summary"`
		Content   string `json:"content"`
		CoverURL  string `json:"cover_url"`
		Tags      string `json:"tags"`
		Status    int    `json:"status"`
		SortOrder int    `json:"sort_order"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		dyErr(c, http.StatusBadRequest, "请填写案例标题")
		return
	}
	updates := map[string]interface{}{
		"title":      req.Title,
		"summary":    strings.TrimSpace(req.Summary),
		"content":    req.Content,
		"cover_url":  strings.TrimSpace(req.CoverURL),
		"tags":       strings.TrimSpace(req.Tags),
		"sort_order": req.SortOrder,
	}
	updates["status"] = 1
	if req.Status == 0 {
		updates["status"] = 0
	}
	database.DB.Model(&cs).Updates(updates)
	dyOK(c, cs)
}

// SuperCaseDelete DELETE /api/super/cases/:id
func SuperCaseDelete(c *gin.Context) {
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = 0", id).Delete(&models.CaseStudy{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "案例不存在")
		return
	}
	dyOK(c, gin.H{"deleted": id})
}
