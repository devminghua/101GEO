// Package card 提供离线卡密（软件授权码）的生成与验证能力。
//
// 原理：卡密 = 授权数据（card_id + tier + nonce）+ Ed25519 非对称签名。
// 私钥仅老板后台持有（用于生成卡密），客户端内置公钥只能验签、无法伪造；
// nonce 随机化防止卡密被枚举/猜解；card_id 用于防重复激活。
package card

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"strings"
)

// 卡密类型（tier）。
const (
	TierTrial3Day = 1 // 3 天试用
	Tier12Month   = 2 // 12 个月
	TierRecharge  = 3 // 充值 token（兑换点数，Points 由 Card 表记录）
)

const (
	prefix     = "LG-"
	payloadLen = 4 + 1 + 8                  // card_id(4) + tier(1) + nonce(8) = 13
	totalLen   = payloadLen + ed25519.SignatureSize // 13 + 64 = 77
	groupSize  = 5
)

// crockford base32 字母表：去除易混字符 I / L / O / U，便于人工输入。
var crockford = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// Generate 用私钥为指定 card_id / tier 生成一张卡密串。
func Generate(priv ed25519.PrivateKey, cardID uint32, tier byte) (string, error) {
	payload := make([]byte, payloadLen)
	binary.BigEndian.PutUint32(payload[0:4], cardID)
	payload[4] = tier
	if _, err := rand.Read(payload[5:13]); err != nil {
		return "", err
	}
	sig := ed25519.Sign(priv, payload)
	raw := make([]byte, 0, totalLen)
	raw = append(raw, payload...)
	raw = append(raw, sig...)
	return encode(raw), nil
}

// Verify 用公钥验证卡密串，返回 card_id 与 tier。任一步失败返回 error。
func Verify(pub ed25519.PublicKey, code string) (cardID uint32, tier byte, err error) {
	raw, err := decode(code)
	if err != nil {
		return 0, 0, err
	}
	if len(raw) != totalLen {
		return 0, 0, errors.New("卡密格式错误")
	}
	payload := raw[:payloadLen]
	sig := raw[payloadLen:]
	if !ed25519.Verify(pub, payload, sig) {
		return 0, 0, errors.New("卡密无效")
	}
	cardID = binary.BigEndian.Uint32(payload[0:4])
	tier = payload[4]
	return cardID, tier, nil
}

// TierDays 返回卡密类型对应的有效天数；未知类型返回 0。
func TierDays(tier byte) int {
	switch tier {
	case TierTrial3Day:
		return 3
	case Tier12Month:
		return 365
	default:
		return 0
	}
}

// TierName 返回卡密类型的中文名。
func TierName(tier byte) string {
	switch tier {
	case TierTrial3Day:
		return "3 天试用"
	case Tier12Month:
		return "12 个月"
	case TierRecharge:
		return "充值 token"
	default:
		return "未知"
	}
}

// encode 将 77 字节原始数据编码为 LG-XXXXX-... 分组卡密串。
func encode(raw []byte) string {
	b32 := crockford.EncodeToString(raw)
	var sb strings.Builder
	sb.WriteString(prefix)
	for i, r := range b32 {
		if i > 0 && i%groupSize == 0 {
			sb.WriteByte('-')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// decode 解析卡密串：去前缀、去分隔符、大小写归一、易混字符容错后 base32 解码。
func decode(code string) ([]byte, error) {
	s := strings.ToUpper(strings.TrimSpace(code))
	// 去掉前缀（兼容 LG- 与漏打连字符的 LG 两种写法）
	s = strings.TrimPrefix(s, "LG-")
	s = strings.TrimPrefix(s, "LG")
	// 去除常见分隔符
	s = strings.NewReplacer("-", "", " ", "", "_", "", ".", "").Replace(s)
	if s == "" {
		return nil, errors.New("卡密为空")
	}
	// Crockford 规范容错：I/L → 1，O → 0，U → V
	s = strings.NewReplacer("I", "1", "L", "1", "O", "0", "U", "V").Replace(s)
	raw, err := crockford.DecodeString(s)
	if err != nil {
		return nil, errors.New("卡密格式错误")
	}
	return raw, nil
}
