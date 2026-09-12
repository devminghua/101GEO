package ai_creation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai"
	"geo-tool/services/ai_platform"
	"geo-tool/services/points"
)

/* ================================================================
 * 智能创作中心 · AI 生成服务
 *
 * 所有 AI 能力统一走 backend/services/ai/client.go 的 OpenAI 兼容客户端，
 * 平台选择统一委托给 services/ai_platform（**唯一权威来源**），
 * 无可用平台时返回明确错误"请先配置 AI 平台"（与小红书 xhs/ai_slogan 同范式）。
 *
 * 覆盖：AI 助手对话、角色设定、文案写作、抖音脚本、小红书文案、
 *      深度学习、洗稿改写、图片生成。
 * ================================================================ */

var (
	// ErrNoPlatform 未配置可用 AI 平台
	ErrNoPlatform = errors.New("请先配置 AI 平台")
	// ErrEmptyGen AI 未返回有效结果
	ErrEmptyGen = errors.New("AI 未返回有效内容，请重试或检查 AI 平台配置")
)

// CreativeSettingKey 租户级配置项（models.Setting 复合主键 KV 表）
const (
	KeyCreativeImageModel = "creative_image_model"  // 文生图模型（默认取平台 Model）
	KeyCreativeImageSize  = "creative_image_size"   // 文生图尺寸（默认 1024x1024）

	// 豆包文生图（Seedream 5.0 Pro）
	KeyDoubaoImageBaseURL = "doubao_image_base_url"
	KeyDoubaoImageModel   = "doubao_image_model"
	KeyDoubaoImageAPIKey  = "doubao_image_api_key"
	KeyDoubaoImageStatus  = "doubao_image_status"
)

// GenResult AI 生成结果（统一结构）
type GenResult struct {
	Text     string `json:"text"`     // 生成文本（文案/脚本JSON/URL 等）
	Platform string `json:"platform"` // AI 平台名
	Model    string `json:"model"`    // 模型名
}

// FirstEnabledPlatform 获取第一个“可用”的 AI 平台。
//
// 保留此函数名仅为兼容既有调用点（话术生成、文章生成等），实现已委托给
// services/ai_platform（唯一权威来源），**不再有自己的一套查询逻辑**。
//
// 平台归属规则（老板 2026-09-12 定）：全部走**分站自己的 Key**。
// 分站只用自己的平台，不继承总后台全局平台；总后台（tenant_id=0）用全局平台。
// 历史实现是「全局优先、分站兜底」，会导致分站用「创作中心」时从总后台账号扣费，
// 与「AI 助手」走分站账号的行为不一致，已于本次重构统一。
func FirstEnabledPlatform(tenantID uint) *models.AiPlatform {
	return ai_platform.FirstUsable(tenantID)
}

// isLocalBaseURL 判断是否为免密钥的自托管本地服务。
// 实现已委托给 ai_platform 内部统一判定，此处保留以兼容包内既有引用。
func isLocalBaseURL(u string) bool {
	return ai_platform.IsLocalBaseURL(u)
}

// GetSetting 读取租户级 KV 配置
func GetSetting(tenantID uint, key string) string {
	var s models.Setting
	if err := database.DB.Where("tenant_id = ? AND key = ?", tenantID, key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// SetSetting 写入租户级 KV 配置
func SetSetting(tenantID uint, key, value string) error {
	if value == "" {
		return database.DB.Where("tenant_id = ? AND key = ?", tenantID, key).Delete(&models.Setting{}).Error
	}
	s := models.Setting{TenantID: tenantID, Key: key, Value: value}
	return database.DB.Save(&s).Error
}

// newClient 从平台配置构造统一 AI 客户端
// newClient 从平台配置构造统一 AI 客户端（挂 token 用量统计元信息）
func newClient(p *models.AiPlatform, tenantID uint) *ai.Client {
	return ai.NewClient(p.BaseURL, p.APIKey, p.Model).WithMeta(tenantID, p.Name, "创作中心")
}

// mustPlatform 取平台（优先全局 tenant_id=0），无则返回 ErrNoPlatform。
// 每次 AI 调用前统一扣 1 点点卡（点数不足返回 ErrInsufficient）。
func mustPlatform(tenantID uint, remark string) (*models.AiPlatform, error) {
	p := FirstEnabledPlatform(tenantID)
	if p == nil || strings.TrimSpace(p.BaseURL) == "" {
		return nil, ErrNoPlatform
	}
	if err := points.DeductOne(tenantID, remark); err != nil {
		return nil, err
	}
	return p, nil
}

//----------- AI 助手多轮对话 ----------

// ChatReq 多轮对话请求
type ChatReq struct {
	System string       // 角色系统提示词（来自会话关联角色或通用提示词）
	Msgs   []ai.Message // 历史消息（不含新输入前的 system）
}

// Chat 执行多轮对话，返回助手回复
func Chat(ctx context.Context, tenantID uint, req ChatReq) (*GenResult, error) {
	p, err := mustPlatform(tenantID, "AI 助手对话")
	if err != nil {
		return nil, err
	}
	sys := strings.TrimSpace(req.System)
	if sys == "" {
		sys = "你是 GEO 工具智能创作中心的 AI 助手，负责协助用户完成各类创作任务。请直接、专业、有条理地回答，不要声称自己是某个具体模型。"
	}
	answer, err := newClient(p, tenantID).Chat(ctx, sys, req.Msgs, 1600, 0.7)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(answer) == "" {
		return nil, ErrEmptyGen
	}
	return &GenResult{Text: answer, Platform: p.Name, Model: p.Model}, nil
}

//----------- 文案写作（角色 + 主题/要求） ----------

// WriteCopy 按角色系统提示词 + 主题/要求生成文案
func WriteCopy(ctx context.Context, tenantID uint, systemPrompt, theme, requirements string) (*GenResult, error) {
	p, err := mustPlatform(tenantID, "文案写作")
	if err != nil {
		return nil, err
	}
	cmd := "请创作一段完整的文案。\n"
	cmd += "- 主题/场景：" + theme + "\n"
	cmd += "- 附加要求：" + (func() string {
		if requirements != "" {
			return requirements
		}
		return "无（按最佳实践自行发挥）"
	})() + "\n"
	cmd += "\n要求：直接输出成稿文案正文，结构清晰、有感染力、可直接使用，不要输出解释性语言或 Markdown 代码块。"
	answer, err := newClient(p, tenantID).Chat(ctx, systemPrompt, []ai.Message{{Role: "user", Content: cmd}}, 1800, 0.8)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(answer) == "" {
		return nil, ErrEmptyGen
	}
	return &GenResult{Text: answer, Platform: p.Name, Model: p.Model}, nil
}

//----------- 抖音热门视频脚本 ----------

// DouyinScript 生成抖音分镜脚本（结构化 JSON 数组，镜头号/景别/画面/台词/字幕/时长/音乐建议）
func DouyinScript(ctx context.Context, tenantID uint, theme string) (*GenResult, error) {
	p, err := mustPlatform(tenantID, "抖音脚本")
	if err != nil {
		return nil, err
	}
	sys := "你是短视频爆款脚本师，深谙抖音平台算法与用户心理：黄金3秒开头、强节奏、钩子反转、情绪递进、结尾引导互动。"
	cmd := "为以下主题创作一段抖音热门视频的分镜脚本：\n【" + theme + "】\n\n" +
		"请输出 8-12 个镜头，每镜头字段：\n" +
		"shot_order（镜头号，从1开始）、shot_size（景别：远景/全景/中景/近景/特写）、" +
		"scene（画面描述）、line（台词/口播文案）、subtitle（字幕文案，可同台词）、" +
		"duration（时长秒数，数值）、music（背景音乐及音效建议）。\n\n" +
		"要求：爆款结构（强钩子开头-过程递进-反转收尾）、每句台词口语化有情绪、给出切换节奏建议。\n" +
		"请严格只输出一个 JSON 数组（不要 Markdown 代码块、不要解释），例如：\n" +
		`[{"shot_order":1,"shot_size":"特写","scene":"...","line":"...","subtitle":"...","duration":2,"music":"..."}]`
	answer, err := newClient(p, tenantID).Chat(ctx, sys, []ai.Message{{Role: "user", Content: cmd}}, 2200, 0.8)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(answer) == "" {
		return nil, ErrEmptyGen
	}
	return &GenResult{Text: answer, Platform: p.Name, Model: p.Model}, nil
}

//----------- 小红书热门文案 ----------

// XhsCopy 生成小红书文案（标题+正文+标签+封面建议）
func XhsCopy(ctx context.Context, tenantID uint, theme string) (*GenResult, error) {
	p, err := mustPlatform(tenantID, "小红书文案")
	if err != nil {
		return nil, err
	}
	sys := "你是小红书资深运营编辑，深谙小红书调性：真实种草、生活化、有情绪价值、emoji 点缀、关键词埋点。"
	cmd := "为以下主题创作一条小红书热门笔记文案：\n【" + theme + "】\n\n" +
		"请严格按以下四个分段输出（每段用【】标记）：\n" +
		"【标题】出一个吸引点击的标题（20字内，可含‘连我这种…都’、数字、反差钩子）\n" +
		"【正文】300-500字真实种草风正文（分段、带适量emoji、首句抓人、埋3-5个搜索关键词）\n" +
		"【标签】给出8-12个#话题标签\n" +
		"【封面建议】给出封面图制作建议（主文案大字+构图+色调）\n\n" +
		"只输出上述四个分段内容，不要其他解释。"
	answer, err := newClient(p, tenantID).Chat(ctx, sys, []ai.Message{{Role: "user", Content: cmd}}, 2000, 0.8)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(answer) == "" {
		return nil, ErrEmptyGen
	}
	return &GenResult{Text: answer, Platform: p.Name, Model: p.Model}, nil
}

//----------- 深度学习（参考风格 + 再创作） ----------

// LearnCopy 用户提供参考文本/链接，AI 学习风格后按新主题再创作
func LearnCopy(ctx context.Context, tenantID uint, reference, theme string) (*GenResult, error) {
	p, err := mustPlatform(tenantID, "深度学习再创作")
	if err != nil {
		return nil, err
	}
	sys := "你是风格学习创作引擎：能够深度拆解参考文本的用词习惯、句式节奏、语气语调、结构组织，并在不复制原文内容的前提下，用学到的风格进行全新创作。"
	cmd := "请完成一次‘深度学习再创作’：\n\n"
	cmd += "【参考文本】\n" + reference + "\n\n"
	cmd += "【创作主题】\n" + theme + "\n\n"
	cmd += "步骤：1) 先一句话概括你从参考文本中学到的风格关键词；2) 依据该风格，围绕创作主题写一篇全新的、完整的作品（内容不得抄袭参考文本原文）。\n"
	cmd += "直接输出‘风格拆解’+‘再创作作品’两部分。"
	answer, err := newClient(p, tenantID).Chat(ctx, sys, []ai.Message{{Role: "user", Content: cmd}}, 2200, 0.8)
	if err != nil {
		return nil, err
	}
	return &GenResult{Text: answer, Platform: p.Name, Model: p.Model}, nil
}

//----------- 洗稿改写（去重保留原意） ----------

// Xiegou 输入原文，AI 改写去重保留原意
func Xiegou(ctx context.Context, tenantID uint, original string) (*GenResult, error) {
	p, err := mustPlatform(tenantID, "洗稿改写")
	if err != nil {
		return nil, err
	}
	sys := "你是资深内容改写专家，擅长在完全保留原意与信息量的前提下，对文本进行深度改写以降低文字重复率，并保持可读性与原有语气。"
	cmd := "请对以下原文进行洗稿改写：\n\n【原文】\n" + original + "\n\n" +
		"要求：\n" +
		"1) 保留全部核心事实、数据、结论与观点，不增删关键信息；\n" +
		"2) 调整句式、词语、段落顺序与表达角度，使文字重合度显著降低；\n" +
		"3) 语言自然流畅，不得出现机翻感或生搬硬套；\n" +
		"4) 如原文含可整理的小标题或要点，请用更清晰的层次重新组织。\n\n" +
		"直接输出改写后的全文。"
	answer, err := newClient(p, tenantID).Chat(ctx, sys, []ai.Message{{Role: "user", Content: cmd}}, 2200, 0.8)
	if err != nil {
		return nil, err
	}
	return &GenResult{Text: answer, Platform: p.Name, Model: p.Model}, nil
}

//----------- 图片生成 ----------

// ImageGen 调用 OpenAI 兼容 /images/generations 文生图
// size 支持 256x256 / 512x512 / 1024x1024 等（或平台自定义值）
func ImageGen(ctx context.Context, tenantID uint, prompt, size, format string) (*GenResult, string, error) {
	// 豆包 Seedream 优先：已配置 base_url + api_key + model 时直接走豆包文生图接口，
	// 不再经过 AiPlatform；否则回退现有 AiPlatform 逻辑。
	if baseURL := GetSetting(tenantID, KeyDoubaoImageBaseURL); baseURL != "" {
		if apiKey := GetSetting(tenantID, KeyDoubaoImageAPIKey); apiKey != "" {
			if model := GetSetting(tenantID, KeyDoubaoImageModel); model != "" {
				if err := points.DeductOne(tenantID, "图片生成（豆包Seedream）"); err != nil {
					return nil, "", err
				}
				return imageGenWithClient(ctx, tenantID, prompt, size, format, ai.NewClient(baseURL, apiKey, model), "豆包Seedream", model)
			}
		}
	}

	p, err := mustPlatform(tenantID, "图片生成")
	if err != nil {
		return nil, "", err
	}
	// 文生图模型：优先租户自定义，否则用平台 Model
	model := p.Model
	if m := GetSetting(tenantID, KeyCreativeImageModel); m != "" {
		model = m
	}
	return imageGenWithClient(ctx, tenantID, prompt, size, format, ai.NewClient(p.BaseURL, p.APIKey, model), p.Name, model)
}

// imageGenWithClient 用指定客户端执行文生图并统一兜底尺寸/格式
func imageGenWithClient(ctx context.Context, tenantID uint, prompt, size, format string, client *ai.Client, platformName, model string) (*GenResult, string, error) {
	if size == "" {
		size = legalSize(GetSetting(tenantID, KeyCreativeImageSize))
	}
	if format == "" {
		format = "b64_json"
	}
	b64, url, err := client.GenerateImage(ctx, prompt, size, format)
	if err != nil {
		return nil, "", err
	}
	if b64 == "" && url == "" {
		return nil, "", ErrEmptyGen
	}
	return &GenResult{Platform: platformName, Model: model}, b64, nil
}

// legalSize 尺寸兜底
func legalSize(s string) string {
	switch s {
	case "256x256", "512x512", "1024x1024", "1024x1792", "1792x1024":
		return s
	default:
		return "1024x1024"
	}
}

// EnsureDir 确保目录存在
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	return nil
}
