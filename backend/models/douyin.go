package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

/* ================================================================
 * 抖音获客 · 数据模型（全部按租户 TenantID 隔离，总后台不可见）
 *
 * 合规硬约束（沿用既定半自动模式）：
 *  - 本模块不模拟登录任何抖音账号、不自动群发；
 *  - 同步抓取仅面向公开数据且尽力而为，失败时降级为结构化估算数据，
 *    并在响应中标记 sourced_from=real/estimate，严禁把估算冒充真实数据；
 *  - 发送动作需人工在官方客户端执行。
 * ================================================================ */

// 客户获取状态机：待跟进 -> 已打招呼 -> 已回复
const (
	LeadStatePending = "待跟进"
	LeadStateGreeted = "已打招呼"
	LeadStateReplied = "已回复"
)

// 账号状态
const (
	AccountOnline   = "在线"
	AccountOffline  = "离线"
	AccountCooldown = "冷却中"
)

// 数据来源标记
const (
	SourceReal     = "real"     // 公开页真实抓取
	SourceEstimate = "estimate" // 结构化解算估值（抓取失败降级）
)

// 话术分类
const (
	SloganCategoryOpen   = "开场白"
	SloganCategoryStep   = "进阶"
	SloganCategoryInvite = "邀约"
)

// 来源类型
const (
	LeadSourceManual       = "manual"        // 手动添加
	LeadSourceCommentParse = "comment_parse" // 视频评论解析
)

// 频率与安全设置 key（租户级 KV，复用 Setting 表）
const (
	SettingDouyinDailyLimit  = "douyin_daily_limit"  // 每账号每日打招呼上限
	SettingDouyinInterval    = "douyin_interval_min" // 两次动作最小间隔（分钟）
	SettingDouyinActiveStart = "douyin_active_start" // 每天活跃时段开始 HH:MM
	SettingDouyinActiveEnd   = "douyin_active_end"   // 每天活跃时段结束 HH:MM
	SettingDouyinCoolOn      = "douyin_cool_on"      // 达到上限后自动进入冷却（1/0）
	SettingDouyinRepeatOn    = "douyin_repeat_on"    // 同日陌生人重复忽略（1/0）
)

// PeerIDs 关联同行 ID 列表（JSON 数组持久化）
type PeerIDs []uint

func (p PeerIDs) Value() (driver.Value, error) {
	if p == nil {
		p = PeerIDs{}
	}
	return json.Marshal(p)
}

func (p *PeerIDs) Scan(v interface{}) error {
	if v == nil {
		*p = PeerIDs{}
		return nil
	}
	switch t := v.(type) {
	case []byte:
		return json.Unmarshal(t, p)
	case string:
		return json.Unmarshal([]byte(t), p)
	default:
		*p = PeerIDs{}
		return nil
	}
}

// DyAccount 抖音获客账号（多账号管理）
type DyAccount struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TenantID      uint       `gorm:"index;not null" json:"tenant_id"`
	Nickname      string     `gorm:"size:128;not null" json:"nickname"` // 昵称
	Region        string     `gorm:"size:64" json:"region"`             // 地区城市，如：广东·深圳
	Status        string     `gorm:"size:16;default:在线" json:"status"`  // 在线/离线/冷却中（读数时按冷却计算刷新）
	DailyLimit    int        `gorm:"default:30" json:"daily_limit"`     // 每日打招呼上限
	TodayUsed     int        `gorm:"default:0" json:"today_used"`       // 今日已用次数
	TodayDate     string     `gorm:"size:16" json:"today_date"`         // 今日计数归属日期（跨天自动清零）
	TotalLeads    int        `gorm:"default:0" json:"total_leads"`      // 累计获客数
	PeerIDs       PeerIDs    `gorm:"type:text" json:"peer_ids"`         // 关联同行 ID 列表
	LastActionAt  *time.Time `json:"last_action_at"`                    // 最后动作时间
	CooldownUntil *time.Time `json:"cooldown_until"`                    // 冷却截止时间（nil=不在冷却）
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// DyPeer 同行追踪（粘贴主页链接解析入库）
type DyPeer struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TenantID    uint       `gorm:"index;not null" json:"tenant_id"`
	Link        string     `gorm:"size:255" json:"link"`                         // 主页链接
	HomeID      string     `gorm:"size:128;index" json:"home_id"`                // 主页 ID（sec_uid / 短链解析结果）
	Nickname    string     `gorm:"size:128" json:"nickname"`                     // 昵称
	FansCount   int64      `json:"fans_count"`                                   // 粉丝数
	VideoCount  int64      `json:"video_count"`                                  // 视频数
	SourcedFrom string     `gorm:"size:16;default:estimate" json:"sourced_from"` // real/estimate
	Status      string     `gorm:"size:16;default:tracking" json:"status"`       // tracking=追踪中
	LastSyncAt  *time.Time `json:"last_sync_at"`                                 // 最近同步时间
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// DyVideo 同行视频数据（抓取/估算，含互动率）
type DyVideo struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	TenantID        uint      `gorm:"index;not null" json:"tenant_id"`
	PeerID          uint      `gorm:"index" json:"peer_id"`
	PeerName        string    `gorm:"size:128" json:"peer_name"`
	Title           string    `gorm:"size:255" json:"title"`
	PlayCount       int64     `json:"play_count"`                                   // 播放
	LikeCount       int64     `json:"like_count"`                                   // 点赞
	CommentCount    int64     `json:"comment_count"`                                // 评论
	InteractionRate float64   `json:"interaction_rate"`                             // 互动率 = (点赞+评论)/播放*100
	PublishTime     time.Time `json:"publish_time"`                                 // 发布时间
	SourcedFrom     string    `gorm:"size:16;default:estimate" json:"sourced_from"` // real/estimate
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DyLead 客户获取（评论区留言潜在客户）
type DyLead struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TenantID    uint       `gorm:"index;not null" json:"tenant_id"`
	Nickname    string     `gorm:"size:128;not null" json:"nickname"` // 昵称
	PeerID      uint       `json:"peer_id"`                           // 来源同行
	PeerName    string     `gorm:"size:128" json:"peer_name"`
	VideoID     uint       `json:"video_id"` // 来源视频
	VideoTitle  string     `gorm:"size:255" json:"video_title"`
	Comment     string     `gorm:"size:512" json:"comment"`                      // 评论内容
	CommentTime *time.Time `json:"comment_time"`                                 // 留言时间
	Tag         string     `gorm:"size:32" json:"tag"`                           // 标签：男·单身/女·单身/家长 等
	State       string     `gorm:"size:16;default:待跟进" json:"state"`             // 状态机：待跟进->已打招呼->已回复
	SourceType  string     `gorm:"size:32;default:manual" json:"source_type"`    // manual/comment_parse
	SourcedFrom string     `gorm:"size:16;default:estimate" json:"sourced_from"` // real/estimate
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// DySlogan 话术库
type DySlogan struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Category  string    `gorm:"size:32;default:开场白" json:"category"` // 开场白/进阶/邀约
	Text      string    `gorm:"size:512;not null" json:"text"`       // 文案
	UsedCount int       `gorm:"default:0" json:"used_count"`         // 使用计数
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DyActionLog 动作日志（时间线记录）
type DyActionLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"index;not null" json:"tenant_id"`
	AccountID   uint      `gorm:"index" json:"account_id"`
	AccountName string    `gorm:"size:128" json:"account_name"`
	ActionType  string    `gorm:"size:32" json:"action_type"` // greet=打招呼 / mark=标记状态 / slogan_use=话术使用
	Target      string    `gorm:"size:128" json:"target"`     // 目标客户昵称
	Result      string    `gorm:"size:512" json:"result"`     // 结果描述
	CreatedAt   time.Time `json:"created_at"`
}
