// Package notify 告警通知：企微/钉钉群机器人 webhook 推送。
// 每日定时汇总「客户到期预警 + 点数不足」推送到配置的机器人；配置存 settings（tenant_id=0）。
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"geo-tool/database"
	"geo-tool/models"
)

// 通知配置 Key（tenant_id=0 的 KV）
const (
	KeyNotifyEnabled        = "notify_enabled"         // 总开关：1 / 0
	KeyNotifyWecomWebhook   = "notify_wecom_webhook"   // 企微群机器人 webhook 地址
	KeyNotifyDingWebhook    = "notify_dingtalk_webhook" // 钉钉群机器人 webhook 地址
	KeyNotifyExpiryDays     = "notify_expiry_days"      // 到期预警天数阈值，默认 30
	KeyNotifyPointsFloor    = "notify_points_floor"     // 点数不足预警阈值，默认 20
	KeyNotifyPushHour       = "notify_push_hour"        // 每日推送时间（小时，0-23），默认 9
)

// 默认值
const (
	DefaultExpiryDays  = 30
	DefaultPointsFloor = 20
	DefaultPushHour    = 9
)

// Config 通知配置（非敏感，直接明文存取）
type Config struct {
	Enabled      bool
	WecomWebhook string
	DingWebhook  string
	ExpiryDays   int
	PointsFloor  int
	PushHour     int
}

// LoadConfig 从全局设置读取通知配置
func LoadConfig() Config {
	m := loadSettings()
	cfg := Config{
		Enabled:      m[KeyNotifyEnabled] == "1",
		WecomWebhook: strings.TrimSpace(m[KeyNotifyWecomWebhook]),
		DingWebhook:  strings.TrimSpace(m[KeyNotifyDingWebhook]),
		ExpiryDays:   atoiDefault(m[KeyNotifyExpiryDays], DefaultExpiryDays),
		PointsFloor:  atoiDefault(m[KeyNotifyPointsFloor], DefaultPointsFloor),
		PushHour:     atoiDefault(m[KeyNotifyPushHour], DefaultPushHour),
	}
	if cfg.ExpiryDays < 1 {
		cfg.ExpiryDays = DefaultExpiryDays
	}
	if cfg.PointsFloor < 1 {
		cfg.PointsFloor = DefaultPointsFloor
	}
	if cfg.PushHour < 0 || cfg.PushHour > 23 {
		cfg.PushHour = DefaultPushHour
	}
	return cfg
}

func loadSettings() map[string]string {
	m := map[string]string{}
	var settings []models.Setting
	if err := database.DB.Where("tenant_id = ?", 0).Find(&settings).Error; err != nil {
		return m
	}
	for _, s := range settings {
		m[s.Key] = s.Value
	}
	return m
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// sendText 向 webhook 推送文本消息。
// 企微与钉钉群机器人的 text 消息体结构一致：{"msgtype":"text","text":{"content":"..."}}
func sendText(webhook, text string) error {
	if webhook == "" {
		return fmt.Errorf("webhook 地址未配置")
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	})
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhook, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 企微/钉钉在 HTTP 200 内也可能返回业务错误码，解析 errcode
	var body struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && body.ErrCode != 0 {
		return fmt.Errorf("errcode=%d %s", body.ErrCode, body.ErrMsg)
	}
	return nil
}

// SendAll 向所有已配置渠道推送文本，返回（成功渠道数, 首个错误）
func SendAll(cfg Config, text string) (int, error) {
	ok := 0
	var firstErr error
	if cfg.WecomWebhook != "" {
		if err := sendText(cfg.WecomWebhook, text); err != nil {
			firstErr = err
		} else {
			ok++
		}
	}
	if cfg.DingWebhook != "" {
		if err := sendText(cfg.DingWebhook, text); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			ok++
		}
	}
	if ok == 0 && firstErr == nil {
		firstErr = fmt.Errorf("未配置任何机器人 webhook")
	}
	return ok, firstErr
}

// expiryRow 到期预警行（口径与总览看板一致：每分站最早创建的 admin 账号）
type expiryRow struct {
	TenantName string     `json:"tenant_name"`
	TenantCode string     `json:"tenant_code"`
	Username   string     `json:"username"`
	RemainDays int        `json:"remain_days"`
	ExpireAt   *time.Time `json:"expire_at"` // 到期时间（SQL 已过滤非空）
}

// BuildDailyMessage 汇总当前告警内容（到期预警 + 点数不足），无告警返回空串
func BuildDailyMessage(cfg Config, now time.Time) string {
	var expiries []expiryRow
	threshold := now.AddDate(0, 0, cfg.ExpiryDays)
	database.DB.Raw(`
		SELECT t.name AS tenant_name, t.code AS tenant_code, u.username,
			u.expire_at AS expire_at
		FROM users u JOIN tenants t ON t.id = u.tenant_id
		WHERE u.tenant_id > 0 AND u.role = 'admin' AND u.status = 1
			AND u.id = (SELECT MIN(id) FROM users x WHERE x.tenant_id = u.tenant_id AND x.role = 'admin')
			AND u.expire_at IS NOT NULL
			AND u.expire_at <= ?
		ORDER BY u.expire_at ASC`, threshold).Scan(&expiries)
	// remain_days 在 Go 端计算（跨库一致，不依赖数据库方言函数）
	for i := range expiries {
		if expiries[i].ExpireAt != nil {
			expiries[i].RemainDays = int(expiries[i].ExpireAt.Sub(now).Hours() / 24)
		}
	}

	var lowPoints []models.Tenant
	database.DB.Where("points <= ? AND status = ?", cfg.PointsFloor, 1).Order("points asc").Find(&lowPoints)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("【LinkGeo 运营告警】%s\n", now.Format("2006-01-02")))
	if len(expiries) > 0 {
		b.WriteString(fmt.Sprintf("\n■ 到期预警（%d 家，%d 天内到期或已到期）\n", len(expiries), cfg.ExpiryDays))
		for _, e := range expiries {
			expDay := ""
			if e.ExpireAt != nil {
				expDay = e.ExpireAt.Format("2006-01-02")
			}
			if e.RemainDays < 0 {
				b.WriteString(fmt.Sprintf("· %s（%s）已到期 %d 天\n", e.TenantName, e.Username, -e.RemainDays))
			} else if e.RemainDays == 0 {
				b.WriteString(fmt.Sprintf("· %s（%s）今天到期\n", e.TenantName, e.Username))
			} else {
				b.WriteString(fmt.Sprintf("· %s（%s）剩 %d 天，%s 到期\n", e.TenantName, e.Username, e.RemainDays, expDay))
			}
		}
	}
	if len(lowPoints) > 0 {
		b.WriteString(fmt.Sprintf("\n■ 点数不足（%d 家，余额 ≤ %d 点）\n", len(lowPoints), cfg.PointsFloor))
		for _, t := range lowPoints {
			b.WriteString(fmt.Sprintf("· %s（%s）剩余 %d 点\n", t.Name, t.Code, t.Points))
		}
	}
	if len(expiries) == 0 && len(lowPoints) == 0 {
		return ""
	}
	b.WriteString("\n—— 来自 LinkGeo SaaS 总后台")
	return b.String()
}

// lastPushDay 进程内去重（当日已推过不再推）
var lastPushDay string

// RunDailyIfDue 每日到点推送一次；返回是否执行了推送
func RunDailyIfDue(now time.Time) bool {
	cfg := LoadConfig()
	if !cfg.Enabled {
		return false
	}
	today := now.Format("2006-01-02")
	if now.Hour() != cfg.PushHour || lastPushDay == today {
		return false
	}
	msg := BuildDailyMessage(cfg, now)
	lastPushDay = today // 无论有无告警都标记，避免整点重复触发
	if msg == "" {
		return false
	}
	if _, err := SendAll(cfg, msg); err != nil {
		// 推送失败不重试（下一小时也不会再推，明日再看）；错误由调用方日志记录
		_ = err
	}
	return true
}
