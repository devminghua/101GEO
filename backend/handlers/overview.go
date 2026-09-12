package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
)

// Overview 总后台 SaaS 运营概览：
// 客户规模（总客户/新注册）+ 点数经营（累计使用/剩余/今日消耗）+ 巡检活跃 + 各分站简况
func Overview(c *gin.Context) {
	var tenantCount, accountCount, taskCount, runningCount, todayTaskCount int64
	var todayHitCount int64 = 0
	var newTenants7d, newTenants30d int64
	var pointsLeft, pointsUsedTotal, pointsUsedToday, pointsUsed7d int64

	// 统计口径统一走业务时区（北京时间）：容器时区为 UTC，
	// 若用 time.Now() 会让「今日」在北京时间 8 点才切换。
	now := biztime.Now()
	midnight := biztime.DayStart(now)
	sevenDaysAgo := midnight.AddDate(0, 0, -6)
	thirtyDaysAgo := midnight.AddDate(0, 0, -29)

	database.DB.Model(&models.Tenant{}).Count(&tenantCount)
	database.DB.Model(&models.User{}).Where("tenant_id > 0").Count(&accountCount)
	database.DB.Model(&models.CheckTask{}).Count(&taskCount)
	database.DB.Model(&models.CheckTask{}).Where("status = ?", "running").Count(&runningCount)
	database.DB.Model(&models.CheckTask{}).Where("started_at >= ?", midnight).Count(&todayTaskCount)
	database.DB.Model(&models.CheckResult{}).
		Where("hit = ?", true).
		Where("created_at >= ?", midnight).Count(&todayHitCount)

	// 新注册客户（按分站创建时间；本地时区零点边界）
	database.DB.Model(&models.Tenant{}).Where("created_at >= ?", sevenDaysAgo).Count(&newTenants7d)
	database.DB.Model(&models.Tenant{}).Where("created_at >= ?", thirtyDaysAgo).Count(&newTenants30d)

	// 点数经营：剩余 = 各分站余额总和；使用 = 消费流水绝对值累计
	database.DB.Model(&models.Tenant{}).Select("coalesce(sum(points),0)").Scan(&pointsLeft)
	database.DB.Model(&models.PointRecord{}).
		Where("type = ?", "consume").
		Select("coalesce(sum(-amount),0)").Scan(&pointsUsedTotal)
	database.DB.Model(&models.PointRecord{}).
		Where("type = ? AND created_at >= ?", "consume", midnight).
		Select("coalesce(sum(-amount),0)").Scan(&pointsUsedToday)
	database.DB.Model(&models.PointRecord{}).
		Where("type = ? AND created_at >= ?", "consume", sevenDaysAgo).
		Select("coalesce(sum(-amount),0)").Scan(&pointsUsed7d)

	// 近 7 天逐日：点数消耗 + 新注册客户（趋势迷你图数据）
	type dayRow struct {
		Day      string `json:"day"`
		Used     int64  `json:"used"`
		NewCusts int64  `json:"new_customers"`
	}
	var usedRows []dayRow
	database.DB.Model(&models.PointRecord{}).
		Select("" + dayExpr() + " as day, sum(-amount) as used").
		Where("type = ? AND created_at >= ?", "consume", sevenDaysAgo).
		Group("" + dayExpr() + "").Scan(&usedRows)
	var newRows []dayRow
	database.DB.Model(&models.Tenant{}).
		Select("" + dayExpr() + " as day, count(*) as new_customers").
		Where("created_at >= ?", sevenDaysAgo).
		Group("" + dayExpr() + "").Scan(&newRows)
	byDay := map[string]*dayRow{}
	for i := 6; i >= 0; i-- {
		d := biztime.Day(-i)
		byDay[d] = &dayRow{Day: d}
	}
	for _, r := range usedRows {
		if v, ok := byDay[r.Day]; ok {
			v.Used = r.Used
		}
	}
	for _, r := range newRows {
		if v, ok := byDay[r.Day]; ok {
			v.NewCusts = r.NewCusts
		}
	}
	days := make([]dayRow, 0, 7)
	for i := 6; i >= 0; i-- {
		d := biztime.Day(-i)
		days = append(days, *byDay[d])
	}

	// 客户到期预警：取每个分站的主账号（最早创建的 admin）到期信息，按剩余天数升序
	// 规则：expire_at 为 nil 表示不限（不预警）；仅列出 30 天内到期或已到期的账号
	type expiryRow struct {
		TenantID   uint       `json:"tenant_id"`
		TenantName string     `json:"tenant_name"`
		TenantCode string     `json:"tenant_code"`
		UserID     uint       `json:"user_id"`
		Username   string     `json:"username"`
		Nickname   string     `json:"nickname"`
		ExpireAt   *time.Time `json:"expire_at"`
		RemainDays int        `json:"remain_days"`
	}
	var expiryRaw []expiryRow
	threshold := now.AddDate(0, 0, 30)
	database.DB.Raw(`
		SELECT u.tenant_id, t.name AS tenant_name, t.code AS tenant_code,
			u.id AS user_id, u.username, u.nickname, u.expire_at
		FROM users u JOIN tenants t ON t.id = u.tenant_id
		WHERE u.tenant_id > 0 AND u.role = 'admin' AND u.status = 1
			AND u.id = (SELECT MIN(id) FROM users x WHERE x.tenant_id = u.tenant_id AND x.role = 'admin')
			AND u.expire_at IS NOT NULL
			AND u.expire_at <= ?
		ORDER BY u.expire_at ASC`, threshold).Scan(&expiryRaw)
	// remain_days 在 Go 端计算（跨数据库兼容）
	for i := range expiryRaw {
		if expiryRaw[i].ExpireAt != nil {
			expiryRaw[i].RemainDays = int(expiryRaw[i].ExpireAt.Sub(now).Hours() / 24)
		}
	}
	expiries := expiryRaw

	// 按分站统计最近 7 天巡检执行情况 + 点数余额/消耗
	type tenantStat struct {
		ID     uint   `json:"id"`
		Name   string `json:"name"`
		Code   string `json:"code"`
		Tasks  int64  `json:"tasks"`
		Hits   int64  `json:"hits"`
		Points int64  `json:"points"`
		Used7d int64  `json:"used_7d"`
	}
	var tenants []models.Tenant
	database.DB.Order("id asc").Find(&tenants)
	stats := make([]tenantStat, 0, len(tenants))
	for _, t := range tenants {
		var tasks, hits, used7d int64
		database.DB.Model(&models.CheckTask{}).Where("tenant_id = ?", t.ID).
			Where("started_at >= ?", sevenDaysAgo).Count(&tasks)
		database.DB.Model(&models.CheckResult{}).Where("tenant_id = ?", t.ID).
			Where("hit = ?", true).
			Where("created_at >= ?", sevenDaysAgo).Count(&hits)
		database.DB.Model(&models.PointRecord{}).
			Where("tenant_id = ? AND type = ? AND created_at >= ?", t.ID, "consume", sevenDaysAgo).
			Select("coalesce(sum(-amount),0)").Scan(&used7d)
		stats = append(stats, tenantStat{ID: t.ID, Name: t.Name, Code: t.Code, Tasks: tasks, Hits: hits, Points: t.Points, Used7d: used7d})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"tenant_count":      tenantCount,
		"account_count":     accountCount,
		"new_customers_7d":  newTenants7d,
		"new_customers_30d": newTenants30d,
		"points_left":       pointsLeft,
		"points_used":       pointsUsedTotal,
		"points_used_today": pointsUsedToday,
		"points_used_7d":    pointsUsed7d,
		"days":              days,
		"expiry_warnings":   expiries,
		"task_count":        taskCount,
		"running_count":     runningCount,
		"today_tasks":       todayTaskCount,
		"today_hits":        todayHitCount,
		"tenants":           stats,
	}})
}
