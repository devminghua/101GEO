package models

import "time"

/* ================================================================
 * 内容投放模块 · 数据模型（全部按租户 TenantID 隔离，总后台不可见）
 *
 * 覆盖功能：
 *  1. 全国主流媒体库      —— CtnMedia（内置预设 + 自定义）
 *  2. AI 自动写软文       —— CtnArticle（软文库，AI 生成入库）
 *  3. 软文发布任务        —— CtnTask（自动/人工/半自动发布）
 *  4. 监控发布质量        —— CtnTask(QualityScore) + CtnMonitor（收录检测记录）
 *  5. 百度收录检测        —— CtnMonitor（BaiduIndexed/CheckMethod/SourcedFrom）
 *  6. 效果分析            —— 由 tasks/articles/monitors 聚合
 *
 * 合规硬约束（沿用项目既定半自动模式）：
 *  - 不模拟登录任何媒体后台、不自动群发；
 *  - "自动发布"仅当租户配置了第三方发稿平台 API 时启用（POST 稿件到该接口），
 *    未配置则任务进入 wait_manual（待人工执行）；
 *  - 百度收录检测尽力而为（请求百度公开搜索页），失败降级 estimate
 *    （indexed=null + note 说明），sourced_from=real/estimate 严格区分，
 *    严禁把估算冒充真实收录。
 * ================================================================ */

// 发布任务状态
const (
	CtnTaskPending    = "pending"      // 待发布
	CtnTaskPublishing = "publishing"   // 发布中
	CtnTaskPublished  = "published"    // 已发布
	CtnTaskFailed     = "failed"       // 失败
	CtnTaskWaitManual = "wait_manual"  // 待人工执行
)

// 收录监控同步状态
const (
	CtnSyncUnchecked = "未同步" // 尚未检测
	CtnSyncIndexed   = "已收录"
	CtnSyncNotIndexed = "未收录"
	CtnSyncUnknown   = "无法验证"
)

// CtnMedia 媒体资源库（按租户隔离）
type CtnMedia struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"index;not null" json:"tenant_id"`
	Name       string    `gorm:"size:128;not null" json:"name"`                 // 媒体名称
	Category   string    `gorm:"size:32" json:"category"`                       // 分类：新闻门户/商业门户/自媒体/行业垂直
	Level      string    `gorm:"size:16" json:"level"`                          // 权重：权威/高/中
	URL        string    `gorm:"size:255" json:"url"`                           // 媒体主页/发稿入口链接
	SourceType string    `gorm:"size:16;default:builtin" json:"source_type"`    // builtin=内置预设 / custom=自定义
	Status     int       `gorm:"default:1" json:"status"`                       // 1 启用 / 0 停用
	Remark     string    `gorm:"size:255" json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CtnArticle 软文库（按租户隔离）
type CtnArticle struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TenantID      uint      `gorm:"index;not null" json:"tenant_id"`
	Title         string    `gorm:"size:255;not null" json:"title"`      // 标题
	Content       string    `gorm:"type:text;not null" json:"content"`   // 软文正文
	BrandKeywords string    `gorm:"size:255" json:"brand_keywords"`      // 品牌关键词（逗号分隔）
	Topic         string    `gorm:"size:255" json:"topic"`               // 投放主题/选题
	Category      string    `gorm:"size:32" json:"category"`             // 场景：品牌宣传/活动推广/知识科普/引流获客
	Length        int       `json:"length"`                              // 字数
	Status        string    `gorm:"size:16;default:draft" json:"status"` // draft=草稿 / ready=就绪 / published=已发布
	Platform      string    `gorm:"size:64" json:"platform"`             // AI 平台名（生成来源）
	Model         string    `gorm:"size:64" json:"model"`                // 模型
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CtnTask 发布任务（按租户隔离）
type CtnTask struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TenantID      uint       `gorm:"index;not null" json:"tenant_id"`
	ArticleID     uint       `gorm:"index" json:"article_id"`
	ArticleTitle  string     `gorm:"size:255" json:"article_title"`           // 软文标题快照
	MediaID       uint       `gorm:"index" json:"media_id"`
	MediaName     string     `gorm:"size:128" json:"media_name"`              // 媒体名快照
	PublishMode   string     `gorm:"size:16;default:auto" json:"publish_mode"` // auto=自动对接发稿平台 / manual=人工发布
	Status        string     `gorm:"size:16;default:pending" json:"status"`   // pending/publishing/published/failed/wait_manual
	ScheduledAt   *time.Time `json:"scheduled_at"`                            // 计划发布时间（nil=立即）
	PublishedAt   *time.Time `json:"published_at"`
	PublishURL    string     `gorm:"size:512" json:"publish_url"`             // 发布后的文章链接
	PublishResult string     `gorm:"type:text" json:"publish_result"`         // 发布/提交结果描述
	ReviewStatus  string     `gorm:"size:32;default:未审核" json:"review_status"` // 未审核/待审核/通过/驳回
	QualityScore  int        `gorm:"default:0" json:"quality_score"`          // 发布质量评分 0-100
	QualityMsg    string     `gorm:"size:512" json:"quality_msg"`
	CheckCount    int        `gorm:"default:0" json:"check_count"`            // 收录检测次数
	SyncState     string     `gorm:"size:16;default:未同步" json:"sync_state"`   // 未同步/已收录/未收录/无法验证
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CtnMonitor 监控记录（每次检测一条，按租户隔离）
type CtnMonitor struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TenantID     uint       `gorm:"index;not null" json:"tenant_id"`
	TaskID       uint       `gorm:"index" json:"task_id"`
	TaskTitle    string     `gorm:"size:255" json:"task_title"`
	MediaName    string     `gorm:"size:128" json:"media_name"`
	PublishURL   string     `gorm:"size:512" json:"publish_url"`
	BaiduIndexed *bool      `json:"baidu_indexed"`                        // 是否被百度收录；nil=无法验证
	IndexedAt    *time.Time `json:"indexed_at"`                           // 收录时间
	CheckMethod  string     `gorm:"size:32" json:"check_method"`          // site_query=site搜索 / url_check=直达检测 / estimate=降级估算
	SourcedFrom  string     `gorm:"size:16" json:"sourced_from"`          // real/estimate
	QualityScore int        `json:"quality_score"`
	Note         string     `gorm:"size:512" json:"note"`
	CreatedAt    time.Time  `json:"created_at"`
}
