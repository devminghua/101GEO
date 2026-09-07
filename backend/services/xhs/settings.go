package xhs

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
 * 小红书获客 · 频率与安全设置（租户级 KV，复用 models.Setting 表）
 *  - 每账号每日打招呼上限
 *  - 两次动作最小间隔（分钟）
 *  - 每天活跃时段起止（HH:MM）
 *  - 达上限自动冷却开关
 *  - 同日陌生人重复忽略开关
 * ================================================================ */

// Frequency 频率与安全设置（解析后的强类型承载，key 前缀 xhs_）
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
	if s := gets(models.SettingXhsDailyLimit); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			f.DailyLimit = n
		}
	}
	if s := gets(models.SettingXhsInterval); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			f.IntervalMin = n
		}
	}
	if s := gets(models.SettingXhsActiveStart); s != "" {
		f.ActiveStart = s
	}
	if s := gets(models.SettingXhsActiveEnd); s != "" {
		f.ActiveEnd = s
	}
	f.CoolOn = boolVal(gets(models.SettingXhsCoolOn), true)
	f.RepeatOn = boolVal(gets(models.SettingXhsRepeatOn), true)
	return f
}

// SaveFrequency 保存租户频率设置（upsert）
func SaveFrequency(tenantID uint, f Frequency) error {
	db := database.DB
	pairs := []struct {
		key   string
		value string
	}{
		{models.SettingXhsDailyLimit, strconv.Itoa(f.DailyLimit)},
		{models.SettingXhsInterval, strconv.Itoa(f.IntervalMin)},
		{models.SettingXhsActiveStart, f.ActiveStart},
		{models.SettingXhsActiveEnd, f.ActiveEnd},
		{models.SettingXhsCoolOn, boolTo01(f.CoolOn)},
		{models.SettingXhsRepeatOn, boolTo01(f.RepeatOn)},
	}
	for _, p := range pairs {
		var setting models.Setting
		err := db.Where("tenant_id = ? AND key = ?", tenantID, p.key).First(&setting).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			setting = models.Setting{TenantID: tenantID, Key: p.key, Value: p.value}
			if e := db.Create(&setting).Error; e != nil {
				return e
			}
		} else if err != nil {
			return err
		} else {
			setting.Value = p.value
			if e := db.Save(&setting).Error; e != nil {
				return e
			}
		}
	}
	return nil
}

func boolTo01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// InActiveWindow 判断当前时间是否在活跃时段内（HH:MM 按当天比较）
func InActiveWindow(f Frequency, now time.Time) bool {
	startMin := parseHM(f.ActiveStart)
	endMin := parseHM(f.ActiveEnd)
	nowMin := now.Hour()*60 + now.Minute()
	return nowMin >= startMin && nowMin <= endMin
}

func parseHM(s string) int {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	h, err1 := strconv.Atoi(parts[0])
	if err1 != nil {
		return 0
	}
	m := 0
	if len(parts) > 1 {
		m, _ = strconv.Atoi(parts[1])
	}
	return h*60 + m
}

// CooldownNextDayStart 计算"达到上限"后的冷却截止时间：次日活跃时段开始
func CooldownNextDayStart(f Frequency, now time.Time) time.Time {
	startMin := parseHM(f.ActiveStart)
	next := now.AddDate(0, 0, 1)
	return time.Date(next.Year(), next.Month(), next.Day(), startMin/60, startMin%60, 0, 0, now.Location())
}
