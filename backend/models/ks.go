package models

import (
	"time"
)

/* ================================================================
 * 快手获客 · 数据模型（全部按租户 TenantID 隔离，总后台不可见）
 *
 * 合规硬约束（延续抖音/小红书获客既定半自动模式，代码注释 & 页面提示双重体现）：
 *  - 本模块不模拟登录任何快手账号、不自动群发；
 *  - 同步抓取仅面向快手公开页面且尽力而为，失败时降级为结构化估算数据，
 *    并在响应中标记 sourced_from=real/estimate，严禁把估算冒充真实数据；
 *  - 发送动作需人工在官方快手客户端执行：本模块只做「AI 生成话术 +
 *    复制话术 + 标记已打招呼」，绝不代发消息。
 * ================================================================ */

// 频率与安全设置 key（租户级 KV，复用 Setting 表）
const (
	SettingKsDailyLimit  = "ks_daily_limit"   // 每账号每日打招呼上限
	SettingKsInterval    = "ks_interval_min"  // 两次动作最小间隔（分钟）
	SettingKsActiveStart = "ks_active_start"  // 每天活跃时段开始 HH:MM
	SettingKsActiveEnd   = "ks_active_end"    // 每天活跃时段结束 HH:MM
	SettingKsCoolOn      = "ks_cool_on"       // 达到上限后自动进入冷却（1/0）
	SettingKsRepeatOn    = "ks_repeat_on"     // 同日陌生人重复忽略（1/0）
)

// KsAccount 快手获客账号（多账号管理，跨天清零与冷却计算与抖音一致）
type KsAccount struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TenantID      uint       `gorm:"index;not null" json:"tenant_id"`
	Nickname      string     `gorm:"size:128;not null" json:"nickname"` // 昵称
	Region        string     `gorm:"size:64" json:"region"`             // 地区城市，如：河南·郑州
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

// KsPeer 同行追踪（支持快手主页链接导入）
type KsPeer struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TenantID    uint       `gorm:"index;not null" json:"tenant_id"`
	Link        string     `gorm:"size:255" json:"link"`                         // 主页链接（原始）
	HomeID      string     `gorm:"size:128;index" json:"home_id"`                // 主页 ID（user_id / 链接解析结果）
	Nickname    string     `gorm:"size:128" json:"nickname"`                     // 昵称
	FansCount   int64      `json:"fans_count"`                                   // 粉丝数
	VideoCount  int64      `json:"video_count"`                                  // 视频数
	SourcedFrom string     `gorm:"size:16;default:estimate" json:"sourced_from"` // real/estimate
	Status      string     `gorm:"size:16;default:tracking" json:"status"`       // tracking=追踪中
	LastSyncAt  *time.Time `json:"last_sync_at"`                                 // 最近同步时间
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// KsVideo 同行视频数据（播放/点赞/评论，含互动率估算）
type KsVideo struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	TenantID        uint      `gorm:"index;not null" json:"tenant_id"`
	PeerID          uint      `gorm:"index" json:"peer_id"`
	PeerName        string    `gorm:"size:128" json:"peer_name"`
	Title           string    `gorm:"size:255" json:"title"`
	PlayCount       int64     `json:"play_count"`                                     // 播放量
	LikeCount       int64     `json:"like_count"`                                     // 点赞
	CommentCount    int64     `json:"comment_count"`                                  // 评论
	InteractionRate float64   `json:"interaction_rate"`                               // 互动率（折算参考值）
	PublishTime     time.Time `json:"publish_time"`                                   // 发布时间
	SourcedFrom     string    `gorm:"size:16;default:estimate" json:"sourced_from"`   // real/estimate
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// KsLead 评论区客户（留言评论潜在客户；可标记有价值 + 意向备注）
type KsLead struct {
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
	IsValuable  bool       `gorm:"default:false" json:"is_valuable"`             // 是否有价值
	ValueRemark string     `gorm:"size:512" json:"value_remark"`                 // 意向备注
	RepliedAt   *time.Time `json:"replied_at"`                                   // 已回复时间
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// KsSlogan 话术库
type KsSlogan struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Category  string    `gorm:"size:32;default:开场白" json:"category"` // 开场白/进阶/邀约
	Text      string    `gorm:"size:512;not null" json:"text"`       // 文案
	UsedCount int       `gorm:"default:0" json:"used_count"`         // 使用计数
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// KsActionLog 动作日志（时间线记录）
type KsActionLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"index;not null" json:"tenant_id"`
	AccountID   uint      `gorm:"index" json:"account_id"`
	AccountName string    `gorm:"size:128" json:"account_name"`
	ActionType  string    `gorm:"size:32" json:"action_type"` // greet / ai_gen / mark / slogan_use
	Target      string    `gorm:"size:128" json:"target"`     // 目标客户昵称
	Result      string    `gorm:"size:512" json:"result"`     // 结果描述
	CreatedAt   time.Time `json:"created_at"`
}
