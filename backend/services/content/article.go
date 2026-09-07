package content

import (
	"context"
	"errors"
	"strings"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai"
	"geo-tool/services/ai_creation"
	"geo-tool/services/points"
)

/* ================================================================
 * 内容投放 · AI 软文生成服务
 *
 * 复用统一 AI 客户端（services/ai），平台取当前租户第一个 enabled 的
 * AiPlatform（services/ai_creation.FirstEnabledPlatform），无平台时返回
 * "请先配置 AI 平台"。生成结果按 TITLE|||正文 结构化入库。
 * ================================================================ */

// ErrEmptyGen AI 未返回有效内容
var ErrEmptyGen = errors.New("AI 未返回有效内容，请重试或检查 AI 平台配置")

// GenReq 软文生成请求
type GenReq struct {
	Topic         string // 投放主题
	BrandKeywords string // 品牌关键词（逗号分隔）
	Category      string // 场景
	Length        int    // 目标字数（0=默认800）
	MediaType     string // 目标媒体类型（用于提示写作风格）
}

// GenerateArticle 生成一篇软文并入库
func GenerateArticle(ctx context.Context, tid uint, req GenReq) (*models.CtnArticle, error) {
	p := ai_creation.FirstEnabledPlatform(tid)
	if p == nil || strings.TrimSpace(p.BaseURL) == "" {
		return nil, ai_creation.ErrNoPlatform
	}
	if err := points.DeductOne(tid, "内容投放·软文生成"); err != nil {
		return nil, err
	}

	length := req.Length
	if length <= 0 {
		length = 800
	}
	brand := strings.TrimSpace(req.BrandKeywords)
	if brand == "" {
		brand = "（无，请勿编造品牌）"
	}
	mediaType := strings.TrimSpace(req.MediaType)
	if mediaType == "" {
		mediaType = "综合媒体"
	}

	sys := "你是资深软文写手兼 SEO 内容专家，深谙内容营销：标题有传播力、正文软植入品牌、规避 AI 痕迹与过度营销、对百度收录友好。你从不提及自己是 AI。"
	cmd := "请围绕主题【" + req.Topic + "】撰写一篇可直接用于媒体发布的软文。\n" +
		"- 品牌关键词（需自然融入 1-3 次，切忌生硬堆砌）：" + brand + "\n" +
		"- 目标发布媒体类型：" + mediaType + "\n" +
		"- 投放场景：" + req.Category + "\n" +
		"- 目标字数：约 " + itoa(length) + " 字\n\n" +
		"要求：\n" +
		"1) 标题新颖有吸引力，不含硬广痕迹；\n" +
		"2) 正文分段清晰、逻辑连贯、口语自然，像真人作者而非 AI；\n" +
		"3) 若无品牌关键词则写通用行业软文。\n\n" +
		"请严格按以下格式输出（不要任何解释、不要 Markdown 代码块）：\n" +
		"第一行输出【标题】，第二行输出【|||】，第三行开始输出正文（可多行）。\n" +
		"即：\n标题\n|||\n正文……"

	answer, err := ai.NewClient(p.BaseURL, p.APIKey, p.Model).Chat(ctx, sys, []ai.Message{{Role: "user", Content: cmd}}, 2400, 0.8)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(answer) == "" {
		return nil, ErrEmptyGen
	}

	// 结构化解析：标题 ||| 正文
	title, body := parseTitleBody(answer)
	if body == "" {
		return nil, ErrEmptyGen
	}

	article := &models.CtnArticle{
		TenantID: tid, Title: title, Content: body,
		BrandKeywords: req.BrandKeywords, Topic: req.Topic, Category: req.Category,
		Length: len([]rune(body)), Status: "draft",
		Platform: p.Name, Model: p.Model,
	}
	if err := database.DB.Create(article).Error; err != nil {
		return nil, err
	}
	return article, nil
}

// parseTitleBody 解析 "标题\n|||\n正文"
func parseTitleBody(s string) (title, body string) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "|||"); idx >= 0 {
		title = strings.TrimSpace(s[:idx])
		body = strings.TrimSpace(s[idx+3:])
		// 去掉标题行可能残留的换行/前缀
		title = strings.Trim(title, "\n \t")
		title = strings.ReplaceAll(title, "\n", " ")
		return title, body
	}
	// 兜底：整段作为正文，首行为标题
	lines := strings.SplitN(s, "\n", 2)
	if len(lines) == 2 {
		return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
	}
	return "", s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
