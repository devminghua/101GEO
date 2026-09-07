// Package crypto 提供可逆加密能力：用于账号密码的加密存储
// （SaaS 总后台需要"小眼睛"查看客户明文密码，因此密码不能只做不可逆哈希）。
// 存储格式前缀：enc:v1:<base64(nonce + ciphertext)>，密钥为服务端 GEO_SECRET_KEY。
// 兼容旧数据：不以 enc:v1: 开头的存量密码按 bcrypt 校验（不可逆、无法查看明文）。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	PrefixEnc   = "enc:v1:"
	DefaultCost = bcrypt.DefaultCost
)

// deriveKey 将任意长度密钥归一化（哈希后取 32 字节）。
func deriveKey(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
}

// Encrypt 使用 AES-GCM 加密明文，返回 enc:v1 格式密文。
func Encrypt(plain string, secret []byte) (string, error) {
	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return PrefixEnc + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 enc:v1 格式密文；非该格式返回错误。
func Decrypt(stored string, secret []byte) (string, error) {
	if !strings.HasPrefix(stored, PrefixEnc) {
		return "", errors.New("非可逆加密格式，无法解密")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, PrefixEnc))
	if err != nil {
		return "", err
	}
	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("密文长度异常")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// IsEncrypted 是否为可逆加密格式（可被 SaaS 端解密查看）。
func IsEncrypted(stored string) bool {
	return strings.HasPrefix(stored, PrefixEnc)
}

// Hash 生成可逆加密后的密码存储值。
func Hash(plain string, secret []byte) (string, error) {
	return Encrypt(plain, secret)
}

// Verify 校验明文是否匹配存储值：enc:v1 走解密比对，否则回退 bcrypt。
func Verify(stored, plain string, secret []byte) bool {
	if IsEncrypted(stored) {
		dec, err := Decrypt(stored, secret)
		if err != nil {
			return false
		}
		return dec == plain
	}
	return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil
}

// Ensure 兼容性提示（保留 bcrypt 生成能力给旧逻辑，新逻辑统一用 Hash）
func BCryptHash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), DefaultCost)
	if err != nil {
		return "", fmt.Errorf("密码哈希失败: %w", err)
	}
	return string(hash), nil
}
