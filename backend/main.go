package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/handlers"
	super_h "geo-tool/handlers/super"
	"geo-tool/models"
	"geo-tool/services/biztime"
	"geo-tool/services/geo"
	"geo-tool/services/notify"
)

//go:embed webdist
var webDistFS embed.FS

// indexHTML 返回内嵌的前端入口页（SPA 兜底与根路由共用），失败时给一个可读的占位页。
func indexHTML() []byte {
	b, err := webDistFS.ReadFile("webdist/index.html")
	if err != nil {
		return []byte("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>LinkGeo</title></head><body>前端资源缺失，请重新安装。</body></html>")
	}
	return b
}

func main() {
	cfg := config.Load()
	database.Init(cfg)

	// sqlmap REST API 守护：SQL 注入检测依赖容器内 sqlmapapi.py（127.0.0.1:8775）。
	// 未运行时自动拉起（python3 + /usr/share/sqlmap/sqlmapapi.py），容器重启后自愈。
	go ensureSqlmapAPI()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS：允许小程序 H5 端（微信开发者工具 / 浏览器演示）跨域访问 API
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// 前端静态资源（go:embed 内嵌，与工作目录/安装目录解耦，杜绝「安装后相对路径失效即崩溃」）
	assetsFS, err := fs.Sub(webDistFS, "webdist/assets")
	if err != nil {
		log.Fatalf("[embed] 内嵌前端资源缺失: %v", err)
	}
	r.StaticFS("/assets", http.FS(assetsFS))
	r.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML())
	})

	// SPA 路由兜底：非 /api 且非 /uploads 的未匹配 GET 均回退到 index.html，保证前端路由刷新不 404
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && !strings.HasPrefix(c.Request.URL.Path, "/api/") && !strings.HasPrefix(c.Request.URL.Path, "/uploads") {
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML())
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "not found"})
	})

	// 智能创作中心生成的图片/视频等静态资产（uploads/creative）+ 系统 Logo（uploads/）
	// 仅注册根目录 /uploads 即可同时覆盖子目录 creative（gin 不允许再注册更具体的同前缀通配路由）
	_ = os.MkdirAll(filepath.Join(cfg.UploadsDir, "creative"), 0o755)
	r.Static("/uploads", cfg.UploadsDir)

	// 公开接口：登录 + 滑块验证码 + 有效期查询 + 系统品牌信息 + 短信验证码 + 自助注册
	r.POST("/api/auth/login", handlers.Login)
	r.GET("/api/auth/captcha", handlers.GetCaptcha)
	r.GET("/api/auth/expiry", handlers.AuthExpiry)
	r.GET("/api/system/info", handlers.GetSystemInfo)
	// 升级日志（公开只读）：历代版本更新记录，客户端系统设置与登录页均可展示
	r.GET("/api/changelog", handlers.Changelog)
	r.POST("/api/auth/sms-code", handlers.SmsSend)
	r.POST("/api/auth/email-code", handlers.EmailCode)
	r.POST("/api/auth/register", handlers.Register)
	// 注册配置（公开）：自助注册开关 + 短信验证开关，前端注册页据此渲染
	r.GET("/api/auth/register-config", handlers.RegisterConfig)

	// 微信小程序：登录/绑定（公开，无需后台鉴权）
	r.POST("/api/miniapp/login", handlers.MiniappLogin)
	r.POST("/api/miniapp/bind", handlers.MiniappBind)

	// 单机版卡密授权：激活状态查询 + 激活（公开，激活前前端仅有此入口）
	r.GET("/api/license/status", handlers.GetLicenseStatus)
	r.POST("/api/license/activate", handlers.ActivateLicense)

	// 点卡扫码充值支付回调（公开路由，不挂登录；验签 + 幂等入账在 handlers 内完成）
	r.POST("/notify/wechat", handlers.NotifyWechat)
	r.POST("/notify/alipay", handlers.NotifyAlipay)

	// 业务 API：需登录，数据按租户隔离
	api := r.Group("/api")
	api.Use(handlers.AuthRequired)
	api.Use(handlers.LicenseGuard())
	api.Use(handlers.FeatureGuard())
	{
		api.GET("/auth/me", handlers.Me)
		// 微信小程序效果总览聚合（登录后）
		api.GET("/miniapp/home", handlers.MiniappHome)
		api.POST("/auth/change-password", handlers.ChangePassword)
		// 登录日志：仅 super / admin 可查（AI 优化员为受限账号，无安全审计权限）
		api.GET("/auth/login-logs", handlers.AdminOnly(), handlers.ListLoginLogs)

		// 系统品牌配置：读公开（所有登录用户，公开路由 GET /api/system/info 已覆盖），写仅 super/admin；Logo 上传仅 super/admin
		api.POST("/system/info", handlers.AdminOnly(), handlers.SaveSystemInfo)
		api.GET("/system/my-brand", handlers.MyBrandInfo)
		api.POST("/system/upload-logo", handlers.AdminOnly(), handlers.UploadLogo)
		api.POST("/system/upload-image", handlers.AdminOnly(), handlers.UploadImage)

		// AI 优化员账号管理（仅 super/admin；super 可跨租户，admin 仅本租户）
		api.GET("/operator/list", handlers.AdminOnly(), handlers.ListOperators)
		api.POST("/operator/create", handlers.AdminOnly(), handlers.CreateOperator)
		api.DELETE("/operator/:id", handlers.AdminOnly(), handlers.DeleteOperator)

		api.GET("/dashboard/overview", handlers.DashboardOverview)
		api.GET("/dashboard/summary", handlers.DashboardSummary)
		api.GET("/dashboard/trend", handlers.DashboardTrend)
		api.GET("/dashboard/platforms", handlers.DashboardPlatforms)
		api.GET("/dashboard/keywords", handlers.DashboardKeywords)
		api.GET("/platforms/health", handlers.PlatformHealthCheck)

		api.GET("/keywords", handlers.ListKeywords)
		api.GET("/keywords/categories", handlers.GetCategories)
		api.POST("/keywords", handlers.CreateKeyword)
		api.POST("/keywords/bulk", handlers.BulkCreateKeywords)
		api.POST("/keywords/batch-delete", handlers.BatchDeleteKeywords)
		api.PUT("/keywords/:id", handlers.UpdateKeyword)
		api.DELETE("/keywords/:id", handlers.DeleteKeyword)

		// 话题簇（提示词聚类）：把扁平关键词按搜索意图聚成话题，支撑按簇的覆盖率诊断
		api.GET("/clusters", handlers.ListClusters)
		api.POST("/clusters", handlers.CreateCluster)
		api.PUT("/clusters/:id", handlers.UpdateCluster)
		api.DELETE("/clusters/:id", handlers.DeleteCluster)
		api.POST("/clusters/assign", handlers.AssignCluster)
		api.POST("/clusters/generate", handlers.GenClusters)

		// AI 平台（API 配置/密钥）：分级管理 —— 分站 admin 可配置自己的平台（自己的 Key），
		// 未配置时继承总后台全局平台(tenant_id=0)；operator 无权限
		platforms := api.Group("/platforms")
		platforms.Use(handlers.AdminOnly())
		{
			platforms.GET("", handlers.ListPlatforms)
			platforms.GET("/templates", handlers.PlatformTemplates)
			platforms.POST("", handlers.CreatePlatform)
			platforms.PUT("/:id", handlers.UpdatePlatform)
			platforms.DELETE("/:id", handlers.DeletePlatform)
			platforms.POST("/:id/test", handlers.TestPlatform)
		}

		// 分站点卡中心：查询余额与流水（admin 权限，仅本租户）
		api.GET("/points", handlers.AdminOnly(), handlers.GetPoints)
		api.GET("/quota/info", handlers.QueryQuotaInfo)
		api.GET("/quota/short-video", handlers.ShortVideoQuotaInfo)

		// 充值卡密兑换 token（分站 admin 用，二选一充值方式之一）
		api.POST("/card/redeem", handlers.AdminOnly(), handlers.RedeemCard)

		// 点卡自助扫码充值（微信/支付宝）：下单 + 轮询状态（admin 权限，仅本租户）
		api.POST("/pay/recharge", handlers.AdminOnly(), handlers.RechargeCreate)
		api.GET("/pay/recharge/:order_no", handlers.AdminOnly(), handlers.RechargeStatus)
		// 价格套餐：客户端查套餐 + 选套餐下单
		api.GET("/recharge/plans", handlers.AdminOnly(), handlers.ListRechargePlans)
		api.POST("/recharge/plans/:id/order", handlers.AdminOnly(), handlers.RechargePlanOrder)

		api.POST("/tasks/run", handlers.RunTask)
		api.GET("/tasks", handlers.ListTasks)
		api.GET("/tasks/:id", handlers.TaskDetail)
		api.GET("/report", handlers.GenerateReport)
		api.GET("/report/data", handlers.ReportData)
		api.GET("/report/export", handlers.ExportReport)
		// 租户设置读写（敏感配置）：仅 super/admin；AI 优化员无配置权限
		api.GET("/settings", handlers.AdminOnly(), handlers.GetSettings)
		api.POST("/settings", handlers.AdminOnly(), handlers.SaveSettings)

		// 站内信：SaaS 端统一推送，客户端按 user/tenant/all 范围匹配
		api.GET("/notifications", handlers.NotificationList)
		api.GET("/notifications/unread_count", handlers.NotificationUnreadCount)
		api.POST("/notifications/:id/read", handlers.NotificationRead)
		api.POST("/notifications/read_all", handlers.NotificationReadAll)
		// 帮助文档（使用教程）：客户端只读（目录树 + 详情）
		api.GET("/help/tree", handlers.HelpTree)
		api.GET("/help/doc/:id", handlers.HelpDocDetail)
		// 成功案例（SaaS 端上传，客户端展示）
		api.GET("/cases", handlers.CaseList)
		api.GET("/cases/:id", handlers.CaseDetail)

		// 百度关键词分析
		api.POST("/baidu/analyze", handlers.AnalyzeBaiduKeyword)
		// 国际搜索优化：Google/Naver 关键词分析（P1：google）
		api.POST("/intl/analyze", handlers.AnalyzeIntlKeyword)
		api.POST("/intl/index-count", handlers.IntlIndexCount)
		api.POST("/intl/trends", handlers.IntlTrends)
		api.GET("/intl/data-source-status", handlers.IntlDataSourceStatus)
		api.GET("/intl/data-source", handlers.IntlDataSourceGet)
		api.POST("/intl/data-source", handlers.IntlDataSourceSave)
		api.POST("/baidu/suggest", handlers.KeywordSuggest)
		// 站点体检（四层）+ 差距诊断（三缺口）
		api.POST("/baidu/site-audit", handlers.SiteAuditDetail)
		// 站点体检 · Nmap 端口扫描（老板 2026-09-14 需求）
		api.POST("/baidu/site-audit/portscan", handlers.SitePortScan)
		// SQL 注入检测（sqlmap 图形界面）
		api.POST("/baidu/site-audit/sqlmap/start", handlers.SQLMapStart)
		api.GET("/baidu/site-audit/sqlmap/status", handlers.SQLMapStatus)
		api.GET("/baidu/site-audit/sqlmap/data", handlers.SQLMapData)
		api.GET("/baidu/site-audit/sqlmap/log", handlers.SQLMapLog)
		api.DELETE("/baidu/site-audit/sqlmap/task", handlers.SQLMapDelete)
		api.GET("/baidu/site-audit/sqlmap/options", handlers.SQLMapOptions)
		api.POST("/baidu/site-audit/sqlmap/analyze", handlers.SQLMapAnalyze)
		api.GET("/baidu/gap-diagnose", handlers.GapDiagnose)
		// 百度指数行业排行（各行业 TOP 品牌指数）
		api.GET("/baidu/industry-rank", handlers.IndustryRank)
		api.POST("/baidu/industry-rank/refresh", handlers.IndustryRankRefresh)
		// 客户网站配置 CRUD（按租户隔离）
		api.GET("/baidu/sites", handlers.BaiduListSites)
		api.POST("/baidu/sites", handlers.BaiduCreateSite)
		api.PUT("/baidu/sites/:id", handlers.BaiduUpdateSite)
		api.DELETE("/baidu/sites/:id", handlers.BaiduDeleteSite)
		// 监控关键词配置 CRUD（按租户隔离）
		api.GET("/baidu/monitor-keywords", handlers.BaiduListMonitorKeywords)
		api.POST("/baidu/monitor-keywords", handlers.BaiduCreateMonitorKeyword)
		api.PUT("/baidu/monitor-keywords/:id", handlers.BaiduUpdateMonitorKeyword)
		api.DELETE("/baidu/monitor-keywords/:id", handlers.BaiduDeleteMonitorKeyword)
		// 排名历史查询（keyword + days）
		api.GET("/baidu/rank-history", handlers.BaiduRankHistory)
	api.GET("/baidu/rank-overview", handlers.RankOverview)
	api.POST("/baidu/index-count", handlers.BaiduIndexCount)

		/* ---- GEO 智能中心（事实库/竞品/引用/指标/缺口/行动/审计） ---- */
		// 品牌事实库
		api.GET("/facts", handlers.ListFacts)
		api.POST("/facts", handlers.CreateFact)
		api.PUT("/facts/:id", handlers.UpdateFact)
		api.DELETE("/facts/:id", handlers.DeleteFact)
		// 竞品库
		api.GET("/competitors", handlers.ListCompetitors)
		api.POST("/competitors", handlers.CreateCompetitor)
		api.PUT("/competitors/:id", handlers.UpdateCompetitor)
		api.DELETE("/competitors/:id", handlers.DeleteCompetitor)
		// 风险词库
		api.GET("/risk-words", handlers.ListRiskWords)
		api.POST("/risk-words", handlers.CreateRiskWord)
		api.DELETE("/risk-words/:id", handlers.DeleteRiskWord)
		// 引用溯源
		api.GET("/citations", handlers.ListCitations)
		api.GET("/citations/domains", handlers.CitationDomains)
		api.GET("/geo/source-gaps", handlers.SourceGaps)
		api.GET("/geo/channels", handlers.ListChannels)
		api.GET("/geo/setup-check", handlers.GeoSetupCheck)
		// 获客工具（前端本地解析，服务器仅扣费；解析接口仅做链接解析，视频仍直连平台）
		api.GET("/tools/status", handlers.ToolStatus)
		api.POST("/tools/consume", handlers.ToolConsume)
		api.POST("/tools/parse-video", handlers.ParseVideo)
		// 邀约奖励 + 成长计划（客户端）
		api.GET("/invite/summary", handlers.InviteSummary)
		api.GET("/growth/summary", handlers.GrowthSummary)
		api.POST("/growth/checkin", handlers.Checkin)
		// 六项核心指标 & 缺口分析
		api.GET("/geo/intel", handlers.GeoIntel)
		api.GET("/geo/gaps", handlers.GeoGaps)
		// 工作日志：把巡检/行动/复测/内容/审计等自动动作归一成时间线，让客户看懂系统干了什么
		api.GET("/geo/worklog", handlers.WorkLog)
		// 效果归因：前后期对比
		api.GET("/geo/compare", handlers.CompareGeoIntel)
		// 优化行动清单
		api.GET("/geo/actions", handlers.ListOptTasks)
		api.POST("/geo/actions/generate", handlers.GenerateOptTasks)
		api.PUT("/geo/actions/:id", handlers.UpdateOptTask)
		api.DELETE("/geo/actions/:id", handlers.DeleteOptTask)
		// 闭环：完成后复测效果 + 四段闭环概览
		api.POST("/geo/actions/:id/verify", handlers.VerifyOptTaskLoop)
		api.GET("/geo/loop-summary", handlers.LoopSummary)
		// 网站 GEO 审计 & 生成器
		api.POST("/geo/audit", handlers.RunAudit)
		api.GET("/geo/audits", handlers.ListAudits)
		api.GET("/geo/llms", handlers.GenerateLLMS)
		api.GET("/geo/schema", handlers.GenerateSchema)

		// AI 数据分析助手（右侧悬浮对话框）：注入本租户真实 GEO 数据，由 DeepSeek 解读优化效果并给改进建议
		api.GET("/assistant/snapshot", handlers.AssistantSnapshot)
		api.GET("/assistant/quick-asks", handlers.AssistantQuickAsks)
		api.POST("/assistant/chat", handlers.AssistantChat)
		api.GET("/assistant/sessions", handlers.AssistantSessions)
		api.GET("/assistant/sessions/:id/messages", handlers.AssistantMessages)
		api.DELETE("/assistant/sessions/:id", handlers.AssistantDeleteSession)

		/* ---- 抖音获客（quank） ---- */
		// 1) 账号管理
		api.GET("/douyin/accounts", handlers.DouyinListAccounts)
		api.POST("/douyin/accounts", handlers.DouyinCreateAccount)
		api.PUT("/douyin/accounts/:id", handlers.DouyinUpdateAccount)
		api.DELETE("/douyin/accounts/:id", handlers.DouyinDeleteAccount)
		api.POST("/douyin/accounts/:id/refresh", handlers.DouyinRefreshAccount)
		// 2) 同行追踪
		api.GET("/douyin/peers", handlers.DouyinListPeers)
		api.POST("/douyin/peers/import", handlers.DouyinImportPeers)
		api.POST("/douyin/peers/refresh-all", handlers.DouyinRefreshAllPeers)
		api.POST("/douyin/peers/:id/refresh", handlers.DouyinRefreshPeer)
		api.DELETE("/douyin/peers/:id", handlers.DouyinDeletePeer)
		// 3) 视频数据
		api.GET("/douyin/videos", handlers.DouyinListVideos)
		api.GET("/douyin/analysis", handlers.DouyinAnalysis)
		// 4) 客户获取
		api.GET("/douyin/leads", handlers.DouyinListLeads)
		api.POST("/douyin/leads/parse", handlers.DouyinParseLeads)
		api.POST("/douyin/leads", handlers.DouyinCreateLead)
		api.PUT("/douyin/leads/:id", handlers.DouyinUpdateLead)
		api.DELETE("/douyin/leads/:id", handlers.DouyinDeleteLead)
		// 5) 话术库
		api.GET("/douyin/slogans", handlers.DouyinListSlogans)
		api.POST("/douyin/slogans", handlers.DouyinCreateSlogan)
		api.PUT("/douyin/slogans/:id", handlers.DouyinUpdateSlogan)
		api.DELETE("/douyin/slogans/:id", handlers.DouyinDeleteSlogan)
		api.POST("/douyin/slogans/:id/use", handlers.DouyinUseSlogan)
		api.POST("/douyin/slogans/ai-generate", handlers.DouyinGenerateSlogan)
		// 6) 频率与安全 + 动作前置校验 + 打招呼（半自动）
		api.GET("/douyin/settings", handlers.DouyinGetSettings)
		api.POST("/douyin/settings", handlers.DouyinSaveSettings)
		api.POST("/douyin/precheck", handlers.DouyinPrecheck)
		api.POST("/douyin/greet", handlers.DouyinGreet)
		// 7) 动作日志
		api.GET("/douyin/logs", handlers.DouyinListActionLogs)

		/* ---- 小红书获客（xhs） ---- */
		// 1) 账号管理
		api.GET("/xhs/accounts", handlers.XhsListAccounts)
		api.POST("/xhs/accounts", handlers.XhsCreateAccount)
		api.PUT("/xhs/accounts/:id", handlers.XhsUpdateAccount)
		api.DELETE("/xhs/accounts/:id", handlers.XhsDeleteAccount)
		api.POST("/xhs/accounts/:id/refresh", handlers.XhsRefreshAccount)
		// 2) 同行追踪
		api.GET("/xhs/peers", handlers.XhsListPeers)
		api.POST("/xhs/peers/import", handlers.XhsImportPeers)
		api.POST("/xhs/peers/:id/refresh", handlers.XhsRefreshPeer)
		api.DELETE("/xhs/peers/:id", handlers.XhsDeletePeer)
		// 3) 笔记数据
		api.GET("/xhs/notes", handlers.XhsListNotes)
		api.GET("/xhs/analysis", handlers.XhsAnalysis)
		// 4) 客户获取
		api.GET("/xhs/leads", handlers.XhsListLeads)
		api.POST("/xhs/leads/parse", handlers.XhsParseLeads)
		api.POST("/xhs/leads", handlers.XhsCreateLead)
		api.PUT("/xhs/leads/:id", handlers.XhsUpdateLead)
		api.DELETE("/xhs/leads/:id", handlers.XhsDeleteLead)
		// 5) 话术库
		api.GET("/xhs/slogans", handlers.XhsListSlogans)
		api.POST("/xhs/slogans", handlers.XhsCreateSlogan)
		api.PUT("/xhs/slogans/:id", handlers.XhsUpdateSlogan)
		api.DELETE("/xhs/slogans/:id", handlers.XhsDeleteSlogan)
		api.POST("/xhs/slogans/:id/use", handlers.XhsUseSlogan)
		// 6) AI 话术生成（复用统一 AI 客户端，OpenAI 兼容）
		api.POST("/xhs/slogans/ai-generate", handlers.XhsGenerateSlogan)
		// 7) 频率与安全 + 动作前置校验 + 打招呼（半自动）
		api.GET("/xhs/settings", handlers.XhsGetSettings)
		api.POST("/xhs/settings", handlers.XhsSaveSettings)
		api.POST("/xhs/precheck", handlers.XhsPrecheck)
		api.POST("/xhs/greet", handlers.XhsGreet)
		// 8) 价值统计 + 通知管理员
		api.GET("/xhs/value-stats", handlers.XhsValueStats)
		api.GET("/xhs/notifications", handlers.XhsListNotifications)
		api.POST("/xhs/notifications/:id/read", handlers.XhsMarkNotificationRead)
		// 9) 动作日志
		api.GET("/xhs/logs", handlers.XhsListActionLogs)

		/* ---- 快手获客（ks） ---- */
		api.GET("/ks/accounts", handlers.KsListAccounts)
		api.POST("/ks/accounts", handlers.KsCreateAccount)
		api.PUT("/ks/accounts/:id", handlers.KsUpdateAccount)
		api.DELETE("/ks/accounts/:id", handlers.KsDeleteAccount)
		api.GET("/ks/peers", handlers.KsListPeers)
		api.POST("/ks/peers/import", handlers.KsImportPeers)
		api.POST("/ks/peers/:id/refresh", handlers.KsRefreshPeer)
		api.DELETE("/ks/peers/:id", handlers.KsDeletePeer)
		api.GET("/ks/videos", handlers.KsListVideos)
		api.GET("/ks/analysis", handlers.KsAnalysis)
		api.GET("/ks/leads", handlers.KsListLeads)
		api.POST("/ks/leads/parse", handlers.KsParseLeads)
		api.POST("/ks/leads", handlers.KsCreateLead)
		api.PUT("/ks/leads/:id", handlers.KsUpdateLead)
		api.DELETE("/ks/leads/:id", handlers.KsDeleteLead)
		api.GET("/ks/slogans", handlers.KsListSlogans)
		api.POST("/ks/slogans", handlers.KsCreateSlogan)
		api.PUT("/ks/slogans/:id", handlers.KsUpdateSlogan)
		api.DELETE("/ks/slogans/:id", handlers.KsDeleteSlogan)
		api.POST("/ks/slogans/:id/use", handlers.KsUseSlogan)
		api.POST("/ks/slogans/ai-generate", handlers.KsGenerateSlogan)
		api.GET("/ks/settings", handlers.KsGetSettings)
		api.POST("/ks/settings", handlers.KsSaveSettings)
		api.POST("/ks/precheck", handlers.KsPrecheck)
		api.POST("/ks/greet", handlers.KsGreet)
		api.GET("/ks/logs", handlers.KsListActionLogs)

		/* ---- 智能创作中心（creation） ---- */
		// 角色设定（内置 ≥6 角色，可增删改查）
		api.GET("/creation/roles", handlers.ListRoles)
		api.POST("/creation/roles", handlers.CreateRole)
		api.PUT("/creation/roles/:id", handlers.UpdateRole)
		api.DELETE("/creation/roles/:id", handlers.DeleteRole)
		// AI 助手对话（多轮会话 + 消息持久化）
		api.GET("/creation/sessions", handlers.ListSessions)
		api.POST("/creation/sessions", handlers.CreateSession)
		api.DELETE("/creation/sessions/:id", handlers.DeleteSession)
		api.GET("/creation/sessions/:id/messages", handlers.ListSessionMessages)
		api.POST("/creation/chat", handlers.ChatSend)
		// 文案写作
		api.POST("/creation/write", handlers.WriteCopy)
		// 抖音热门视频脚本
		api.POST("/creation/script", handlers.DouyinScript)
		// 小红书热门文案
		api.POST("/creation/xhs", handlers.XhsCopy)
		// 深度学习再创作
		api.POST("/creation/learn", handlers.LearnCopy)
		// 洗稿（去重保意）
		api.POST("/creation/xiegou", handlers.Xiegou)
		// 图片生成（OpenAI 兼容文生图）
		api.POST("/creation/image", handlers.GenerateImage)
		// 生成记录 & 素材库
		api.GET("/creation/records", handlers.ListRecords)
		api.DELETE("/creation/records/:id", handlers.DeleteRecord)
		api.GET("/creation/materials", handlers.ListMaterials)
		api.POST("/creation/materials", handlers.SaveMaterial)
		api.DELETE("/creation/materials/:id", handlers.DeleteMaterial)

		/* ---- Token 用量看板 ---- */
		api.GET("/usage/overview", handlers.UsageOverview)

		/* ---- 内容投放（content） ---- */
		// 1) 媒体库
		api.GET("/content/media", handlers.ListContentMedia)
		api.POST("/content/media", handlers.CreateContentMedia)
		api.PUT("/content/media/:id", handlers.UpdateContentMedia)
		api.DELETE("/content/media/:id", handlers.DeleteContentMedia)
		// 2) 软文生成（AI 写软文）
		api.POST("/content/articles/generate", handlers.GenerateContentArticle)
		api.POST("/content/articles/generate-batch", handlers.GenerateContentArticlesBatch)
		api.GET("/content/articles", handlers.ListContentArticles)
		api.POST("/content/articles", handlers.CreateContentArticle)
		api.PUT("/content/articles/:id", handlers.UpdateContentArticle)
		api.DELETE("/content/articles/:id", handlers.DeleteContentArticle)
		// 3) 发布任务（自动/半自动）
		api.POST("/content/tasks", handlers.CreateContentTask)
		api.POST("/content/tasks/batch", handlers.CreateContentTasksBatch)
		api.PUT("/content/tasks/:id", handlers.UpdateContentTask)
		api.GET("/content/tasks", handlers.ListContentTasks)
		api.POST("/content/tasks/:id/publish", handlers.PublishContentTask)
		api.POST("/content/tasks/:id/finish", handlers.FinishContentTask)
		api.POST("/content/tasks/:id/check", handlers.CheckContentTask)
		api.DELETE("/content/tasks/:id", handlers.DeleteContentTask)
		// 4) 监控记录
		api.GET("/content/monitors", handlers.ListContentMonitors)
		// 5) 发稿平台配置
		api.GET("/content/publish-config", handlers.GetContentPublishConfig)
		api.POST("/content/publish-config", handlers.SaveContentPublishConfig)
		// 6) 效果分析
		api.GET("/content/analysis", handlers.ContentAnalysis)

		// 总后台管理（仅 super 角色）
		super := api.Group("/super")
		super.Use(handlers.SuperRequired)
		{
		super.GET("/overview", handlers.Overview)
		// 渠道管理：创建渠道（含登录账号）/ 编辑品牌客服 / 启停
		super.GET("/channels", handlers.ChannelList)
		super.POST("/channels", handlers.CreateChannel)
		super.PUT("/channels/:id", handlers.UpdateChannel)
		super.PUT("/channels/:id/status", handlers.UpdateChannelStatus)
		super.POST("/channels/:id/recharge", handlers.ChannelRecharge)
		super.POST("/channels/:id/simulate-login", handlers.SimulateChannelLogin)
		super.GET("/channels/:id/plaintext", handlers.ChannelPlaintextPassword)
		super.PUT("/channels/:id/password", handlers.ChannelResetPassword)
		// 融合客户管理：一个客户=一个分站+一个登录账号，一键开通/列表/查询
		super.GET("/customers", handlers.ListCustomers)
		super.GET("/online-count", handlers.SuperOnlineCount)
		super.POST("/customers", handlers.CreateCustomer)
		super.GET("/tenants", handlers.ListTenants)
			super.POST("/tenants", handlers.CreateTenant)
			super.PUT("/tenants/:id", handlers.UpdateTenant)
			super.DELETE("/tenants/:id", handlers.DeleteTenant)
			super.POST("/tenants/:id/simulate-login", handlers.SimulateTenantLogin)
			// 点卡充值：为分站充值点数并记录流水
			super.POST("/tenants/:id/recharge", handlers.RechargeTenant)
			super.GET("/users", handlers.ListUsers)
			super.POST("/users", handlers.CreateUser)
			super.GET("/users/:id/plaintext", handlers.UserPlaintextPassword)
			super.PUT("/users/:id/password", handlers.ResetUserPassword)
			// 续费/延长开通时长（到期预警行一键续费）
			super.POST("/users/:id/extend", handlers.ExtendUserService)
			// 续费流水（对账用）
			super.GET("/extend-records", handlers.ListExtendRecords)
			super.PUT("/users/:id/status", handlers.UpdateUserStatus)
			super.DELETE("/users/:id", handlers.DeleteUser)
			// 支付设置：微信/支付宝点卡扫码充值配置（仅 super，密钥脱敏）
			super.GET("/pay/config", handlers.GetPayConfig)
			super.POST("/pay/config", handlers.SavePayConfig)
			super.POST("/notify/test", handlers.NotifyTestService)
			super.GET("/notify/preview", handlers.NotifyPreviewService)
			// SaaS 端统一站内信推送 + 列表
			super.POST("/notifications", super_h.NotificationPush)
			super.GET("/notifications", super_h.NotificationList)
			// 卡密管理：密钥对（公钥供打包）+ 批量生成 + 列表
			super.GET("/license/key", handlers.GetLicenseKey)
			super.POST("/license/key", handlers.RegenerateLicenseKey)
			super.POST("/license/key/import", handlers.ImportLicenseKey)
			super.POST("/cards/generate", handlers.GenerateCards)
			super.GET("/cards", handlers.ListCards)
			// 短信设置：注册短信验证开关 + 短信服务商配置（全部分站注册共用）
			super.GET("/sms/config", handlers.GetSmsConfig)
			super.POST("/sms/config", handlers.SaveSmsConfig)
			// 第三方数据 API（Just One API）token 配置：抖音/小红书稳定数据抓取
			super.GET("/data-api/config", handlers.DataAPIConfig)
			super.POST("/data-api/config", handlers.SaveDataAPIConfig)
			// 帮助文档（使用教程）：分类 + 文档管理
			super.GET("/help/categories", handlers.HelpCategories)
			super.POST("/help/categories", handlers.HelpCreateCategory)
			super.PUT("/help/categories/:id", handlers.HelpUpdateCategory)
			super.DELETE("/help/categories/:id", handlers.HelpDeleteCategory)
			super.GET("/help/docs", handlers.HelpDocs)
			super.GET("/help/doc/:id", handlers.HelpDocDetailSuper)
			super.POST("/help/docs", handlers.HelpSaveDoc)
			super.DELETE("/help/docs/:id", handlers.HelpDeleteDoc)
			// 成功案例管理（SaaS 端上传）
			super.GET("/cases", handlers.SuperCaseList)
			super.POST("/cases", handlers.SuperCaseSave)
			super.PUT("/cases/:id", handlers.SuperCaseUpdate)
			super.DELETE("/cases/:id", handlers.SuperCaseDelete)
			// 价格套餐：总后台设置充值套餐
			super.GET("/plans", handlers.ListAllPlans)
			super.POST("/plans", handlers.CreatePlan)
			super.PUT("/plans/:id", handlers.UpdatePlan)
			super.DELETE("/plans/:id", handlers.DeletePlan)
		}

		// 渠道后台（channel 角色）：管理自己渠道下的分站（客户管理全能力）+ 品牌/客服设置
		ch := api.Group("/channel")
		ch.Use(handlers.ChannelRequired)
		{
			ch.GET("/profile", handlers.ChannelProfile)
			ch.PUT("/profile", handlers.ChannelProfile)
			ch.GET("/customers", handlers.ListCustomers)
			ch.POST("/customers", handlers.CreateCustomer)
			ch.PUT("/customers/:id", handlers.UpdateTenant)
			ch.DELETE("/customers/:id", handlers.DeleteTenant)
			ch.POST("/customers/:id/simulate-login", handlers.SimulateTenantLogin)
			ch.POST("/customers/:id/recharge", handlers.RechargeTenant)
			ch.PUT("/users/:id/password", handlers.ResetUserPassword)
			ch.GET("/users/:id/plaintext", handlers.UserPlaintextPassword)
			ch.POST("/users/:id/extend", handlers.ExtendUserService)
			ch.PUT("/users/:id/status", handlers.UpdateUserStatus)
			ch.DELETE("/users/:id", handlers.DeleteUser)
		}
	}

	// 回收上次进程中断遗留的僵尸巡检任务，避免永久阻塞租户发起新巡检
	geo.RecoverStaleTasks()

	// 初始化百度指数行业排行种子数据（表为空时写入）
	handlers.SeedBaiduIndustryRank()

	// 定时自动巡检：定时器驱动，对所有启用分站执行
	if config.AutoCheckEnabled() {
		minutes := config.AutoCheckMinutes()
		go func() {
			ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				runAutoCheck()
				// 关闭超时未支付的充值订单（status=0 且已过 expire_time -> status=2）
				if _, err := handlers.RechargeCloseExpired(); err != nil {
					log.Printf("[cron] 关闭超时支付订单失败: %v", err)
				}
			}
		}()
		log.Printf("[cron] 自动巡检已启用，间隔 %d 分钟", minutes)
	}

	// 每日告警推送：每小时检查一次，到推送时刻且当日未推过则推送（到期预警 + 点数不足）
	// 注意传 biztime.Now()：告警的「推送时刻」与「当日去重」都是业务时间，
	// 容器时区为 UTC，若传 time.Now() 会让「设在几点推」实际偏移 8 小时。
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if notify.RunDailyIfDue(biztime.Now()) {
				log.Println("[notify] 每日告警已推送")
			}
		}
	}()

	// 百度指数行业排行：每日定时更新（每天 06:05 触发一次，**北京时间**）
	go func() {
		for {
			now := biztime.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 6, 5, 0, 0, biztime.Zone())
			if now.After(next) {
				next = next.AddDate(0, 0, 1)
			}
			time.Sleep(time.Until(next))
			if err := handlers.RefreshIndustryRank(); err != nil {
				log.Printf("[baidu-rank] 更新失败: %v", err)
			} else {
				log.Println("[baidu-rank] 每日更新完成")
			}
		}
	}()

	// 监听端口：优先用配置端口，被占用则顺延（最多 +20），避免客户机器端口冲突导致「启动即崩溃」。
	listener, actualPort := listenFree(cfg.Port)
	log.Printf("[server] GEO 多租户后台启动于 :%s", actualPort)

	// 桌面安装场景：启动后自动打开浏览器（Windows / macOS；GEO_NO_BROWSER=1 可禁用）。
	// 容器/开发环境（Linux / 显式禁用）不触发，避免干扰。
	if (runtime.GOOS == "windows" || runtime.GOOS == "darwin") && os.Getenv("GEO_NO_BROWSER") != "1" {
		go func() {
			time.Sleep(800 * time.Millisecond)
			_ = openBrowser("http://localhost:" + actualPort)
		}()
	}

	if err := http.Serve(listener, r); err != nil {
		log.Fatalf("[server] 启动失败: %v", err)
	}
}

// listenFree 尝试监听指定端口，被占用则顺延（最多 +20）。返回实际监听器与端口。
func listenFree(port string) (net.Listener, string) {
	p, err := strconv.Atoi(port)
	if err != nil || p <= 0 {
		p = 8080
	}
	for i := 0; i <= 20; i++ {
		addr := fmt.Sprintf(":%d", p+i)
		if ln, err := net.Listen("tcp", addr); err == nil {
			return ln, strconv.Itoa(p + i)
		}
	}
	log.Fatalf("[server] 无法监听端口：%s 起连续 21 个端口均被占用", port)
	return nil, ""
}

// openBrowser 尽力打开系统默认浏览器，失败静默（不影响服务本身）。
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// runAutoCheck 在自动巡检时段（**北京时间** [AutoCheckStartHour, AutoCheckEndHour)）
// 内，为所有启用分站各触发一轮自动巡检。
//
// 时段判断必须用 biztime（北京时间）：容器镜像默认时区是 UTC，
// 若直接 time.Now().Hour() 取到的是 UTC 小时，会让「8-22 点」实际落在
// 北京时间 16:00 ~ 次日 06:00 —— 客户白天（尤其 9-16 点）完全不巡检，
// 深夜反而频繁巡检。详见 services/biztime 包注释。
//
// 时段常量取自 config（唯一权威来源），与「工作日志」页面展示的承诺时段同源，
// 避免页面写 8-22、实际跑成别的区间。
func runAutoCheck() {
	hour := biztime.Hour()
	if hour < config.AutoCheckStartHour || hour >= config.AutoCheckEndHour {
		log.Printf("[cron] 非巡检时段（北京时间 %d-%d 点，当前 %d 点），跳过",
			config.AutoCheckStartHour, config.AutoCheckEndHour, hour)
		return
	}
	var tenants []models.Tenant
	database.DB.Where("status = ?", 1).Find(&tenants)
	log.Printf("[cron] 开始自动巡检：%d 个启用分站（北京时间 %d 点）", len(tenants), hour)
	for _, t := range tenants {
		geo.RunTenantTask(t.ID, "auto")
	}
}

// ensureSqlmapAPI 守护 sqlmap REST API（未运行则拉起，供 SQL 注入检测使用）
func ensureSqlmapAPI() {
	for {
		time.Sleep(15 * time.Second)
		conn, err := net.DialTimeout("tcp", "127.0.0.1:8775", 2*time.Second)
		if err == nil {
			conn.Close()
			continue
		}
		// 尝试拉起（仅在有 python3+sqlmap 的环境；失败静默，接口会报明确错误）
		cmd := exec.Command("python3", "/usr/share/sqlmap/sqlmapapi.py", "-s", "-H", "127.0.0.1", "-p", "8775")
		cmd.Start()
	}
}
