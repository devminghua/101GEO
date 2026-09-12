package handlers

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
	"geo-tool/services/card"
	"geo-tool/services/crypto"
)

// licenseEnabled 卡密授权是否启用（GEO_LICENSE_MODE=on）。
func licenseEnabled() bool {
	return config.Load().LicenseMode
}

// licensePublicKey 解析配置中的卡密验签公钥。
func licensePublicKey() (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(config.Load().LicensePubKey)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(raw), nil
}

// signActivation 对授权关键数据做 HMAC-SHA256 签名（密钥派生自 PayloadSecret），
// 用于校验 Activation 记录是否被直接改库篡改（改 expire_at 等字段会导致签名不匹配）。
func signActivation(cardID uint32, tier byte, activatedAt, expireAt time.Time) string {
	data := fmt.Sprintf("%d|%d|%d|%d", cardID, tier, activatedAt.Unix(), expireAt.Unix())
	mac := hmac.New(sha256.New, config.Load().PayloadSecret())
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// activationValid 校验 Activation 记录的签名，防改库篡改。
func activationValid(a *models.Activation) bool {
	return a.Sig == signActivation(a.CardID, a.Tier, a.ActivatedAt, a.ExpireAt)
}

// GetLicenseStatus 公开接口：查询软件激活状态（前端激活页使用）。
func GetLicenseStatus(c *gin.Context) {
	if !licenseEnabled() {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"license_mode": false, "activated": true}})
		return
	}
	var a models.Activation
	if err := database.DB.Order("id desc").First(&a).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"license_mode": true, "activated": false}})
		return
	}
	now := time.Now()
	expired := a.ExpireAt.Before(now)
	remainDays := 0
	if !expired {
		remainDays = int(a.ExpireAt.Sub(now).Hours()/24) + 1
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"license_mode": true,
		"activated":    true,
		"tier":         a.Tier,
		"tier_name":    card.TierName(a.Tier),
		"activated_at": a.ActivatedAt.Format("2006-01-02"),
		"expire_at":    a.ExpireAt.Format("2006-01-02"),
		"remain_days":  remainDays,
		"expired":      expired,
	}})
}

// ActivateLicense 公开接口：输入卡密激活软件，激活成功后随机化 super 密码并返回。
func ActivateLicense(c *gin.Context) {
	if !licenseEnabled() {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "当前版本无需激活"})
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请输入卡密"})
		return
	}
	pub, err := licensePublicKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "授权配置错误"})
		return
	}
	cardID, tier, err := card.Verify(pub, req.Code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	days := card.TierDays(tier)
	if days <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "卡密类型无效"})
		return
	}
	// 防重复激活：同一卡密 ID 只能用一次
	var dup models.Activation
	if database.DB.Where("card_id = ?", cardID).First(&dup).Error == nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该卡密已被使用"})
		return
	}

	now := time.Now()
	expireAt := now.AddDate(0, 0, days)
	a := models.Activation{CardID: cardID, Tier: tier, ActivatedAt: now, ExpireAt: expireAt}
	a.Sig = signActivation(cardID, tier, now, expireAt)
	if err := database.DB.Create(&a).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "激活失败，请稍后重试"})
		return
	}

	// super 密码随机化：首次激活后不再使用默认密码，避免「随包下发公开密码」。
	newPwd := randomPassword(12)
	if enc, err := crypto.Hash(newPwd, config.Load().PayloadSecret()); err == nil {
		database.DB.Model(&models.User{}).Where("role = ?", "super").Update("password", enc)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "激活成功", "data": gin.H{
		"tier_name":      card.TierName(tier),
		"expire_at":      expireAt.Format("2006-01-02"),
		"super_username": "admin",
		"super_password": newPwd,
	}})
}

// LicenseGuard 软件级授权守卫：启用卡密授权时，未激活或已到期则拒绝业务接口。
// 挂在 AuthRequired 之后（先鉴权，再校验软件激活状态）。
func LicenseGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !licenseEnabled() {
			c.Next()
			return
		}
		var a models.Activation
		if err := database.DB.Order("id desc").First(&a).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "软件未激活，请输入卡密激活后使用"})
			c.Abort()
			return
		}
		if !activationValid(&a) {
			c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "授权数据异常，请重新激活"})
			c.Abort()
			return
		}
		if a.ExpireAt.Before(time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "软件已到期，请联系续费激活"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// randomPassword 生成不含易混字符的随机密码。
func randomPassword(n int) string {
	const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 熵源异常时退回时间戳兜底，避免返回空密码
		return "lg" + biztime.DateCompact()
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}
