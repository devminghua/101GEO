package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// 帮助文档（使用教程）：SaaS 后台维护「分类 + 文档」，客户端左目录右详情只读展示。

/* ============ 客户端只读 ============ */

// HelpTree 客户端目录树：GET /api/help/tree
// 返回 [{id,name,docs:[{id,title}]}]，供左侧导航。
func HelpTree(c *gin.Context) {
	var cats []models.HelpCategory
	database.DB.Order("sort_order asc, id asc").Find(&cats)
	var docs []models.HelpDoc
	database.DB.Order("sort_order asc, id asc").Find(&docs)
	docMap := map[uint][]gin.H{}
	for _, d := range docs {
		docMap[d.CategoryID] = append(docMap[d.CategoryID], gin.H{"id": d.ID, "title": d.Title})
	}
	out := make([]gin.H, 0, len(cats))
	for _, c := range cats {
		list := docMap[c.ID]
		if list == nil {
			list = []gin.H{}
		}
		out = append(out, gin.H{"id": c.ID, "name": c.Name, "docs": list})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

// HelpDocDetail 客户端读文档详情：GET /api/help/doc/:id
func HelpDocDetail(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var d models.HelpDoc
	if err := database.DB.First(&d, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "文档不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"id": d.ID, "category_id": d.CategoryID, "title": d.Title, "content": d.Content,
	}})
}

/* ============ SaaS 后台管理 ============ */

// HelpCategories 后台分类列表：GET /super/help/categories
func HelpCategories(c *gin.Context) {
	var cats []models.HelpCategory
	database.DB.Order("sort_order asc, id asc").Find(&cats)
	var docs []models.HelpDoc
	database.DB.Order("sort_order asc, id asc").Find(&docs)
	docMap := map[uint]int{}
	for _, d := range docs {
		docMap[d.CategoryID]++
	}
	type catOut struct {
		models.HelpCategory
		DocCount int `json:"doc_count"`
	}
	out := make([]catOut, 0, len(cats))
	for _, c := range cats {
		out = append(out, catOut{HelpCategory: c, DocCount: docMap[c.ID]})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

// HelpCreateCategory 新增分类：POST /super/help/categories
func HelpCreateCategory(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		SortOrder *int   `json:"sort_order"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "分类名不能为空"})
		return
	}
	sort := 0
	if req.SortOrder != nil {
		sort = *req.SortOrder
	}
	cat := models.HelpCategory{Name: req.Name, SortOrder: sort}
	if err := database.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已新增", "data": cat})
}

// HelpUpdateCategory 改分类：PUT /super/help/categories/:id
func HelpUpdateCategory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cat models.HelpCategory
	if err := database.DB.First(&cat, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分类不存在"})
		return
	}
	var req struct {
		Name      *string `json:"name"`
		SortOrder *int    `json:"sort_order"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Name != nil {
		cat.Name = strings.TrimSpace(*req.Name)
	}
	if req.SortOrder != nil {
		cat.SortOrder = *req.SortOrder
	}
	database.DB.Save(&cat)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
}

// HelpDeleteCategory 删分类（级联删文档）：DELETE /super/help/categories/:id
func HelpDeleteCategory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	tx := database.DB.Begin()
	tx.Where("category_id = ?", id).Delete(&models.HelpDoc{})
	if err := tx.Delete(&models.HelpCategory{}, id).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "删除失败"})
		return
	}
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}

// HelpDocs 后台文档列表：GET /super/help/docs?category_id=
func HelpDocs(c *gin.Context) {
	q := database.DB.Order("sort_order asc, id asc")
	if v := c.Query("category_id"); v != "" {
		if cid, err := strconv.Atoi(v); err == nil {
			q = q.Where("category_id = ?", cid)
		}
	}
	var docs []models.HelpDoc
	q.Find(&docs)
	type docOut struct {
		ID         uint   `json:"id"`
		CategoryID uint   `json:"category_id"`
		Title      string `json:"title"`
		SortOrder  int    `json:"sort_order"`
	}
	out := make([]docOut, 0, len(docs))
	for _, d := range docs {
		out = append(out, docOut{ID: d.ID, CategoryID: d.CategoryID, Title: d.Title, SortOrder: d.SortOrder})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

// HelpDocDetailSuper 后台读文档详情：GET /super/help/doc/:id
func HelpDocDetailSuper(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var d models.HelpDoc
	if err := database.DB.First(&d, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "文档不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": d})
}

// HelpSaveDoc 新增/更新文档：POST /super/help/docs（有 id 则更新）
func HelpSaveDoc(c *gin.Context) {
	var req struct {
		ID         uint   `json:"id"`
		CategoryID uint   `json:"category_id"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		SortOrder  *int   `json:"sort_order"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "标题不能为空"})
		return
	}
	if req.CategoryID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请选择分类"})
		return
	}
	var doc models.HelpDoc
	if req.ID > 0 {
		if err := database.DB.First(&doc, req.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "文档不存在"})
			return
		}
		doc.CategoryID = req.CategoryID
		doc.Title = req.Title
		doc.Content = req.Content
		if req.SortOrder != nil {
			doc.SortOrder = *req.SortOrder
		}
		database.DB.Save(&doc)
	} else {
		sort := 0
		if req.SortOrder != nil {
			sort = *req.SortOrder
		}
		doc = models.HelpDoc{CategoryID: req.CategoryID, Title: req.Title, Content: req.Content, SortOrder: sort}
		database.DB.Create(&doc)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存", "data": doc})
}

// HelpDeleteDoc 删文档：DELETE /super/help/docs/:id
func HelpDeleteDoc(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := database.DB.Delete(&models.HelpDoc{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}
