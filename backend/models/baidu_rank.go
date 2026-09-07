package models

import "time"

// BaiduIndustryRank 百度指数行业排行（每个行业 TOP 品牌的指数值，日榜/周榜，四类指标）。
type BaiduIndustryRank struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	IndustryCode string    `gorm:"size:64;index" json:"industry_code"` // 行业代码 automobile / mobile 等
	IndustryName string    `gorm:"size:64" json:"industry_name"`       // 行业中文名 汽车 / 手机 等
	IndustryEn   string    `gorm:"size:64" json:"industry_en"`         // 行业英文名
	Metric       string    `gorm:"size:32;index" json:"metric"`        // brand / search / news / interact
	Period       string    `gorm:"size:8;index" json:"period"`         // day / week
	Rank         int       `gorm:"index" json:"rank"`                  // 排名（1 起）
	BrandName    string    `gorm:"size:128" json:"brand_name"`         // 品牌名
	IndexValue   float64   `json:"index_value"`                        // 指数值
	StatDate     string    `gorm:"size:10;index" json:"stat_date"`     // 统计日期 YYYY-MM-DD
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BaiduIndustry 行业定义（含展示用配色，配色由前端决定，这里仅存行业元信息）。
type BaiduIndustry struct {
	Code string `json:"code"`
	Name string `json:"name"`
	En   string `json:"en"`
}
