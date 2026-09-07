package models

import "time"

// Tenant 分站（租户）。总后台统一管理分站。
type Tenant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`            // 分站名称
	Code      string    `gorm:"size:64;uniqueIndex;not null" json:"code"` // 分站标识（登录/调用时区分）
	Logo      string    `gorm:"size:255" json:"logo"`                     // 分站 Logo（SaaS 端上传，客户端调用）
	Status    int       `gorm:"default:1" json:"status"`                  // 1 启用 / 0 停用
	Features  string    `gorm:"type:text" json:"features"`                // 该分站授权的功能模块 key 列表（JSON 数组字符串，如 ["dashboard","keywords"]；空=全部开放）
	Points    int64     `gorm:"default:0" json:"points"`                  // 点卡余额（每次 AI 调用固定扣 1 点；由总后台充值）
	DailyQueryLimit int `gorm:"default:3" json:"daily_query_limit"`       // 每日查询次数上限（跨小红书/抖音/百度统一配额；高级版本可解锁更多）
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QueryQuota 每日查询配额计数：记录某分站某天某模块已用的查询次数。
// 跨小红书/抖音/百度统一计数（module=total 或具体模块名），达到分站每日上限后拒绝查询。
type QueryQuota struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Day       string    `gorm:"size:16;index;not null" json:"day"`   // 日期 YYYY-MM-DD
	Module    string    `gorm:"size:16;index;not null" json:"module"` // total / baidu / douyin / xhs
	Count     int       `gorm:"default:0" json:"count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExtendRecord 续费流水（对账用）：谁在什么时候给哪个客户（分站主账号）续了几个月。
// Operator 为操作者（当前登录的总后台管理员）；ExpireBefore/After 记录续费前后到期时间，便于审计。
type ExtendRecord struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TenantID     uint       `gorm:"index" json:"tenant_id"`
	TenantName   string     `gorm:"size:128" json:"tenant_name"`
	UserID       uint       `json:"user_id"`
	Username     string     `gorm:"size:64" json:"username"`
	Operator     string     `gorm:"size:64" json:"operator"` // 操作者（总后台管理员）
	Months       int        `json:"months"`                  // 本次续费月数
	PriceFen     int64      `json:"price_fen"`               // 续费时月单价快照（分），配置改价不影响历史
	AmountFen    int64      `json:"amount_fen"`              // 金额 = 月单价 × 月数（分）
	PayMethod    string     `gorm:"size:16" json:"pay_method"` // 收款方式：微信/支付宝/银行转账/现金/赠送
	ExpireBefore *time.Time `json:"expire_before"`           // 续费前到期时间（nil=原不限）
	ExpireAfter  time.Time  `json:"expire_after"`            // 续费后到期时间
	Remark       string     `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time  `json:"created_at"`
}

// PointRecord 点卡流水（按租户隔离）。amount 正=充值 / 负=消费；balance_after 为操作后余额。
type PointRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TenantID     uint      `gorm:"index;not null" json:"tenant_id"`
	Amount       int64     `json:"amount"`              // 正=充值 / 负=消费
	Type         string    `gorm:"size:16" json:"type"` // recharge 充值 / consume 消费
	Remark       string    `gorm:"size:255" json:"remark"`
	BalanceAfter int64     `json:"balance_after"` // 操作后余额
	CreatedAt    time.Time `json:"created_at"`
}

// User 后台账号。TenantID=0 表示总后台超级管理员，>0 表示该分站的管理账号。
type User struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	TenantID   uint       `gorm:"index" json:"tenant_id"`
	Username   string     `gorm:"size:64;uniqueIndex" json:"username"`
	Password   string     `gorm:"size:255;not null" json:"-"`
	Nickname   string     `gorm:"size:64" json:"nickname"`
	Avatar     string     `gorm:"size:255" json:"avatar"` // NFT 数字头像 URL（identicon，注册时自动生成）
	Role       string     `gorm:"size:16;default:admin" json:"role"` // super=总后台 / admin=分站 / operator=AI优化员（受限，仅日常业务，无配置API/查看密钥/改密权限）
	Status     int        `gorm:"default:1" json:"status"`
	OpenMonths int        `json:"open_months"` // 开通月数（1-36）；0 表示未设置/不限
	ExpireAt   *time.Time `json:"expire_at"`   // 服务到期时间（nil 表示不限）
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Notification 站内信（SaaS 端统一推送；客户端按 user/tenant/all 三种范围匹配读取）
// TargetType: "all"=全部用户 / "tenant"=指定租户全员 / "user"=指定用户
// 已读用 ReadUserIDs 记录（JSON 数组字符串 [1,3,5]），避免关联表；读取/标已读都在 Go 侧处理。
// 简化场景适用前提：单租户平均用户数 < 1000，单条站内信已读用户 < 1万；如未来站内信量级暴涨应改为 notification_reads 关联表。
type Notification struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TargetType   string     `gorm:"size:16;index;not null" json:"target_type"`
	TenantID     uint       `gorm:"index" json:"tenant_id"`        // TargetType=tenant 时指向租户
	UserID       uint       `gorm:"index" json:"user_id"`          // TargetType=user 时指向用户
	Title        string     `gorm:"size:128;not null" json:"title"`
	Content      string     `gorm:"type:text" json:"content"`
	ReadUserIDs  string     `gorm:"type:text" json:"-"`            // JSON 数组字符串 [1,3,5]，记录已读用户 id
	CreatedBy    uint       `json:"created_by"`                    // 推送人 user.id（super 管理员 ID）
	CreatedAt    time.Time  `json:"created_at"`
}

// AiPlatform AI 平台配置（按租户隔离）
type AiPlatform struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"index;not null" json:"tenant_id"`
	Name       string    `gorm:"size:64" json:"name"`
	BaseURL    string    `gorm:"size:255" json:"base_url"`
	APIKey     string    `gorm:"size:255" json:"api_key"`
	Model      string    `gorm:"size:64" json:"model"`
	Enabled    bool      `gorm:"default:true" json:"enabled"`
	SortOrder  int       `gorm:"default:0" json:"sort_order"`
	IntervalMs int       `gorm:"default:0" json:"interval_ms"` // 同平台两次请求最小间隔（毫秒）：0=默认800ms节流；用于规避 RPM 限流(429)
	ConfigJSON string    `gorm:"type:text" json:"config_json"` // 扩展配置 JSON
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GeoKeyword GEO 关键词（按租户隔离）
type GeoKeyword struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TenantID      uint      `gorm:"index;not null" json:"tenant_id"`
	Question      string    `gorm:"size:255" json:"question"`
	BrandKeywords string    `gorm:"size:255" json:"brand_keywords"`
	Category      string    `gorm:"size:64" json:"category"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CheckTask 巡检任务（按租户隔离）
type CheckTask struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TenantID     uint       `gorm:"index;not null" json:"tenant_id"`
	Mode         string     `gorm:"size:16" json:"mode"` // manual/auto
	Status       string     `gorm:"size:16" json:"status"`
	TotalQueries int        `json:"total_queries"`
	HitCount     int        `json:"hit_count"`
	MissCount    int        `json:"miss_count"`
	ErrorCount   int        `json:"error_count"`
	Coverage     int        `json:"coverage"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CheckResult 巡检明细（按租户隔离）
type CheckResult struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TenantID      uint      `gorm:"index" json:"tenant_id"`
	TaskID        uint      `gorm:"index" json:"task_id"`
	PlatformID    uint      `json:"platform_id"`
	PlatformName  string    `gorm:"size:64" json:"platform_name"`
	KeywordID     uint      `json:"keyword_id"`
	Question      string    `gorm:"size:255" json:"question"`
	BrandKeywords string    `gorm:"size:255" json:"brand_keywords"`
	Response      string    `gorm:"type:text" json:"response"`
	Hit           bool      `json:"hit"`
	HitPosition   int       `json:"hit_position"`
	MentionCount  int       `json:"mention_count"`
	CostMs        int64     `json:"cost_ms"`
	ErrorMsg      string    `gorm:"type:text" json:"error_msg"`
	CreatedAt     time.Time `json:"created_at"`
}

// Setting 租户级配置，复合主键 (tenant_id, key)
type Setting struct {
	TenantID uint   `gorm:"primaryKey" json:"tenant_id"`
	Key      string `gorm:"primaryKey;size:64" json:"key"`
	Value    string `gorm:"type:text" json:"value"`
}

// RechargeOrder 点卡自助扫码充值订单（微信 Native / 支付宝当面付）。
// status：0 待支付 / 1 已支付 / 2 已关闭 / 3 失败。
// 入账幂等依赖 status 条件更新（WHERE status=0 原子抢占）。
type RechargeOrder struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	OrderNo    string     `gorm:"size:64;uniqueIndex;not null" json:"order_no"` // 商户订单号
	TenantID   uint       `gorm:"index;not null" json:"tenant_id"`              // 充值租户
	Channel    string     `gorm:"size:16;not null" json:"channel"`              // wechat / alipay
	AmountFen  int64      `json:"amount_fen"`                                   // 订单金额（分）
	Points     int64      `json:"points"`                                       // 到账点数
	DailyQueryLimit int   `gorm:"default:0" json:"daily_query_limit"`           // 套餐解锁的每日查询上限（0=不改变）
	Status     int        `gorm:"default:0" json:"status"`                      // 0待支付/1已支付/2已关闭/3失败
	CodeURL    string     `gorm:"type:text" json:"code_url"`                    // 二维码内容（微信 code_url / 支付宝 qr_code）
	PrepayID   string     `gorm:"size:128" json:"prepay_id"`                    // 第三方预下单号（微信 prepay_id / 支付宝 out_trade_no 等同）
	TradeNo    string     `gorm:"size:128" json:"trade_no"`                     // 第三方交易号（支付成功后的交易流水号）
	ExpireTime time.Time  `json:"expire_time"`                                  // 订单过期时间
	PayTime    *time.Time `json:"pay_time"`                                     // 支付成功时间
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// PayCallbackLog 支付回调原始报文日志（用于排查与对账）。
type PayCallbackLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Channel   string    `gorm:"size:16;index" json:"channel"` // wechat / alipay
	OrderNo   string    `gorm:"size:64;index" json:"order_no"`
	RawBody   string    `gorm:"type:text" json:"raw_body"` // 回调原始报文
	VerifyOK  bool      `json:"verify_ok"`                 // 验签是否通过
	CreatedAt time.Time `json:"created_at"`
}

// LoginLog 账号登录日志。记录每次登录尝试（成功/失败、账号、角色、租户、IP、原因、时间）。
// TenantID=0 表示总后台账号；>0 为分站账号。供系统设置页「登录日志」展示。
type LoginLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index" json:"tenant_id"`
	Username  string    `gorm:"size:64;index" json:"username"`
	Nickname  string    `gorm:"size:64" json:"nickname"`
	Role      string    `gorm:"size:16" json:"role"` // super=总后台 / admin=分站 / operator=AI优化员
	IP        string    `gorm:"size:64" json:"ip"`
	Status    string    `gorm:"size:16;index" json:"status"` // success=成功 / fail=失败
	Reason    string    `gorm:"size:255" json:"reason"`      // 失败原因（成功时为"登录成功"）
	CreatedAt time.Time `json:"created_at"`
}

// LoginGuard 登录失败锁定状态（持久化，重启/多实例不丢失）。
// Key = "username|ip"；Until 为零值表示未锁定；LockCount 累计锁定次数用于升级锁定时长。
type LoginGuard struct {
	Key        string    `gorm:"primaryKey;size:128" json:"key"` // username|ip
	Fails      int       `json:"fails"`                          // 当前连续失败次数
	LockCount  int       `json:"lock_count"`                     // 累计锁定次数：1=首次(5分钟)，>=2=再次(1小时)
	Until      time.Time `json:"until"`                          // 锁定期限（零值=未锁定）
	LastActive time.Time `json:"last_active"`                    // 最近一次失败时间，用于长期无活动清理
	UpdatedAt  time.Time `json:"updated_at"`
}

// HelpCategory 帮助文档分类（全局 tenant_id=0，SaaS 后台统一维护）
type HelpCategory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64" json:"name"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HelpDoc 帮助文档（全局 tenant_id=0，属于某个分类）
type HelpDoc struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CategoryID uint      `gorm:"index;not null" json:"category_id"`
	Title      string    `gorm:"size:128" json:"title"`
	Content    string    `gorm:"type:text" json:"content"`
	SortOrder  int       `gorm:"default:0" json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
