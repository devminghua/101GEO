package models

import (
	"time"
)

/* ================================================================
 * 小红书获客 · 数据模型（全部按租户 TenantID 隔离，总后台不可见）
 *
 * 合规硬约束（延续抖音获客既定半自动模式，代码注释 & 页面提示双重体现）：
 *  - 本模块不模拟登录任何小红书账号、不自动群发；
 *  - 同步抓取仅面向小红书公开数据且尽力而为，失败时降级为结构化估算数据，
 *    并在响应中标记 sourced_from=real/estimate，严禁把估算冒充真实数据；
 *  - 发送动作需人工在官方小红书客户端执行：本模块只做「AI 生成话术 +
 *    复制话术 + 标记已打招呼」，绝不代发消息。
 * ================================================================ */

// 频率与安全设置 key（租户级 KV，复用 Setting 表）
const (
	SettingXhsDailyLimit  = "xhs_daily_limit"  // 每账号每日打招呼上限
	SettingXhsInterval    = "xhs_interval_min" // 两次动作最小间隔（分钟）
	SettingXhsActiveStart = "xhs_active_start" // 每天活跃时段开始 HH:MM
	SettingXhsActiveEnd   = "xhs_active_end"   // 每天活跃时段结束 HH:MM
	SettingXhsCoolOn      = "xhs_cool_on"      // 达到上限后自动进入冷却（1/0）
	SettingXhsRepeatOn    = "xhs_repeat_on"    // 同日陌生人重复忽略（1/0）
)

// XhsAccount 小红书获客账号（多账号管理，跨天清零与冷却计算与抖音一致）
type XhsAccount struct {
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

// XhsPeer 同行追踪（支持小红书主页链接与 xhslink.com 短链导入解析）
type XhsPeer struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TenantID    uint       `gorm:"index;not null" json:"tenant_id"`
	Link        string     `gorm:"size:255" json:"link"`                         // 主页链接（原始）
	HomeID      string     `gorm:"size:128;index" json:"home_id"`                // 主页 ID（user_id / 短链解析结果）
	Nickname    string     `gorm:"size:128" json:"nickname"`                     // 昵称
	FansCount   int64      `json:"fans_count"`                                   // 粉丝数
	NoteCount   int64      `json:"note_count"`                                   // 笔记数
	SourcedFrom string     `gorm:"size:16;default:estimate" json:"sourced_from"` // real/estimate
	Status      string     `gorm:"size:16;default:tracking" json:"status"`       // tracking=追踪中
	LastSyncAt  *time.Time `json:"last_sync_at"`                                 // 最近同步时间
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// XhsNote 同行笔记数据（点赞/收藏/评论，含互动率估算）
type XhsNote struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	TenantID        uint      `gorm:"index;not null" json:"tenant_id"`
	PeerID          uint      `gorm:"index" json:"peer_id"`
	PeerName        string    `gorm:"size:128" json:"peer_name"`
	Title           string    `gorm:"size:255" json:"title"`
	LikeCount       int64     `json:"like_count"`                                   // 点赞
	SaveCount       int64     `json:"save_count"`                                   // 收藏
	CommentCount    int64     `json:"comment_count"`                                // 评论
	InteractionRate float64   `json:"interaction_rate"`                             // 互动率（折算参考值，计算方式见 estimate.go）
	PublishTime     time.Time `json:"publish_time"`                                 // 发布时间
	SourcedFrom     string    `gorm:"size:16;default:estimate" json:"sourced_from"` // real/estimate
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// XhsLead 评论区客户（留言评论潜在客户；可标记有价值 + 意向备注）
type XhsLead struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TenantID    uint       `gorm:"index;not null" json:"tenant_id"`
	Nickname    string     `gorm:"size:128;not null" json:"nickname"` // 昵称
	PeerID      uint       `json:"peer_id"`                           // 来源同行
	PeerName    string     `gorm:"size:128" json:"peer_name"`
	NoteID      uint       `json:"note_id"` // 来源笔记
	NoteTitle   string     `gorm:"size:255" json:"note_title"`
	Comment     string     `gorm:"size:512" json:"comment"`                      // 评论内容
	CommentTime *time.Time `json:"comment_time"`                                 // 留言时间
	Tag         string     `gorm:"size:32" json:"tag"`                           // 标签：男·单身/女·单身/家长 等
	State       string     `gorm:"size:16;default:待跟进" json:"state"`             // 状态机：待跟进->已打招呼->已回复
	SourceType  string     `gorm:"size:32;default:manual" json:"source_type"`    // manual/comment_parse
	SourcedFrom string     `gorm:"size:16;default:estimate" json:"sourced_from"` // real/estimate
	IsValuable  bool       `gorm:"default:false" json:"is_valuable"`             // 是否有价值（客户已回复且价值高）
	ValueRemark string     `gorm:"size:512" json:"value_remark"`                 // 意向备注（有价值时的备注）
	RepliedAt   *time.Time `json:"replied_at"`                                   // 已回复时间
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// XhsSlogan 话术库
type XhsSlogan struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Category  string    `gorm:"size:32;default:开场白" json:"category"` // 开场白/进阶/邀约
	Text      string    `gorm:"size:512;not null" json:"text"`       // 文案
	UsedCount int       `gorm:"default:0" json:"used_count"`         // 使用计数
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// XhsActionLog 动作日志（时间线记录，含 AI 生成话术/打招呼/标记价值等动作）
type XhsActionLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"index;not null" json:"tenant_id"`
	AccountID   uint      `gorm:"index" json:"account_id"`
	AccountName string    `gorm:"size:128" json:"account_name"`
	ActionType  string    `gorm:"size:32" json:"action_type"` // greet=打招呼 / ai_gen=AI生成话术 / mark=标记状态/价值 / slogan_use=话术使用
	Target      string    `gorm:"size:128" json:"target"`     // 目标客户昵称
	Result      string    `gorm:"size:512" json:"result"`     // 结果描述
	CreatedAt   time.Time `json:"created_at"`
}

// XhsNotification 站内通知（「发给管理员」的落地形式；receiver_role=admin 供分站管理员查看）
type XhsNotification struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TenantID     uint      `gorm:"index;not null" json:"tenant_id"`
	ReceiverRole string    `gorm:"size:16;default:admin" json:"receiver_role"` // admin=分站管理员
	Title        string    `gorm:"size:128" json:"title"`                      // 标题
	Content      string    `gorm:"size:2048" json:"content"`                   // 内容（价值客户统计摘要）
	IsRead       bool      `gorm:"default:false" json:"is_read"`               // 已读状态
	CreatedAt    time.Time `json:"created_at"`
}
