package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/crypto"
)

// customerRow 融合后的客户列表行：分站信息 + 主账号（登录账号）+ 点卡余额
// 一个客户（分站）对应一个主登录账号，总后台一眼看清「名称/账号/密码/功能/点数/有效期」。
type customerRow struct {
	ID        uint       `json:"id"` // tenant id
	Name      string     `json:"name"`
	Remark    string     `json:"remark"`
	Status    int        `json:"status"`
	Features  []string   `json:"features"`
	Points    int64      `json:"points"`
	DailyQueryLimit int `json:"daily_query_limit"`
	ChannelID uint       `json:"channel_id"`
	CreatedAt time.Time  `json:"created_at"`
	// 主账号（该分站最早的 admin 账号）
	UserID     uint       `json:"user_id"`
	Username   string     `json:"username"`
	Nickname   string     `json:"nickname"`
	UserStatus int        `json:"user_status"`
	OpenMonths int        `json:"open_months"`
	ExpireAt   *time.Time `json:"expire_at"`
	RemainDays int        `json:"remain_days"`
}

// ListCustomers 融合客户列表：GET /api/super/customers（channel 角色复用：仅返回自己渠道的分站）
func ListCustomers(c *gin.Context) {
	var tenants []models.Tenant
	q := database.DB.Order("id asc")
	if CurrentRole(c) == "channel" {
		q = q.Where("channel_id = ?", ChannelID(c))
	}
	q.Find(&tenants)

	rows := make([]customerRow, 0, len(tenants))
	for _, t := range tenants {
		row := customerRow{
			ID: t.ID, Name: t.Name, Remark: t.Remark, Status: t.Status,
			Features: parseFeatures(t.Features), Points: t.Points, DailyQueryLimit: t.DailyQueryLimit, CreatedAt: t.CreatedAt,
			ChannelID: t.ChannelID,
		}
		// 主账号：该分站最早的 admin 账号（一个客户一个登录账号）
		var owner models.User
		if err := database.DB.Where("tenant_id = ? AND role = ?", t.ID, "admin").
			Order("id asc").First(&owner).Error; err == nil {
			row.UserID = owner.ID
			row.Username = owner.Username
			row.Nickname = owner.Nickname
			row.UserStatus = owner.Status
			row.OpenMonths = owner.OpenMonths
			row.ExpireAt = owner.ExpireAt
			row.RemainDays = remainDaysOf(&owner)
		}
		rows = append(rows, row)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}

type customerReq struct {
	Name       string   `json:"name"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Nickname   string   `json:"nickname"`
	OpenMonths int      `json:"open_months"` // 开通月数（1-36，默认 12）
	Trial      bool     `json:"trial"`       // true=7 天试用（优先于 open_months）
	Features   []string `json:"features"`    // 空=全部开放
	Remark     string   `json:"remark"`
	ChannelID  uint     `json:"channel_id"` // 归属渠道（仅 super 可选；channel 角色忽略，固定为自己）
}

// CreateCustomer 一键开通客户：分站 + 主账号 + 密码 + 功能授权，事务内原子完成。
// 分站标识（code）自动由登录账号派生，无需人工填写（对客户无意义，对系统用于区分）。
func CreateCustomer(c *gin.Context) {
	var req customerReq
	if !jsonBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Username = strings.TrimSpace(req.Username)
	if req.Name == "" || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "客户名称、登录账号、登录密码均不能为空"})
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": msg})
		return
	}
	if req.OpenMonths < 1 || req.OpenMonths > 36 {
		req.OpenMonths = 12
	}
	var uc int64
	database.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&uc)
	if uc > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "登录账号已存在，请更换"})
		return
	}
	// 分站标识自动派生：账号清洗后作为 code，冲突则追加数字后缀
	code := sanitizeCode(req.Username)
	if code == "" {
		code = "c"
	}
	base := code
	for i := 2; ; i++ {
		var cc int64
		database.DB.Model(&models.Tenant{}).Where("code = ?", code).Count(&cc)
		if cc == 0 {
			break
		}
		code = fmt.Sprintf("%s%d", base, i)
	}

	encPwd, err := crypto.Hash(req.Password, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密码加密失败"})
		return
	}
	// 试用期优先：trial=true 则 7 天试用（默认 7，可全局设置 register_trial_days 覆盖）
	var expireAt time.Time
	if req.Trial {
		trialDays := intSetting("register_trial_days", defTrialDays)
		expireAt = time.Now().AddDate(0, 0, trialDays)
	} else {
		expireAt = time.Now().AddDate(0, req.OpenMonths, 0)
	}

	tx := database.DB.Begin()
	// 归属渠道：channel 角色固定为自己；super 可选（0=平台直营）
	channelID := req.ChannelID
	if CurrentRole(c) == "channel" {
		channelID = ChannelID(c)
	}
	t := models.Tenant{
		Name: req.Name, Code: code, Status: 1, ChannelID: channelID,
		Features: marshalFeatures(req.Features), Remark: req.Remark,
	}
	if err := tx.Create(&t).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建客户失败：" + err.Error()})
		return
	}
	u := models.User{
		TenantID: t.ID, Username: req.Username, Password: encPwd,
		Nickname: req.Nickname, Role: "admin", Status: 1,
		OpenMonths: req.OpenMonths, ExpireAt: &expireAt,
	}
	if err := tx.Create(&u).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建登录账号失败：" + err.Error()})
		return
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  fmt.Sprintf("客户已开通：账号 %s，服务有效期至 %s", req.Username, expireAt.Format("2006-01-02")),
		"data": gin.H{
			"tenant_id": t.ID, "username": req.Username,
			"expire_at": expireAt.Format("2006-01-02"),
		},
	})
}

// sanitizeCode 从账号派生分站标识：仅保留字母/数字/连字符，转小写；为空返回空串。
func sanitizeCode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}
