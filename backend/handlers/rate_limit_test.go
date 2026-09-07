package handlers

import (
	"testing"
	"time"
)

func TestRateLimiterBasic(t *testing.T) {
	rl := newRateLimiter(time.Minute, 3)
	for i := 0; i < 3; i++ {
		if !rl.allow("1.1.1.1") {
			t.Fatalf("前 %d 次应允许", i+1)
		}
	}
	if rl.allow("1.1.1.1") {
		t.Fatal("超过阈值应拒绝")
	}
	if !rl.allow("2.2.2.2") {
		t.Fatal("不同 key 不应受影响")
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	rl := newRateLimiter(50*time.Millisecond, 1)
	if !rl.allow("3.3.3.3") {
		t.Fatal("首次应允许")
	}
	if rl.allow("3.3.3.3") {
		t.Fatal("窗口内第二次应拒绝")
	}
	time.Sleep(60 * time.Millisecond)
	if !rl.allow("3.3.3.3") {
		t.Fatal("窗口过期后应重新允许")
	}
}
