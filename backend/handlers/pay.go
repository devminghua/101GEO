// 点卡自助扫码充值相关 HTTP 处理器。
// 包括：分站充值下单与轮询、微信/支付宝支付回调（公开路由，不挂登录）、总后台支付配置读写。
package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/pay"
	"geo-tool/services/recharge"
)

// RechargeCreate 分站点卡扫码充值下单：POST /api/pay/recharge
// body: { "channel": "wechat"|"alipay", "points": 100 }
// 返回 order_no / code_url / amount_fen / points / expire_time，前端据此渲染二维码。
func RechargeCreate(c *gin.Context) {
	var req struct {
		Channel string `json:"channel"`
		Points  int64  `json:"points"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Channel = strings.ToLower(strings.TrimSpace(req.Channel))
	if req.Channel != pay.ChannelWechat && req.Channel != pay.ChannelAlipay {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "支付渠道非法，仅支持 wechat / alipay"})
		return
	}
	// 校验渠道是否启用
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
	view, err := recharge.CreateOrder(c.Request.Context(), TenantID(c), req.Channel, req.Points)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "下单失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": view})
}

// RechargeStatus 分站轮询订单状态：GET /api/pay/recharge/:order_no
func RechargeStatus(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "订单号不能为空"})
		return
	}
	order, err := recharge.GetByOrderNo(orderNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	// 仅允许查看本租户订单
	if order.TenantID != TenantID(c) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权查看该订单"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"order_no":    order.OrderNo,
		"status":      order.Status,
		"channel":     order.Channel,
		"amount_fen":  order.AmountFen,
		"points":      order.Points,
		"expire_time": order.ExpireTime,
		"pay_time":    order.PayTime,
	}})
}

// NotifyWechat 微信 Native 支付回调（公开路由，不挂登录中间件）。
// 验签 -> 写回调日志 -> 幂等入账 -> 返回微信要求的成功应答。
func NotifyWechat(c *gin.Context) {
	handlePayNotify(c, pay.ChannelWechat)
}

// NotifyAlipay 支付宝当面付异步通知（公开路由，不挂登录中间件）。
func NotifyAlipay(c *gin.Context) {
	handlePayNotify(c, pay.ChannelAlipay)
}

// handlePayNotify 支付回调统一处理：读取原始报文 -> 记录回调日志 -> 解析验签 -> 幂等入账。
func handlePayNotify(c *gin.Context, channel string) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "read body failed")
		return
	}
	// 构造验签所需请求头（微信依赖 Wechatpay-* 头；支付宝走 form 解析，无需头）
	headers := map[string]string{}
	for _, k := range []string{
		"Wechatpay-Timestamp", "Wechatpay-Nonce", "Wechatpay-Signature", "Wechatpay-Serial",
	} {
		if v := c.GetHeader(k); v != "" {
			headers[k] = v
		}
	}
	provider, err := pay.New(channel)
	if err != nil {
		c.String(http.StatusInternalServerError, "provider init failed")
		return
	}
	res, err := provider.NotifyParse(c.Request.Context(), body, headers)
	if err != nil {
		c.String(http.StatusBadRequest, "notify parse failed")
		return
	}
	// 记录原始回调日志（无论验签是否通过），用于对账排查
	logEntry := models.PayCallbackLog{
		Channel:  channel,
		OrderNo:  res.OrderNo,
		RawBody:  string(body),
		VerifyOK: res.VerifyOK,
	}
	if err := database.DB.Create(&logEntry).Error; err != nil {
		// 日志落库失败不阻断主流程，仅打印
		c.String(http.StatusInternalServerError, "log save failed")
		return
	}
	// 验签失败或未确认订单：不触发入账，但返回成功应答避免渠道重复推送
	if !res.VerifyOK {
		ackPayNotify(c, channel)
		return
	}
	// 幂等入账
	if _, err := recharge.HandlePaid(res.OrderNo, res.TradeNo, res.AmountFen); err != nil {
		// 入账失败（订单不存在等）：仍返回成功应答，但记录到日志供排查
		c.String(http.StatusOK, "ok")
		return
	}
	ackPayNotify(c, channel)
}

// ackPayNotify 按渠道要求返回成功应答：
// 微信要求 200 + JSON {"code":"SUCCESS"}；支付宝要求纯文本 success。
func ackPayNotify(c *gin.Context, channel string) {
	if channel == pay.ChannelWechat {
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS"})
		return
	}
	c.String(http.StatusOK, "success")
}

// GetPayConfig 总后台读取支付配置：GET /api/super/pay/config
// 密钥字段一律脱敏为占位符，绝不下发明文。
func GetPayConfig(c *gin.Context) {
	cfg, err := pay.GetPublicConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

// SavePayConfig 总后台保存支付配置：POST /api/super/pay/config
// body 为 { key: value } 映射；密钥字段留空或为占位符表示保留原值，非空则加密后落库。
func SavePayConfig(c *gin.Context) {
	var values map[string]string
	if !jsonBody(c, &values) {
		return
	}
	if err := pay.SaveConfig(values); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "保存失败：" + err.Error()})
		return
	}
	cfg, _ := pay.GetPublicConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "支付配置已保存", "data": cfg})
}

// RechargeCloseExpired 定时任务辅助：关闭超时未支付订单（由 main.go 定时调用）。
func RechargeCloseExpired() (int64, error) {
	return recharge.CloseExpired()
}
