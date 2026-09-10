package handlers

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/auth"
	"geo-tool/services/crypto"
	"geo-tool/services/identicon"
	"geo-tool/services/sms"
)

// 注册开通试用配置（可通过总后台全局设置覆盖，均留默认值即开箱可用）：
//   register_enabled      = "0" 关闭自助注册（默认开启）
//   register_trial_days   = 试用天数（默认 7 天）
//   register_trial_points = 试用赠送点数（默认 100 点，便于试用期内跑巡检）
const (
	defTrialDays   = 7
	defTrialPoints = 100
)

// phoneRe 中国大陆手机号（11 位，1 开头第二位 3-9）。
var phoneRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

type smsSendReq struct {
	Phone string `json:"phone"`
}

// SmsSend 发送注册短信验证码（公开接口）。
// Mock 阶段验证码打印到服务日志，并在响应中回传 debug_code 便于联调；接入真实短信后移除 debug_code。
func SmsSend(c *gin.Context) {
	// 非短信验证模式时不再发送短信
	if verifyMode() != "sms" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "当前无需短信验证"})
		return
	}
	// 按总后台短信设置切换发送器（阿里云真实发送 / Mock）
	setupSender()
	if !smsLimiter.allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 1, "msg": "发送过于频繁，请稍后再试"})
		return
	}
	var req smsSendReq
	if !jsonBody(c, &req) {
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if !phoneRe.MatchString(req.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "手机号格式不正确"})
		return
	}
	code, err := sms.Send(req.Phone)
	if err != nil {
		if err == sms.ErrTooFrequent {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
			return
		}
		log.Printf("[SMS] 发送失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "验证码发送失败，请稍后再试"})
		return
	}
	data := gin.H{"phone": req.Phone}
	if sms.IsMock() {
		data["debug_code"] = code // 【Mock】仅联调用，接入真实短信后移除
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "验证码已发送", "data": data})
}

// setupSender 根据总后台短信设置切换发送器：配置完整（阿里云 + 密钥 + 签名 + 模板）则真实发送，否则 Mock。
func setupSender() {
	p := getGlobalSetting(settingSmsProvider)
	akID := getGlobalSetting(settingSmsAKID)
	akSecret := getGlobalSetting(settingSmsAKSecret)
	sign := getGlobalSetting(settingSmsSignName)
	tpl := getGlobalSetting(settingSmsTemplate)
	if p == "aliyun" && akID != "" && akSecret != "" && sign != "" && tpl != "" {
		sms.SetSender(&sms.AliyunSender{
			AccessKeyID:     akID,
			AccessKeySecret: akSecret,
			SignName:        sign,
			TemplateCode:    tpl,
		})
		return
	}
	sms.SetSender(sms.MockSender{})
}

type registerReq struct {
	Username    string `json:"username"` // 登录账号（客户自填；手机号/邮箱仅用于收验证码）
	Phone       string `json:"phone"`
	Email       string `json:"email"` // 邮箱验证模式下接收验证码
	Code        string `json:"code"`
	Password    string `json:"password"`
	CompanyName string `json:"company_name"`
	Ref         string `json:"ref"` // 邀请码（被邀请注册时带上，注册成功给邀请人发奖励）
}

// usernameRe 登录账号：字母/数字/下划线/连字符，3-32 位。
var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// Register 自助注册：短信验证 → 创建分站 + 管理员账号 → 自动开通试用 → 直接登录。
// 一个客户 = 一个分站（tenant）+ 一个登录账号（admin）；登录账号即手机号。
func Register(c *gin.Context) {
	if !registerLimiter.allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 1, "msg": "操作过于频繁，请稍后再试"})
		return
	}
	// 总后台可通过全局设置 register_enabled=0 关闭自助注册
	if getGlobalSetting("register_enabled") == "0" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "暂未开放自助注册，请联系管理员开通"})
		return
	}
	var req registerReq
	if !jsonBody(c, &req) {
		return
	}
	mode := verifyMode()
	req.Username = strings.TrimSpace(req.Username)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Code = strings.TrimSpace(req.Code)
	req.CompanyName = strings.TrimSpace(req.CompanyName)

	// 登录账号：客户自填（字母/数字/下划线/连字符），全局唯一
	if !usernameRe.MatchString(req.Username) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "登录账号仅限 3-32 位字母/数字/下划线/连字符"})
		return
	}

	// 验证码接收载体：邮箱验证模式用邮箱；短信模式用手机号
	verifyTarget := req.Phone
	if mode == "email" {
		verifyTarget = req.Email
		if !emailRe.MatchString(verifyTarget) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "邮箱格式不正确"})
			return
		}
	} else if !phoneRe.MatchString(req.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "手机号格式不正确"})
		return
	}
	if req.Code == "" && mode != "off" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请输入验证码"})
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": msg})
		return
	}
	if req.CompanyName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请填写公司/机构名称"})
		return
	}
	if len([]rune(req.CompanyName)) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "公司/机构名称过长（最多 64 字）"})
		return
	}

	// 1) 验证码校验（一次性）—— 总后台可在「注册验证设置」选择短信/邮箱/关闭
	verifyOK := mode == "off"
	if mode == "sms" {
		verifyOK = sms.Verify(req.Phone, req.Code)
	} else if mode == "email" {
		verifyOK = sms.EmailVerify(req.Email, req.Code)
	}
	if !verifyOK {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "验证码错误或已过期，请重新获取"})
		return
	}
	// 2) 登录账号全局唯一
	var uc int64
	database.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&uc)
	if uc > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该登录账号已被占用，请更换"})
		return
	}
	// 3) 分站标识自动派生（登录账号清洗为 code，冲突追加数字后缀）
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
	// 4) 密码加密 + 试用期计算
	encPwd, err := crypto.Hash(req.Password, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "注册失败，请稍后再试"})
		return
	}
	trialDays := intSetting("register_trial_days", defTrialDays)
	trialPoints := int64(intSetting("register_trial_points", defTrialPoints))
	expireAt := time.Now().AddDate(0, 0, trialDays)

	// 5) 事务创建分站 + 管理员账号 + 试用点数
	tx := database.DB.Begin()
	t := models.Tenant{Name: req.CompanyName, Code: code, Status: 1, Points: trialPoints}
	if err := tx.Create(&t).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建分站失败：" + err.Error()})
		return
	}
	u := models.User{
		TenantID: t.ID, Username: req.Username, Password: encPwd,
		Nickname: req.CompanyName, Role: "admin", Status: 1,
		OpenMonths: 0, ExpireAt: &expireAt,
	}
	if err := tx.Create(&u).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建账号失败：" + err.Error()})
		return
	}
	if trialPoints > 0 {
		tx.Create(&models.PointRecord{
			TenantID: t.ID, Amount: trialPoints, Type: "recharge",
			Remark: "注册试用赠送", BalanceAfter: trialPoints,
		})
	}
	tx.Commit()

	// 邀约奖励：被邀请人注册成功，给邀请人发奖励（防刷：手机号唯一索引）
	if req.Ref != "" {
		settleInvite(req.Ref, t.ID, req.Phone)
	}

	// 注册成功 → 自动生成 NFT 数字头像（基于 username hash 的 5×5 对称 SVG）。
	// 写文件失败不阻塞注册流程，前端会按 username 首字母兜底渲染。
	if avatar, err := identicon.Save(u.ID, u.Username); err == nil && avatar != "" {
		database.DB.Model(&models.User{}).Where("id = ?", u.ID).Update("avatar", avatar)
		u.Avatar = avatar
	}

	// 6) 直接签发 token，注册即登录
	token, err := auth.Sign(u.ID, u.Username, u.TenantID, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "注册成功但签发凭证失败，请用手机号登录"})
		return
	}
	recordLoginLog(u.Username, u.Nickname, u.Role, u.TenantID, c.ClientIP(), "success", "注册并登录")
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  fmt.Sprintf("注册成功，已开通 %d 天试用", trialDays),
		"data": gin.H{"token": token, "user": userInfoMap(&u)},
	})
}

// getGlobalSetting 读取总后台全局设置（tenant_id=0）。
func getGlobalSetting(key string) string {
	var s models.Setting
	if err := database.DB.Where("tenant_id = ? AND key = ?", 0, key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// intSetting 读取全局数字设置，缺失/非法返回默认值。
func intSetting(key string, def int) int {
	v := strings.TrimSpace(getGlobalSetting(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
