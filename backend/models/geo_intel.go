package models

import "time"

// FactItem 品牌事实库条目：用于 AI 回答"描述准确率"比对的人工可校正基准。
// 每个条目定义"应被 AI 正确描述的期望事实"与"不应出现的错误/越界表述"。
type FactItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Category  string    `gorm:"size:64" json:"category"`        // 品牌定位/品类/能力/适用场景/不适用/服务边界/竞品边界/可公开案例
	Question  string    `gorm:"size:255" json:"question"`       // 触发场景（可选）
	Fact      string    `gorm:"size:1024;not null" json:"fact"` // 正确事实描述
	NotFact   string    `gorm:"size:1024" json:"not_fact"`      // 错误/越界/过度承诺表述（命中即风险）
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Competitor 竞品库：用于竞品提及统计与 SOV（AI 声量份额）对比。
type Competitor struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Name      string    `gorm:"size:128;not null" json:"name"` // 竞品名称（可逗号分隔同义名）
	Remark    string    `gorm:"size:255" json:"remark"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Citation 引用溯源：从 AI 回答文本中提取的引用 URL 记录，保证"原始答案 + 引用"可追溯。
type Citation struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TenantID     uint      `gorm:"index;not null" json:"tenant_id"`
	ResultID     uint      `gorm:"index;not null" json:"result_id"` // 关联 CheckResult.ID
	TaskID       uint      `gorm:"index" json:"task_id"`
	PlatformName string    `gorm:"size:64;index" json:"platform_name"`
	Question     string    `gorm:"size:255" json:"question"`
	URL          string    `gorm:"size:1024;not null" json:"url"`
	Domain       string    `gorm:"size:255;index" json:"domain"`
	Title        string    `gorm:"size:512" json:"title"`
	Position     int       `json:"position"` // 引用出现顺序（在回答文本中的序号）
	CreatedAt    time.Time `json:"created_at"`
}

// RiskWord 风险词库：命中即认为回答存在风险（幻觉/过度承诺/合规风险）。
// 内置默认词条，租户可增删。
type RiskWord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Word      string    `gorm:"size:128;not null" json:"word"`
	Reason    string    `gorm:"size:255" json:"reason"` // 命中该词代表什么风险
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// TenantPlatformOverride 分站对全局平台的自定义覆盖层：
// 分站不复制平台，只记录「覆盖全局平台的 Key / 启用状态」。
// APIKey 为空 = 沿用全局；Enabled 为 nil = 沿用全局。
type TenantPlatformOverride struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"index;not null" json:"tenant_id"`
	PlatformID uint      `gorm:"index;not null" json:"platform_id"` // 关联 ai_platforms(tenant_id=0) 的 ID
	APIKey     string    `gorm:"size:255" json:"api_key"`           // 加密存储；空=沿用全局
	Enabled    *bool     `json:"enabled"`                           // nil=沿用全局
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// OptTask 优化行动清单：由系统自动生成（缺口/风险/审计结论），供运营逐项执行闭环。
type OptTask struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	TenantID  uint   `gorm:"index;not null" json:"tenant_id"`
	Type      string `gorm:"size:32" json:"type"` // gap(内容缺口)/risk(风险修复)/audit(网站优化)/citation(引用提升)/competitor(竞品应对)
	Title     string `gorm:"size:255;not null" json:"title"`
	Detail    string `gorm:"type:text" json:"detail"`
	Priority  int    `gorm:"default:3" json:"priority"`             // 1最高 ... 5最低
	RiskLevel string `gorm:"size:16;default:low" json:"risk_level"` // low(低风险快速优化)/observe(需观察)/high(高风险技术改造)
	Status    string `gorm:"size:16;default:open" json:"status"`    // open/doing/done
	Source    string `gorm:"size:255" json:"source"`                // 关联的问题/URL，便于追溯
	// 闭环追踪：任务完成不代表生效，需回测验证。
	// DoneAt 记录完成时间；VerifyStatus 记录复测结论（pending 待复测 / improved 已改善 / unchanged 无变化 / worse 变差）。
	// VerifyNote 存复测时的指标快照，避免「标记完成但没人验证」的假闭环。
	DoneAt       *time.Time `json:"done_at"`
	VerifyStatus string     `gorm:"size:16;default:''" json:"verify_status"`
	VerifyNote   string     `gorm:"size:512" json:"verify_note"`
	VerifiedAt   *time.Time `json:"verified_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// AuditResult 网站 GEO 审计记录：抓取站点检查 AI 可读性/结构/引用友好度并打分。
type AuditResult struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"index;not null" json:"tenant_id"`
	URL        string    `gorm:"size:512;not null" json:"url"`
	Score      int       `json:"score"`                       // 综合分 0-100
	Level      string    `gorm:"size:16" json:"level"`        // excellent/good/medium/poor
	Dimensions string    `gorm:"type:text" json:"dimensions"` // JSON：各维度分与说明
	Findings   string    `gorm:"type:text" json:"findings"`   // JSON：问题清单
	CreatedAt  time.Time `json:"created_at"`
}
