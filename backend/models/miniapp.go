package models

import "time"

// MiniappBinding 微信小程序绑定关系：openid ↔ 分站账号。
// 客户首次在小程序用分站账号密码绑定后，之后微信登录即可免密进入。
type MiniappBinding struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OpenID    string    `gorm:"size:64;uniqueIndex;not null" json:"open_id"` // 微信 openid
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`             // 绑定的分站
	UserID    uint      `gorm:"index" json:"user_id"`                        // 绑定的分站账号
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
