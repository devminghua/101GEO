package geo

import (
	"context"
	"testing"
	"time"
)

// TestPlatformBreakerThreshold 熔断器按「连续失败达阈值」触发，成功即清零。
//
// 背景：任务 #468 中豆包账户欠费后仍被追问 93 次、智谱连续 429 也不停，
// 白白扣掉客户 274 点并让任务跑了 69 分钟。熔断是该问题的核心防线，
// 阈值语义一旦写错（如写成累计失败、或成功不清零），会出现「偶发抖动误熔断」
// 或「故障平台永远熔断」两类事故。
func TestPlatformBreakerThreshold(t *testing.T) {
	pid := uint(35)

	t.Run("连续失败达阈值才熔断", func(t *testing.T) {
		b := newPlatformBreaker()
		for i := 1; i < platformFailThreshold; i++ {
			if b.record(pid, false) {
				t.Fatalf("第 %d 次失败不应触发熔断（阈值 %d）", i, platformFailThreshold)
			}
			if b.trippedNow(pid) {
				t.Fatalf("第 %d 次失败后不应处于熔断态", i)
			}
		}
		if !b.record(pid, false) {
			t.Fatalf("第 %d 次失败应触发熔断", platformFailThreshold)
		}
		if !b.trippedNow(pid) {
			t.Fatal("触发后应处于熔断态")
		}
	})

	t.Run("成功即清零连续失败计数", func(t *testing.T) {
		b := newPlatformBreaker()
		// 失败到差一次触发
		for i := 1; i < platformFailThreshold; i++ {
			b.record(pid, false)
		}
		// 一次成功应清零
		b.record(pid, true)
		for i := 1; i < platformFailThreshold; i++ {
			if b.record(pid, false) {
				t.Fatalf("清零后第 %d 次失败不应熔断（说明计数未重置）", i)
			}
		}
	})

	t.Run("平台之间互不影响", func(t *testing.T) {
		b := newPlatformBreaker()
		other := uint(33)
		for i := 0; i < platformFailThreshold+2; i++ {
			b.record(pid, false)
		}
		if !b.trippedNow(pid) {
			t.Fatal("平台 A 应已熔断")
		}
		if b.trippedNow(other) {
			t.Fatal("平台 B 不应受平台 A 影响")
		}
	})

	t.Run("熔断后不重复报告", func(t *testing.T) {
		b := newPlatformBreaker()
		for i := 0; i < platformFailThreshold; i++ {
			b.record(pid, false)
		}
		// 阈值触发的那次已返回 true，继续失败不应反复返回 true（避免日志刷屏）
		if b.record(pid, false) {
			t.Fatal("已熔断状态下的再次失败不应重复报告触发")
		}
	})
}

// TestPlatformGateCancelable 节流门的等待必须可被取消。
//
// 背景：Kimi 等平台 interval_ms 建议 20000ms。若整轮巡检已超时，
// 而节流门仍在 time.Sleep 傻等，任务会在超时后又空转很久，
// 使 maxTaskDuration 形同虚设（历史上有任务因此跑到 9.8 小时）。
func TestPlatformGateCancelable(t *testing.T) {
	g := &platformGate{interval: 10 * time.Second}
	// 先占一次时间戳，使下一次进入必须等待
	g.lastAt = time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	executed := false
	g.do(ctx, func() { executed = true })

	if executed {
		t.Fatal("ctx 已取消时不应执行请求体（应直接返回，避免超时空转）")
	}
}
