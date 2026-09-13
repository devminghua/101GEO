package handlers

import (
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
	"geo-tool/services/pay"
)

// ListTenants 分站列表（含每个分站的账号数、任务数与功能授权数组）
func ListTenants(c *gin.Context) {
	var tenants []models.Tenant
	database.DB.Order("id asc").Find(&tenants)

	type tenantRow struct {
		ID           uint      `json:"id"`
		Name         string    `json:"name"`
		Code         string    `json:"code"`
		Status       int       `json:"status"`
		Remark       string    `json:"remark"`
		Features     []string  `json:"features"`
		CreatedAt    time.Time `json:"created_at"`
		AccountCount int64     `json:"account_count"`
		TaskCount    int64     `json:"task_count"`
	}
	rows := make([]tenantRow, 0, len(tenants))
	for _, t := range tenants {
		var acc, tasks int64
		database.DB.Model(&models.User{}).Where("tenant_id = ?", t.ID).Count(&acc)
		database.DB.Model(&models.CheckTask{}).Where("tenant_id = ?", t.ID).Count(&tasks)
		rows = append(rows, tenantRow{
			ID: t.ID, Name: t.Name, Code: t.Code, Status: t.Status, Remark: t.Remark,
			Features: parseFeatures(t.Features), CreatedAt: t.CreatedAt,
			AccountCount: acc, TaskCount: tasks,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}

type tenantReq struct {
	Name     string   `json:"name"`
	Code     string   `json:"code"`
	Logo     string   `json:"logo"`
	Remark   string   `json:"remark"`
	Features []string `json:"features"`
}

// CreateTenant 建立分站
func CreateTenant(c *gin.Context) {
	var req tenantReq
	if !jsonBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if req.Name == "" || req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "分站名称与标识不能为空"})
		return
	}
	var count int64
	database.DB.Model(&models.Tenant{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "分站标识已存在"})
		return
	}
	t := models.Tenant{Name: req.Name, Code: req.Code, Logo: req.Logo, Remark: req.Remark, Status: 1, Features: marshalFeatures(req.Features)}
	if err := database.DB.Create(&t).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "分站已创建", "data": t})
}

// UpdateTenant 更新分站名称/状态/备注
func UpdateTenant(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var t models.Tenant
	if err := database.DB.First(&t, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在"})
		return
	}
	if !tenantOwnedByChannel(c, t.ID) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该客户"})
		return
	}
	var req struct {
		Name            *string  `json:"name"`
		Logo            *string  `json:"logo"`
		Status          *int     `json:"status"`
		Remark          *string  `json:"remark"`
		Features        []string `json:"features"`
		DailyQueryLimit *int     `json:"daily_query_limit"` // 每日查询上限（0=不限）
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Name != nil {
		t.Name = strings.TrimSpace(*req.Name)
	}
	if req.Logo != nil {
		t.Logo = strings.TrimSpace(*req.Logo)
	}
	if req.Status != nil {
		if *req.Status != 0 && *req.Status != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "状态值非法"})
			return
		}
		t.Status = *req.Status
	}
	if req.Remark != nil {
		t.Remark = *req.Remark
	}
	if req.Features != nil {
		t.Features = marshalFeatures(req.Features)
	}
	if req.DailyQueryLimit != nil {
		if *req.DailyQueryLimit < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "每日查询上限不能为负数"})
			return
		}
		t.DailyQueryLimit = *req.DailyQueryLimit
	}
	database.DB.Save(&t)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
}

// DeleteTenant 停用并删除分站（同时清理账号与业务数据）
func DeleteTenant(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var t models.Tenant
	if err := database.DB.First(&t, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在"})
		return
	}
	if !tenantOwnedByChannel(c, t.ID) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该客户"})
		return
	}
	tx := database.DB.Begin()
	tx.Where("tenant_id = ?", t.ID).Delete(&models.User{})
	tx.Where("tenant_id = ?", t.ID).Delete(&models.AiPlatform{})
	tx.Where("tenant_id = ?", t.ID).Delete(&models.GeoKeyword{})
	tx.Where("tenant_id = ?", t.ID).Delete(&models.CheckTask{})
	tx.Where("tenant_id = ?", t.ID).Delete(&models.CheckResult{})
	tx.Where("tenant_id = ?", t.ID).Delete(&models.Setting{})
	tx.Delete(&t)
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "分站已删除"})
}

// ListUsers 账号列表（分站账号列表 + 总后台账号）
func ListUsers(c *gin.Context) {
	all := c.DefaultQuery("all", "0") == "1"
	db := database.DB.Model(&models.User{})
	if !all {
		// 默认只列分站账号
		db = db.Where("tenant_id > 0")
	}
	var users []models.User
	if err := db.Order("id asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	// 附加有效期剩余天数（-1=不限，0=已到期）
	type userRow struct {
		models.User
		RemainDays int `json:"remain_days"`
	}
	rows := make([]userRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, userRow{User: u, RemainDays: remainDaysOf(&u)})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}

type userReq struct {
	TenantID   uint   `json:"tenant_id"`
	TenantCode string `json:"tenant_code"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	OpenMonths int    `json:"open_months"` // 开通月数（1-36）
}

// CreateUser 新建账号并设置账号密码，按开通月数计算服务到期时间
func CreateUser(c *gin.Context) {
	var req userReq
	if !jsonBody(c, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "账号与密码不能为空"})
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": msg})
		return
	}
	if req.OpenMonths < 1 || req.OpenMonths > 36 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请选择开通月数（1-36 个月）"})
		return
	}
	var count int64
	database.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "账号已存在"})
		return
	}
	// 确定所属分站：优先按分站标识，其次按分站 ID
	tenantID := req.TenantID
	if code := strings.TrimSpace(req.TenantCode); code != "" {
		var t models.Tenant
		if err := database.DB.Where("code = ?", code).First(&t).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在，请先建立该分站"})
			return
		}
		tenantID = t.ID
	} else if tenantID > 0 {
		var t models.Tenant
		if err := database.DB.First(&t, tenantID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在，请先建立该分站"})
			return
		}
	}
	if tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请指定所属分站（tenant_code 或 tenant_id）"})
		return
	}
	encPwd, err := crypto.Hash(req.Password, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密码加密失败"})
		return
	}
	expireAt := time.Now().AddDate(0, req.OpenMonths, 0)
	u := models.User{
		TenantID: tenantID, Username: req.Username, Password: encPwd,
		Nickname: req.Nickname, Role: "admin", Status: 1,
		OpenMonths: req.OpenMonths, ExpireAt: &expireAt,
	}
	if err := database.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败：" + err.Error()})
		return
	}
	u.Password = ""
	u.ExpireAt = &expireAt
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "账号已创建，服务有效期至 " + expireAt.Format("2006-01-02"), "data": u})
}

// ExtendUserService 总后台为账号续费/延长开通时长：POST /api/super/users/:id/extend
// 从当前到期时间顺延 N 个月；已到期（或未设置到期时间）则从今天起算。
// 同步更新 open_months 记录累计开通时长，便于账号列表展示口径一致。
func ExtendUserService(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u models.User
	if err := database.DB.First(&u, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "账号不存在"})
		return
	}
	if u.Role == "super" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "总后台账号无需续费"})
		return
	}
	if !userOwnedByChannel(c, u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该账号"})
		return
	}
	var req struct {
		Months    int    `json:"months"`
		Remark    string `json:"remark"`
		PayMethod string `json:"pay_method"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Months < 1 || req.Months > 36 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "续费月数须为 1-36 个月"})
		return
	}
	// 收款方式白名单；赠送则金额记 0（对账时不计收入）
	payMethod := strings.TrimSpace(req.PayMethod)
	switch payMethod {
	case "微信", "支付宝", "银行转账", "现金", "赠送":
	default:
		payMethod = "微信"
	}
	// 金额 = 续费月单价 × 月数（单价取全局支付配置，写入快照，改价不影响历史流水）；赠送记 0
	priceFen := pay.DefaultExtendPriceFenMonth
	if cfg, err := pay.LoadSecretConfig(); err == nil {
		priceFen = cfg.ExtendMonthPrice()
	}
	amountFen := priceFen * int64(req.Months)
	if payMethod == "赠送" {
		amountFen = 0
	}
	now := time.Now()
	// 起算点：未到期取当前到期时间；已到期或未设置则从现在起算
	base := now
	if u.ExpireAt != nil && u.ExpireAt.After(now) {
		base = *u.ExpireAt
	}
	newExpire := base.AddDate(0, req.Months, 0)
	// 续费前到期时间须在 Updates 前做「值拷贝」快照：
	// u.ExpireAt 是指针，GORM Updates 会就地改写其指向的 time.Time，浅拷贝指针会跟着变。
	var before *time.Time
	if u.ExpireAt != nil {
		t := *u.ExpireAt
		before = &t
	}
	updates := map[string]interface{}{
		"expire_at":   newExpire,
		"open_months": u.OpenMonths + req.Months,
	}
	if err := database.DB.Model(&u).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "续费失败：" + err.Error()})
		return
	}
	// 续费流水（对账用）：记录操作者、客户、月数、收款方式、金额与续费前后到期时间
	var tenantName string
	database.DB.Model(&models.Tenant{}).Where("id = ?", u.TenantID).Pluck("name", &tenantName)
	operator := "admin"
	var opUser models.User
	if uid := CurrentUserID(c); uid > 0 {
		if err := database.DB.First(&opUser, uid).Error; err == nil && opUser.Username != "" {
			operator = opUser.Username
		}
	}
	rec := models.ExtendRecord{
		TenantID: u.TenantID, TenantName: tenantName, UserID: u.ID, Username: u.Username,
		Operator: operator, Months: req.Months, PriceFen: priceFen, AmountFen: amountFen, PayMethod: payMethod,
		ExpireBefore: before, ExpireAfter: newExpire,
		Remark: strings.TrimSpace(req.Remark),
	}
	database.DB.Create(&rec)
	// 注意：GORM Updates 已就地改写 u.OpenMonths（= 原值+月数），响应直接取它，勿再叠加月数
	newOpenMonths := u.OpenMonths
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "续费成功，服务有效期至 " + newExpire.Format("2006-01-02"), "data": gin.H{
		"id": u.ID, "username": u.Username, "tenant_id": u.TenantID,
		"expire_at": newExpire, "open_months": newOpenMonths,
		"months": req.Months, "price_fen": priceFen, "amount_fen": amountFen, "pay_method": payMethod,
	}})
}

// ListExtendRecords 续费流水查询：GET /api/super/extend-records
// 支持按分站（tenant_id）与月份范围过滤，按时间倒序，供对账。
func ListExtendRecords(c *gin.Context) {
	// 过滤条件独立成闭包：列表与聚合两处复用，避免链式污染
	scoped := func() *gorm.DB {
		db := database.DB.Model(&models.ExtendRecord{})
		if v := c.Query("tenant_id"); v != "" {
			if id, err := strconv.Atoi(v); err == nil && id > 0 {
				db = db.Where("tenant_id = ?", id)
			}
		}
		if v := c.Query("month"); v != "" {
			// month=YYYY-MM：过滤该自然月内的续费（跨数据库兼容，Go 端算月份范围）
			if t, err := time.Parse("2006-01", v); err == nil {
				db = db.Where("created_at >= ? AND created_at < ?", t, t.AddDate(0, 1, 0))
			}
		}
		return db
	}
	limit := 200
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	var records []models.ExtendRecord
	scoped().Order("id desc").Limit(limit).Find(&records)
	if records == nil {
		records = []models.ExtendRecord{}
	}
	// 汇总（当前过滤条件下）：总条数、总月数、总金额（分）。
	// 🔴 必须独立查询：若复用上面带 Order/Limit 的链，PG 下聚合查询会报
	// 「column must appear in GROUP BY」，错误被 GORM 吞掉后 sums 全是 0，
	// 续费对账页合计永远是 0（v1.0.47 修复）。
	var total int64
	var sums struct {
		Months    int64
		AmountFen int64
	}
	scoped().Select("coalesce(sum(months),0) as months, coalesce(sum(amount_fen),0) as amount_fen").Scan(&sums)
	scoped().Count(&total)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"list": records, "total": total,
		"total_months":    sums.Months,
		"total_amount_fen": sums.AmountFen,
	}})
}

// ResetUserPassword 重置账号密码
func ResetUserPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u models.User
	if err := database.DB.First(&u, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "账号不存在"})
		return
	}
	if !userOwnedByChannel(c, u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该账号"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": msg})
		return
	}
	encPwd, err := crypto.Hash(req.Password, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密码加密失败"})
		return
	}
	database.DB.Model(&u).Update("password", encPwd)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "密码已重置"})
}

// UserPlaintextPassword SaaS 总后台"小眼睛"查看客户账号明文密码。
// 仅支持可逆加密格式（重置后）；历史 bcrypt 密码不可逆，返回提示。
func UserPlaintextPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u models.User
	if err := database.DB.First(&u, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "账号不存在"})
		return
	}
	if u.Role == "super" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "不允许查看总后台账号"})
		return
	}
	if !userOwnedByChannel(c, u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该账号"})
		return
	}
	if !crypto.IsEncrypted(u.Password) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该账号为历史加密密码，无法直接查看；重置一次密码后即可查看明文"})
		return
	}
	plain, err := crypto.Decrypt(u.Password, config.Load().PayloadSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "密码解密失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"id": u.ID, "username": u.Username, "tenant_id": u.TenantID, "plaintext": plain,
	}})
}

// UpdateUserStatus 启用/停用账号
func UpdateUserStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Status int `json:"status"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if !userOwnedByChannel(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该账号"})
		return
	}
	database.DB.Model(&models.User{}).Where("id = ?", id).Update("status", req.Status)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
}

// DeleteUser 删除账号
func DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u models.User
	if err := database.DB.First(&u, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "账号不存在"})
		return
	}
	if u.Role == "super" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "不允许删除总后台账号"})
		return
	}
	if !userOwnedByChannel(c, u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该账号"})
		return
	}
	database.DB.Delete(&u)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "账号已删除"})
}

// SimulateTenantLogin 总后台一键模拟登录指定分站：签发该分站首个启用 admin 账号的 JWT。
// 返回结构与登录接口一致 {token, user}。
func SimulateTenantLogin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var t models.Tenant
	if err := database.DB.First(&t, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在"})
		return
	}
	if !tenantOwnedByChannel(c, t.ID) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权操作该客户"})
		return
	}
	if t.Status != 1 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "分站已停用，无法模拟登录"})
		return
	}
	var u models.User
	if err := database.DB.Where("tenant_id = ? AND role = ? AND status = 1", t.ID, "admin").Order("id asc").First(&u).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该分站暂无启用账号，请先在账号管理创建账号后再模拟登录"})
		return
	}
	if u.ExpireAt != nil && u.ExpireAt.Before(time.Now()) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "该分站账号已于 " + u.ExpireAt.Format("2006-01-02") + " 到期，无法模拟登录"})
		return
	}
	token, err := auth.Sign(u.ID, u.Username, u.TenantID, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "签发凭证失败"})
		return
	}
	recordLoginLog(u.Username, u.Nickname, u.Role, u.TenantID, c.ClientIP(), "success", "总后台模拟登录")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"token": token, "user": userInfoMap(&u)}})
}
