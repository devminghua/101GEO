package models

import "time"

// InviteRecord 邀约奖励：记录邀请关系与奖励结算。
// 邀请码 = 邀请人分站的 code；被邀请人注册成功后，邀请人获得积分奖励。
type InviteRecord struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	InviterID    uint       `gorm:"index;not null" json:"inviter_id"` // 邀请人 tenant_id
	InviteeID    uint       `json:"invitee_id"`                       // 被邀请人 tenant_id（注册成功后回填）
	InviteCode   string     `gorm:"size:64;index" json:"invite_code"` // 邀请码
	Phone        string     `gorm:"size:20;uniqueIndex" json:"phone"` // 被邀请人手机号（唯一，防刷）
	Status       int        `gorm:"default:0" json:"status"`          // 0=待注册 1=已注册奖励
	RewardPoints int64      `gorm:"default:0" json:"reward_points"`   // 本次奖励积分
	CreatedAt    time.Time  `json:"created_at"`
	RewardedAt   *time.Time `json:"rewarded_at"`
}

// CheckinRecord 成长计划·每日签到：记录签到日期与奖励。
type CheckinRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Day       string    `gorm:"size:16;index;not null" json:"day"` // 签到日期 YYYY-MM-DD
	Points    int64     `json:"points"`                            // 本次签到奖励
	Streak    int       `json:"streak"`                            // 连续签到天数
	CreatedAt time.Time `json:"created_at"`
}
