package handlers

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/auth"
	"geo-tool/services/captcha"
	"geo-tool/services/crypto"
)

// ---- 登录防爆破（持久化）：同一账号+IP 连续输错 3 次锁定 5 分钟，再次输错 3 次锁定 1 小时 ----
const (
	maxLoginFails = 3               // 连续失败 3 次触发锁定
	firstLockTime = 5 * time.Minute // 第一次锁定 5 分钟
	nextLockTime  = 1 * time.Hour   // 再次锁定 1 小时
	guardIdleTTL  = 24 * time.Hour  // 连续 24 小时无活动则彻底清理该账号的失败记录
)

func guardKey(username, ip string) string { return username + "|" + ip }

// guardLocked 检查是否已锁定；返回 (是否锁定, 剩余秒数, 锁定级别)。
// 状态持久化于数据库，重启/多实例不丢失。惰性处理：锁定到期解除但保留 LockCount（升级时长），
// 连续 24 小时无活动则清理记录，避免表无限增长。
func guardLocked(key string) (bool, int, int) {
	now := time.Now()
	// 惰性清理：删除 24 小时无活动的记录（锁定最长 1 小时，超时记录早已到期）
	database.DB.Where("last_active < ?", now.Add(-guardIdleTTL)).Delete(&models.LoginGuard{})

	var e models.LoginGuard
	if err := database.DB.Where("key = ?", key).First(&e).Error; err != nil {
		return false, 0, 0
	}
	if !e.Until.IsZero() && e.Until.Before(now) {
		// 锁定到期：解除锁定但保留 LockCount（用于「再次锁定升级时长」）
		e.Until = time.Time{}
		database.DB.Model(&models.LoginGuard{}).Where("key = ?", key).Update("until", time.Time{})
	}
	if e.Until.After(now) {
		return true, int(time.Until(e.Until).Seconds()) + 1, e.LockCount
	}
	return false, 0, 0
}

// guardFail 记录一次失败；达到阈值后锁定。事务保证 read-modify-write 原子，
// 返回 (当前连续失败次数, 是否刚触发锁定, 锁定时长秒, 锁定级别)。
func guardFail(key string) (fails int, locked bool, lockSeconds int, lockLevel int) {
	_ = database.DB.Transaction(func(tx *gorm.DB) error {
		var e models.LoginGuard
		if err := tx.Where("key = ?", key).First(&e).Error; err != nil {
			e = models.LoginGuard{Key: key}
		}
		e.LastActive = time.Now()
		e.Fails++
		fails = e.Fails
		if e.Fails >= maxLoginFails {
			e.LockCount++
			d := firstLockTime
			if e.LockCount > 1 {
				d = nextLockTime
			}
			e.Until = time.Now().Add(d)
			e.Fails = 0
			locked = true
			lockSeconds = int(d.Seconds())
			lockLevel = e.LockCount
		}
		return tx.Save(&e).Error
	})
	return
}

func guardClear(key string) {
	database.DB.Where("key = ?", key).Delete(&models.LoginGuard{})
}

type loginReq struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CaptchaID string    `json:"captcha_id"`
	SlideX    float64   `json:"slide_x"` // 手机端可能提交浮点坐标，取整容错
	Track     []float64 `json:"track"`   // 拖动轨迹采样（x 坐标序列），手机端为浮点数
}

// GetCaptcha 下发一次性滑动解锁凭证（无图像，纯滑块）
func GetCaptcha(c *gin.Context) {
	if !captchaLimiter.allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 1, "msg": "请求过于频繁，请稍后再试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{"id": captcha.Generate()},
	})
}

// recordLoginLog 记录一次登录尝试（成功/失败），供系统设置页「登录日志」展示。
// 失败时可能拿不到完整账号信息（如账号不存在），仅记录已知字段。
func recordLoginLog(username, nickname, role string, tenantID uint, ip, status, reason string) {
	if username == "" {
		return
	}
	database.DB.Create(&models.LoginLog{
		TenantID: tenantID,
		Username: username,
		Nickname: nickname,
		Role:     role,
		IP:       ip,
		Status:   status,
		Reason:   reason,
	})
}

// lockMsg 根据锁定级别返回提示文案
func lockMsg(level int) string {
	if level > 1 {
		return "多次输错密码，账号已锁定 1 小时，请稍后再试"
	}
	return "连续输错密码 3 次，账号已锁定 5 分钟，请稍后再试"
}

// Login 登录：总后台账号与分站账号均通过此接口，按角色返回能力
func Login(c *gin.Context) {
	ip := c.ClientIP()
	// 0) IP 维度限流：挡掉单 IP 高频爆破（分级锁定按账号+IP 维度，换 IP 可绕过，此处兜底）
	if !loginLimiter.allow(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 1, "msg": "登录尝试过于频繁，请稍后再试"})
		return
	}
	var req loginReq
	if !jsonBody(c, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请输入账号和密码"})
		return
	}
	guard := guardKey(req.Username, ip)

	// 1) 滑块验证：先过人机校验再进入账号逻辑，拦截自动化爆破/账号探测
	// 手机端轨迹为浮点数（触摸坐标带小数），四舍五入取整后再校验
	slideX := int(math.Round(req.SlideX))
	track := make([]int, len(req.Track))
	for i, v := range req.Track {
		track[i] = int(math.Round(v))
	}
	if req.CaptchaID == "" || !captcha.Verify(req.CaptchaID, slideX, track) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "滑块验证未通过，请重新滑动"})
		return
	}

	// 2) 防爆破锁定检查：连续输错 3 次锁 5 分钟，再次输错 3 次锁 1 小时
	if locked, secs, level := guardLocked(guard); locked {
		c.JSON(http.StatusOK, gin.H{
			"code": 1, "msg": lockMsg(level),
			"locked": true, "lock_seconds": secs, "lock_level": level,
		})
		return
	}

	var user models.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		fails, locked, lockSeconds, lockLevel := guardFail(guard)
		recordLoginLog(req.Username, "", "", 0, ip, "fail", "账号不存在")
		c.JSON(http.StatusOK, loginFailResp(fails, locked, lockSeconds, lockLevel))
		return
	}
	if user.Status != 1 {
		recordLoginLog(user.Username, user.Nickname, user.Role, user.TenantID, ip, "fail", "账号已停用")
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "账号已停用，请联系总后台"})
		return
	}
	// 客户账号（分站）服务到期后禁止登录；总后台不受限
	if user.TenantID > 0 && user.ExpireAt != nil && user.ExpireAt.Before(time.Now()) {
		recordLoginLog(user.Username, user.Nickname, user.Role, user.TenantID, ip, "fail", "账号已到期")
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "账号已于 " + user.ExpireAt.Format("2006-01-02") + " 到期，请联系总后台续费开通",
		})
		return
	}
	if !crypto.Verify(user.Password, req.Password, config.Load().PayloadSecret()) {
		fails, locked, lockSeconds, lockLevel := guardFail(guard)
		recordLoginLog(user.Username, user.Nickname, user.Role, user.TenantID, ip, "fail", "密码错误")
		c.JSON(http.StatusOK, loginFailResp(fails, locked, lockSeconds, lockLevel))
		return
	}
	guardClear(guard)

	token, err := auth.Sign(user.ID, user.Username, user.TenantID, user.Role)
	if err != nil {
		recordLoginLog(user.Username, user.Nickname, user.Role, user.TenantID, ip, "fail", "签发凭证失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "签发凭证失败"})
		return
	}

	recordLoginLog(user.Username, user.Nickname, user.Role, user.TenantID, ip, "success", "登录成功")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"token": token, "user": userInfoMap(&user)}})
}

// loginFailResp 构造登录失败响应：未锁定时提示还差几次锁定，已锁定时返回锁定秒数与级别
func loginFailResp(fails int, locked bool, lockSeconds, lockLevel int) gin.H {
	if locked {
		return gin.H{
			"code": 1, "msg": lockMsg(lockLevel),
			"locked": true, "lock_seconds": lockSeconds, "lock_level": lockLevel,
		}
	}
	return gin.H{
		"code": 1,
		"msg":          fmt.Sprintf("账号或密码错误，连续输错 %d 次将锁定账号", maxLoginFails-fails),
		"remain_fails": maxLoginFails - fails,
	}
}

// Me 当前登录用户信息
func Me(c *gin.Context) {
	var user models.User
	if err := database.DB.First(&user, CurrentUserID(c)).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "账号不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": userInfoMap(&user)})
}
