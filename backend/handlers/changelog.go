package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
)

// 升级日志：把历代版本更新记录内嵌在二进制里（无需数据库、无需联网、单机版同样可用），
// 通过只读接口 GET /api/changelog 返回，客户端「系统设置 → 升级日志」按时间线渲染。
//
// 维护约定：每完成一轮版本迭代，除了递增 config.Version 与补 docs/开发文档.md，
// 必须同步在下面 changelogEntries 头部插入一条新记录（最新在最前）。

// ChangelogItem 单条变更：type 取值 feature(新增) / improve(优化) / fix(修复) / security(安全)
type ChangelogItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ChangelogEntry 一个版本的更新记录
type ChangelogEntry struct {
	Version string          `json:"version"`
	Date    string          `json:"date"`
	Title   string          `json:"title"`
	Items   []ChangelogItem `json:"items"`
}

func items(pairs ...string) []ChangelogItem {
	out := make([]ChangelogItem, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, ChangelogItem{Type: pairs[i], Text: pairs[i+1]})
	}
	return out
}

var changelogEntries = []ChangelogEntry{
	{
		Version: "1.0.32", Date: "2026-09-11", Title: "强化闭环：信号驱动迭代 + 行动项复测",
		Items: items(
			"feature", "行动清单接入话题簇信号：自动识别「整簇覆盖不足」（结构性缺口，按话题批量出选题）与「整簇从未巡检」（数据缺口），比逐词提示更能命中要害",
			"feature", "新增信源建设任务：自动找出「竞品被 AI 引用、品牌从未被引用」的信源域名，给出该域名的引用偏好分析与投放建议——这是引用提升最省力的突破口",
			"feature", "新增行动项「复测」：标记完成后可一键回测效果，系统对比完成前 7 天与完成后 7 天的同口径指标，自动判定「已改善 / 无变化 / 变差」并记录指标快照",
			"feature", "新增「优化闭环进度」看板：监测 → 诊断 → 行动 → 复测 四段各自完成情况与积压提示，一眼看出闭环卡在哪一步",
			"improve", "重开已完成的行动项时会清除旧的复测结论，避免残留数据误导判断",
		),
	},
	{
		Version: "1.0.31", Date: "2026-09-11", Title: "新增话题簇（提示词聚类）",
		Items: items(
			"feature", "新增「话题簇」功能，对应 GEO 八阶段管线的第③步「提示词聚类」：把扁平关键词按搜索意图聚成话题，解决「整体覆盖率被少数品牌词拉高、掩盖结构性缺口」的问题",
			"feature", "AI 一键聚类：AI 读取现有全部关键词，按认知型 / 对比型 / 决策型 / 品牌型四类搜索意图自动分成 3~8 个话题簇，并逐条归位；支持填写业务背景提升准确率",
			"feature", "话题簇按簇统计品牌出现率 / 引用率 / 事实一致率 / 平均提及次数，并以卡片着色区分「覆盖领先 / 正常 / 偏弱 / 未巡检」",
			"feature", "支持手动新建与编辑话题簇（名称 / 意图 / 描述 / 排序），并可勾选调整关键词归属；删除簇时关键词自动退回「未归类」而非被删除",
			"improve", "GEO 智能中心页签从 10 个扩展为 11 个，新增「⑤ 话题簇」并顺延后续编号",
		),
	},
	{
		Version: "1.0.30", Date: "2026-09-11", Title: "对照 GEO 行业标准架构的能力升级",
		Items: items(
			"feature", "新增「AI 可见度评分（AIVS）」：把六项原始指标合成 0~100 综合分，含曝光度/推荐位次/可信度/合规安全四维拆解、行业基准对标刻度与优先改进建议",
			"feature", "巡检支持多采样：AI 平台可配置每问采样 1~5 次，按命中比例判定「稳定可见度」，消除大模型输出随机性造成的指标抖动",
			"feature", "结构化数据生成升级为三类分块输出：Organization（含 sameAs 实体锚定、knowsAbout 领域）+ FAQPage（问答对）+ ItemList（事实清单），可分别复制部署",
			"fix", "修复「AI 爬虫 UA 实测」形同虚设的问题：改为用 GPTBot / Google-Extended / PerplexityBot / ClaudeBot / Bytespider 等真实爬虫 UA 逐个实测，可发现「浏览器能访问但 AI 被 WAF 拦截」",
			"fix", "修复 Schema 生成的语义错误：事实条目不再塞进 hasCredential（该字段专指资质证书），改用 knowsAbout 与 DefinedTerm ItemList",
			"fix", "修复引用溯源落库时序问题：原在记录落库前调用导致 ResultID 为 0，引用数据可能丢失",
		),
	},
	{
		Version: "1.0.29", Date: "2026-09-11", Title: "客户端系统设置新增「升级日志」",
		Items: items(
			"feature", "客户端「系统设置」新增「升级日志」页签，按时间线展示历代版本更新内容（版本号 / 日期 / 变更类型）",
			"feature", "日志数据内嵌在后端二进制中，SaaS 端、客户端与单机版共用同一份，无需联网、无需额外配置",
			"improve", "自动标注当前运行版本，一眼判断是否已是最新版本",
		),
	},
	{
		Version: "1.0.28", Date: "2026-09-11", Title: "GEO 智能页区块间距修正",
		Items: items(
			"fix", "「各 AI 平台表现明细」与上方 KPI 区块间距由 0px 修正为 20px，大屏与移动端均自适应",
		),
	},
	{
		Version: "1.0.27", Date: "2026-09-11", Title: "系统设置保存按钮宽度优化",
		Items: items(
			"improve", "系统设置各页保存按钮由撑满整行缩至容器 1/3（约 185~318px），靠左对齐，界面不再头重脚轻",
		),
	},
	{
		Version: "1.0.26", Date: "2026-09-11", Title: "支付设置并入「系统设置」",
		Items: items(
			"feature", "「微信设置」「支付宝设置」并入系统设置，顺序排在 OSS 设置之后，另含充值定价与告警通知",
			"improve", "拆分为独立页签后只提交当前页字段，避免跨页空值覆盖已配置项",
			"improve", "顶部独立「支付设置」菜单移除，配置入口统一收敛到系统设置",
		),
	},
	{
		Version: "1.0.25", Date: "2026-09-11", Title: "注册页《网站注册安全协议》",
		Items: items(
			"feature", "注册页新增《网站注册安全协议》勾选，未勾选无法提交注册",
			"feature", "SaaS 端可编辑协议标题与正文，并可开关是否强制勾选",
			"security", "后端对未勾选做兜底校验，防止绕过前端直接调用接口注册",
		),
	},
	{
		Version: "1.0.24", Date: "2026-09-10", Title: "文案调整",
		Items: items(
			"improve", "GEO 情报页「默认品牌词」更名为「优化的关键词」，表意更准确",
		),
	},
	{
		Version: "1.0.23", Date: "2026-09-10", Title: "客户端「Token 用量」看板",
		Items: items(
			"feature", "新增 Token 用量看板：请求级真实 usage 统计，含平台分布、使用场景、趋势与明细",
			"feature", "官方演示数据一并展示，新用户也能看到看板形态",
		),
	},
	{
		Version: "1.0.22", Date: "2026-09-10", Title: "内容投放注册入口引导",
		Items: items(
			"feature", "内容投放发稿平台配置新增注册入口引导（4 家主流平台），配置前先注册不再摸黑",
		),
	},
	{
		Version: "1.0.21", Date: "2026-09-10", Title: "注册支持自定义登录账号",
		Items: items(
			"feature", "注册时可填写自定义登录账号，账号与手机号 / 邮箱分离，全局唯一",
		),
	},
	{
		Version: "1.0.20", Date: "2026-09-10", Title: "注册邮箱验证",
		Items: items(
			"feature", "注册验证方式三选一：短信验证 / 邮箱验证 / 全部关闭，并支持 SMTP 配置",
		),
	},
	{
		Version: "1.0.19", Date: "2026-09-10", Title: "国内主流 AI 平台整合",
		Items: items(
			"feature", "整合国内主流 AI 平台至 14 家，新增百度文心、MiniMax、讯飞星火、零一万物、百川、阶跃、小米、商汤",
			"improve", "启动自动补种，新平台默认停用，按需开启",
		),
	},
	{
		Version: "1.0.18", Date: "2026-09-10", Title: "安全审计修复",
		Items: items(
			"security", "JWT 兜底密钥随机化，杜绝默认密钥被伪造 token",
			"security", "帮助文档内容接入 DOMPurify 清洗，修复 XSS 风险",
		),
	},
	{
		Version: "1.0.17", Date: "2026-09-09", Title: "渠道管理增强",
		Items: items(
			"feature", "渠道管理支持查看渠道明文密码，编辑时可选填重置密码",
		),
	},
	{
		Version: "1.0.16", Date: "2026-09-09", Title: "文案调整",
		Items: items(
			"improve", "昵称输入框占位字「如 红娘小兰」三处全部去掉",
		),
	},
	{
		Version: "1.0.15", Date: "2026-09-09", Title: "渠道后台界面精简",
		Items: items(
			"improve", "渠道后台去掉侧栏 Token 卡片（客户端保留）",
		),
	},
	{
		Version: "1.0.14", Date: "2026-09-09", Title: "客户端新增「充值中心」入口",
		Items: items(
			"feature", "客户端菜单新增「充值中心」独立入口，侧栏折叠后也不会丢失充值入口",
		),
	},
	{
		Version: "1.0.13", Date: "2026-09-09", Title: "总后台一键登录渠道后台",
		Items: items(
			"feature", "总后台支持一键登录渠道后台",
			"fix", "切回总后台时菜单未恢复的问题修复",
		),
	},
	{
		Version: "1.0.12", Date: "2026-09-09", Title: "渠道点数链条",
		Items: items(
			"feature", "平台给渠道充值 → 渠道拨付客户，余额不足时直接拒绝，防止超发",
		),
	},
	{
		Version: "1.0.11", Date: "2026-09-09", Title: "渠道分发能力",
		Items: items(
			"feature", "渠道可自建分站，具备品牌客服、客户管理全能力与品牌三层回退",
		),
	},
	{
		Version: "1.0.10", Date: "2026-09-08", Title: "手机端登录兼容",
		Items: items(
			"fix", "手机端登录滑块浮点轨迹容错（SlideX / Track 支持 float64 并做 Round）",
		),
	},
}

// Changelog 升级日志：GET /api/changelog（公开只读，登录页与客户端均可取）
func Changelog(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"current": config.Version,
		"entries": changelogEntries,
	}})
}
