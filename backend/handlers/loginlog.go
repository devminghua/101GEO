package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// ListLoginLogs 登录日志查询（仅 super / admin；AI 优化员无权限，路由已加 AdminOnly 中间件）。
// super 可看全部租户，admin 仅看本租户。支持 ?limit= 数量（默认 100，最大 500）与 ?status=success|fail 过滤。
func ListLoginLogs(c *gin.Context) {
	db := database.DB.Model(&models.LoginLog{})
	if CurrentRole(c) == "admin" {
		db = db.Where("tenant_id = ?", TenantID(c))
	}
	if status := c.Query("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	limit := 100
	if v := c.DefaultQuery("limit", "100"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	var logs []models.LoginLog
	if err := db.Order("id desc").Limit(limit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	if logs == nil {
		logs = []models.LoginLog{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": logs})
}
