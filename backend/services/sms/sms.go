// Package sms 提供短信验证码能力：6 位验证码、内存存储、重发冷却、一次性校验。
// 当前为 Mock 阶段（老板拍板「先 Mock 跑通」）：验证码仅打印到日志，不真正调用短信网关。
// 接入真实短信（阿里云）时，实现 Sender 接口替换 MockSender 即可，其余逻辑无需改动。
package sms

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"
)

// ErrTooFrequent 同一手机号发送过于频繁。
var ErrTooFrequent = errors.New("验证码发送过于频繁，请稍后再试")

const (
	codeLen      = 6                 // 验证码位数
	codeTTL      = 5 * time.Minute   // 验证码有效期
	sendCooldown = 60 * time.Second  // 同一手机号两次发送的最小间隔
)

// Sender 短信发送抽象：Mock 阶段打日志，接入真实短信后替换为阿里云实现。
type Sender interface {
	Send(phone, code string) error
}

// MockSender 模拟发送：仅打印日志，便于联调跑通注册全链路。
type MockSender struct{}

// Send 打印验证码日志。
func (MockSender) Send(phone, code string) error {
	log.Printf("[SMS][mock] 向 %s 发送验证码：%s（%s 内有效）", phone, code, codeTTL)
	return nil
}

type entry struct {
	code     string
	expires  time.Time
	lastSent time.Time
}

// Service 验证码服务：内存存储 + 发送冷却 + 一次性校验。单实例部署，进程重启即失效。
type Service struct {
	mu       sync.Mutex
	store    map[string]*entry
	sender   Sender
	cooldown time.Duration
	ttl      time.Duration
}

// New 创建验证码服务；sender 为空时使用 MockSender。
func New(sender Sender) *Service {
	if sender == nil {
		sender = MockSender{}
	}
	return &Service{
		store:    make(map[string]*entry),
		sender:   sender,
		cooldown: sendCooldown,
		ttl:      codeTTL,
	}
}

// Send 生成并发送验证码，返回验证码明文（Mock 联调用）。
// 同一手机号冷却期内重复请求返回 ErrTooFrequent。
func (s *Service) Send(phone string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.store[phone]; ok && time.Since(e.lastSent) < s.cooldown {
		return "", ErrTooFrequent
	}
	code, err := genCode(codeLen)
	if err != nil {
		return "", err
	}
	if err := s.sender.Send(phone, code); err != nil {
		return "", err
	}
	s.store[phone] = &entry{code: code, expires: time.Now().Add(s.ttl), lastSent: time.Now()}
	return code, nil
}

// Verify 校验验证码；成功即作废（一次性使用）。
func (s *Service) Verify(phone, code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.store[phone]
	if !ok {
		return false
	}
	if time.Now().After(e.expires) {
		delete(s.store, phone) // 过期清理
		return false
	}
	if e.code != code {
		return false
	}
	delete(s.store, phone)
	return true
}

func genCode(n int) (string, error) {
	if n <= 0 {
		n = codeLen
	}
	max := big.NewInt(1)
	for i := 0; i < n; i++ {
		max.Mul(max, big.NewInt(10))
	}
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", n, v.Int64()), nil
}

// Default 全局验证码服务实例（Mock 发送）。
var Default = New(nil)

// Send 发送验证码（便捷函数）。
func Send(phone string) (string, error) { return Default.Send(phone) }

// Verify 校验验证码（便捷函数）。
func Verify(phone, code string) bool { return Default.Verify(phone, code) }

// SetSender 动态切换发送器（总后台短信设置配置阿里云后，切换到真实发送）。
func SetSender(sender Sender) {
	Default.mu.Lock()
	Default.sender = sender
	Default.mu.Unlock()
}

// IsMock 当前是否 Mock 模式。默认 Mock；切换到阿里云发送器后返回 false。
func IsMock() bool {
	Default.mu.Lock()
	defer Default.mu.Unlock()
	switch Default.sender.(type) {
	case MockSender, *MockSender:
		return true
	default:
		return false
	}
}
