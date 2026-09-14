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
			"serper_enabled": SerperKeyFor(0) != "",
			"serper_masked":  maskToken(SerperKeyFor(0)),
			"serpapi_enabled": SerpAPIKeyFor(0) != "",
			"serpapi_masked":  maskToken(SerpAPIKeyFor(0)),
			"datalab_enabled": DatalabClientIDFor(0) != "" && DatalabClientSecretFor(0) != "",
			"datalab_masked":  maskToken(DatalabClientIDFor(0)),
		},
	})
}

// SaveDataAPIConfig 保存第三方数据 API token：POST /super/data-api/config
func SaveDataAPIConfig(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Serper   string `json:"serper_key"`
		SerpAPI  string `json:"serpapi_key"`
		DLClientID string `json:"datalab_client_id"`
		DLSecret   string `json:"datalab_client_secret"`
	}
	if !jsonBody(c, &req) {
		return
	}
	saveSetting(0, social.TokenKey, req.Token)
	if req.Serper != "" {
		saveSetting(0, keySerperAPIKey, req.Serper)
	}
	if req.SerpAPI != "" {
		saveSetting(0, keySerpAPIKey, req.SerpAPI)
	}
	if req.DLClientID != "" {
		saveSetting(0, keyDatalabClientID, req.DLClientID)
	}
	if req.DLSecret != "" {
		saveSetting(0, keyDatalabClientSecret, req.DLSecret)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存", "data": gin.H{
		"enabled":        req.Token != "",
		"token_masked":   maskToken(req.Token),
		"serper_enabled":  SerperKeyFor(0) != "",
		"serper_masked":   maskToken(SerperKeyFor(0)),
		"serpapi_enabled": SerpAPIKeyFor(0) != "",
		"serpapi_masked":  maskToken(SerpAPIKeyFor(0)),
		"datalab_enabled": DatalabClientIDFor(0) != "" && DatalabClientSecretFor(0) != "",
		"datalab_masked":  maskToken(DatalabClientIDFor(0)),
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
