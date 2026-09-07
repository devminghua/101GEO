package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// 功能模块 key 全量（与前端 src/features.ts 保持一致）
var FeatureKeys = []string{
	"dashboard", "keywords", "platforms", "tasks", "report",
	"content", "baidu", "douyin", "xhs", "creation", "tools", "geo_intel", "settings",
}

// featureOfPath 根据请求路径前缀判断所属功能模块；不归属任何模块（基础接口）返回 ""
func featureOfPath(path string) string {
	// 基础接口：始终开放
	if strings.HasPrefix(path, "/api/auth/me") ||
		strings.HasPrefix(path, "/api/auth/change-password") ||
		strings.HasPrefix(path, "/api/super") {
		return ""
	}
	prefixMap := []struct {
		prefix  string
		feature string
	}{
		{"/api/dashboard", "dashboard"},
		{"/api/keywords", "keywords"},
		{"/api/platforms", "platforms"},
		{"/api/tasks", "tasks"},
		{"/api/report", "report"},
		{"/api/content", "content"},
		{"/api/baidu", "baidu"},
		{"/api/douyin", "douyin"},
		{"/api/xhs", "xhs"},
		{"/api/creation", "creation"},
		{"/api/tools", "tools"},
		{"/api/geo", "geo_intel"},
		{"/api/facts", "geo_intel"},
		{"/api/competitors", "geo_intel"},
		{"/api/risk-words", "geo_intel"},
		{"/api/citations", "geo_intel"},
		{"/api/settings", "settings"},
		{"/api/operator", "settings"},
		{"/api/system/info", "settings"},
		{"/api/system/upload-logo", "settings"},
		{"/api/auth/login-logs", "settings"},
	}
	for _, m := range prefixMap {
		if strings.HasPrefix(path, m.prefix) {
			return m.feature
		}
	}
	return ""
}

// tenantFeatureEnabled 判断某分站是否授权了某功能；分站不存在或 features 为空时视为全部开放
func tenantFeatureEnabled(tenantID uint, feature string) bool {
	var t models.Tenant
	if err := database.DB.First(&t, tenantID).Error; err != nil {
		return true
	}
	if strings.TrimSpace(t.Features) == "" {
		return true
	}
	var feats []string
	if err := json.Unmarshal([]byte(t.Features), &feats); err != nil {
		return true
	}
	for _, f := range feats {
		if f == feature {
			return true
		}
	}
	return false
}

// parseFeatures 解析分站功能 JSON 数组；空/非法返回 nil
func parseFeatures(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// marshalFeatures 将功能数组编码为 JSON 字符串；空返回 ""
func marshalFeatures(fs []string) string {
	if len(fs) == 0 {
		return ""
	}
	b, _ := json.Marshal(fs)
	return string(b)
}

// FeatureGuard 功能授权中间件：super 角色全放行；分站账号按租户授权校验所属功能模块，未授权返回 403。
// 挂在 AuthRequired 之后。
func FeatureGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentRole(c) == "super" {
			c.Next()
			return
		}
		feature := featureOfPath(c.Request.URL.Path)
		if feature == "" {
			c.Next()
			return
		}
		if tenantFeatureEnabled(TenantID(c), feature) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "该功能未授权，请联系总后台开通"})
		c.Abort()
	}
}
