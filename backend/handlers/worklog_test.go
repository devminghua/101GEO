package handlers

import (
	"strings"
	"testing"
	"time"

	"geo-tool/config"
	"geo-tool/services/biztime"
)

// 「预计下次自动巡检」这段逻辑最容易错在三处：
//   ① 顺延到次日 8:00 时用了 UTC 日历，跨时区后差 8 小时；
//   ② 把「19:30 完成 + 60 分钟 = 20:30」误判为落在时段外；
//   ③ 时间已过期却仍显示一个过去的时间，让客户困惑。
// 这里逐条钉住。
func TestNextAutoCheckText(t *testing.T) {
	interval := time.Duration(config.AutoCheckMinutes()) * time.Minute
	zone := biztime.Zone()

	// ⚠️ 用「未来日期」是刻意的：若用当天日期，算出的时间会早于此刻，
	// 触发「已过期 → 随时」的回落分支，把这些用例真正想验的
	// 间隔/时段顺延逻辑整体掩盖掉。
	const y = 2099

	// 场景 A：16:20 完成 + 60 分钟 = 17:20，仍在 [8,22) 内 → 显示当天 17:20
	finA := time.Date(y, 9, 12, 16, 20, 0, 0, zone)
	if got := nextAutoCheckText(&finA, &finA); got != "09-12 17:20" {
		t.Errorf("场景A：期望 09-12 17:20，实际 %s", got)
	}

	// 场景 B：21:30 完成 + 60 分钟 = 22:30，越过时段终点 → 顺延次日 08:00
	// （关键是「次日」要按北京日历算，不能被 UTC 影响）
	finB := time.Date(y, 9, 12, 21, 30, 0, 0, zone)
	if got := nextAutoCheckText(&finB, &finB); got != "09-13 08:00" {
		t.Errorf("场景B：期望 09-13 08:00，实际 %s", got)
	}

	// 场景 C：凌晨 3:00 完成（理论不该发生，但进程重启补跑可能出现）
	// 3:00 + 60min = 4:00 早于时段起点 → 应提前到「当天」08:00 而非次日
	finC := time.Date(y, 9, 12, 3, 0, 0, 0, zone)
	if got := nextAutoCheckText(&finC, &finC); got != "09-12 08:00" {
		t.Errorf("场景C：期望 09-12 08:00，实际 %s", got)
	}

	// 场景 D：很久以前完成 → 计算出的时间已早于此刻，应回落为「随时」提示，
	// 而不是给客户显示一个已经过去的日期。
	finD := biztime.Now().Add(-72 * time.Hour)
	if got := nextAutoCheckText(&finD, &finD); !strings.Contains(got, "随时") {
		t.Errorf("场景D：过期时间应回落为「随时」，实际 %s", got)
	}

	// 场景 E：从未巡检过（finished/created 均为空）→ 必须给出非空指引，不能返回空串
	zero := time.Time{}
	if got := nextAutoCheckText(&zero, &zero); got == "" {
		t.Errorf("场景E：从未巡检时不应返回空串")
	}

	// 间隔小时数换算自检（防止有人改配置后忘同步文案）
	if interval.Minutes() != float64(config.AutoCheckMinutes()) {
		t.Errorf("间隔换算异常：%v", interval)
	}
}

// truncateRunes 必须按字符而非字节截断，否则中文会被切成乱码。
func TestTruncateRunes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"短", 10, "短"},
		{"零一二三四五", 3, "零一二…"},
		{"  前后空格  ", 10, "前后空格"},
		{"abcdef", 3, "abc…"},
	}
	for _, c := range cases {
		if got := truncateRunes(c.in, c.n); got != c.want {
			t.Errorf("truncateRunes(%q,%d) = %q，期望 %q", c.in, c.n, got, c.want)
		}
	}
}

// 创作记录标题常自带类型前缀，拼接前必须剥掉，否则出现
// 「生成小红书文案：小红书文案：相亲平台种草」这种重复文案（实测发现）。
func TestStripKindPrefix(t *testing.T) {
	cases := []struct{ title, kind, want string }{
		{"小红书文案：相亲平台种草", "小红书文案", "相亲平台种草"},
		{"抖音脚本:同城相亲", "抖音脚本", "同城相亲"},
		{"营销文案", "营销文案", "营销文案"},          // 只有类型名没有正文 → 保留原样，不能剥成空
		{"小红书文案：", "小红书文案", "小红书文案："},   // 剥完为空则放弃剥离
		{"相亲平台种草", "小红书文案", "相亲平台种草"},   // 不含前缀 → 原样
		{" 前后空格 内容", "小红书文案", "前后空格 内容"}, // 去首尾空白
		// 简称前缀：作者只写了「文案」，应能识别为「营销文案」的简写
		{"文案：婚恋品牌 GEO 优化", "营销文案", "婚恋品牌 GEO 优化"},
		// 正常标题里含分隔符但前缀与类型无关 → 不能误剥
		{"相亲指南：如何选择平台", "营销文案", "相亲指南：如何选择平台"},
		// 前缀太短（1 字）不足以判定 → 保守保留
		{"文：测试", "营销文案", "文：测试"},
	}
	for _, c := range cases {
		if got := stripKindPrefix(c.title, c.kind); got != c.want {
			t.Errorf("stripKindPrefix(%q,%q) = %q，期望 %q", c.title, c.kind, got, c.want)
		}
	}
}

// 复测结论与行动类型的中文映射必须覆盖所有取值，避免界面上出现英文枚举。
func TestVerifyStatusTextCoverage(t *testing.T) {
	for _, s := range []string{"improved", "unchanged", "worse", "pending", ""} {
		got := verifyStatusText(s)
		if got == "" {
			t.Errorf("verifyStatusText(%q) 返回空", s)
		}
		// 不能把原始枚举直接抛给客户
		if got == s && s != "" {
			t.Errorf("verifyStatusText(%q) 未翻译", s)
		}
	}
	for _, s := range []string{"gap", "risk", "audit", "citation", "competitor", ""} {
		if optTaskTypeTextOrRaw(s) == "" {
			t.Errorf("optTaskTypeTextOrRaw(%q) 返回空", s)
		}
	}
}
