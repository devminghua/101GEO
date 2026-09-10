package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"geo-tool/database"
	"geo-tool/models"
)

// UsageOverview 客户端「Token 用量」看板数据：GET /api/usage/overview?days=7
// 返回：KPI 汇总 + 按天趋势 + 按平台分布 + 按场景分布 + 最近明细。
func UsageOverview(c *gin.Context) {
	tid := TenantID(c)
	days := 7
	if v := c.Query("days"); v != "" {
		switch v {
		case "1", "3", "7", "30":
			days = atoiQ(v)
		}
	}
	since := time.Now().AddDate(0, 0, -days)

	type aggRow struct {
		Tokens  int64 `json:"tokens"`
		Calls   int64 `json:"calls"`
		InTok   int64 `json:"in_tokens"`
		OutTok  int64 `json:"out_tokens"`
		Key     string `json:"key"`
		Model   string `json:"model"`
	}
	q := database.DB.Model(&models.AiUsageRecord{}).Where("tenant_id = ? AND created_at >= ?", tid, since)

	// KPI
	var kpi aggRow
	q.Session(&gorm.Session{}).Select(
		"COALESCE(SUM(total_tokens),0) AS tokens, COUNT(*) AS calls, COALESCE(SUM(prompt_tokens),0) AS in_tok, COALESCE(SUM(completion_tokens),0) AS out_tok",
	).Scan(&kpi)

	// 按天（本地日期，PG 用 date(created_at) —— 兼容函数已建）
	var daily []aggRow
	database.DB.Model(&models.AiUsageRecord{}).
		Select("COALESCE(SUM(total_tokens),0) AS tokens, COUNT(*) AS calls, TO_CHAR(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD') AS key").
		Where("tenant_id = ? AND created_at >= ?", tid, since).
		Group("key").Order("key asc").Scan(&daily)

	// 按平台
	var byPlatform []aggRow
	database.DB.Model(&models.AiUsageRecord{}).
		Select("COALESCE(SUM(total_tokens),0) AS tokens, COUNT(*) AS calls, platform_name AS key, MAX(model) AS model").
		Where("tenant_id = ? AND created_at >= ?", tid, since).
		Group("platform_name").Order("tokens desc").Scan(&byPlatform)

	// 按场景
	var byScene []aggRow
	database.DB.Model(&models.AiUsageRecord{}).
		Select("COALESCE(SUM(total_tokens),0) AS tokens, COUNT(*) AS calls, scene AS key").
		Where("tenant_id = ? AND created_at >= ?", tid, since).
		Group("scene").Order("tokens desc").Scan(&byScene)

	// 最近明细
	var recent []models.AiUsageRecord
	database.DB.Where("tenant_id = ?", tid).Order("id desc").Limit(20).Find(&recent)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"days":        days,
		"kpi":         kpi,
		"daily":       daily,
		"by_platform": byPlatform,
		"by_scene":    byScene,
		"recent":      recent,
	}})
}

func atoiQ(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
