package models

import (
	"time"
)

/* ================================================================
 * 百度关键词分析 · 数据模型（全部按租户 TenantID 隔离，总后台不可见）
 *
 * 三张表：
 *  1. BaiduSite            客户网站表（域名/名称/是否启用）
 *  2. BaiduMonitorKeyword  监控关键词表（关键词文本/关联网站/是否启用）
 *  3. BaiduRankSnapshot    百度排名历史快照表（关键词/网站域名/最佳排名/出现次数/抓取日期）
 * ================================================================ */

// BaiduSite 客户网站表：客户自己添加自己的网站地址，用于百度排名监控。
type BaiduSite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Domain    string    `gorm:"size:255;not null" json:"domain"` // 网站域名，如 example.com（去 www）
	Name      string    `gorm:"size:128" json:"name"`             // 网站名称，便于识别
	Enabled   bool      `gorm:"default:true" json:"enabled"`      // 是否启用监控
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BaiduMonitorKeyword 监控关键词表：客户添加要监控排名的关键词。
type BaiduMonitorKeyword struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"index;not null" json:"tenant_id"`
	Keyword    string    `gorm:"size:255;not null" json:"keyword"` // 关键词文本
	SiteID     uint      `gorm:"index" json:"site_id"`             // 关联网站 ID（BaiduSite.ID）
	SiteDomain string    `gorm:"size:255" json:"site_domain"`      // 关联网站域名（冗余，便于查询展示）
	Enabled    bool      `gorm:"default:true" json:"enabled"`      // 是否启用监控
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// BaiduRankSnapshot 百度排名历史快照表：每次分析后写入该词该域名当日最佳排名。
// 去重口径：同 TenantID + Keyword + SiteDomain + Date 唯一，有则更新无则插入。
type BaiduRankSnapshot struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"index;not null" json:"tenant_id"`
	Keyword     string    `gorm:"size:255;index;not null" json:"keyword"`                 // 关键词
	SiteDomain  string    `gorm:"size:255;index;not null" json:"site_domain"`             // 网站域名
	BestRank    int       `gorm:"default:0" json:"best_rank"`                             // 该词该域名下最佳排名（0=未上榜）
	Occurrences int       `gorm:"default:0" json:"occurrences"`                           // 该词该域名出现次数
	Date        string    `gorm:"size:10;index;not null" json:"date"`                     // 抓取日期 YYYY-MM-DD
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 覆盖默认表名，避免与未来其它表冲突。
func (BaiduSite) TableName() string            { return "baidu_sites" }
func (BaiduMonitorKeyword) TableName() string  { return "baidu_monitor_keywords" }
func (BaiduRankSnapshot) TableName() string    { return "baidu_rank_snapshots" }
