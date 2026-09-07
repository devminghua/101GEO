package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"geo-tool/config"
)

// Claims JWT 载荷：包含租户与角色信息
type Claims struct {
	UserID   uint   `json:"uid"`
	Username string `json:"username"`
	TenantID uint   `json:"tid"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp"`
}

func b64e(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func b64d(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

func hmacSHA256(secret []byte, data string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// Sign 签发 JWT（HS256，有效期 7 天）
func Sign(userID uint, username string, tenantID uint, role string) (string, error) {
	claims := Claims{
		UserID: userID, Username: username, TenantID: tenantID, Role: role,
		Exp: time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	signingInput := b64e(header) + "." + b64e(payload)
	sig := hmacSHA256(config.Load().TokenSecret(), signingInput)
	return signingInput + "." + b64e(sig), nil
}

// Parse 解析并校验 JWT，返回 Claims
func Parse(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("凭证格式错误")
	}
	secret := config.Load().TokenSecret()
	expected := hmacSHA256(secret, parts[0]+"."+parts[1])
	got, err := b64d(parts[2])
	if err != nil || !hmac.Equal(expected, got) {
		return nil, errors.New("凭证签名无效")
	}
	payload, err := b64d(parts[1])
	if err != nil {
		return nil, errors.New("凭证载荷错误")
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("凭证载荷错误")
	}
	if claims.Exp < time.Now().Unix() {
		return nil, errors.New("凭证已过期")
	}
	return &claims, nil
}
