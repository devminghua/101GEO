package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/points"
)

// 获客工具扣费规则：余额需 > 门槛，每次解析扣固定点数。
const (
	toolMinBalance = 2000 // 获客工具最低 Token 余额门槛（需 > 该值）
	toolCostPoints = 100  // 每次解析消耗 Token
)

// ToolConsume 获客工具扣费：POST /api/tools/consume
// 前置校验 Token 余额 > 2000，每次解析扣 100 Token。解析本身在客户端浏览器本地完成，服务器只做扣费。
func ToolConsume(c *gin.Context) {
	tid := TenantID(c)
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "总后台账号无需使用获客工具"})
		return
	}
	var req struct {
		Tool string `json:"tool"`
	}
	_ = jsonBody(c, &req)

	var t models.Tenant
	if database.DB.First(&t, tid).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在"})
		return
	}
	// 门槛校验：余额需 > 2000
	if t.Points <= toolMinBalance {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "获客工具需 Token 余额高于 2000，请先充值"})
		return
	}
	remark := "获客工具·短视频去水印"
	if req.Tool == "video2text" {
		remark = "获客工具·视频转文案"
	}
	if err := points.Deduct(tid, toolCostPoints, remark); err != nil {
		if err == points.ErrInsufficient {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "Token 余额不足，请先充值"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "扣费失败：" + err.Error()})
		return
	}
	// 返回最新余额
	var fresh models.Tenant
	database.DB.Select("points").First(&fresh, tid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已消耗 100 Token", "data": gin.H{"balance": fresh.Points}})
}

// ToolStatus 获客工具状态：GET /api/tools/status
// 返回当前余额 + 是否可用（余额 > 2000）+ 单次消耗点数，供前端判断是否展示/解锁。
func ToolStatus(c *gin.Context) {
	tid := TenantID(c)
	if tid == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"available": true, "balance": 0, "min_balance": toolMinBalance, "cost": toolCostPoints}})
		return
	}
	var t models.Tenant
	database.DB.Select("points").First(&t, tid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"available":   t.Points > toolMinBalance,
		"balance":     t.Points,
		"min_balance": toolMinBalance,
		"cost":        toolCostPoints,
	}})
}
