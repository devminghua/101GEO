package models

import "time"

// Channel 渠道商：可自建分站、自定义品牌与客服联系方式的分销体系。
// 渠道账号（users.role=channel）通过 users.channel_id 关联；其下分站通过 tenants.channel_id 归属。
type Channel struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:64;not null" json:"name"`          // 渠道名称
	BrandName     string    `gorm:"size:128" json:"brand_name"`            // 品牌名（其下分站未自设品牌时的默认品牌）
	Logo          string    `gorm:"size:255" json:"logo"`                  // 品牌 Logo URL
	Copyright     string    `gorm:"size:255" json:"copyright"`             // 版权文案
	ServicePhone  string    `gorm:"size:32" json:"service_phone"`          // 客服电话
	ServiceWechat string    `gorm:"size:128" json:"service_wechat"`        // 客服微信
	Points        int64     `gorm:"default:0" json:"points"`               // 渠道点卡余额（平台充值，渠道可拨付给旗下客户）
	Status        int       `gorm:"default:1" json:"status"`               // 1 启用 / 0 停用
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
