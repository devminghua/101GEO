package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/services/notify"
)

// NotifyTestService 发送测试告警：POST /api/super/notify/test
// 按当前通知配置向所有已配置渠道（企微/钉钉）推一条测试消息，用于验证 webhook 是否可用。
func NotifyTestService(c *gin.Context) {
	cfg := notify.LoadConfig()
	text := "【LinkGeo】测试消息：告警机器人配置成功 ✓\n后续将在每日推送时间自动汇总「到期预警 + 点数不足」到本群。"
	ok, err := notify.SendAll(cfg, text)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "推送失败：" + err.Error() + "（已成功 " + strconv.Itoa(ok) + " 个渠道）"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "测试消息已推送到 " + strconv.Itoa(ok) + " 个渠道"})
}

// NotifyPreviewService 预览当前告警内容：GET /api/super/notify/preview
// 不实际推送，返回此刻汇总的告警文本（到期预警 + 点数不足），供设置页预览。
func NotifyPreviewService(c *gin.Context) {
	cfg := notify.LoadConfig()
	msg := notify.BuildDailyMessage(cfg, time.Now())
	if msg == "" {
		msg = "当前无告警：没有 " + strconv.Itoa(cfg.ExpiryDays) + " 天内到期的客户，也没有余额 ≤ " + strconv.Itoa(cfg.PointsFloor) + " 点的分站。"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"enabled": cfg.Enabled, "push_hour": cfg.PushHour,
		"wecom_configured": cfg.WecomWebhook != "", "dingtalk_configured": cfg.DingWebhook != "",
		"message": msg,
	}})
}
