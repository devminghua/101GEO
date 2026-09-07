package handlers

// 客户端站内信 API：列表 / 未读数 / 标已读 / 全部已读。
// 范围匹配：target_type IN ('all', 'tenant:tenant_id=current', 'user:user_id=current')。
// 已读状态用 Notification.ReadUserIDs (JSON 数组字符串)，简化场景适用：单租户 < 1000 用户。

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// notifScope 当前用户应当看到的站内信 ID 列表（按范围筛选）。
// 返回的是「未过滤已读」的全量，由 caller 用 readUserIDs 二次过滤。
func notifScope(u models.User) []models.Notification {
	var list []models.Notification
	q := database.DB.Model(&models.Notification{}).
		Where("target_type IN ?", []string{"all"})
	if u.Role != "super" {
		// 非 super 客户端：all + 自己租户 + 自己 user
		q = q.Or("target_type = ? AND tenant_id = ?", "tenant", u.TenantID).
			Or("target_type = ? AND user_id = ?", "user", u.ID)
	}
	q.Order("id DESC").Limit(200).Find(&list)
	return list
}

// parseReadUserIDs 解析 ReadUserIDs JSON 数组。空字段返 nil。
func parseReadUserIDs(s string) map[uint]bool {
	out := map[uint]bool{}
	if s == "" {
		return out
	}
	var arr []uint
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return out
	}
	for _, id := range arr {
		out[id] = true
	}
	return out
}

// markRead 把 userID 加入 ReadUserIDs（去重）。返回新 JSON 字符串。
func markRead(s string, userID uint) string {
	set := parseReadUserIDs(s)
	set[userID] = true
	arr := make([]uint, 0, len(set))
	for id := range set {
		arr = append(arr, id)
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

// notifDTO 列表返回结构（带 read 字段）
type notifDTO struct {
	models.Notification
	Read bool `json:"read"`
}

// NotificationList 拉取当前用户站内信列表 GET /api/notifications?limit=20
func NotificationList(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		return
	}
	limit := 20
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	list := notifScope(u)
	// 倒序截 limit
	if len(list) > limit {
		list = list[:limit]
	}
	out := make([]notifDTO, 0, len(list))
	for _, n := range list {
		read := parseReadUserIDs(n.ReadUserIDs)[u.ID]
		out = append(out, notifDTO{Notification: n, Read: read})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

// NotificationUnreadCount GET /api/notifications/unread_count
func NotificationUnreadCount(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		return
	}
	list := notifScope(u)
	cnt := 0
	for _, n := range list {
		if !parseReadUserIDs(n.ReadUserIDs)[u.ID] {
			cnt++
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"unread": cnt}})
}

// NotificationRead POST /api/notifications/:id/read
func NotificationRead(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "id 非法"})
		return
	}
	var n models.Notification
	if err := database.DB.First(&n, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "通知不存在"})
		return
	}
	// 范围校验：只允许范围内通知标已读
	if !inScope(n, u) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作"})
		return
	}
	database.DB.Model(&models.Notification{}).Where("id = ?", id).
		Update("read_user_ids", markRead(n.ReadUserIDs, u.ID))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// NotificationReadAll POST /api/notifications/read_all
func NotificationReadAll(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		return
	}
	list := notifScope(u)
	for _, n := range list {
		if !parseReadUserIDs(n.ReadUserIDs)[u.ID] {
			database.DB.Model(&models.Notification{}).Where("id = ?", n.ID).
				Update("read_user_ids", markRead(n.ReadUserIDs, u.ID))
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// inScope 判断通知是否在当前用户范围
func inScope(n models.Notification, u models.User) bool {
	switch n.TargetType {
	case "all":
		return true
	case "tenant":
		return n.TenantID == u.TenantID
	case "user":
		return n.UserID == u.ID
	}
	return false
}

// currentUser 从 context 取 user（AuthRequired 注入 ctx_user_id / ctx_role）
func currentUser(c *gin.Context) (models.User, bool) {
	uid := CurrentUserID(c)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 1, "msg": "未登录"})
		return models.User{}, false
	}
	var u models.User
	if err := database.DB.First(&u, uid).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 1, "msg": "用户不存在"})
		return models.User{}, false
	}
	// 中间件已注入角色，但以库中记录为准（避免角色变更后越权）
	if r := CurrentRole(c); r != "" {
		u.Role = r
	}
	return u, true
}
