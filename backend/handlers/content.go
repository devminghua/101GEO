package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	svccontent "geo-tool/services/content"
)

/* ================================================================
 * 内容投放 · HTTP 处理层（结构对齐 抖音/小红书/创作中心 模块）
 *
 * 合规硬约束（全链路体现）：
 *  - 不模拟登录任何媒体后台、不自动群发；
 *  - "自动发布"仅当租户配置发稿平台 API 时生效，未配置转 wait_manual；
 *  - 百度收录检测尽力而为，失败降级 estimate（sourced_from 严格区分）。
 *
 * 所有查询均按当前登录租户 TenantID 隔离，总后台（tid=0）不可见。
 * ================================================================ */

// ---------- 1) 媒体库 ----------

// ListContentMedia 媒体库列表（首次进入自动写入内置预设）
func ListContentMedia(c *gin.Context) {
	tid := TenantID(c)
	svccontent.EnsureBuiltin(tid)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("category"); v != "" {
		q = q.Where("category = ?", v)
	}
	if v := c.Query("level"); v != "" {
		q = q.Where("level = ?", v)
	}
	if v := c.Query("keyword"); v != "" {
		q = q.Where("name LIKE ?", "%"+v+"%")
	}
	var list []models.CtnMedia
	q.Order("source_type ASC, id ASC").Find(&list)
	dyOK(c, list)
}

// CreateContentMedia 新增自定义媒体
func CreateContentMedia(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Level    string `json:"level"`
		URL      string `json:"url"`
		Remark   string `json:"remark"`
	}
	if !jsonBody(c, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		dyErr(c, http.StatusBadRequest, "媒体名称不能为空")
		return
	}
	if body.Category == "" {
		body.Category = "自媒体"
	}
	if body.Level == "" {
		body.Level = "中"
	}
	rec := models.CtnMedia{
		TenantID: tid, Name: body.Name, Category: body.Category, Level: body.Level,
		URL: body.URL, SourceType: "custom", Status: 1, Remark: body.Remark,
	}
	if err := database.DB.Create(&rec).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	dyOK(c, rec)
}

// UpdateContentMedia 更新媒体
func UpdateContentMedia(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var body struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Level    string `json:"level"`
		URL      string `json:"url"`
		Remark   string `json:"remark"`
		Status   *int   `json:"status"`
	}
	if !jsonBody(c, &body) {
		return
	}
	var rec models.CtnMedia
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&rec).Error; err != nil {
		dyErr(c, http.StatusNotFound, "媒体不存在")
		return
	}
	updates := map[string]interface{}{}
	if body.Name != "" {
		updates["name"] = body.Name
	}
	if body.Category != "" {
		updates["category"] = body.Category
	}
	if body.Level != "" {
		updates["level"] = body.Level
	}
	if body.URL != "" {
		updates["url"] = body.URL
	}
	updates["remark"] = body.Remark
	if body.Status != nil {
		updates["status"] = *body.Status
	}
	if err := database.DB.Model(&rec).Updates(updates).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&rec)
	dyOK(c, rec)
}

// DeleteContentMedia 删除媒体
func DeleteContentMedia(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.CtnMedia{}).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	dyOK(c, gin.H{"id": id})
}

// ---------- 2) 软文生成（AI 写软文） ----------

// GenerateContentArticle AI 生成软文并入库
func GenerateContentArticle(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Topic         string `json:"topic"`
		BrandKeywords string `json:"brand_keywords"`
		Category      string `json:"category"`
		Length        int    `json:"length"`
		MediaType     string `json:"media_type"`
	}
	if !jsonBody(c, &body) {
		return
	}
	body.Topic = strings.TrimSpace(body.Topic)
	if body.Topic == "" {
		dyErr(c, http.StatusBadRequest, "请填写投放主题")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()
	article, err := svccontent.GenerateArticle(ctx, tid, svccontent.GenReq{
		Topic: body.Topic, BrandKeywords: body.BrandKeywords,
		Category: body.Category, Length: body.Length, MediaType: body.MediaType,
	})
	if err != nil {
		dyErr(c, http.StatusInternalServerError, err.Error())
		return
	}
	dyOK(c, article)
}

// GenerateContentArticlesBatch 批量生成软文（每行一个主题，逐篇调用 AI，部分失败不影响其它）
func GenerateContentArticlesBatch(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Topics        []string `json:"topics"`
		BrandKeywords string   `json:"brand_keywords"`
		Category      string   `json:"category"`
		Length        int      `json:"length"`
		MediaType     string   `json:"media_type"`
	}
	if !jsonBody(c, &body) {
		return
	}
	var topics []string
	for _, t := range body.Topics {
		if s := strings.TrimSpace(t); s != "" {
			topics = append(topics, s)
		}
	}
	if len(topics) == 0 {
		dyErr(c, http.StatusBadRequest, "请至少输入一个主题")
		return
	}
	var created []models.CtnArticle
	var failed []gin.H
	for _, t := range topics {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
		art, err := svccontent.GenerateArticle(ctx, tid, svccontent.GenReq{
			Topic: t, BrandKeywords: body.BrandKeywords,
			Category: body.Category, Length: body.Length, MediaType: body.MediaType,
		})
		cancel()
		if err != nil {
			failed = append(failed, gin.H{"topic": t, "error": err.Error()})
			continue
		}
		created = append(created, *art)
	}
	dyOK(c, gin.H{
		"created":       created,
		"failed":        failed,
		"created_count": len(created),
		"failed_count":  len(failed),
	})
}

// CreateContentArticle 手动录入软文
func CreateContentArticle(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Title         string `json:"title"`
		Content       string `json:"content"`
		BrandKeywords string `json:"brand_keywords"`
		Topic         string `json:"topic"`
		Category      string `json:"category"`
		Status        string `json:"status"`
	}
	if !jsonBody(c, &body) {
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Content = strings.TrimSpace(body.Content)
	if body.Title == "" || body.Content == "" {
		dyErr(c, http.StatusBadRequest, "请填写标题与正文")
		return
	}
	rec := models.CtnArticle{
		TenantID: tid, Title: body.Title, Content: body.Content,
		BrandKeywords: body.BrandKeywords, Topic: body.Topic,
		Category: body.Category, Length: len([]rune(body.Content)),
		Status: "draft",
	}
	if body.Status != "" {
		rec.Status = body.Status
	}
	if err := database.DB.Create(&rec).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	dyOK(c, rec)
}

// ListContentArticles 软文库列表
func ListContentArticles(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("keyword"); v != "" {
		like := "%" + v + "%"
		q = q.Where("title LIKE ? OR topic LIKE ? OR content LIKE ?", like, like, like)
	}
	var list []models.CtnArticle
	q.Order("id DESC").Find(&list)
	dyOK(c, list)
}

// UpdateContentArticle 编辑软文（标题/正文/状态）
func UpdateContentArticle(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Status  string `json:"status"`
	}
	if !jsonBody(c, &body) {
		return
	}
	var rec models.CtnArticle
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&rec).Error; err != nil {
		dyErr(c, http.StatusNotFound, "软文不存在")
		return
	}
	updates := map[string]interface{}{}
	if strings.TrimSpace(body.Title) != "" {
		updates["title"] = strings.TrimSpace(body.Title)
	}
	if strings.TrimSpace(body.Content) != "" {
		updates["content"] = body.Content
		updates["length"] = len([]rune(body.Content))
	}
	if body.Status != "" {
		updates["status"] = body.Status
	}
	if err := database.DB.Model(&rec).Updates(updates).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&rec)
	dyOK(c, rec)
}

// DeleteContentArticle 删除软文
func DeleteContentArticle(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.CtnArticle{}).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	dyOK(c, gin.H{"id": id})
}

// ---------- 3) 发布任务 ----------

// CreateContentTask 创建发布任务
func CreateContentTask(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		ArticleID   uint       `json:"article_id"`
		MediaID     uint       `json:"media_id"`
		PublishMode string     `json:"publish_mode"`
		ScheduledAt *time.Time `json:"scheduled_at"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if body.ArticleID == 0 || body.MediaID == 0 {
		dyErr(c, http.StatusBadRequest, "请选择软文与媒体")
		return
	}
	var art models.CtnArticle
	if err := database.DB.Where("id = ? AND tenant_id = ?", body.ArticleID, tid).First(&art).Error; err != nil {
		dyErr(c, http.StatusNotFound, "软文不存在")
		return
	}
	var media models.CtnMedia
	if err := database.DB.Where("id = ? AND tenant_id = ?", body.MediaID, tid).First(&media).Error; err != nil {
		dyErr(c, http.StatusNotFound, "媒体不存在")
		return
	}
	mode := strings.TrimSpace(body.PublishMode)
	if mode != "auto" && mode != "manual" {
		mode = "auto"
	}
	task := models.CtnTask{
		TenantID: tid, ArticleID: art.ID, ArticleTitle: art.Title,
		MediaID: media.ID, MediaName: media.Name,
		PublishMode: mode, Status: models.CtnTaskPending,
		ScheduledAt: body.ScheduledAt,
		SyncState:   models.CtnSyncUnchecked,
	}
	if art.Status != "published" {
		database.DB.Model(&art).Update("status", "ready")
	}
	if err := database.DB.Create(&task).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	dyOK(c, task)
}

// CreateContentTasksBatch 一键批量发布：多篇软文 × 多个媒体，批量创建发布任务
func CreateContentTasksBatch(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		ArticleIDs  []uint `json:"article_ids"`
		MediaIDs    []uint `json:"media_ids"`
		PublishMode string `json:"publish_mode"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if len(body.ArticleIDs) == 0 || len(body.MediaIDs) == 0 {
		dyErr(c, http.StatusBadRequest, "请选择软文与媒体")
		return
	}
	mode := strings.TrimSpace(body.PublishMode)
	if mode != "auto" && mode != "manual" {
		mode = "auto"
	}
	created := 0
	for _, aid := range body.ArticleIDs {
		var art models.CtnArticle
		if err := database.DB.Where("id = ? AND tenant_id = ?", aid, tid).First(&art).Error; err != nil {
			continue
		}
		for _, mid := range body.MediaIDs {
			var media models.CtnMedia
			if err := database.DB.Where("id = ? AND tenant_id = ?", mid, tid).First(&media).Error; err != nil {
				continue
			}
			task := models.CtnTask{
				TenantID: tid, ArticleID: art.ID, ArticleTitle: art.Title,
				MediaID: media.ID, MediaName: media.Name,
				PublishMode: mode, Status: models.CtnTaskPending,
				SyncState: models.CtnSyncUnchecked,
			}
			if err := database.DB.Create(&task).Error; err == nil {
				created++
			}
		}
		if art.Status != "published" {
			database.DB.Model(&art).Update("status", "ready")
		}
	}
	dyOK(c, gin.H{"created": created})
}

// UpdateContentTask 修改发布任务（仅待发布状态可改）
func UpdateContentTask(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var body struct {
		PublishMode string     `json:"publish_mode"`
		ScheduledAt *time.Time `json:"scheduled_at"`
	}
	if !jsonBody(c, &body) {
		return
	}
	var task models.CtnTask
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&task).Error; err != nil {
		dyErr(c, http.StatusNotFound, "任务不存在")
		return
	}
	if task.Status != models.CtnTaskPending {
		dyErr(c, http.StatusBadRequest, "仅待发布任务可修改")
		return
	}
	updates := map[string]interface{}{}
	if body.PublishMode == "auto" || body.PublishMode == "manual" {
		updates["publish_mode"] = body.PublishMode
	}
	if body.ScheduledAt != nil {
		updates["scheduled_at"] = body.ScheduledAt
	}
	if len(updates) > 0 {
		if err := database.DB.Model(&task).Updates(updates).Error; err != nil {
			dyErr(c, http.StatusInternalServerError, "更新失败: "+err.Error())
			return
		}
	}
	database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&task)
	dyOK(c, task)
}

// ListContentTasks 发布任务列表
func ListContentTasks(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("media_id"); v != "" {
		q = q.Where("media_id = ?", v)
	}
	var list []models.CtnTask
	q.Order("id DESC").Find(&list)
	dyOK(c, list)
}

// PublishContentTask 发布任务（自动对接或转待人工）
func PublishContentTask(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	if err := svccontent.SubmitTask(ctx, tid, id); err != nil {
		dyErr(c, http.StatusNotFound, "任务不存在")
		return
	}
	var task models.CtnTask
	database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&task)
	dyOK(c, task)
}

// FinishContentTask 人工发布完成回填（半自动：用户已在媒体后台发布后回填链接）
func FinishContentTask(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var body struct {
		PublishURL    string `json:"publish_url"`
		PublishResult string `json:"publish_result"`
	}
	if !jsonBody(c, &body) {
		return
	}
	var task models.CtnTask
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&task).Error; err != nil {
		dyErr(c, http.StatusNotFound, "任务不存在")
		return
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status": models.CtnTaskPublished, "published_at": &now,
		"publish_url": strings.TrimSpace(body.PublishURL),
	}
	if strings.TrimSpace(body.PublishResult) != "" {
		updates["publish_result"] = body.PublishResult
	} else {
		updates["publish_result"] = "人工发布完成"
	}
	if err := database.DB.Model(&task).Updates(updates).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&task)
	dyOK(c, task)
}

// CheckContentTask 对任务执行百度收录检测（含人工发布回填后的自动检测）
func CheckContentTask(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	mon, err := svccontent.CheckTask(ctx, tid, id)
	if err != nil {
		dyErr(c, http.StatusNotFound, "任务不存在")
		return
	}
	var task models.CtnTask
	database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&task)
	dyOK(c, gin.H{"monitor": mon, "task": task})
}

// DeleteContentTask 删除发布任务
func DeleteContentTask(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.CtnTask{}).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	database.DB.Where("tenant_id = ? AND task_id = ?", tid, id).Delete(&models.CtnMonitor{})
	dyOK(c, gin.H{"id": id})
}

// ---------- 4) 监控记录 ----------

// ListContentMonitors 监控记录列表
func ListContentMonitors(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("task_id"); v != "" {
		q = q.Where("task_id = ?", v)
	}
	var list []models.CtnMonitor
	q.Order("id DESC").Find(&list)
	dyOK(c, list)
}

// ---------- 5) 发稿平台配置 ----------

// GetContentPublishConfig 读取发稿平台配置（Key 脱敏）
func GetContentPublishConfig(c *gin.Context) {
	tid := TenantID(c)
	dyOK(c, svccontent.GetPublishConfig(tid))
}

// SaveContentPublishConfig 保存发稿平台配置
func SaveContentPublishConfig(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Enabled bool   `json:"enabled"`
		API     string `json:"api"`
		Key     string `json:"key"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if err := svccontent.SavePublishConfig(tid, svccontent.PublishConfig{
		Enabled: body.Enabled, API: body.API,
	}, body.Key); err != nil {
		dyErr(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	dyOK(c, svccontent.GetPublishConfig(tid))
}

// ---------- 6) 效果分析 ----------

// ContentAnalysis 效果分析
func ContentAnalysis(c *gin.Context) {
	tid := TenantID(c)
	resp, err := svccontent.Analysis(tid)
	if err != nil {
		dyErr(c, http.StatusInternalServerError, "分析失败: "+err.Error())
		return
	}
	dyOK(c, resp)
}
