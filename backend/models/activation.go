package models

import "time"

// Activation 单机版软件级激活状态（单行记录，非多租户）。
// 卡密激活后写入一条记录：卡密 ID / 类型 / 激活时间 / 到期时间；
// 未激活时表为空（无记录），由授权守卫据此判定为「未激活」。
type Activation struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CardID      uint32    `gorm:"not null" json:"card_id"` // 已激活卡密序号（防重复激活）
	Tier        byte      `gorm:"not null" json:"tier"`    // 卡密类型（1=3天试用 2=12个月）
	ActivatedAt time.Time `json:"activated_at"`            // 激活时间（首次激活计时的起点）
	ExpireAt    time.Time `gorm:"index" json:"expire_at"`  // 到期时间
	MachineID   string    `gorm:"size:64" json:"machine_id"` // 机器码（预留；离线版可空）
	Sig         string    `gorm:"size:64" json:"-"`          // 授权数据 HMAC 签名（防改库篡改）
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
