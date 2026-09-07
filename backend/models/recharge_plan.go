package models

import "time"

// RechargePlan 价格套餐（充值中心「选择套餐」充值）。
// 总后台设置套餐（名称/token 数/售价/标签/启用），客户端选套餐扫码充值，享受套餐优惠价。
type RechargePlan struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`      // 套餐名，如"基础包"
	Points    int64     `gorm:"not null" json:"points"`            // token 数
	PriceFen  int64     `gorm:"not null" json:"price_fen"`         // 售价（分）
	OrigFen   int64     `gorm:"default:0" json:"orig_fen"`         // 原价（分），用于展示优惠，0=无原价
	Tag       string    `gorm:"size:32" json:"tag"`                // 标签，如"最受欢迎"
	DailyQueryLimit int `gorm:"default:0" json:"daily_query_limit"` // 购买后解锁的每日查询上限（0=不改变客户当前上限）
	Enabled   bool      `gorm:"default:true" json:"enabled"`       // 是否启用
	Sort      int       `gorm:"default:0" json:"sort"`             // 排序（越小越靠前）
	CreatedAt time.Time `json:"created_at"`
}
