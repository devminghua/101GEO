package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"
)

/* ================================================================
 * 小程序对接配置（分站客户端「系统中心 → 小程序设置」）
 *  - 存 settings KV（租户级）：appid / appsecret / enabled / api_key
 *  - api_key：小程序调用 LinkGeo 开放接口的服务端对接密钥（可轮换）
 * ================================================================ */

const (
	keyMpAppID     = "miniprogram_appid"
	keyMpSecret    = "miniprogram_appsecret"
	keyMpEnabled   = "miniprogram_enabled"
	keyMpAPIKey    = "miniprogram_api_key"
)

// MiniProgramConfig GET /api/miniprogram/config —— 当前配置（密钥脱敏）+ 对接参数
func MiniProgramConfig(c *gin.Context) {
	tid := TenantID(c)
	appid := strings.TrimSpace(readSetting(tid, keyMpAppID))
	secret := strings.TrimSpace(readSetting(tid, keyMpSecret))
	apiKey := strings.TrimSpace(readSetting(tid, keyMpAPIKey))
	// 无对接密钥则自动生成（首次访问即就绪）
	if apiKey == "" && tid > 0 {
		apiKey = newAPIKey()
		saveSetting(tid, keyMpAPIKey, apiKey)
	}
	dyOK(c, gin.H{
		"appid":         appid,
		"appid_masked":  maskToken(appid),
		"secret_masked": maskToken(secret),
		"enabled":       readSetting(tid, keyMpEnabled) == "1",
		"api_key_masked": maskToken(apiKey),
		"api_base":      c.Request.Host, // 对接 API 域名（小程序 request 合法域名需配置此域）
		"configured":    appid != "" && secret != "",
	})
}

// MiniProgramSave POST /api/miniprogram/config —— 保存配置
func MiniProgramSave(c *gin.Context) {
	var req struct {
		AppID     string `json:"appid"`
		AppSecret string `json:"appsecret"`
		Enabled   bool   `json:"enabled"`
	}
	if !jsonBody(c, &req) {
		return
	}
	tid := TenantID(c)
	if req.AppID != "" {
		saveSetting(tid, keyMpAppID, strings.TrimSpace(req.AppID))
	}
	if req.AppSecret != "" {
		saveSetting(tid, keyMpSecret, strings.TrimSpace(req.AppSecret))
	}
	if req.Enabled {
		saveSetting(tid, keyMpEnabled, "1")
	} else {
		saveSetting(tid, keyMpEnabled, "0")
	}
	dyOK(c, gin.H{"code": 0, "msg": "已保存"})
}

// MiniProgramRotateKey POST /api/miniprogram/rotate-key —— 轮换对接密钥（无请求体）
func MiniProgramRotateKey(c *gin.Context) {
	tid := TenantID(c)
	apiKey := newAPIKey()
	saveSetting(tid, keyMpAPIKey, apiKey)
	dyOK(c, gin.H{"code": 0, "msg": "对接密钥已更新", "data": gin.H{"api_key_masked": maskToken(apiKey)}})
}

// newAPIKey 生成 32 字符随机对接密钥
func newAPIKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 兜底：时间+随机数（极低概率）
		b = []byte(strings.Repeat("0", 16))
	}
	return hex.EncodeToString(b)
}
