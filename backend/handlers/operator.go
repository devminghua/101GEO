package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/crypto"
)

// tenantNameOf 取分站名称（用于列表展示）
func tenantNameOf(id uint) string {
	var t models.Tenant
	if database.DB.First(&t, id).Error == nil {
		return t.Name
	}
	return ""
}

// ListOperators 列出 AI 优化员账号（super 看全部租户，admin 只看本租户）
func ListOperators(c *gin.Context) {
	db := database.DB.Model(&models.User{}).Where("role = ?", "operator")
	if CurrentRole(c) == "admin" {
		db = db.Where("tenant_id = ?", TenantID(c))
	}
	var users []models.User
	if err := db.Order("id asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	type row struct {
		models.User
		TenantName string `json:"tenant_name"`
		RemainDays int    `json:"remain_days"`
	}
	rows := make([]row, 0, len(users))
	for _, u := range users {
		rows = append(rows, row{User: u, TenantName: tenantNameOf(u.TenantID), RemainDays: remainDaysOf(&u)})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}

type createOperatorReq struct {
	TenantID uint   `json:"tenant_id"` // super 指定；admin 忽略（固定本租户）
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

// CreateOperator 添加 AI 优化员（受限角色 operator）。
// super 可指定任意租户；分站 admin 只能在本租户下添加。
func CreateOperator(c *gin.Context) {
	var req createOperatorReq
	if !jsonBody(c, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "账号与密码不能为空"})
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": msg})
		return
	}
	role := CurrentRole(c)
	tenantID := req.TenantID
	switch role {
	case "admin":
		// 分站 admin 只能给本租户添加优化员
		tenantID = TenantID(c)
	case "super":
		if tenantID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请选择所属租户"})
			return
		}
		var t models.Tenant
		if database.DB.First(&t, tenantID).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在"})
			return
		}
	default:
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无添加权限"})
		return
	}

	var count int64
	database.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "账号已存在"})
		return
	}
	encPwd, err := crypto.Hash(req.Password, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密码加密失败"})
		return
	}
	u := models.User{
		TenantID: tenantID, Username: req.Username, Password: encPwd,
		Nickname: req.Nickname, Role: "operator", Status: 1,
	}
	if err := database.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败：" + err.Error()})
		return
	}
	u.Password = ""
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "AI 优化员账号已创建", "data": u})
}

// DeleteOperator 删除 AI 优化员（super 任意租户；admin 仅本租户）
func DeleteOperator(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u models.User
	if err := database.DB.First(&u, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "账号不存在"})
		return
	}
	if u.Role != "operator" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "仅可删除 AI 优化员账号"})
		return
	}
	if CurrentRole(c) == "admin" && u.TenantID != TenantID(c) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "只能删除本租户下的 AI 优化员账号"})
		return
	}
	database.DB.Delete(&u)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "AI 优化员账号已删除"})
}
