package handlers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/services/crypto"
	"geo-tool/services/sms"
)

// 注册验证 / 短信 / 邮箱配置存储键（全局设置 tenant_id=0，SaaS 端统一配置，全部分站注册共用）。
const (
	settingSmsRequired = "register_sms_required" // 兼容旧字段："0"=关闭验证直接注册；"1"=需验证码（默认）
	settingVerifyMode  = "register_verify_mode"  // 注册验证方式：sms（短信验证码）/ email（邮箱验证码）/ off（关闭验证）
	settingSmsProvider = "sms_provider"          // mock / aliyun
	settingSmsAKID     = "sms_access_key_id"
	settingSmsAKSecret = "sms_access_key_secret"
	settingSmsSignName = "sms_sign_name"
	settingSmsTemplate = "sms_template_code"
	// SMTP 邮箱配置（发送注册验证码）
	settingSmtpHost = "smtp_host"
	settingSmtpPort = "smtp_port"
	settingSmtpUser = "smtp_user"
	settingSmtpPass = "smtp_pass" // enc:v1 加密存储
	settingSmtpFrom = "smtp_from"
	// 网站注册安全协议（注册页勾选，内容由 SaaS 端维护）
	settingAgreementEnabled = "register_agreement_enabled" // "1"=注册需勾选同意（默认开启）；"0"=不需要
	settingAgreementTitle   = "register_agreement_title"   // 协议标题
	settingAgreementContent = "register_agreement_content" // 协议正文（支持换行纯文本）
	settingAgreementUpdate  = "register_agreement_updated" // 最近修改时间（YYYY-MM-DD）
)

// defAgreementTitle / defAgreementContent 未配置时的默认注册安全协议。
const (
	defAgreementTitle   = "网站注册安全协议"
	defAgreementContent = `欢迎注册使用本平台（以下简称"本平台"）。为保障您的账号与数据安全，请在注册前仔细阅读并同意以下条款：

一、账号安全
1. 您注册时填写的登录账号、手机号或邮箱应真实有效，且为您本人所有或已获合法授权。
2. 请妥善保管账号与密码，不要向他人泄露。因您保管不善导致的损失，由您自行承担。
3. 本平台采用加密方式存储密码，任何工作人员均不会向您索要密码。

二、信息收集与使用
1. 本平台仅收集为提供服务所必需的信息（登录账号、手机号/邮箱、公司/机构名称等）。
2. 您的信息仅用于身份验证、服务开通、安全风控与必要的服务通知，不会向无关第三方出售或非法披露。
3. 本平台将依照《中华人民共和国网络安全法》《中华人民共和国个人信息保护法》等法律法规处理您的个人信息。

三、用户行为规范
1. 您承诺不利用本平台从事任何违法违规活动，不发布违法违规内容。
2. 不得使用自动化工具恶意注册、批量注册或攻击本平台服务。

四、协议变更
本协议内容可能根据法律法规或业务需要更新，更新后将在注册页公示。继续使用本平台即视为接受更新后的协议。

如您同意以上条款，请勾选"我已阅读并同意"后完成注册。`
)

// agreementEnabled 注册是否需勾选安全协议（默认需要）。
func agreementEnabled() bool {
	return getGlobalSetting(settingAgreementEnabled) != "0"
}

// agreementTitle / agreementContent 读取协议标题与正文（未配置走默认文案）。
func agreementTitle() string {
	if t := strings.TrimSpace(getGlobalSetting(settingAgreementTitle)); t != "" {
		return t
	}
	return defAgreementTitle
}

func agreementContent() string {
	if s := getGlobalSetting(settingAgreementContent); strings.TrimSpace(s) != "" {
		return s
	}
	return defAgreementContent
}

// emailRe 邮箱格式校验。
var emailRe = regexp.MustCompile(`^[\w.+-]+@[\w-]+(\.[\w-]+)+$`)

// smsRequired 注册是否要求短信验证码（兼容旧字段；默认需要）。
func smsRequired() bool {
	return getGlobalSetting(settingSmsRequired) != "0"
}

// verifyMode 注册验证方式：sms / email / off。新字段优先，兼容旧 register_sms_required。
func verifyMode() string {
	m := strings.TrimSpace(getGlobalSetting(settingVerifyMode))
	if m == "sms" || m == "email" || m == "off" {
		return m
	}
	if getGlobalSetting(settingSmsRequired) == "0" {
		return "off"
	}
	return "sms"
}

// setupEmailSender 根据 SMTP 配置切换邮箱发送器：配置完整则真实发送，否则 Mock。
func setupEmailSender() {
	host := strings.TrimSpace(getGlobalSetting(settingSmtpHost))
	port := intSetting(settingSmtpPort, 465)
	user := strings.TrimSpace(getGlobalSetting(settingSmtpUser))
	from := strings.TrimSpace(getGlobalSetting(settingSmtpFrom))
	if from == "" {
		from = user
	}
	pass := getGlobalSetting(settingSmtpPass)
	if enc := pass; strings.HasPrefix(enc, crypto.PrefixEnc) {
		if dec, err := crypto.Decrypt(enc, config.Load().PayloadSecret()); err == nil {
			pass = dec
		}
	}
	if host == "" || user == "" || pass == "" || from == "" {
		sms.SetEmailSender(sms.MockEmailSender{})
		return
	}
	sms.SetEmailSender(sms.EmailSender{Host: host, Port: port, User: user, Pass: pass, From: from})
}

// EmailCode 发送邮箱注册验证码：POST /api/auth/email-code {email}
func EmailCode(c *gin.Context) {
	if verifyMode() != "email" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "当前未开启邮箱验证"})
		return
	}
	setupEmailSender()
	if !smsLimiter.allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 1, "msg": "发送过于频繁，请稍后再试"})
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !emailRe.MatchString(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "邮箱格式不正确"})
		return
	}
	code, err := sms.EmailSend(req.Email)
	if err != nil {
		if err == sms.ErrTooFrequent {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "验证码发送失败：" + err.Error()})
		return
	}
	data := gin.H{"email": req.Email}
	if sms.EmailIsMock() {
		data["debug_code"] = code // 【Mock】仅联调用，接入真实 SMTP 后移除
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "验证码已发送至邮箱", "data": data})
}

// RegisterConfig 公开接口：返回自助注册开关与验证方式（前端注册页据此动态渲染）。
func RegisterConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"register_enabled": getGlobalSetting("register_enabled") != "0",
		"verify_mode":      verifyMode(), // sms / email / off
		"sms_required":     verifyMode() == "sms",
		// 网站注册安全协议（SaaS 端可编辑）
		"agreement_enabled": agreementEnabled(),
		"agreement_title":   agreementTitle(),
		"agreement_content": agreementContent(),
		"agreement_updated": getGlobalSetting(settingAgreementUpdate),
	}})
}

// GetSmsConfig 读取注册验证配置（super；密钥脱敏回显）。
func GetSmsConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"verify_mode":       verifyMode(),
		"sms_required":      verifyMode() == "sms",
		"provider":          getGlobalSetting(settingSmsProvider),
		"access_key_id":     getGlobalSetting(settingSmsAKID),
		"access_key_secret": maskSecret(getGlobalSetting(settingSmsAKSecret)),
		"sign_name":         getGlobalSetting(settingSmsSignName),
		"template_code":     getGlobalSetting(settingSmsTemplate),
		// SMTP 邮箱配置
		"smtp_host": getGlobalSetting(settingSmtpHost),
		"smtp_port": intSetting(settingSmtpPort, 465),
		"smtp_user": getGlobalSetting(settingSmtpUser),
		"smtp_pass": maskSecret(smtpPlainPassword()),
		"smtp_from": getGlobalSetting(settingSmtpFrom),
		// 网站注册安全协议
		"agreement_enabled": agreementEnabled(),
		"agreement_title":   agreementTitle(),
		"agreement_content": agreementContent(),
		"agreement_updated": getGlobalSetting(settingAgreementUpdate),
	}})
}

// smtpPlainPassword 解密 SMTP 密码（回显脱敏用）。
func smtpPlainPassword() string {
	enc := getGlobalSetting(settingSmtpPass)
	if strings.HasPrefix(enc, crypto.PrefixEnc) {
		if dec, err := crypto.Decrypt(enc, config.Load().PayloadSecret()); err == nil {
			return dec
		}
	}
	return enc
}

// SaveSmsConfig 保存注册验证配置（super）。密钥类字段留空或含 * 表示不修改（脱敏回显场景）。
func SaveSmsConfig(c *gin.Context) {
	var req struct {
		VerifyMode      string `json:"verify_mode"`  // sms / email / off
		SmsRequired     bool   `json:"sms_required"` // 兼容旧前端：true=短信
		Provider        string `json:"provider"`
		AccessKeyID     string `json:"access_key_id"`
		AccessKeySecret string `json:"access_key_secret"`
		SignName        string `json:"sign_name"`
		TemplateCode    string `json:"template_code"`
		SmtpHost        string `json:"smtp_host"`
		SmtpPort        int    `json:"smtp_port"`
		SmtpUser        string `json:"smtp_user"`
		SmtpPass        string `json:"smtp_pass"`
		SmtpFrom        string `json:"smtp_from"`
		// 网站注册安全协议
		AgreementEnabled bool    `json:"agreement_enabled"`
		AgreementTitle   *string `json:"agreement_title"`
		AgreementContent *string `json:"agreement_content"`
	}
	if !jsonBody(c, &req) {
		return
	}
	// 验证方式：新字段优先；旧字段兼容
	mode := strings.TrimSpace(req.VerifyMode)
	if mode != "sms" && mode != "email" && mode != "off" {
		if req.SmsRequired {
			mode = "sms"
		} else {
			mode = "off"
		}
	}
	upsertGlobalSetting(settingVerifyMode, mode)
	if req.SmsRequired {
		upsertGlobalSetting(settingSmsRequired, "1")
	} else {
		upsertGlobalSetting(settingSmsRequired, "0")
	}
	upsertGlobalSetting(settingSmsProvider, strings.TrimSpace(req.Provider))
	upsertGlobalSetting(settingSmsAKID, strings.TrimSpace(req.AccessKeyID))
	secret := strings.TrimSpace(req.AccessKeySecret)
	if secret != "" && !strings.Contains(secret, "*") {
		upsertGlobalSetting(settingSmsAKSecret, secret)
	}
	upsertGlobalSetting(settingSmsSignName, strings.TrimSpace(req.SignName))
	upsertGlobalSetting(settingSmsTemplate, strings.TrimSpace(req.TemplateCode))
	// SMTP 配置
	upsertGlobalSetting(settingSmtpHost, strings.TrimSpace(req.SmtpHost))
	upsertGlobalSetting(settingSmtpPort, strconv.Itoa(req.SmtpPort))
	upsertGlobalSetting(settingSmtpUser, strings.TrimSpace(req.SmtpUser))
	smtpPass := strings.TrimSpace(req.SmtpPass)
	if smtpPass != "" && !strings.Contains(smtpPass, "*") {
		if enc, err := crypto.Encrypt(smtpPass, config.Load().PayloadSecret()); err == nil {
			upsertGlobalSetting(settingSmtpPass, enc)
		}
	}
	upsertGlobalSetting(settingSmtpFrom, strings.TrimSpace(req.SmtpFrom))
	// 网站注册安全协议：Enabled 用普通 bool；标题/正文用指针区分"未传"与"清空"
	if req.AgreementEnabled {
		upsertGlobalSetting(settingAgreementEnabled, "1")
	} else {
		upsertGlobalSetting(settingAgreementEnabled, "0")
	}
	changed := false
	if req.AgreementTitle != nil {
		if t := strings.TrimSpace(*req.AgreementTitle); t != "" {
			upsertGlobalSetting(settingAgreementTitle, t)
			changed = true
		}
	}
	if req.AgreementContent != nil {
		if v := strings.TrimSpace(*req.AgreementContent); v != "" {
			upsertGlobalSetting(settingAgreementContent, v)
			changed = true
		}
	}
	if changed {
		upsertGlobalSetting(settingAgreementUpdate, time.Now().Format("2006-01-02"))
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存"})
}

// maskSecret 密钥脱敏：dfb747ea...8263 → dfb7****8263。
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
