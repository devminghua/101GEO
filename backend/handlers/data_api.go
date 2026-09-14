package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"geo-tool/services/social"
)

// DataAPIConfig 读取第三方数据 API 配置：GET /super/data-api/config
// 返回 token 是否已配置（脱敏，不回传明文）。
func DataAPIConfig(c *gin.Context) {
	token := social.Token()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"enabled":       token != "",
			"token_masked":  maskToken(token),
			"base_url":      social.BaseURL,
			"serper_enabled": SerperKey() != "",
			"serper_masked":  maskToken(SerperKey()),
		},
	})
}

// SaveDataAPIConfig 保存第三方数据 API token：POST /super/data-api/config
func SaveDataAPIConfig(c *gin.Context) {
	var req struct {
		Token  string `json:"token"`
		Serper string `json:"serper_key"`
	}
	if !jsonBody(c, &req) {
		return
	}
	saveSetting(0, social.TokenKey, req.Token)
	if req.Serper != "" {
		saveSetting(0, keySerperAPIKey, req.Serper)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存", "data": gin.H{
		"enabled":       req.Token != "",
		"token_masked":  maskToken(req.Token),
		"serper_enabled": SerperKey() != "",
		"serper_masked":  maskToken(SerperKey()),
	}})
}

// maskToken 脱敏展示 token（保留前 6 后 4）。
func maskToken(t string) string {
	if t == "" {
		return ""
	}
	if len(t) <= 10 {
		return "****"
	}
	return t[:6] + "****" + t[len(t)-4:]
}
