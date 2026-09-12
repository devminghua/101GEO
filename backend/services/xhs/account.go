package xhs

import (
	"strconv"
	"time"

	"geo-tool/models"
	"geo-tool/services/biztime"
)

/* ================================================================
 * 小红书获客 · 账号状态机与动作前置校验
 *  - TodayUsed 跨天自动清零（按 TodayDate 归属日期判断）
 *  - CooldownUntil 到期自动恢复（冷却中 -> 在线）
 *  - 动作前置校验：活跃时段 / 当日上限 / 间隔 / 冷却 / 同日去重
 * 结构完全对齐抖音获客模块（半自动合规：服务端只做权威校验，不代发）
 * ================================================================ */

// RefreshAccountState 刷新账号运行态：跨天清零 + 冷却到期恢复。
// 返回是否发生了状态变更。读取列表/编辑前调用，保证状态实时正确。
func RefreshAccountState(a *models.XhsAccount, f Frequency, now time.Time) bool {
	changed := false
	today := now.Format("2006-01-02")
	if a.TodayDate != today {
		if a.TodayUsed != 0 || a.LastActionAt != nil {
			a.TodayUsed = 0
			a.TodayDate = today
			a.LastActionAt = nil
			changed = true
		}
	}
	// 冷却到期恢复
	if a.CooldownUntil != nil && now.After(*a.CooldownUntil) {
		a.CooldownUntil = nil
		if a.Status == models.AccountCooldown {
			a.Status = models.AccountOnline
			changed = true
		}
	} else if a.CooldownUntil != nil {
		// 冷却中（即使状态字段被手工改成非冷却也不信）
		if a.Status != models.AccountCooldown {
			a.Status = models.AccountCooldown
			changed = true
		}
	} else if a.Status == models.AccountCooldown {
		// 无冷却截止时间却标记冷却：视为异常，恢复在线
		a.Status = models.AccountOnline
		changed = true
	}
	return changed
}

// PrecheckResult 动作前置校验结果（服务端权威判定，前端只读展示）
type PrecheckResult struct {
	Can           bool       `json:"can"`
	Reason        string     `json:"reason"`        // 不可执行的原因（can=false 时）
	ActiveWindow  bool       `json:"active_window"` // 是否在活跃时段
	DailyOK       bool       `json:"daily_ok"`      // 是否未达当日上限
	IntervalOK    bool       `json:"interval_ok"`   // 是否满足动作间隔
	NotCooling    bool       `json:"not_cooling"`   // 是否非冷却中
	TodayUsed     int        `json:"today_used"`
	DailyLimit    int        `json:"daily_limit"`
	CooldownUntil *time.Time `json:"cooldown_until"` // 冷却截止时间（如有）
}

// Precheck 对某账号执行打招呼动作前置校验
func Precheck(a *models.XhsAccount, f Frequency, now time.Time, lastActionAt *time.Time) PrecheckResult {
	RefreshAccountState(a, f, now)

	res := PrecheckResult{
		Can:           false,
		ActiveWindow:  InActiveWindow(f, now),
		DailyOK:       a.TodayUsed < a.DailyLimit,
		IntervalOK:    true,
		NotCooling:    a.CooldownUntil == nil || now.After(*a.CooldownUntil),
		TodayUsed:     a.TodayUsed,
		DailyLimit:    a.DailyLimit,
		CooldownUntil: a.CooldownUntil,
	}

	var reasons []string
	if res.NotCooling {
		if !res.ActiveWindow {
			reasons = append(reasons, "当前不在活跃时段（"+f.ActiveStart+"~"+f.ActiveEnd+"）")
		}
		if !res.DailyOK {
			reasons = append(reasons, "已达当日打招呼上限（"+cutItoa(a.DailyLimit)+"）")
		}
		if lastActionAt != nil {
			elapsed := now.Sub(*lastActionAt)
			if int(elapsed.Minutes()) < f.IntervalMin {
				res.IntervalOK = false
				reasons = append(reasons, "距上次动作不足最小间隔（"+cutItoa(f.IntervalMin)+" 分钟）")
			}
		}
	} else {
		if a.CooldownUntil != nil {
			reasons = append(reasons, "账号冷却中，预计 "+a.CooldownUntil.Format("01-02 15:04")+" 恢复")
		} else {
			reasons = append(reasons, "账号冷却中")
		}
	}

	res.Can = len(reasons) == 0
	if len(reasons) > 0 {
		res.Reason = reasons[0]
	}
	return res
}

// NextCooldown 计算账号下次冷却截止时间（达上限自动冷却）
func NextCooldown(account *models.XhsAccount, f Frequency) {
	account.CooldownUntil = ptrTime(CooldownNextDayStart(f, biztime.Now()))
	account.Status = models.AccountCooldown
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func cutItoa(n int) string {
	return strconv.Itoa(n)
}
