package biztime

import (
	"os"
	"testing"
	"time"
)

// TestZoneIsAlwaysBeijing 验证 biztime 不依赖系统 TZ / tzdata：
// 无论进程时区被设成 UTC 还是别的，Now() 都必须是北京时间。
// 这是本次修复的核心保证——容器镜像默认 UTC，客户机可能是任意时区。
func TestZoneIsAlwaysBeijing(t *testing.T) {
	// 备份并强制把进程时区设为 UTC（等价于容器环境）
	old := os.Getenv("TZ")
	defer func() {
		if old == "" {
			os.Unsetenv("TZ")
		} else {
			os.Setenv("TZ", old)
		}
		time.Local = time.UTC
	}()

	os.Setenv("TZ", "UTC")
	time.Local = time.UTC

	utcNow := time.Now().UTC()
	bjNow := Now()

	// 注意：In(zone) 不改变绝对时刻，Sub() 恒为 0，必须比较**墙上时钟**。
	// 用 HH:MM 的分钟数差判断时区偏移是否为 +8 小时（允许跨天，故取模 24h）。
	utcMin := utcNow.Hour()*60 + utcNow.Minute()
	bjMin := bjNow.Hour()*60 + bjNow.Minute()
	delta := ((bjMin - utcMin) % (24 * 60)) // 墙上时钟差
	if delta < 0 {
		delta += 24 * 60
	}
	if delta != 8*60 {
		t.Fatalf("Now() 墙上时钟应比 UTC 快 8 小时，实际 %d 分钟（进程 TZ=%s）", delta, os.Getenv("TZ"))
	}

	_, offset := bjNow.Zone()
	if offset != 8*3600 {
		t.Fatalf("Now() 时区偏移应为 +28800 秒，实际 %d", offset)
	}
	if bjNow.Location() == time.UTC {
		t.Fatalf("Now() 不应返回 UTC 时区")
	}
	t.Logf("UTC=%s  biztime=%s  墙上时钟差=%d 分钟  offset=%ds",
		utcNow.Format("15:04"), bjNow.Format("15:04"), delta, offset)
}

// TestHourMatchesBeijingHour 验证 Hour() 在进程时区为 UTC 时仍返回北京时间小时。
// 这正是原 bug 的根源：runAutoCheck 用 time.Now().Hour() 拿到 UTC 小时，
// 使「8-22 点巡检」实际落在北京时间 16:00~次日 06:00。
func TestHourMatchesBeijingHour(t *testing.T) {
	oldLocal := time.Local
	defer func() { time.Local = oldLocal }()
	time.Local = time.UTC

	bjHour := Now().Hour()
	utcHour := time.Now().UTC().Hour()
	wantBj := (utcHour + 8) % 24

	if Hour() != bjHour {
		t.Fatalf("Hour() 应等于北京时间小时 %d，实际 %d", bjHour, Hour())
	}
	if bjHour != wantBj {
		t.Fatalf("北京时间小时应为 %d，实际 %d", wantBj, bjHour)
	}
	// 明确记录原 bug 的后果，防止回归
	t.Logf("UTC %d 点 → 北京时间 %d 点；若误用 time.Now().Hour() 会得到 %d（偏 8 小时）",
		utcHour, bjHour, utcHour)
}

// TestDayOffsetUsesBeijingCalendar 验证 Day(offset) 按北京日历推进，
// 而不是按 UTC 日历——否则北京时间 0-8 点算出的日期会差一天。
func TestDayOffsetUsesBeijingCalendar(t *testing.T) {
	if got, want := Day(0), Now().Format("2006-01-02"); got != want {
		t.Fatalf("Day(0) 应为今天 %s，实际 %s", want, got)
	}
	today := Now()
	want := today.AddDate(0, 0, -7).Format("2006-01-02")
	if got := Day(-7); got != want {
		t.Fatalf("Day(-7) 应为 %s，实际 %s", want, got)
	}
	if got, want := Day(1), today.AddDate(0, 0, 1).Format("2006-01-02"); got != want {
		t.Fatalf("Day(1) 应为 %s，实际 %s", want, got)
	}
	t.Logf("今天=%s  7天前=%s  明天=%s", Day(0), Day(-7), Day(1))
}

// TestDayStartIsBeijingMidnight 验证 DayStart 返回北京时间零点，
// 而不是进程本地（UTC）零点——后者等于北京时间早上 8 点。
func TestDayStartIsBeijingMidnight(t *testing.T) {
	oldLocal := time.Local
	defer func() { time.Local = oldLocal }()
	time.Local = time.UTC

	ms := DayStart(Now())
	if h, m, s := ms.Hour(), ms.Minute(), ms.Second(); h != 0 || m != 0 || s != 0 {
		t.Fatalf("DayStart 应为整点零点，实际 %02d:%02d:%02d", h, m, s)
	}
	// 北京时间零点 = UTC 前一天 16:00
	if h := ms.UTC().Hour(); h != 16 {
		t.Fatalf("北京时间零点对应 UTC 16 点，实际 %d", h)
	}
	t.Logf("北京时间零点 %s = UTC %s", ms.Format("2006-01-02 15:04"), ms.UTC().Format("2006-01-02 15:04"))
}

// TestSinceMatchesDayWindow 验证 Since(n) 与 Day(-n) 指向同一业务日，
// 保证「窗口下界」与「补空天的日期键」口径一致。
func TestSinceMatchesDayWindow(t *testing.T) {
	for _, n := range []int{1, 6, 7, 29, 30} {
		if got, want := Since(n).Format("2006-01-02"), Day(-n); got != want {
			t.Fatalf("Since(%d)=%s 应与 Day(-%d)=%s 同一天", n, got, n, want)
		}
	}
}

// TestFormatConvertsToBeijing 验证 Format 把任意时区的时间转成北京时间再格式化
func TestFormatConvertsToBeijing(t *testing.T) {
	// 构造 UTC 2026-09-12 20:00 = 北京时间 2026-09-13 04:00（跨天）
	utc := time.Date(2026, 9, 12, 20, 0, 0, 0, time.UTC)
	if got, want := Format(utc, "2006-01-02 15:04"), "2026-09-13 04:00"; got != want {
		t.Fatalf("Format 应换算为北京时间 %s，实际 %s", want, got)
	}
	t.Logf("UTC %s → 北京时间 %s",
		utc.Format("2006-01-02 15:04"), Format(utc, "2006-01-02 15:04"))
}
