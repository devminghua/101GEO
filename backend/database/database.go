package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"geo-tool/config"
	"geo-tool/models"
	"geo-tool/services/crypto"
	"geo-tool/services/identicon"
)

var DB *gorm.DB

// DaySQL 返回"按天分组"的 SQL 表达式（PostgreSQL）。
func DaySQL(col string) string {
	return "to_char(" + col + ", 'YYYY-MM-DD')"
}

// TrueCond 返回布尔字段为真的条件表达式（PostgreSQL）。
func TrueCond(col string) string {
	return col + " = true"
}

// FalseCond 返回布尔字段为假的条件表达式（PostgreSQL）。
func FalseCond(col string) string {
	return col + " = false"
}

// IsPostgres 恒为 true（系统已统一使用 PostgreSQL）。
func IsPostgres() bool { return true }

// Init 初始化数据库连接并自动建表，随后写入种子数据。
// 单机版（GEO_LICENSE_MODE=true）用 SQLite 本地文件库；服务器/SaaS 版用 PostgreSQL。
func Init(cfg *config.Config) {
	var db *gorm.DB
	var err error
	if cfg.LicenseMode {
		// 单机版：SQLite 本地文件数据库（客户单机无需安装 PostgreSQL）
		db, err = gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
		if err != nil {
			log.Fatalf("[db] 打开 SQLite 失败: %v", err)
		}
	} else {
		// 服务器/SaaS 版：PostgreSQL
		if cfg.DBDriver != "postgres" {
			log.Fatalf("[db] 仅支持 PostgreSQL 驱动，收到 GEO_DB_DRIVER=%q，请检查配置", cfg.DBDriver)
		}
		if cfg.DBDSN == "" {
			log.Fatalf("[db] 未配置 GEO_DB_DSN，PostgreSQL 模式必须提供连接串")
		}
		db, err = gorm.Open(postgres.Open(cfg.DBDSN), &gorm.Config{})
		if err != nil {
			log.Fatalf("[db] 连接 PostgreSQL 失败: %v", err)
		}
	}
	// 连接池：支持并发
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
	if !cfg.LicenseMode {
		// 兼容历史 SQL 语法：date(timestamptz, 'localtime') 重载为返回 Asia/Shanghai 日期（仅 PG）
		if err := db.Exec(`CREATE OR REPLACE FUNCTION date(timestamptz, text) RETURNS date AS $$
			SELECT ($1 AT TIME ZONE 'Asia/Shanghai')::date
		$$ LANGUAGE sql IMMUTABLE`).Error; err != nil {
			log.Printf("[db] 创建 date 兼容函数失败（非致命）: %v", err)
		}
		log.Printf("[db] 已连接 PostgreSQL（%s）", maskDSN(cfg.DBDSN))
	} else {
		log.Printf("[db] 已打开 SQLite（单机版）：%s", cfg.DBPath)
	}
	DB = db

	migrate()
	seed()
	seedGlobalPlatforms()
	fillMissingAvatars()
}

// defaultGlobalPlatforms 国内主流 AI 平台默认清单（tenant_id=0 全局）。
// 启动时补缺（按名称），不覆盖已有配置；新增平台默认停用（enabled=false），配 Key 后启用。
var defaultGlobalPlatforms = []models.AiPlatform{
	{Name: "DeepSeek", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash"},
	{Name: "豆包", BaseURL: "https://ark.cn-beijing.volces.com/api/v3", Model: "doubao-seed-evolving"},
	{Name: "通义千问", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen3.6-plus"},
	{Name: "智谱 GLM", BaseURL: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.7-flash"},
	{Name: "Kimi（月之暗面）", BaseURL: "https://api.moonshot.cn/v1", Model: "kimi-k2.6"},
	{Name: "腾讯混元", BaseURL: "https://tokenhub.tencentmaas.com/v1/", Model: "hy4-preview"},
	{Name: "百度文心（千帆）", BaseURL: "https://qianfan.baidubce.com/v2", Model: "ernie-5.0"},
	{Name: "MiniMax（稀宇）", BaseURL: "https://api.minimaxi.com/v1", Model: "MiniMax-M2.7"},
	{Name: "讯飞星火", BaseURL: "https://spark-api-open.xf-yun.com/v1", Model: "spark-4.0-ultra"},
	{Name: "零一万物", BaseURL: "https://api.lingyiwanwu.com/v1", Model: "yi-large"},
	{Name: "百川智能", BaseURL: "https://api.baichuan-ai.com/v1", Model: "baichuan4-turbo"},
	{Name: "阶跃星辰", BaseURL: "https://api.stepfun.com/v1", Model: "step-2-16k"},
	{Name: "小米 MiMo", BaseURL: "https://api.xiaomimimo.com/v1", Model: "mimo-v2.5-pro"},
	{Name: "商汤日日新", BaseURL: "https://api.sensenova.cn/v1", Model: "SenseChat-5"},
}

// seedGlobalPlatforms 补种缺失的全局平台（按名称匹配，仅插入不更新，保护已配置的 Key/间隔）。
func seedGlobalPlatforms() {
	for _, p := range defaultGlobalPlatforms {
		var cnt int64
		DB.Model(&models.AiPlatform{}).Where("tenant_id = 0 AND name = ?", p.Name).Count(&cnt)
		if cnt > 0 {
			continue
		}
		p.TenantID = 0
		p.Enabled = true // 先按模型默认值插入（bool false 会被 gorm default:true 吞掉，Create 后再显式 Update）
		p.IntervalMs = 2000
		if err := DB.Create(&p).Error; err != nil {
			log.Printf("[db] 补种平台 %s 失败: %v", p.Name, err)
			continue
		}
		// 新增平台默认停用，避免无 Key 拖累巡检；配 Key 后由管理员启用
		if err := DB.Model(&p).Update("enabled", false).Error; err != nil {
			log.Printf("[db] 停用新平台 %s 失败: %v", p.Name, err)
		}
		log.Printf("[db] 已补种默认平台：%s（%s，默认停用）", p.Name, p.Model)
	}
}

// maskDSN 隐藏 DSN 中的密码，避免日志泄露。
func maskDSN(dsn string) string {
	if i := strings.Index(dsn, "password="); i >= 0 {
		rest := dsn[i+len("password="):]
		end := strings.IndexAny(rest, " ")
		if end < 0 {
			end = len(rest)
		}
		return dsn[:i+len("password=")] + "***" + rest[end:]
	}
	return dsn
}

// fillMissingAvatars 为所有 avatar 为空的老用户补一张 NFT 头像（避免历史账号进内页没头像）。
// 写文件失败只记日志，不影响启动。
func fillMissingAvatars() {
	var users []models.User
	if err := DB.Where("avatar = '' OR avatar IS NULL").Find(&users).Error; err != nil {
		log.Printf("[avatar] 扫描缺失头像失败: %v", err)
		return
	}
	if len(users) == 0 {
		return
	}
	log.Printf("[avatar] 为 %d 个老用户补 NFT 头像", len(users))
	filled := 0
	for i := range users {
		u := &users[i]
		seed := u.Username
		if seed == "" {
			seed = fmt.Sprintf("user-%d", u.ID)
		}
		avatar, err := identicon.Save(u.ID, seed)
		if err != nil || avatar == "" {
			continue
		}
		if err := DB.Model(&models.User{}).Where("id = ?", u.ID).Update("avatar", avatar).Error; err == nil {
			filled++
		}
	}
	log.Printf("[avatar] 已补 %d 张 NFT 头像", filled)
}

func migrate() {
	err := DB.AutoMigrate(
		&models.Tenant{},
		&models.User{},
		&models.Channel{},
		&models.QueryQuota{},
		&models.AiPlatform{},
		// 分站对全局平台的自定义覆盖层（Key / 启用状态）
		&models.TenantPlatformOverride{},
		&models.AiUsageRecord{},
		// 百度指数行业排行
		&models.BaiduIndustryRank{},
		// 邀约奖励 + 成长计划签到
		&models.InviteRecord{},
		&models.CheckinRecord{},
		&models.PointRecord{},
		&models.GeoKeyword{},
		&models.CheckTask{},
		&models.CheckResult{},
		&models.Setting{},
		&models.LoginLog{},
		&models.LoginGuard{},
		// 续费流水（总后台给分站账号续费的记录，对账用）
		&models.ExtendRecord{},

		&models.Notification{},
		// 微信小程序绑定
		&models.MiniappBinding{},
		// 点卡自助扫码充值（微信 Native / 支付宝当面付）
		&models.RechargeOrder{},
		&models.RechargePlan{},
		&models.PayCallbackLog{},
		// 抖音获客模块
		&models.DyAccount{},
		&models.DyPeer{},
		&models.DyVideo{},
		&models.DyLead{},
		&models.DySlogan{},
		&models.DyActionLog{},
		// 小红书获客模块
		&models.XhsAccount{},
		&models.XhsPeer{},
		&models.XhsNote{},
		&models.XhsLead{},
		&models.XhsSlogan{},
		&models.XhsActionLog{},
		&models.XhsNotification{},
		// 智能创作中心模块
		&models.CreativeRole{},
		&models.ChatSession{},
		&models.ChatMessage{},
		&models.CreativeMaterial{},
		&models.CreativeRecord{},
		// 内容投放模块
		&models.CtnMedia{},
		&models.CtnArticle{},
		&models.CtnTask{},
		&models.CtnMonitor{},
		// GEO 智能中心模块
		&models.FactItem{},
		&models.Competitor{},
		&models.Citation{},
		&models.RiskWord{},
		&models.OptTask{},
		&models.AuditResult{},
		// 百度关键词分析模块（按租户隔离）
		&models.BaiduSite{},
		&models.BaiduMonitorKeyword{},
		&models.BaiduRankSnapshot{},
		// 单机版软件级授权激活状态（卡密）
		&models.Activation{},
		// 卡密记录（总后台生成/导出）
		&models.Card{},
		// 帮助文档（分类 + 文档，全局）
		&models.HelpCategory{},
		&models.HelpDoc{},
	)
	if err != nil {
		log.Fatalf("[db] 迁移失败: %v", err)
	}
}

func seed() {
	// 总后台超级管理员：admin / admin123
	var adminCount int64
	DB.Model(&models.User{}).Where("role = ?", "super").Count(&adminCount)
	if adminCount == 0 {
		enc, _ := crypto.Hash("admin123", config.Load().PayloadSecret())
		admin := models.User{
			TenantID: 0, Username: "admin", Password: enc,
			Nickname: "超级管理员", Role: "super", Status: 1,
		}
		DB.Create(&admin)
		log.Printf("[db] 已创建总后台账号 admin / admin123")
	}

	// 默认分站 + 分站账号（便于开箱快速体验）
	tenant := models.Tenant{Name: "示例分站", Code: "demo", Status: 1, Remark: "系统内置示例分站"}
	if err := DB.Where("code = ?", "demo").FirstOrCreate(&tenant).Error; err == nil && tenant.ID > 0 {
		var uCount int64
		DB.Model(&models.User{}).Where("tenant_id = ?", tenant.ID).Count(&uCount)
		if uCount == 0 {
			expireAt := time.Now().AddDate(0, 12, 0)
			enc, _ := crypto.Hash("demo123", config.Load().PayloadSecret())
			DB.Create(&models.User{
				TenantID: tenant.ID, Username: "demo", Password: enc,
				Nickname: "示例分站", Role: "admin", Status: 1,
				OpenMonths: 12, ExpireAt: &expireAt,
			})
			log.Printf("[db] 已创建示例分站账号 demo / demo123（服务有效期 12 个月）")
		} else {
			upgradeDemoPassword(tenant.ID)
		}
	}

	// 默认关键词文案（为每个租户准备一份默认关键词，便于直接跑巡检）
	seedDefaultKeyword(tenant)

	// GEO 智能：内置风险词库 + 竞品库 + 事实库示例（仅演示租户，真实租户各自维护）
	seedGeoIntel(tenant)
	log.Printf("[db] 种子数据就绪")
}

// seedGeoIntel 为演示租户写入 GEO 智能模块的种子数据（风险词/竞品/事实库）
func seedGeoIntel(tenant models.Tenant) {
	if tenant.ID == 0 {
		return
	}
	// 风险词库（命中即代表回答存在幻觉/过度承诺/合规风险）
	var rc int64
	DB.Model(&models.RiskWord{}).Where("tenant_id = ?", tenant.ID).Count(&rc)
	if rc == 0 {
		defaults := []models.RiskWord{
			{Word: "100%保证", Reason: "绝对化承诺，涉嫌虚假宣传"},
			{Word: "保证成功", Reason: "绝对化承诺"},
			{Word: "肯定能成", Reason: "绝对化承诺"},
			{Word: "稳赚不赔", Reason: "金融/投资类违规用语"},
			{Word: "国家级", Reason: "绝对化用语，违反广告法"},
			{Word: "最高级", Reason: "绝对化用语，违反广告法"},
			{Word: "独一无二", Reason: "绝对化用语"},
			{Word: "包治", Reason: "医疗类违规用语"},
			{Word: "贷款", Reason: "婚恋场景涉金融诱导"},
		}
		for i := range defaults {
			defaults[i].TenantID = tenant.ID
		}
		DB.Create(&defaults)
	}
	// 竞品库示例（占位，运营可按需增删）
	var cc int64
	DB.Model(&models.Competitor{}).Where("tenant_id = ?", tenant.ID).Count(&cc)
	if cc == 0 {
		DB.Create(&models.Competitor{TenantID: tenant.ID, Name: "珍爱网", Remark: "行业头部竞品示例，可修改或删除", Enabled: true})
		DB.Create(&models.Competitor{TenantID: tenant.ID, Name: "百合网", Remark: "行业头部竞品示例，可修改或删除", Enabled: true})
		DB.Create(&models.Competitor{TenantID: tenant.ID, Name: "世纪佳缘", Remark: "行业头部竞品示例，可修改或删除", Enabled: true})
	}
	// 事实库示例（婚恋 SaaS 场景，运营可按需完善）
	var fc int64
	DB.Model(&models.FactItem{}).Where("tenant_id = ?", tenant.ID).Count(&fc)
	if fc == 0 {
		brand := config.Load().DefaultBrand
		if brand == "" {
			brand = "轻媒"
		}
		facts := []models.FactItem{
			{Category: "品类", Fact: brand + " 是面向婚恋门店的 CRM / 谈单 SaaS 系统，服务门店老板与红娘机构", Enabled: true},
			{Category: "能力", Fact: "支持资料录入、匹配对象、服务套餐、收银台、合同签署等全流程管理", Enabled: true},
			{Category: "适用场景", Fact: "适用于婚恋门店、红娘机构的客户管理与谈单数字化", Enabled: true},
			{Category: "不适用", Fact: "不是面向 C 端用户的陌生人社交 App", NotFact: "交友软件/社交App", Enabled: true},
			{Category: "服务边界", Fact: "系统不提供情感咨询或线下婚介撮合服务本身，而是赋能门店经营管理", NotFact: "保证脱单", Enabled: true},
		}
		for i := range facts {
			facts[i].TenantID = tenant.ID
		}
		DB.Create(&facts)
	}
}

// upgradeDemoPassword 将内置演示账号 demo 的 bcrypt 旧密码升级为可逆加密格式，仅当仍为初始密码时执行；
// 升级后 SaaS 总后台即可通过"小眼睛"直接查看明文。用户改过的密码不受影响。
func upgradeDemoPassword(tenantID uint) {
	var u models.User
	if err := DB.Where("tenant_id = ? AND username = ?", tenantID, "demo").First(&u).Error; err != nil {
		return
	}
	if crypto.IsEncrypted(u.Password) {
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("demo123")) != nil {
		return // 已改密，跳过升级
	}
	enc, err := crypto.Hash("demo123", config.Load().PayloadSecret())
	if err != nil {
		return
	}
	DB.Model(&u).Update("password", enc)
	log.Printf("[db] 演示账号 demo 密码已升级为可逆加密格式")
}

func seedDefaultKeyword(tenant models.Tenant) {
	if tenant.ID == 0 {
		return
	}
	var count int64
	DB.Model(&models.GeoKeyword{}).Where("tenant_id = ?", tenant.ID).Count(&count)
	if count > 0 {
		return
	}
	brand := config.Load().DefaultBrand
	kw := models.GeoKeyword{TenantID: tenant.ID, Category: "品牌", Enabled: true}
	if brand != "" {
		kw.Question = "推荐一下" + brand + "？它怎么样？"
		kw.BrandKeywords = brand
	} else {
		kw.Question = "推荐一下示例品牌？它怎么样？"
		kw.BrandKeywords = "示例品牌"
	}
	DB.Create(&kw)
}
