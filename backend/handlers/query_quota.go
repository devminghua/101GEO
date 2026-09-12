package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
)

// 每日查询配额：跨小红书/抖音/百度统一计数，分站每日上限由 SaaS 端设置（默认 3 次，高级版本可解锁更多）。

const quotaModuleTotal = "total"

// CheckQueryQuota 检查并扣减每日查询配额。
// 返回 (是否放行, 已用次数, 每日上限)。limit<=0 表示不限量。
func CheckQueryQuota(c *gin.Context) (allowed bool, used int, limit int) {
	tid := TenantID(c)
	var tenant models.Tenant
	if database.DB.Where("id = ?", tid).First(&tenant).Error != nil {
		return true, 0, 0 // 分站不存在不拦截（容错）
	}
	limit = tenant.DailyQueryLimit
	if limit <= 0 {
		return true, 0, 0 // 0 或负数 = 不限量
	}
	// 业务日期用北京时间（biztime）：容器时区为 UTC，若用 time.Now() 会让配额在
	// **北京时间早上 8 点**才重置，而给客户看的提示写的是「明天 0 点自动重置」，
	// 行为与文案不符（详见 services/biztime 包注释）。
	day := biztime.Today()
	var quota models.QueryQuota
	database.DB.Where("tenant_id = ? AND day = ? AND module = ?", tid, day, quotaModuleTotal).First(&quota)
	used = quota.Count
	if used >= limit {
		return false, used, limit
	}
	// 扣减：无记录则创建，有记录则 +1
	if quota.ID == 0 {
		database.DB.Create(&models.QueryQuota{TenantID: tid, Day: day, Module: quotaModuleTotal, Count: 1})
	} else {
		database.DB.Model(&quota).Update("count", used+1)
	}
	return true, used + 1, limit
}

// QuotaGuard 配额守卫：超限时写入响应并返回 false；放行返回 true。
// 三个模块（百度/抖音/小红书）的查询入口统一调用：if !QuotaGuard(c) { return }。
func QuotaGuard(c *gin.Context) bool {
	if ok, used, limit := CheckQueryQuota(c); !ok {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": fmt.Sprintf("今日查询次数已用完（%d/%d），明天 0 点自动重置，或联系服务商升级版本解锁更多次数", used, limit)})
		return false
	}
	return true
}

// QueryQuotaInfo 查询当前分站的每日配额使用情况（供前端展示剩余次数）。
func QueryQuotaInfo(c *gin.Context) {
	tid := TenantID(c)
	var tenant models.Tenant
	if database.DB.Where("id = ?", tid).First(&tenant).Error != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"limit": 0, "used": 0, "remain": -1}})
		return
	}
	// 与 CheckQueryQuota 用同一业务日期口径（北京时间），否则「已用次数」会错位
	day := biztime.Today()
	var quota models.QueryQuota
	database.DB.Where("tenant_id = ? AND day = ? AND module = ?", tid, day, quotaModuleTotal).First(&quota)
	limit := tenant.DailyQueryLimit
	remain := -1 // -1 = 不限量
	if limit > 0 {
		remain = limit - quota.Count
		if remain < 0 {
			remain = 0
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"limit": limit, "used": quota.Count, "remain": remain}})
}
