package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/crypto"
)

// remainDaysOf 计算账号剩余可用天数。
// 返回：-1=不限，0=已到期，>0=剩余天数。
func remainDaysOf(u *models.User) int {
	if u.ExpireAt == nil {
		return -1
	}
	now := time.Now()
	if u.ExpireAt.Before(now) {
		return 0
	}
	return int(u.ExpireAt.Sub(now).Hours()/24) + 1
}

// userInfoMap 组装登录/当前用户返回信息（含有效期展示字段）
func userInfoMap(u *models.User) gin.H {
	info := gin.H{
		"id": u.ID, "username": u.Username, "nickname": u.Nickname,
		"avatar": u.Avatar,
		"tenant_id": u.TenantID, "role": u.Role,
		"open_months": u.OpenMonths,
		"remain_days": remainDaysOf(u),
	}
	if u.ExpireAt != nil {
		info["expire_at"] = u.ExpireAt.Format("2006-01-02")
	} else {
		info["expire_at"] = nil
	}
	if u.TenantID > 0 {
		var t models.Tenant
		if database.DB.First(&t, u.TenantID).Error == nil {
			info["tenant_name"] = t.Name
			info["tenant_code"] = t.Code
			info["tenant_logo"] = t.Logo
			info["features"] = parseFeatures(t.Features)
			// 分站品牌名（自定义 system_name，分站级优先全局，空则走默认 LinkGeo）
			info["brand_name"] = readSetting(u.TenantID, KeySystemName)
		}
	}
	return info
}

// AuthExpiry 公开接口：登录前按账号查询剩余有效期天数（供登录框旁展示）。
// 仅返回客户（分站）账号信息；总后台账号 / 不存在的账号统一返回 null，避免泄露。
func AuthExpiry(c *gin.Context) {
	username := strings.TrimSpace(c.Query("username"))
	if username == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": nil})
		return
	}
	var u models.User
	if err := database.DB.Where("username = ?", username).First(&u).Error; err != nil || u.TenantID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"username":    u.Username,
		"expire_at":   func() any { if u.ExpireAt == nil { return nil }; return u.ExpireAt.Format("2006-01-02") }(),
		"remain_days": remainDaysOf(&u),
		"open_months": u.OpenMonths,
	}})
}

// ChangePassword 当前登录用户修改自己的密码。
// AI 优化员（operator）为受限账号，禁止自助修改后台密码（设置后台密码属管理员操作）。
func ChangePassword(c *gin.Context) {
	if isOperator(c) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "AI 优化员账号不允许修改后台密码，请联系管理员"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if msg := validatePassword(req.NewPassword); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "新" + msg})
		return
	}
	var u models.User
	if err := database.DB.First(&u, CurrentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "账号不存在"})
		return
	}
	if !crypto.Verify(u.Password, req.OldPassword, config.Load().PayloadSecret()) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "原密码不正确"})
		return
	}
	enc, err := crypto.Hash(req.NewPassword, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密码加密失败"})
		return
	}
	database.DB.Model(&u).Update("password", enc)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "密码已修改，请使用新密码重新登录"})
}
