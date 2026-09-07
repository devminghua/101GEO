package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/pay"
	"geo-tool/services/recharge"
)

// ListRechargePlans 客户端查询启用中的套餐（按排序，供充值中心「选择套餐」）。
func ListRechargePlans(c *gin.Context) {
	var plans []models.RechargePlan
	database.DB.Where("enabled = ?", true).Order("sort asc, id asc").Find(&plans)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": plans})
}

// RechargePlanOrder 分站选套餐下单：POST /api/recharge/plans/:id/order
// body: { "channel": "wechat"|"alipay" }，用套餐的 token 数与售价下单。
func RechargePlanOrder(c *gin.Context) {
	planID := parseID(c)
	var req struct {
		Channel string `json:"channel"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Channel = strings.ToLower(strings.TrimSpace(req.Channel))
	if req.Channel != pay.ChannelWechat && req.Channel != pay.ChannelAlipay {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "支付渠道非法，仅支持 wechat / alipay"})
		return
	}
	cfg, err := pay.LoadSecretConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	enabled := false
	if req.Channel == pay.ChannelWechat {
		enabled = cfg.WechatPayEnabled
	} else {
		enabled = cfg.AlipayPayEnabled
	}
	if !enabled {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "该支付渠道暂未开通，请联系总后台"})
		return
	}
	view, err := recharge.CreateOrderByPlan(c.Request.Context(), TenantID(c), req.Channel, planID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "下单失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": view})
}

// ---------- 总后台套餐管理 ----------

// ListAllPlans 总后台套餐列表（含停用）。
func ListAllPlans(c *gin.Context) {
	var plans []models.RechargePlan
	database.DB.Order("sort asc, id asc").Find(&plans)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": plans})
}

type planReq struct {
	Name     string `json:"name"`
	Points   int64  `json:"points"`
	PriceFen int64  `json:"price_fen"`
	OrigFen  int64  `json:"orig_fen"`
	Tag      string `json:"tag"`
	DailyQueryLimit int `json:"daily_query_limit"` // 购买后解锁的每日查询上限（0=不改变）
	Enabled  *bool  `json:"enabled"`
	Sort     int    `json:"sort"`
}

// CreatePlan 新增套餐。
func CreatePlan(c *gin.Context) {
	var req planReq
	if !jsonBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Points <= 0 || req.PriceFen <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "套餐名、token 数与价格均必填且需大于 0"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := models.RechargePlan{
		Name: req.Name, Points: req.Points, PriceFen: req.PriceFen,
		OrigFen: req.OrigFen, Tag: strings.TrimSpace(req.Tag),
		DailyQueryLimit: req.DailyQueryLimit, Enabled: enabled, Sort: req.Sort,
	}
	if err := database.DB.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

// UpdatePlan 更新套餐。
func UpdatePlan(c *gin.Context) {
	id := parseID(c)
	var req planReq
	if !jsonBody(c, &req) {
		return
	}
	var p models.RechargePlan
	if err := database.DB.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "套餐不存在"})
		return
	}
	updates := map[string]interface{}{}
	if strings.TrimSpace(req.Name) != "" {
		updates["name"] = strings.TrimSpace(req.Name)
	}
	if req.Points > 0 {
		updates["points"] = req.Points
	}
	if req.PriceFen > 0 {
		updates["price_fen"] = req.PriceFen
	}
	updates["orig_fen"] = req.OrigFen
	updates["tag"] = strings.TrimSpace(req.Tag)
	updates["daily_query_limit"] = req.DailyQueryLimit
	updates["sort"] = req.Sort
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if err := database.DB.Model(&p).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "更新失败：" + err.Error()})
		return
	}
	database.DB.First(&p, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

// DeletePlan 删除套餐。
func DeletePlan(c *gin.Context) {
	id := parseID(c)
	if err := database.DB.Delete(&models.RechargePlan{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "删除失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}
