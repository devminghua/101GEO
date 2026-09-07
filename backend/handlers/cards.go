package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/card"
	"geo-tool/services/crypto"
	"geo-tool/services/points"
)

const (
	settingLicensePrivKey = "license_private_key"
	settingLicensePubKey  = "license_public_key"
)

// loadOrCreateKeyPair 读取总后台卡密私钥（enc:v1 加密存储）；不存在则生成新密钥对并持久化。
func loadOrCreateKeyPair() (ed25519.PrivateKey, error) {
	secret := config.Load().PayloadSecret()
	if enc := getGlobalSetting(settingLicensePrivKey); enc != "" {
		if raw, err := crypto.Decrypt(enc, secret); err == nil {
			if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == ed25519.PrivateKeySize {
				return ed25519.PrivateKey(b), nil
			}
		}
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	privB64 := base64.StdEncoding.EncodeToString(priv)
	encPriv, err := crypto.Encrypt(privB64, secret)
	if err != nil {
		return nil, err
	}
	upsertGlobalSetting(settingLicensePrivKey, encPriv)
	upsertGlobalSetting(settingLicensePubKey, base64.StdEncoding.EncodeToString(pub))
	return priv, nil
}

// upsertGlobalSetting 写入全局设置（tenant_id=0）。
func upsertGlobalSetting(key, value string) {
	var s models.Setting
	if err := database.DB.Where("tenant_id = ? AND key = ?", 0, key).First(&s).Error; err == nil {
		database.DB.Model(&s).Update("value", value)
		return
	}
	database.DB.Create(&models.Setting{TenantID: 0, Key: key, Value: value})
}

// GetLicenseKey 返回公钥（供打包客户端写入 GEO_LICENSE_PUBLIC_KEY）与是否已有私钥。
func GetLicenseKey(c *gin.Context) {
	pub := getGlobalSetting(settingLicensePubKey)
	hasPriv := getGlobalSetting(settingLicensePrivKey) != ""
	if pub == "" {
		if _, err := loadOrCreateKeyPair(); err == nil {
			pub = getGlobalSetting(settingLicensePubKey)
			hasPriv = true
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"public_key":  pub,
		"has_private": hasPriv,
	}})
}

// RegenerateLicenseKey 重新生成密钥对（会使旧卡密全部失效，需前端二次确认）。
func RegenerateLicenseKey(c *gin.Context) {	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "生成密钥失败"})
		return
	}
	secret := config.Load().PayloadSecret()
	encPriv, err := crypto.Encrypt(base64.StdEncoding.EncodeToString(priv), secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密钥加密失败"})
		return
	}
	upsertGlobalSetting(settingLicensePrivKey, encPriv)
	upsertGlobalSetting(settingLicensePubKey, base64.StdEncoding.EncodeToString(pub))
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"public_key": base64.StdEncoding.EncodeToString(pub)}})
}

// ImportLicenseKey 导入私钥（用于让总后台与已打包客户端配对：打包时生成的私钥导入后，
// 总后台生成的卡密才能被客户端验签通过）。私钥用 GEO_SECRET_KEY 加密后存储。
func ImportLicenseKey(c *gin.Context) {
	var req struct {
		PrivateKey string `json:"private_key"`
	}
	if !jsonBody(c, &req) {
		return
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.PrivateKey))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "私钥格式错误"})
		return
	}
	priv := ed25519.PrivateKey(raw)
	pub := priv.Public().(ed25519.PublicKey)
	encPriv, err := crypto.Encrypt(base64.StdEncoding.EncodeToString(priv), config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "私钥加密失败"})
		return
	}
	upsertGlobalSetting(settingLicensePrivKey, encPriv)
	upsertGlobalSetting(settingLicensePubKey, base64.StdEncoding.EncodeToString(pub))
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"public_key": base64.StdEncoding.EncodeToString(pub)}})
}

// GenerateCards 批量生成卡密（每张 card_id = 记录自增 ID）。
func GenerateCards(c *gin.Context) {
	var req struct {
		Tier   byte   `json:"tier"`
		Count  int    `json:"count"`
		Points int64  `json:"points"` // 充值卡密(tier=3)兑换的 token 数
		Remark string `json:"remark"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Count <= 0 || req.Count > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "数量需在 1-1000 之间"})
		return
	}
	if req.Tier == card.TierRecharge {
		if req.Points <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "充值卡密需指定兑换 token 数"})
			return
		}
	} else if card.TierDays(req.Tier) <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "卡密类型无效"})
		return
	}
	priv, err := loadOrCreateKeyPair()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密钥初始化失败"})
		return
	}
	codes := make([]string, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		rec := models.Card{Tier: req.Tier, Points: req.Points, Remark: req.Remark, Status: 0}
		if err := database.DB.Create(&rec).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "生成卡密失败"})
			return
		}
		code, err := card.Generate(priv, uint32(rec.ID), req.Tier)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "生成卡密失败"})
			return
		}
		if err := database.DB.Model(&rec).Update("code", code).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "生成卡密失败"})
			return
		}
		codes = append(codes, code)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"count":     len(codes),
		"tier":      req.Tier,
		"tier_name": card.TierName(req.Tier),
		"points":    req.Points,
		"codes":     codes,
	}})
}

// ListCards 卡密列表（倒序，最多 500 条）。
func ListCards(c *gin.Context) {
	var cards []models.Card
	database.DB.Order("id desc").Limit(500).Find(&cards)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cards})
}

// RedeemCard 分站用充值卡密兑换 token：POST /api/card/redeem
// 输入卡密 → 公钥验签 → 查 Card 表 → 事务入账 points → 标记卡密已用（防并发重复兑换）。
func RedeemCard(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Code string `json:"code"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请输入卡密"})
		return
	}
	pub := getGlobalSetting(settingLicensePubKey)
	if pub == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "卡密系统未初始化"})
		return
	}
	pubKey, err := base64.StdEncoding.DecodeString(pub)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "卡密系统配置错误"})
		return
	}
	cardID, tier, err := card.Verify(ed25519.PublicKey(pubKey), req.Code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	if tier != card.TierRecharge {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该卡密不是充值卡密，无法兑换 token"})
		return
	}
	var cRec models.Card
	if err := database.DB.First(&cRec, cardID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "卡密不存在"})
		return
	}
	if cRec.Status != 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该卡密已被使用"})
		return
	}
	if cRec.Points <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "卡密配置异常，请联系总后台"})
		return
	}
	// 事务：入账 + 标记卡密已用；事务内重新校验 status，防并发重复兑换
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var rec models.Card
		if err := tx.First(&rec, cardID).Error; err != nil {
			return err
		}
		if rec.Status != 0 {
			return errors.New("used")
		}
		if err := points.RechargeTx(tx, tid, cRec.Points, "卡密兑换 token"); err != nil {
			return err
		}
		return tx.Model(&rec).Update("status", 1).Error
	})
	if err != nil {
		if err.Error() == "used" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该卡密已被使用"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "兑换失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "兑换成功", "data": gin.H{"points": cRec.Points}})
}
