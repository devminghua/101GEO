package super

// 总后台站内信推送 API：SaaS 端统一推送，支持三种范围。
// POST /api/super/notifications
// body: { target_type: "all"|"tenant"|"user", target_id?: uint, title, content }

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

type notifPushReq struct {
	TargetType string `json:"target_type"`
	TargetID   uint   `json:"target_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}

// NotificationPush SaaS 端统一推送站内信。
func NotificationPush(c *gin.Context) {
	var req notifPushReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "参数格式错误"})
		return
	}
	switch req.TargetType {
	case "all", "tenant", "user":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "target_type 必须为 all/tenant/user"})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "标题不能为空"})
		return
	}
	if req.TargetType == "tenant" && req.TargetID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "指定租户时 target_id 必填"})
		return
	}
	if req.TargetType == "user" && req.TargetID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "指定用户时 target_id 必填"})
		return
	}

	// 推送人：AuthRequired 注入的 ctx_user_id
	createdBy := c.GetUint("ctx_user_id")

	n := models.Notification{
		TargetType: req.TargetType,
		TenantID:   req.TargetID, // 复用 target_id 字段：tenant 时即 tenant.id
		UserID:     req.TargetID, // user 时即 user.id
		Title:      req.Title,
		Content:    req.Content,
		CreatedBy:  createdBy,
	}
	if err := database.DB.Create(&n).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "推送失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "推送成功", "data": gin.H{"id": n.ID}})
}

// NotificationList 总后台查看所有站内信记录 GET /api/super/notifications
func NotificationList(c *gin.Context) {
	var list []models.Notification
	database.DB.Order("id DESC").Limit(100).Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}
