package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/auth"
	"geo-tool/services/crypto"
	"geo-tool/services/points"
)

// 渠道商（channel）体系：
// - super 管理渠道：创建（含登录账号）/ 编辑 / 启停。
// - channel 角色登录渠道后台：管理自己渠道下的分站（客户管理全能力）+ 品牌与客服设置。
// - 渠道品牌/客服：其下分站客户端未自设品牌时生效（见 BrandOf 链路）。

const ctxChannelID = "ctx_channel_id"

// ChannelRequired 渠道后台权限中间件：仅 channel 角色，且账号必须关联有效渠道。
func ChannelRequired(c *gin.Context) {
	if c.GetString(ctxRole) != "channel" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无渠道后台访问权限"})
		c.Abort()
		return
	}
	var u models.User
	if err := database.DB.First(&u, CurrentUserID(c)).Error; err != nil || u.ChannelID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "渠道信息无效"})
		c.Abort()
		return
	}
	c.Set(ctxChannelID, u.ChannelID)
	c.Next()
}

// ChannelID 当前渠道 ID（channel 角色下有效，否则 0）
func ChannelID(c *gin.Context) uint { return c.GetUint(ctxChannelID) }

// tenantOwnedByChannel 校验分站是否属于当前渠道；非 channel 角色恒为 true。
func tenantOwnedByChannel(c *gin.Context, tid uint) bool {
	if CurrentRole(c) != "channel" {
		return true
	}
	var cnt int64
	database.DB.Model(&models.Tenant{}).Where("id = ? AND channel_id = ?", tid, ChannelID(c)).Count(&cnt)
	return cnt > 0
}

// userOwnedByChannel 校验账号是否属于当前渠道下的分站；非 channel 角色恒为 true。
func userOwnedByChannel(c *gin.Context, uid uint) bool {
	if CurrentRole(c) != "channel" {
		return true
	}
	var u models.User
	if err := database.DB.First(&u, uid).Error; err != nil {
		return false
	}
	return tenantOwnedByChannel(c, u.TenantID)
}

/* ============ super 端：渠道管理 ============ */

type channelRow struct {
	models.Channel
	Username    string `json:"username"`     // 渠道登录账号
	TenantCount int64  `json:"tenant_count"` // 旗下分站数
}

// ChannelList 渠道列表：GET /api/super/channels
func ChannelList(c *gin.Context) {
	var chs []models.Channel
	database.DB.Order("id asc").Find(&chs)
	out := make([]channelRow, 0, len(chs))
	for _, ch := range chs {
		row := channelRow{Channel: ch}
		database.DB.Model(&models.Tenant{}).Where("channel_id = ?", ch.ID).Count(&row.TenantCount)
		var u models.User
		if err := database.DB.Where("role = ? AND channel_id = ?", "channel", ch.ID).First(&u).Error; err == nil {
			row.Username = u.Username
		}
		out = append(out, row)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

type channelReq struct {
	Name          string `json:"name"`
	Username      string `json:"username"` // 创建时：渠道登录账号
	Password      string `json:"password"` // 创建时：渠道登录密码
	BrandName     string `json:"brand_name"`
	Logo          string `json:"logo"`
	Copyright     string `json:"copyright"`
	ServicePhone  string `json:"service_phone"`
	ServiceWechat string `json:"service_wechat"`
}

// CreateChannel 创建渠道（含登录账号）：POST /api/super/channels
func CreateChannel(c *gin.Context) {
	var req channelReq
	if !jsonBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Username = strings.TrimSpace(req.Username)
	if req.Name == "" || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "渠道名称、登录账号、登录密码均不能为空"})
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": msg})
		return
	}
	var uc int64
	database.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&uc)
	if uc > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "登录账号已存在，请更换"})
		return
	}
	encPwd, err := crypto.Hash(req.Password, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密码加密失败"})
		return
	}
	tx := database.DB.Begin()
	ch := models.Channel{
		Name: req.Name, BrandName: req.BrandName, Logo: req.Logo,
		Copyright: req.Copyright, ServicePhone: req.ServicePhone, ServiceWechat: req.ServiceWechat,
		Status: 1,
	}
	if err := tx.Create(&ch).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建渠道失败：" + err.Error()})
		return
	}
	u := models.User{
		ChannelID: ch.ID, Username: req.Username, Password: encPwd,
		Nickname: req.Name, Role: "channel", Status: 1,
	}
	if err := tx.Create(&u).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建渠道账号失败：" + err.Error()})
		return
	}
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "渠道已创建", "data": gin.H{"id": ch.ID, "username": req.Username}})
}

// UpdateChannel 编辑渠道：PUT /api/super/channels/:id（品牌/客服；不含账号密码）
func UpdateChannel(c *gin.Context) {
	var ch models.Channel
	if err := database.DB.First(&ch, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "渠道不存在"})
		return
	}
	var req channelReq
	if !jsonBody(c, &req) {
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		ch.Name = strings.TrimSpace(req.Name)
	}
	ch.BrandName = req.BrandName
	ch.Logo = req.Logo
	ch.Copyright = req.Copyright
	ch.ServicePhone = req.ServicePhone
	ch.ServiceWechat = req.ServiceWechat
	database.DB.Save(&ch)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存"})
}

// UpdateChannelStatus 启停渠道：PUT /api/super/channels/:id/status
func UpdateChannelStatus(c *gin.Context) {
	var req struct {
		Status int `json:"status"`
	}
	if !jsonBody(c, &req) {
		return
	}
	database.DB.Model(&models.Channel{}).Where("id = ?", c.Param("id")).Update("status", req.Status)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
}

// SimulateChannelLogin 总后台一键登录渠道后台：POST /api/super/channels/:id/simulate-login
// 签发该渠道启用账号的 JWT，返回结构与登录接口一致 {token, user}。
func SimulateChannelLogin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch models.Channel
	if err := database.DB.First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "渠道不存在"})
		return
	}
	if ch.Status != 1 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "渠道已停用，无法登录"})
		return
	}
	var u models.User
	if err := database.DB.Where("role = ? AND channel_id = ? AND status = 1", "channel", ch.ID).Order("id asc").First(&u).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该渠道暂无启用账号"})
		return
	}
	token, err := auth.Sign(u.ID, u.Username, u.TenantID, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "签发凭证失败"})
		return
	}
	recordLoginLog(u.Username, u.Nickname, u.Role, u.TenantID, c.ClientIP(), "success", "总后台模拟登录渠道")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"token": token, "user": userInfoMap(&u)}})
}

/* ============ 渠道点数：平台充值 → 渠道拨付给客户 ============ */

// ChannelRecharge 平台给渠道充值点卡：POST /api/super/channels/:id/recharge
func ChannelRecharge(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
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
	remark := strings.TrimSpace(req.Remark)
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var ch models.Channel
		if err := tx.First(&ch, id).Error; err != nil {
			return errors.New("渠道不存在")
		}
		if err := tx.Model(&ch).Update("points", gorm.Expr("points + ?", req.Amount)).Error; err != nil {
			return err
		}
		var after int64
		tx.Model(&models.Channel{}).Select("points").Where("id = ?", id).Scan(&after)
		if remark == "" {
			remark = "平台充值"
		}
		return tx.Create(&models.PointRecord{
			TenantID: 0, Amount: req.Amount, Type: "channel_recharge",
			Remark: fmt.Sprintf("渠道「%s」：%s", ch.Name, remark), BalanceAfter: after,
		}).Error
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "渠道点数已到账"})
}

// ChannelIssuePoints 渠道从自己余额拨付点数给旗下分站（单事务：扣渠道余额 + 分站入账 + 双流水）。
func ChannelIssuePoints(channelID, tenantID uint, amount int64, remark string) error {
	if amount <= 0 {
		return errors.New("点数必须大于 0")
	}
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var ch models.Channel
		if err := tx.First(&ch, channelID).Error; err != nil {
			return errors.New("渠道不存在")
		}
		if ch.Points < amount {
			return fmt.Errorf("渠道余额不足（当前 %d 点，需要 %d 点）", ch.Points, amount)
		}
		if err := tx.Model(&ch).Update("points", gorm.Expr("points - ?", amount)).Error; err != nil {
			return err
		}
		var t models.Tenant
		if err := tx.First(&t, tenantID).Error; err != nil {
			return errors.New("客户不存在")
		}
		if strings.TrimSpace(remark) == "" {
			remark = "渠道拨付"
		}
		if err := tx.Create(&models.PointRecord{
			TenantID: 0, Amount: -amount, Type: "channel_issue",
			Remark: fmt.Sprintf("渠道「%s」拨付给「%s」：%s", ch.Name, t.Name, remark), BalanceAfter: ch.Points - amount,
		}).Error; err != nil {
			return err
		}
		return points.RechargeTx(tx, tenantID, amount, fmt.Sprintf("渠道「%s」拨付：%s", ch.Name, remark))
	})
}

/* ============ 渠道端：品牌与客服设置 ============ */

// ChannelProfile 渠道自己的品牌/客服信息：GET/PUT /api/channel/profile
func ChannelProfile(c *gin.Context) {
	if c.Request.Method == http.MethodPut {
		var req channelReq
		if !jsonBody(c, &req) {
			return
		}
		database.DB.Model(&models.Channel{}).Where("id = ?", ChannelID(c)).Updates(map[string]interface{}{
			"brand_name": req.BrandName, "logo": req.Logo, "copyright": req.Copyright,
			"service_phone": req.ServicePhone, "service_wechat": req.ServiceWechat,
			"updated_at": time.Now(),
		})
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存"})
		return
	}
	var ch models.Channel
	if err := database.DB.First(&ch, ChannelID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "渠道不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ch})
}
