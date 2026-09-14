package models

import "time"

/* ================================================================
 * 成功案例（SaaS 端上传 → 客户端展示）
 *
 * TenantID 语义：
 *  0        = 平台级案例（SaaS 端上传，所有分站客户端可见）
 *  >0       = 预留：分站自己的案例（当前未开放，仅平台级）
 * Status：1=发布（客户端可见），0=草稿（仅 SaaS 端可见）
 * ================================================================ */

// CaseStudy 成功案例
type CaseStudy struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"` // 0=平台级
	Title     string    `gorm:"size:128;not null" json:"title"`  // 案例标题
	Summary   string    `gorm:"size:512" json:"summary"`         // 摘要（卡片列表展示）
	Content   string    `gorm:"type:text" json:"content"`        // 正文（支持换行，详情展示）
	CoverURL  string    `gorm:"size:512" json:"cover_url"`       // 封面图 URL
	Tags      string    `gorm:"size:255" json:"tags"`            // 标签，逗号分隔，如：婚恋,GEO
	Status    int       `json:"status"`             // 1=发布 0=草稿（零值有语义，禁止 gorm default）
	SortOrder int       `gorm:"default:0" json:"sort_order"`     // 排序（越大越靠前）
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
