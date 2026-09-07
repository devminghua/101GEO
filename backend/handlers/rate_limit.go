package handlers

import (
	"sync"
	"time"
)

// rlWindow 固定时间窗口内的计数
type rlWindow struct {
	start time.Time
	count int
}

// rateLimiter 基于 key（IP）的固定窗口限流。内存实现、重启不保留，用于「软限流」防刷；
// 与持久化的分级锁定互补：锁定防「单账号+IP 爆破」，限流防「单 IP 高频爆破/刷接口」。
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]*rlWindow
	window  time.Duration
	limit   int
}

func newRateLimiter(window time.Duration, limit int) *rateLimiter {
	return &rateLimiter{windows: make(map[string]*rlWindow), window: window, limit: limit}
}

// allow 判断该 key 当前是否允许一次请求；允许则计数 +1。
func (r *rateLimiter) allow(key string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	// 惰性清理过期窗口，防止 map 无限增长
	for k, w := range r.windows {
		if now.Sub(w.start) >= r.window {
			delete(r.windows, k)
		}
	}
	w := r.windows[key]
	if w == nil || now.Sub(w.start) >= r.window {
		w = &rlWindow{start: now}
		r.windows[key] = w
	}
	if w.count >= r.limit {
		return false
	}
	w.count++
	return true
}

// 登录入口公开接口的限流实例。
var (
	loginLimiter    = newRateLimiter(60*time.Second, 10)  // 登录：同一 IP 60 秒最多 10 次尝试
	captchaLimiter  = newRateLimiter(60*time.Second, 60)  // 验证码：同一 IP 60 秒最多 60 次（登录失败会刷新验证码，阈值放宽）
	smsLimiter      = newRateLimiter(10*time.Minute, 20)  // 短信验证码：同一 IP 10 分钟最多 20 次（防刷短信）
	registerLimiter = newRateLimiter(10*time.Minute, 20)  // 注册：同一 IP 10 分钟最多 20 次尝试
)
