package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// ListKeywords 关键词列表（当前租户）
func ListKeywords(c *gin.Context) {
	tid := TenantID(c)
	var list []models.GeoKeyword
	database.DB.Where("tenant_id = ?", tid).Order("id desc").Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// GetCategories 分类列表
func GetCategories(c *gin.Context) {
	tid := TenantID(c)
	var cats []string
	database.DB.Model(&models.GeoKeyword{}).
		Where("tenant_id = ? AND category <> ''", tid).
		Distinct("category").Order("category").Pluck("category", &cats)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cats})
}

// isHan 判断是否为汉字（用于短词边界判断）
func isHan(r rune) bool {
	return r >= 0x4e00 && r <= 0x9fff
}

// matchBannedWord 违禁词匹配（精度优化）：
// 长词（>=3 字）用包含匹配；短词（1-2 字）用词边界匹配（前后不能是汉字），
// 避免「第一」误伤「第一时间」、「唯一」误伤「唯一性」等。
func matchBannedWord(word, text string) bool {
	if word == "" || text == "" {
		return false
	}
	if utf8.RuneCountInString(word) >= 3 {
		return strings.Contains(text, word)
	}
	tr := []rune(text)
	wr := []rune(word)
	// 短词（1-2 字）：后边界匹配 —— 词后面不是汉字即命中。
	// 既能拦截「服务第一」「行业唯一」等词尾用法，又不误伤「第一时间」「唯一性」等复合词。
	for i := 0; i+len(wr) <= len(tr); i++ {
		if string(tr[i:i+len(wr)]) != word {
			continue
		}
		after := rune(0)
		if i+len(wr) < len(tr) {
			after = tr[i+len(wr)]
		}
		if !isHan(after) {
			return true
		}
	}
	return false
}

// checkBannedWord 检查文本是否命中违禁词（全局 tenant_id=0 内置词库 + 分站自定义），
// 命中返回 (违禁词, 风险说明, true)
func checkBannedWord(tid uint, text string) (string, string, bool) {
	if strings.TrimSpace(text) == "" {
		return "", "", false
	}
	var words []models.RiskWord
	database.DB.Where("tenant_id IN ? AND enabled = ?", []uint{0, tid}, true).Find(&words)
	for _, w := range words {
		if matchBannedWord(w.Word, text) {
			return w.Word, w.Reason, true
		}
	}
	return "", "", false
}

type keywordReq struct {
	Question      string `json:"question"`
	BrandKeywords string `json:"brand_keywords"`
	Category      string `json:"category"`
	Enabled       *bool  `json:"enabled"`
}

// CreateKeyword 新增关键词
func CreateKeyword(c *gin.Context) {
	tid := TenantID(c)
	var req keywordReq
	if !jsonBody(c, &req) {
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请输入问题内容"})
		return
	}
	// 违禁词校验：关键词或品牌词命中违禁词则拒绝提交
	if w, reason, hit := checkBannedWord(tid, req.Question+" "+req.BrandKeywords); hit {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "包含违禁词「" + w + "」(" + reason + ")，请修改后重试"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	k := models.GeoKeyword{
		TenantID: tid, Question: req.Question, BrandKeywords: strings.TrimSpace(req.BrandKeywords),
		Category: req.Category, Enabled: enabled,
	}
	if err := database.DB.Create(&k).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已新增", "data": k})
}

// BulkCreateKeywords 批量新增关键词（前端提交 questions 数组 + 统一品牌词/分类）
func BulkCreateKeywords(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Questions     []string `json:"questions"`
		BrandKeywords string   `json:"brand_keywords"`
		Category      string   `json:"category"`
		Enabled       *bool    `json:"enabled"`
	}
	if !jsonBody(c, &body) {
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	// 违禁词校验：任一关键词命中违禁词则整体拒绝
	for _, q := range body.Questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if w, reason, hit := checkBannedWord(tid, q+" "+body.BrandKeywords); hit {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "「" + q + "」包含违禁词「" + w + "」(" + reason + ")，请修改后重试"})
			return
		}
	}
	created := 0
	skipped := 0
	for _, q := range body.Questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		var count int64
		database.DB.Model(&models.GeoKeyword{}).
			Where("tenant_id = ? AND question = ?", tid, q).Count(&count)
		if count > 0 {
			skipped++
			continue
		}
		if err := database.DB.Create(&models.GeoKeyword{
			TenantID: tid, Question: q, BrandKeywords: strings.TrimSpace(body.BrandKeywords),
			Category: body.Category, Enabled: enabled,
		}).Error; err == nil {
			created++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{"created": created, "skipped": skipped},
		"msg":  "新增 " + strconv.Itoa(created) + " 条，跳过重复 " + strconv.Itoa(skipped) + " 条",
	})
}

// UpdateKeyword 更新关键词
func UpdateKeyword(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var k models.GeoKeyword
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&k).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "关键词不存在"})
		return
	}
	var req struct {
		Question      *string `json:"question"`
		BrandKeywords *string `json:"brand_keywords"`
		Category      *string `json:"category"`
		Enabled       *bool   `json:"enabled"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Question != nil {
		k.Question = strings.TrimSpace(*req.Question)
	}
	if req.BrandKeywords != nil {
		k.BrandKeywords = strings.TrimSpace(*req.BrandKeywords)
	}
	if req.Category != nil {
		k.Category = *req.Category
	}
	if req.Enabled != nil {
		k.Enabled = *req.Enabled
	}
	database.DB.Save(&k)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
}

// DeleteKeyword 删除关键词
func DeleteKeyword(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	result := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.GeoKeyword{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "关键词不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}

// BatchDeleteKeywords 批量删除关键词（仅删本租户的，防止越权）
func BatchDeleteKeywords(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		IDs []uint `json:"ids"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请选择要删除的关键词"})
		return
	}
	result := database.DB.Where("tenant_id = ? AND id IN ?", tid, body.IDs).Delete(&models.GeoKeyword{})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除 " + strconv.Itoa(int(result.RowsAffected)) + " 条"})
}
