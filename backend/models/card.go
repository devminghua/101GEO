package models

import "time"

// Card 卡密记录（总后台生成，客户端离线激活 / 在线兑换 token）。
// ID 即卡密串内的 card_id 序号；Status 预留在线授权上报用。
type Card struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tier      byte      `gorm:"not null" json:"tier"`          // 1=3天试用 2=12个月 3=充值token
	Code      string    `gorm:"size:256;not null" json:"code"` // 卡密串 LG-...
	Points    int64     `gorm:"default:0" json:"points"`       // 充值卡密(tier=3)兑换的 token 数；授权卡密为 0
	Status    int       `gorm:"default:0" json:"status"`       // 0未使用 1已使用（激活或兑换后标记）
	Remark    string    `gorm:"size:255" json:"remark"`        // 批次备注
	CreatedAt time.Time `json:"created_at"`
}
