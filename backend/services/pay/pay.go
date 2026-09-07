// Package pay 提供统一的第三方扫码支付抽象（微信 Native / 支付宝当面付）。
// 分站「点卡中心」自助扫码充值通过本包下单，支付回调由 services/recharge 幂等入账。
package pay

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/crypto"
)

// 支付渠道标识（与 RechargeOrder.Channel 一致）
const (
	ChannelWechat = "wechat"
	ChannelAlipay = "alipay"
)

// 全局支付配置 Key（tenant_id=0 的 KV）。
const (
	KeyWechatPayEnabled    = "wechat_pay_enabled"    // 微信支付启用：1 / 0
	KeyWechatPayMchID      = "wechat_pay_mch_id"     // 微信商户号
	KeyWechatPayAppID      = "wechat_pay_app_id"     // 微信 AppID
	KeyWechatPaySerialNo   = "wechat_pay_serial_no"  // 微信商户证书序列号
	KeyWechatPayAPIv3Key   = "wechat_pay_api_v3_key" // 微信 APIv3 密钥（敏感，加密存储）
	KeyWechatPayPrivateKey = "wechat_pay_private_key" // 微信商户 API 私钥 PEM（敏感，加密存储）

	KeyAlipayPayEnabled    = "alipay_pay_enabled"    // 支付宝启用：1 / 0
	KeyAlipayPayAppID      = "alipay_pay_app_id"     // 支付宝应用 AppID
	KeyAlipayPayPrivateKey = "alipay_pay_private_key" // 支付宝应用私钥 PEM（敏感，加密存储）
	KeyAlipayPayPublicKey  = "alipay_pay_public_key"  // 支付宝公钥 PEM（敏感，加密存储）

	KeyWechatPayNotifyBaseURL = "wechat_pay_notify_base_url" // 微信支付回调基础地址（外网可访问，不含 /notify/wechat）
	KeyAlipayPayNotifyBaseURL = "alipay_pay_notify_base_url" // 支付宝回调基础地址（外网可访问，不含 /notify/alipay）
	KeyPointPriceFen          = "point_price_fen"            // 每点价格（分），默认 100 分/点
	KeyExtendPriceFenMonth    = "extend_price_fen_month"     // 续费月单价（分），默认 9800 分 = 98 元/月
)

// DefaultExtendPriceFenMonth 续费月单价默认值（分）
const DefaultExtendPriceFenMonth int64 = 9800

// ErrNotEnabled 支付渠道未启用
var ErrNotEnabled = errors.New("支付渠道未启用")

// SecretConfig 全局支付配置（含解密后的敏感项，仅限服务内部使用，严禁外发/落日志）
type SecretConfig struct {
	WechatPayEnabled    bool
	WechatPayMchID      string
	WechatPayAppID      string
	WechatPaySerialNo   string
	WechatPayAPIv3Key   string
	WechatPayPrivateKey string

	AlipayPayEnabled    bool
	AlipayPayAppID      string
	AlipayPayPrivateKey string
	AlipayPayPublicKey  string

	WechatNotifyBaseURL string
	AlipayNotifyBaseURL string
	PointPriceFen       int64
	ExtendPriceFenMonth int64 // 续费月单价（分）
}

// PointPrice 返回当前全局单点价格（分），保证 > 0
func (c *SecretConfig) PointPrice() int64 {
	if c.PointPriceFen <= 0 {
		return 100
	}
	return c.PointPriceFen
}

// ExtendMonthPrice 返回当前续费月单价（分），保证 > 0
func (c *SecretConfig) ExtendMonthPrice() int64 {
	if c.ExtendPriceFenMonth <= 0 {
		return DefaultExtendPriceFenMonth
	}
	return c.ExtendPriceFenMonth
}

// WechatNotifyURL / AlipayNotifyURL 拼接完整回调地址
func (c *SecretConfig) WechatNotifyURL() string {
	return strings.TrimRight(c.WechatNotifyBaseURL, "/") + "/notify/wechat"
}

func (c *SecretConfig) AlipayNotifyURL() string {
	return strings.TrimRight(c.AlipayNotifyBaseURL, "/") + "/notify/alipay"
}

// 是否需要脱敏的密钥类配置 Key（前端回显一律 ******）
func IsSecretKey(key string) bool {
	switch key {
	case KeyWechatPayAPIv3Key, KeyWechatPayPrivateKey,
		KeyAlipayPayPrivateKey, KeyAlipayPayPublicKey:
		return true
	}
	return false
}

// LoadSecretConfig 从全局设置（tenant_id=0）读取支付配置，敏感项自动解密。
func LoadSecretConfig() (*SecretConfig, error) {
	cfg := config.Load()
	var settings []models.Setting
	if err := database.DB.Where("tenant_id = ?", 0).Find(&settings).Error; err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, s := range settings {
		m[s.Key] = s.Value
	}
	dec := func(key string) string {
		v := m[key]
		if v == "" {
			return ""
		}
		if crypto.IsEncrypted(v) {
			plain, err := crypto.Decrypt(v, cfg.PayloadSecret())
			if err != nil {
				return ""
			}
			return plain
		}
		return v
	}
	price, _ := parseInt(m[KeyPointPriceFen])
	extendPrice, _ := parseInt(m[KeyExtendPriceFenMonth])
	return &SecretConfig{
		WechatPayEnabled:    m[KeyWechatPayEnabled] == "1",
		WechatPayMchID:      m[KeyWechatPayMchID],
		WechatPayAppID:      m[KeyWechatPayAppID],
		WechatPaySerialNo:   m[KeyWechatPaySerialNo],
		WechatPayAPIv3Key:   dec(KeyWechatPayAPIv3Key),
		WechatPayPrivateKey: dec(KeyWechatPayPrivateKey),

		AlipayPayEnabled:    m[KeyAlipayPayEnabled] == "1",
		AlipayPayAppID:      m[KeyAlipayPayAppID],
		AlipayPayPrivateKey: dec(KeyAlipayPayPrivateKey),
		AlipayPayPublicKey:  dec(KeyAlipayPayPublicKey),

		WechatNotifyBaseURL: m[KeyWechatPayNotifyBaseURL],
		AlipayNotifyBaseURL: m[KeyAlipayPayNotifyBaseURL],
		PointPriceFen:       price,
		ExtendPriceFenMonth: extendPrice,
	}, nil
}

// GetPublicConfig 返回可回显配置（敏感项用占位符 ******，绝不下发明文）。
func GetPublicConfig() (map[string]string, error) {
	var settings []models.Setting
	if err := database.DB.Where("tenant_id = ?", 0).Find(&settings).Error; err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, s := range settings {
		if IsSecretKey(s.Key) {
			if s.Value != "" {
				m[s.Key] = "******"
			} else {
				m[s.Key] = ""
			}
		} else {
			m[s.Key] = s.Value
		}
	}
	// 确保单价字段总有回显
	if m[KeyPointPriceFen] == "" {
		m[KeyPointPriceFen] = "100"
	}
	if m[KeyExtendPriceFenMonth] == "" {
		m[KeyExtendPriceFenMonth] = fmt.Sprintf("%d", DefaultExtendPriceFenMonth)
	}
	return m, nil
}

// SaveConfig 保存全局支付配置。敏感项：留空或仍为占位符时保留原值（不覆盖）。
func SaveConfig(values map[string]string) error {
	cfg := config.Load()
	for k, v := range values {
		if strings.TrimSpace(k) == "" {
			continue
		}
		v = strings.TrimSpace(v)
		if IsSecretKey(k) {
			if v == "" || v == "******" {
				continue // 留空 = 保留原密钥
			}
			// 已加密则跳过（前端不应回传密文，兜底判断）
			if crypto.IsEncrypted(v) {
				continue
			}
			enc, err := crypto.Encrypt(v, cfg.PayloadSecret())
			if err != nil {
				continue
			}
			v = enc
		}
		if err := database.DB.Exec(
			"INSERT INTO settings (tenant_id, key, value) VALUES (?, ?, ?) "+
				"ON CONFLICT(tenant_id, key) DO UPDATE SET value = excluded.value",
			0, k, v,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func parseInt(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// PayProvider 统一支付渠道接口。
type PayProvider interface {
	// CreateQR 预下单并返回二维码内容（codeUrl）。orderNo 为商户订单号，amountFen 为金额（分）。
	CreateQR(ctx context.Context, orderNo string, amountFen int64, desc string) (codeURL string, err error)
	// NotifyParse 解析并验签回调报文，返回验签是否通过及支付信息。
	NotifyParse(ctx context.Context, body []byte, headers map[string]string) (NotifyResult, error)
}

// NotifyResult 回调解析结果
type NotifyResult struct {
	VerifyOK  bool   // 验签是否通过
	OrderNo   string // 商户订单号
	AmountFen int64  // 实付金额（分）
	TradeNo   string // 第三方交易号
}

// New 按渠道创建支付 Provider；未启用或配置不完整时返回错误。
func New(channel string) (PayProvider, error) {
	cfg, err := LoadSecretConfig()
	if err != nil {
		return nil, err
	}
	switch channel {
	case ChannelWechat:
		if !cfg.WechatPayEnabled {
			return nil, ErrNotEnabled
		}
		return newWechatProvider(cfg)
	case ChannelAlipay:
		if !cfg.AlipayPayEnabled {
			return nil, ErrNotEnabled
		}
		return newAlipayProvider(cfg)
	default:
		return nil, errors.New("不支持的支付渠道")
	}
}

// GenOrderNo 生成商户订单号：时间戳（精确到秒）+ 随机后缀，保证唯一且可排序。
func GenOrderNo() string {
	return fmt.Sprintf("%d%06d", time.Now().Unix(), rand.Intn(1000000))
}
