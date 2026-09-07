package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 短信 / 注册配置存储键（全局设置 tenant_id=0，SaaS 端统一配置，全部分站注册共用）。
const (
	settingSmsRequired = "register_sms_required" // "0"=关闭短信验证直接注册；"1"=需短信验证码（默认）
	settingSmsProvider = "sms_provider"          // mock / aliyun
	settingSmsAKID     = "sms_access_key_id"
	settingSmsAKSecret = "sms_access_key_secret"
	settingSmsSignName = "sms_sign_name"
	settingSmsTemplate = "sms_template_code"
)

// smsRequired 注册是否要求短信验证码（默认需要；仅当显式配置为 "0" 时关闭）。
func smsRequired() bool {
	return getGlobalSetting(settingSmsRequired) != "0"
}

// RegisterConfig 公开接口：返回自助注册开关与短信验证开关（前端注册页据此动态渲染）。
func RegisterConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"register_enabled": getGlobalSetting("register_enabled") != "0",
		"sms_required":     smsRequired(),
	}})
}

// GetSmsConfig 读取短信配置（super；密钥脱敏回显）。
func GetSmsConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"sms_required":      smsRequired(),
		"provider":          getGlobalSetting(settingSmsProvider),
		"access_key_id":     getGlobalSetting(settingSmsAKID),
		"access_key_secret": maskSecret(getGlobalSetting(settingSmsAKSecret)),
		"sign_name":         getGlobalSetting(settingSmsSignName),
		"template_code":     getGlobalSetting(settingSmsTemplate),
	}})
}

// SaveSmsConfig 保存短信配置（super）。access_key_secret 留空或含 * 表示不修改（脱敏回显场景）。
func SaveSmsConfig(c *gin.Context) {
	var req struct {
		SmsRequired     bool   `json:"sms_required"`
		Provider        string `json:"provider"`
		AccessKeyID     string `json:"access_key_id"`
		AccessKeySecret string `json:"access_key_secret"`
		SignName        string `json:"sign_name"`
		TemplateCode    string `json:"template_code"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.SmsRequired {
		upsertGlobalSetting(settingSmsRequired, "1")
	} else {
		upsertGlobalSetting(settingSmsRequired, "0")
	}
	upsertGlobalSetting(settingSmsProvider, strings.TrimSpace(req.Provider))
	upsertGlobalSetting(settingSmsAKID, strings.TrimSpace(req.AccessKeyID))
	secret := strings.TrimSpace(req.AccessKeySecret)
	if secret != "" && !strings.Contains(secret, "*") {
		upsertGlobalSetting(settingSmsAKSecret, secret)
	}
	upsertGlobalSetting(settingSmsSignName, strings.TrimSpace(req.SignName))
	upsertGlobalSetting(settingSmsTemplate, strings.TrimSpace(req.TemplateCode))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存"})
}

// maskSecret 密钥脱敏：dfb747ea...8263 → dfb7****8263。
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
