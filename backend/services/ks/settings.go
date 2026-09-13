package ks

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"geo-tool/database"
	"geo-tool/models"
)

/* ================================================================
 * 快手获客 · 频率与安全设置（租户级 KV，复用 models.Setting 表）
 * 结构与抖音/小红书完全一致，key 前缀 ks_。
 * ================================================================ */

// Frequency 频率与安全设置（解析后的强类型承载，key 前缀 ks_）
type Frequency struct {
	DailyLimit  int    `json:"daily_limit"`  // 每账号每日打招呼上限
	IntervalMin int    `json:"interval_min"` // 两次动作最小间隔（分钟）
	ActiveStart string `json:"active_start"` // 活跃时段开始 HH:MM
	ActiveEnd   string `json:"active_end"`   // 活跃时段结束 HH:MM
	CoolOn      bool   `json:"cool_on"`      // 达上限自动冷却
	RepeatOn    bool   `json:"repeat_on"`    // 同日陌生人重复忽略
}

// DefaultFrequency 默认频率设置
func DefaultFrequency() Frequency {
	return Frequency{
		DailyLimit:  30,
		IntervalMin: 5,
		ActiveStart: "09:00",
		ActiveEnd:   "22:00",
		CoolOn:      true,
		RepeatOn:    true,
	}
}

func boolVal(s string, def bool) bool {
	if s == "" {
		return def
	}
	return s == "1" || strings.EqualFold(s, "true")
}

// LoadFrequency 读取租户频率设置（缺省回落默认值）
func LoadFrequency(tenantID uint) Frequency {
	f := DefaultFrequency()
	db := database.DB
	var v string
	gets := func(key string) string {
		db.Model(&models.Setting{}).Where("tenant_id = ? AND key = ?", tenantID, key).Pluck("value", &v)
		return v
	}
	if s := gets(models.SettingKsDailyLimit); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			f.DailyLimit = n
		}
	}
	if s := gets(models.SettingKsInterval); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			f.IntervalMin = n
		}
	}
	if s := gets(models.SettingKsActiveStart); s != "" {
		f.ActiveStart = s
	}
	if s := gets(models.SettingKsActiveEnd); s != "" {
		f.ActiveEnd = s
	}
	f.CoolOn = boolVal(gets(models.SettingKsCoolOn), true)
	f.RepeatOn = boolVal(gets(models.SettingKsRepeatOn), true)
	return f
}

// SaveFrequency 保存频率设置（逐 key upsert，复用 Setting 表）
func SaveFrequency(tenantID uint, f Frequency) error {
	if f.DailyLimit < 1 || f.DailyLimit > 200 {
		return errors.New("每日上限需在 1~200 之间")
	}
	if f.IntervalMin < 0 || f.IntervalMin > 1440 {
		return errors.New("最小间隔需在 0~1440 分钟之间")
	}
	if f.ActiveStart == "" || f.ActiveEnd == "" {
		return errors.New("请填写活跃时段")
	}
	db := database.DB
	type kv struct{ k, v string }
	kvs := []kv{
		{models.SettingKsDailyLimit, strconv.Itoa(f.DailyLimit)},
		{models.SettingKsInterval, strconv.Itoa(f.IntervalMin)},
		{models.SettingKsActiveStart, f.ActiveStart},
		{models.SettingKsActiveEnd, f.ActiveEnd},
		{models.SettingKsCoolOn, boolToStr(f.CoolOn)},
		{models.SettingKsRepeatOn, boolToStr(f.RepeatOn)},
	}
	for _, item := range kvs {
		var s models.Setting
		err := db.Where("tenant_id = ? AND key = ?", tenantID, item.k).First(&s).Error
		if err == gorm.ErrRecordNotFound {
			db.Create(&models.Setting{TenantID: tenantID, Key: item.k, Value: item.v})
			continue
		}
		if err != nil {
			return err
		}
		db.Model(&s).Update("value", item.v)
	}
	return nil
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// RefreshAccountState 刷新单个账号状态：跨天清零 + 冷却恢复 + 状态计算。
// 返回是否有变更（需要落库）。与抖音/小红书模块保持同一套口径。
func RefreshAccountState(a *models.KsAccount, f Frequency, now time.Time) bool {
	changed := false
	today := now.Format("2006-01-02")
	// 跨天清零
	if a.TodayDate != today {
		a.TodayUsed = 0
		a.TodayDate = today
		changed = true
	}
	// 冷却恢复
	if a.CooldownUntil != nil && now.After(*a.CooldownUntil) {
		a.CooldownUntil = nil
		changed = true
	}
	// 状态计算
	st := "在线"
	if a.CooldownUntil != nil && now.Before(*a.CooldownUntil) {
		st = "冷却中"
	}
	if a.Status != st {
		a.Status = st
		changed = true
	}
	return changed
}

// CoolUntil 计算冷却截止时间
func CoolUntil(now time.Time, f Frequency) time.Time {
	// 冷却到当天活跃时段结束后（次日重新活跃）
	end := f.ActiveEnd
	t, err := time.Parse("15:04", end)
	if err != nil {
		return now.Add(4 * time.Hour)
	}
	cool := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if !cool.After(now) {
		cool = cool.Add(24 * time.Hour)
	}
	return cool
}
