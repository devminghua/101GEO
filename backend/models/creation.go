package models

import "time"

/* ================================================================
 * 智能创作中心 · 数据模型（全部按租户 TenantID 隔离，总后台不可见）
 *
 * 覆盖功能：
 *  1. AI 助手对话   —— ChatSession / ChatMessage（多轮会话持久化）
 *  2. 角色设定       —— CreativeRole（预制角色/提示词模板，内置≥6 角色）
 *  3. 文案写作/脚本/小红书/学习/洗稿 —— CreativeRecord 生成记录
 *  4. 素材库         —— CreativeMaterial（保存的文案/脚本等素材）
 *  5. 图片生成       —— CreativeRecord(Kind=image) 保存 base64/URL
 * ================================================================ */

// CreativeRole 智能创作 · 角色设定模板（提示词可控，按租户隔离）
type CreativeRole struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TenantID     uint      `gorm:"index;not null" json:"tenant_id"`
	Name         string    `gorm:"size:64;not null" json:"name"`           // 角色名（如 专业文案）
	Category     string    `gorm:"size:32" json:"category"`                // 分类（文案/脚本/运营/策划…）
	Description  string    `gorm:"size:255" json:"description"`            // 一句话简介
	SystemPrompt string    `gorm:"type:text" json:"system_prompt"`         // 角色系统提示词（可控）
	Builtin      bool      `json:"builtin"`                                // 是否内置（内置可编辑但不可删除）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ChatSession AI 助手对话会话（按租户隔离）
type ChatSession struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TenantID     uint      `gorm:"index;not null" json:"tenant_id"`
	Title        string    `gorm:"size:128" json:"title"`    // 会话标题
	RoleID       uint      `json:"role_id"`                  // 关联角色（0=通用助手）
	RoleName     string    `gorm:"size:64" json:"role_name"` // 角色名快照
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ChatMessage AI 助手对话消息（按租户隔离）
type ChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	SessionID uint      `gorm:"index" json:"session_id"`
	Role      string    `gorm:"size:16" json:"role"` // user / assistant（对话不落 system，角色走 Session.RoleID）
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// CreativeMaterial 素材库（按租户隔离）
type CreativeMaterial struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Title     string    `gorm:"size:128" json:"title"` // 素材标题
	Kind      string    `gorm:"size:32" json:"kind"`   // copy/script/xhs/learn/xiegou/imitate/other
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// CreativeRecord 生成记录（按租户隔离）
// Kind: copy(文案)/script(抖音脚本)/xhs(小红书文案)/learn(深度学习)/xiegou(洗稿原文改写)/image(图片)/video(视频)/imitate(拆解模仿)
type CreativeRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Kind      string    `gorm:"size:32;index" json:"kind"`
	Title     string    `gorm:"size:255" json:"title"`      // 标题/主题
	Prompt    string    `gorm:"type:text" json:"prompt"`    // 输入提示词/正文
	Output    string    `gorm:"type:text" json:"output"`    // 生成结果（文本/JSON/URL）
	Platform  string    `gorm:"size:64" json:"platform"`    // 使用的 AI 平台名
	Model     string    `gorm:"size:64" json:"model"`       // 使用的模型
	TaskID    string    `gorm:"size:128" json:"task_id"`    // 异步任务 ID（视频）
	ImageURL  string    `gorm:"size:255" json:"image_url"`  // 图片/结果访问地址
	ImageB64  string    `gorm:"type:text" json:"-"`         // 图片 base64（不随列表返回）
	Status    string    `gorm:"size:16;default:done" json:"status"` // pending/processing/done/failed
	Error     string    `gorm:"type:text" json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
