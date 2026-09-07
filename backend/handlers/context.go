package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/auth"
)

const (
	ctxUserID   = "ctx_user_id"
	ctxTenantID = "ctx_tenant_id"
	ctxRole     = "ctx_role"
)

// AuthRequired 登录鉴权中间件：解析 Bearer Token 并注入用户上下文
func AuthRequired(c *gin.Context) {
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if token == "" || token == c.GetHeader("Authorization") {
		abort(c, "未登录或凭证缺失")
		return
	}
	claims, err := auth.Parse(token)
	if err != nil {
		abort(c, "凭证无效或已过期")
		return
	}
	if claims.Role == "" {
		claims.Role = "admin"
	}
	// 学员 token 仅用于 C 端小程序接口，禁止访问后台
	if claims.Role == "learner" {
		abort(c, "无后台访问权限")
		return
	}
	c.Set(ctxUserID, claims.UserID)
	c.Set(ctxTenantID, claims.TenantID)
	c.Set(ctxRole, claims.Role)
	// 在线心跳：每个请求顺带记录租户最后活跃时间（供总后台在线统计）
	onlineTracker.Touch(claims.TenantID)
	c.Next()
}

// SuperRequired 总后台权限中间件：仅 super 角色可访问
func SuperRequired(c *gin.Context) {
	if c.GetString(ctxRole) != "super" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无总后台访问权限"})
		c.Abort()
		return
	}
	c.Next()
}

// CurrentUserID 当前登录用户 ID
func CurrentUserID(c *gin.Context) uint {
	return c.GetUint(ctxUserID)
}

// TenantID 当前登录用户所属租户 ID（总后台为 0）
func TenantID(c *gin.Context) uint {
	return c.GetUint(ctxTenantID)
}

// BrandOf 返回当前租户的默认品牌词：
// 优先取分站设置 settings.default_brand（可在「系统设置」页修改），回退到全局 GEO_DEFAULT_BRAND。
func BrandOf(c *gin.Context) string {
	tid := TenantID(c)
	if tid > 0 {
		var s models.Setting
		if err := database.DB.Where("tenant_id = ? AND key = ?", tid, "default_brand").First(&s).Error; err == nil {
			if v := strings.TrimSpace(s.Value); v != "" {
				return v
			}
		}
	}
	return config.Load().DefaultBrand
}

// CurrentRole 当前角色
func CurrentRole(c *gin.Context) string {
	return c.GetString(ctxRole)
}

// jsonBody 读取请求体，失败返回 false 并写好 400
func jsonBody(c *gin.Context, v interface{}) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "参数错误：" + err.Error()})
		return false
	}
	return true
}

func abort(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, gin.H{"code": 1, "msg": msg})
	c.Abort()
}
