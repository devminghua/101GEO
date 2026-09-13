package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
)

// settleInvite 被邀请人注册成功后结算奖励（2026-09-13 老板拍板规则）：
//   - 邀请人：+2000 token + 服务延长 1 个月（上不封顶）
//   - 被邀请人：注册成功自动充值 2000 token（由 register.go 根据返回值发放）
// 防刷：一个手机号全局只能被邀请一次（Phone 唯一索引）；邀请人不能是自己。
// 返回是否首次结算成功（false=无效邀请码/重复/自邀，被邀请人不发新人奖励）。
func settleInvite(refCode string, inviteeID uint, phone string) bool {
	var inviter models.Tenant
	if err := database.DB.Where("code = ? AND status = ?", refCode, 1).First(&inviter).Error; err != nil {
		return false // 邀请码无效
	}
	if inviter.ID == inviteeID {
		return false // 不能邀请自己
	}
	// 手机号防刷：已存在该手机号的邀请记录则不重复奖励
	var dup int64
	database.DB.Model(&models.InviteRecord{}).Where("phone = ?", phone).Count(&dup)
	if dup > 0 {
		return false
	}
	reward := inviteRewardPoints()
	now := time.Now()
	tx := database.DB.Begin()
	if err := tx.Create(&models.InviteRecord{
		InviterID: inviter.ID, InviteeID: inviteeID, InviteCode: refCode,
		Phone: phone, Status: 1, RewardPoints: reward, RewardedAt: &now,
	}).Error; err != nil {
		tx.Rollback()
		return false
	}
	// 邀请人 +2000 token
	newBal := inviter.Points + reward
	tx.Model(&inviter).Update("points", newBal)
	tx.Create(&models.PointRecord{
		TenantID: inviter.ID, Amount: reward, Type: "recharge",
		Remark: "邀约奖励（邀请新客户注册）", BalanceAfter: newBal,
	})
	// 邀请人服务延长 1 个月：给分站 admin 账号的 ExpireAt 顺延 30 天（上不封顶）。
	// 取最新创建的 admin（Order id desc）：分站可能存在多个 admin（如测试账号），
	// 应延到实际运营账号上（v1.0.53 实测：First 按主键升序取到了 demo 测试账号）。
	var admin models.User
	if err := tx.Where("tenant_id = ? AND role = ?", inviter.ID, "admin").Order("id desc").First(&admin).Error; err == nil {
		base := time.Now()
		if admin.ExpireAt != nil && admin.ExpireAt.After(base) {
			base = *admin.ExpireAt
		}
		newExpire := base.AddDate(0, 1, 0)
		tx.Model(&admin).Updates(map[string]interface{}{
			"expire_at":   newExpire,
			"open_months": admin.OpenMonths + 1,
		})
	}
	tx.Commit()
	return true
}

// inviteRewardPoints 每次成功邀约的奖励 token（默认 2000，可全局设置 invite_reward_points 覆盖）
func inviteRewardPoints() int64 {
	v := int64(intSetting("invite_reward_points", 2000))
	if v <= 0 {
		return 2000
	}
	return v
}

// InviteSummary 邀约奖励总览：GET /api/invite/summary
// 返回我的邀请码、累计邀请数、累计奖励积分、邀请记录列表。
func InviteSummary(c *gin.Context) {
	tid := TenantID(c)
	if tid == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"super": true}})
		return
	}
	var tenant models.Tenant
	database.DB.First(&tenant, tid)
	code := tenant.Code

	var totalInvited, totalReward int64
	database.DB.Model(&models.InviteRecord{}).Where("inviter_id = ? AND status = ?", tid, 1).Count(&totalInvited)
	database.DB.Model(&models.InviteRecord{}).Where("inviter_id = ? AND status = ?", tid, 1).
		Select("COALESCE(SUM(reward_points),0)").Scan(&totalReward)

	var records []models.InviteRecord
	database.DB.Where("inviter_id = ?", tid).Order("id desc").Limit(100).Find(&records)

	// 被邀请人注册后的展示名（分站名称）
	type rec struct {
		ID           uint    `json:"id"`
		Phone        string  `json:"phone"`
		Status       int     `json:"status"`
		RewardPoints int64   `json:"reward_points"`
		CompanyName  string  `json:"company_name"`
		CreatedAt    string  `json:"created_at"`
	}
	out := make([]rec, 0, len(records))
	for _, r := range records {
		company := ""
		if r.InviteeID > 0 {
			var t models.Tenant
			if database.DB.First(&t, r.InviteeID).Error == nil {
				company = t.Name
			}
		}
		out = append(out, rec{
			ID: r.ID, Phone: maskPhone(r.Phone), Status: r.Status,
			RewardPoints: r.RewardPoints, CompanyName: company,
			CreatedAt: r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"invite_code":    code,
		"total_invited":  totalInvited,
		"total_reward":   totalReward,
		"reward_per":     inviteRewardPoints(),
		"records":        out,
	}})
}

// maskPhone 手机号脱敏：138****1234
func maskPhone(p string) string {
	if len(p) < 7 {
		return p
	}
	return p[:3] + "****" + p[len(p)-4:]
}

// GrowthSummary 成长计划总览：GET /api/growth/summary
// 返回等级、当前积分、连续签到天数、本月签到记录、今日是否已签到。
func GrowthSummary(c *gin.Context) {
	tid := TenantID(c)
	if tid == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"super": true}})
		return
	}
	var tenant models.Tenant
	database.DB.First(&tenant, tid)
	points := tenant.Points

	// 连续签到天数（从今天往前推）
	streak := 0
	for i := 0; ; i++ {
		day := biztime.Day(-i)
		var c int64
		database.DB.Model(&models.CheckinRecord{}).Where("tenant_id = ? AND day = ?", tid, day).Count(&c)
		if c == 0 {
			if i == 0 {
				// 今天没签，看昨天是否连续（用于展示"昨天已断"）——简化：今天没签 streak=0
			}
			break
		}
		streak++
	}

	// 今日是否已签到
	today := biztime.Today()
	var todayCount int64
	database.DB.Model(&models.CheckinRecord{}).Where("tenant_id = ? AND day = ?", tid, today).Count(&todayCount)

	// 本月签到记录
	monthPrefix := biztime.Month()
	var checkins []models.CheckinRecord
	database.DB.Where("tenant_id = ? AND day LIKE ?", tid, monthPrefix+"%").Order("day desc").Find(&checkins)

	level, levelName, nextNeed := growthLevel(points)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"points":        points,
		"level":         level,
		"level_name":    levelName,
		"next_need":     nextNeed,
		"streak":        streak,
		"today_checked": todayCount > 0,
		"checkins":      checkins,
	}})
}

// growthLevel 成长等级（按累计积分）
func growthLevel(points int64) (level int, name string, nextNeed int64) {
	switch {
	case points >= 20000:
		return 4, "铂金会员", 0
	case points >= 5000:
		return 3, "黄金会员", 20000 - points
	case points >= 1000:
		return 2, "白银会员", 5000 - points
	default:
		return 1, "青铜会员", 1000 - points
	}
}

// Checkin 每日签到：POST /api/growth/checkin
// 连续签到奖励递增：第 1 天 5 点，之后每天 +2（封顶 50 点）。
func Checkin(c *gin.Context) {
	tid := TenantID(c)
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "总后台账号无签到"})
		return
	}
	today := biztime.Today()
	var todayCount int64
	database.DB.Model(&models.CheckinRecord{}).Where("tenant_id = ? AND day = ?", tid, today).Count(&todayCount)
	if todayCount > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "今日已签到，明天再来"})
		return
	}
	// 连续签到天数（昨天往前）
	streak := 0
	for i := 1; ; i++ {
		day := biztime.Day(-i)
		var n int64
		database.DB.Model(&models.CheckinRecord{}).Where("tenant_id = ? AND day = ?", tid, day).Count(&n)
		if n == 0 {
			break
		}
		streak++
	}
	newStreak := streak + 1
	reward := int64(5 + (newStreak-1)*2)
	if reward > 50 {
		reward = 50
	}

	tx := database.DB.Begin()
	if err := tx.Create(&models.CheckinRecord{TenantID: tid, Day: today, Points: reward, Streak: newStreak}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "签到失败"})
		return
	}
	// 签到积分加到租户余额 + 记录
	var t models.Tenant
	tx.First(&t, tid)
	tx.Model(&t).Update("points", t.Points+reward)
	tx.Create(&models.PointRecord{TenantID: tid, Amount: reward, Type: "recharge", Remark: "每日签到奖励", BalanceAfter: t.Points + reward})
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "签到成功", "data": gin.H{"points": reward, "streak": newStreak}})
}
