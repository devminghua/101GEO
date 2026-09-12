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

// classifyFailReason 把平台原文归成客户能理解的原因。
//
// 为什么必须测：这些用例全部来自任务 #468 的真实 error_msg。
// 尤其「点卡余额不足」同时含「余额不足」，若判定顺序写反（先判平台欠费），
// 会把「请充值点卡」误导成「AI 平台账户欠费」——客户会去找错对象。
func TestClassifyFailReason(t *testing.T) {
	cases := []struct {
		in   string
		want string
		desc string
	}{
		// 真实样例（任务 #468）
		{`HTTP 403: {"error":{"code":"AccountOverdueError","message":"The request failed because your account has an overdue bill."}}`, whyArrears, "豆包账户欠费"},
		{`HTTP 401: {"error":{"code":"AuthenticationError","message":"The API key doesn't exist."}}`, whyBadKey, "Key 失效"},
		{`HTTP 429: {"error":{"code":"1305","message":"该模型当前访问量过大，请您稍后再试"}}`, whyRate, "限流"},
		{`请求失败: Post "https://ark.cn-beijing.volces.com/...": context deadline exceeded`, whyTimeout, "超时"},
		{"响应为空", whyEmpty, "空响应"},
		// 我方点卡不足：必须与「平台欠费」区分开
		{"点卡余额不足，请联系总后台充值", whyBalance, "点卡不足（不得误判为平台欠费）"},
		{"平台已熔断（连续失败达阈值），本轮跳过", whyBreaker, "熔断跳过"},
		{"某种没见过的错误", whyUnknown, "未识别"},
	}
	for _, c := range cases {
		if got := classifyFailReason(c.in); got != c.want {
			t.Errorf("%s：classifyFailReason(%q) = %q，期望 %q", c.desc, c.in, got, c.want)
		}
	}
}

// 失败文案必须讲清「什么原因 + 该做什么」，并点名**真正需要客户处理**的平台。
// 反例（v1.0.43 及之前）：「AI 平台返回异常或点卡不足」——两种原因混在一句，
// 客户既不知是谁的问题也不知该找谁，等于没给信息。
//
// 本用例数据取自任务 #468 真实分布（空响应 109 / 豆包欠费 93），
// 专门钉住「点名的是欠费的豆包，而不是失败次数最多的 DeepSeek」这一判定。
func TestFailedReasonText(t *testing.T) {
	fs := &taskFailStat{
		Failed: 222,
		Cost:   222,
		ByWhy:  map[string]int{whyEmpty: 109, whyArrears: 93},
		ByPlatform: map[string]int64{
			"豆包（火山方舟）": 102,
			"DeepSeek":   117, // 失败点数更多，但全是空响应（我方问题）
		},
		Critical: map[string]int64{"豆包（火山方舟）": 93},
		Sample:   "HTTP 403: AccountOverdueError",
	}

	got := failedReasonText(fs)
	// 必须包含失败次数
	if !strings.Contains(got, "222") {
		t.Errorf("应包含失败次数，实际 %q", got)
	}
	// 必须给出可执行动作（欠费 → 提示充值/续费）
	if !strings.Contains(got, "充值") && !strings.Contains(got, "续费") {
		t.Errorf("欠费类失败应给出充值/续费指引，实际 %q", got)
	}
	// 关键：应点名真正欠费的平台，而非失败次数更多的那个
	if !strings.Contains(got, "豆包（火山方舟）") {
		t.Errorf("应点名欠费平台「豆包（火山方舟）」，实际 %q", got)
	}
	if strings.Contains(got, "问题平台：DeepSeek") {
		t.Errorf("不应把空响应为主的 DeepSeek 报为问题平台，实际 %q", got)
	}
	// 不允许再出现两种原因混为一谈的旧文案
	if strings.Contains(got, "或点卡不足") {
		t.Errorf("不应出现含糊文案，实际 %q", got)
	}

	// 空统计不能崩，且要给出兜底说明
	if s := failedReasonText(nil); s == "" {
		t.Error("nil 统计不应返回空串")
	}
	if s := failedReasonText(&taskFailStat{ByWhy: map[string]int{}}); s == "" {
		t.Error("无失败明细不应返回空串")
	}

	// 非「需客户处理」的故障（限流等临时问题）不点名平台，避免误导客户去折腾
	fsRate := &taskFailStat{
		Failed:     5,
		ByWhy:      map[string]int{whyRate: 5},
		ByPlatform: map[string]int64{"DeepSeek": 5},
		Critical:   map[string]int64{}, // 限流不计入 Critical
	}
	if s := failedReasonText(fsRate); strings.Contains(s, "问题平台") {
		t.Errorf("临时性故障不应点名平台，实际 %q", s)
	}
}

// needActionWhy 只把确定性故障（欠费/Key）划为「需客户处理」。
// 划错会导致客户被无效指引骚扰（如为限流去换 Key）。
func TestNeedActionWhy(t *testing.T) {
	if !needActionWhy(whyArrears) || !needActionWhy(whyBadKey) {
		t.Error("欠费与 Key 失效应属需客户处理")
	}
	for _, w := range []string{whyRate, whyTimeout, whyEmpty, whyBalance, whyBreaker, whyUnknown} {
		if needActionWhy(w) {
			t.Errorf("%s 不应被划为需客户处理的确定性故障", w)
		}
	}
}

// topWhys 排序与截断：失败详情只展示前二主因，排序错了会展示次要原因。
func TestTopWhys(t *testing.T) {
	fs := &taskFailStat{ByWhy: map[string]int{
		whyEmpty:   109,
		whyArrears: 93,
		whyRate:    10,
	}}
	got := topWhys(fs, 2)
	if len(got) != 2 {
		t.Fatalf("应返回 2 项，实际 %d", len(got))
	}
	if got[0] != whyEmpty || got[1] != whyArrears {
		t.Errorf("排序错误：期望 [%s %s]，实际 %v", whyEmpty, whyArrears, got)
	}
	if first := topWhy(fs); first != whyEmpty {
		t.Errorf("topWhy 期望 %s，实际 %s", whyEmpty, first)
	}
	// 请求数量超过实际种类时不 panic
	if len(topWhys(fs, 99)) != 3 {
		t.Errorf("请求超出种类数时应返回全部 3 项")
	}
}

// 空统计的 topPlatform 不能 panic（map 为 nil 也安全）
func TestTopPlatformNilSafe(t *testing.T) {
	if p := (&taskFailStat{}).topPlatform(); p != "" {
		t.Errorf("nil map 应返回空串，实际 %q", p)
	}
}
