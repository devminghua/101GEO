package ks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"geo-tool/services/ai"
	"geo-tool/services/ai_platform"
	"geo-tool/services/points"
)

var (
	errNoPlatform = errors.New("请先配置 AI 平台")
	errNoNickname = errors.New("请提供客户昵称")
	errEmptyGen   = errors.New("AI 未返回有效话术，请重试或检查 AI 平台配置")
)

/* ================================================================
 * 快手获客 · AI 话术生成服务
 *
 * 复用统一 AI 客户端与平台归属（services/ai_platform 唯一权威来源，
 * 分站只用自己平台）；生成 1-3 条真人化开场话术，结构化返回。
 *
 * 合规半自动边界：AI 只负责「生成话术建议」，发送动作仍由人工在
 * 官方快手客户端执行——本服务绝不代发、不模拟登录、不自动群发。
 * ================================================================ */

// GenerateReq AI 话术生成请求参数
type GenerateReq struct {
	Nickname     string `json:"nickname"`      // 客户昵称（必填）
	Comment      string `json:"comment"`       // 客户评论内容
	Tag          string `json:"tag"`           // 客户标签
	AccountStyle string `json:"account_style"` // 账号风格
	Count        int    `json:"count"`         // 期望条数（1-3，默认 3）
}

// GenerateResult AI 生成结果
type GenerateResult struct {
	Slogans     []string `json:"slogans"`
	Platform    string   `json:"platform"`
	Model       string   `json:"model"`
	SourcedFrom string   `json:"sourced_from"` // 固定 ai
}

// GenerateSlogans 调用 AI 生成 1-3 条开场话术
func GenerateSlogans(ctx context.Context, tenantID uint, req GenerateReq) (*GenerateResult, error) {
	platform := ai_platform.FirstUsable(tenantID)
	if platform == nil {
		return nil, errNoPlatform
	}
	if strings.TrimSpace(req.Nickname) == "" {
		return nil, errNoNickname
	}
	if err := points.DeductOne(tenantID, "快手话术生成"); err != nil {
		return nil, err
	}
	count := req.Count
	if count < 1 || count > 3 {
		count = 3
	}

	client := ai.NewClient(platform.BaseURL, platform.APIKey, platform.Model)
	answer, err := client.Ask(ctx, buildPrompt(req, count))
	if err != nil {
		return nil, err
	}
	list := parseSloganList(answer)
	if len(list) == 0 {
		return nil, errEmptyGen
	}
	if len(list) > count {
		list = list[:count]
	}
	return &GenerateResult{Slogans: list, Platform: platform.Name, Model: platform.Model, SourcedFrom: "ai"}, nil
}

func buildPrompt(req GenerateReq, count int) string {
	var b strings.Builder
	b.WriteString("你是快手获客运营助手，帮运营人员生成用于私信打招呼的开场话术。\n\n")
	b.WriteString("客户信息：\n")
	b.WriteString("- 客户昵称：" + req.Nickname + "\n")
	if strings.TrimSpace(req.Comment) != "" {
		b.WriteString("- 客户评论：" + req.Comment + "\n")
	}
	if strings.TrimSpace(req.Tag) != "" && req.Tag != "其他" {
		b.WriteString("- 客户标签：" + req.Tag + "\n")
	}
	if strings.TrimSpace(req.AccountStyle) != "" {
		b.WriteString("- 打招呼账号风格：" + req.AccountStyle + "\n")
	}
	b.WriteString("\n请生成 ")
	b.WriteString(strconv.Itoa(count))
	b.WriteString(" 条开场话术，要求：\n")
	b.WriteString("1) 真人化、个性化、口语化，像真实用户在私信聊天；\n")
	b.WriteString("2) 结合客户的昵称/评论内容/标签（家长类客户注意措辞得体）；\n")
	b.WriteString("3) 简短自然，单条不超过50字；\n")
	b.WriteString("4) 坚决不带广告、营销、推销语，不得出现“免费”“优惠”“加微信”“报名”等强营销词；\n")
	b.WriteString("5) 不要问对方电话/住址等隐私，不要有攻击性或越界内容。\n\n")
	b.WriteString("请严格以 JSON 字符串数组格式返回，只输出数组本身，例如：\n")
	b.WriteString(`["嗨，看你也在xxx，好奇最近过得怎么样？", "看到你评论了，同感，可以聊聊~"]` + "\n")
	b.WriteString("不要输出任何解释或 Markdown 代码块。")
	return b.String()
}

// parseSloganList 解析 AI 返回：优先 JSON 数组，失败则逐行拆解
func parseSloganList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			var out []string
			for _, s := range arr {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "1.")
		line = strings.TrimPrefix(line, "2.")
		line = strings.TrimPrefix(line, "3.")
		line = strings.TrimPrefix(line, "- ")
		line = strings.Trim(line, `"“”'，,。 `)
		if len([]rune(line)) > 0 {
			out = append(out, line)
		}
	}
	if len(out) <= 1 {
		var quoted []string
		for _, part := range strings.Split(raw, `"`) {
			part = strings.TrimSpace(part)
			if part != "" && !strings.HasPrefix(part, "[") && !strings.HasSuffix(part, "]") && !strings.Contains(part, "：") {
				quoted = append(quoted, part)
			}
		}
		if len(quoted) > len(out) {
			out = quoted
		}
	}
	return out
}
