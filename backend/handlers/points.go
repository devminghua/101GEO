package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/services/points"
)

// RechargeTenant 总后台为分站充值点卡：POST /api/super/tenants/:id/recharge
// body: { "amount": 100, "remark": "2026-09 月包" }
func RechargeTenant(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "租户 ID 非法"})
		return
	}
	if !tenantOwnedByChannel(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该客户"})
		return
	}
	var req struct {
		Amount int64  `json:"amount"`
		Remark string `json:"remark"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "充值点数必须大于 0"})
		return
	}
	// 渠道角色：从渠道自己的余额拨付给客户（余额不足拒绝）
	if CurrentRole(c) == "channel" {
		if err := ChannelIssuePoints(ChannelID(c), uint(id), req.Amount, strings.TrimSpace(req.Remark)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": fmt.Sprintf("已从渠道余额拨付 %d 点", req.Amount)})
		return
	}
	if err := points.Recharge(uint(id), req.Amount, strings.TrimSpace(req.Remark)); err != nil {
		if err.Error() == "租户不存在" {
			c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "充值失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "充值成功"})
}

// GetPoints 分站查询点卡余额与流水：GET /api/points
func GetPoints(c *gin.Context) {
	tid := TenantID(c)
	balance, err := points.Balance(tid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "查询失败：" + err.Error()})
		return
	}
	records, err := points.ListRecords(tid, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "查询失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"balance": balance,
		"records": records,
	}})
}
