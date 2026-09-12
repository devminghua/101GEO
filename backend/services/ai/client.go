package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/crypto"
)

// Client 基于 OpenAI 兼容协议的统一 AI 客户端
// 市面上主流 AI 平台（OpenAI/DeepSeek/Kimi/通义/智谱/豆包/混元/Ollama 等）
// 都提供 OpenAI 兼容的 /v1/chat/completions 接口，因此一个客户端即可全覆盖。
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
	HTTP    *http.Client

	// Thinking 推理模型思维链控制：nil=平台默认；Type=disabled 时关闭思考。
	// 由 DisableThinking() 设置，仅对当前 client 实例生效。
	Thinking *thinkingOptions

	// Token 用量统计元信息（WithMeta 设置后，每次成功调用自动写入 ai_usage_records）
	TenantID     uint   // 0 = 不记录
	PlatformName string // 空 = 不记录
	Scene        string
	LastUsage    Usage // 最近一次成功调用的 token 用量
}

// Usage OpenAI 兼容响应中的 token 用量。
type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// WithMeta 设置用量统计元信息（租户 + 平台名 + 场景），返回自身便于链式调用。
func (c *Client) WithMeta(tenantID uint, platform, scene string) *Client {
	c.TenantID = tenantID
	c.PlatformName = platform
	c.Scene = scene
	return c
}

func NewClient(baseURL, apiKey, model string) *Client {
	// 兼容加密存储：api_key 为 enc:v1 密文时自动解密后使用（加密迁移后无需改各调用点）
	if strings.HasPrefix(apiKey, crypto.PrefixEnc) {
		if dec, err := crypto.Decrypt(apiKey, config.Load().PayloadSecret()); err == nil {
			apiKey = dec
		}
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Timeout: 120 * time.Second,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	// NoThink 关闭推理模型的思维链。
	//
	// 为什么必须支持：DeepSeek-V4 / Qwen3 / GLM-4.7 等推理模型会先输出一大段
	// reasoning_content，而这段内容同样计入 max_tokens。做「把 100+ 个关键词
	// 聚成话题簇」这类长任务时，推理过程会吃光整个预算，返回 finish_reason=length
	// 且 content 为空（实测 1500 tokens 全部消耗在 reasoning_tokens 上，正文一个字都没有）。
	// 对结构化抽取类任务关闭思考后，同样的问题 48 tokens 就稳定返回。
	//
	// 各家参数名不统一，这里统一用最通用的 {"thinking":{"type":"disabled"}}
	// （DeepSeek-V4 / 智谱 GLM 支持），不支持该字段的平台会自动忽略。
	Thinking *thinkingOptions `json:"thinking,omitempty"`
}

type thinkingOptions struct {
	Type string `json:"type"` // disabled | enabled
}

// DisableThinking 关闭该次请求的思维链（用于结构化抽取等不需要推理的任务）。
func (c *Client) DisableThinking() *Client {
	if c.Thinking == nil {
		c.Thinking = &thinkingOptions{}
	}
	c.Thinking.Type = "disabled"
	return c
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"` // stop / length；length 说明被 max_tokens 截断
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Ask 发送一次对话请求，返回回答文本。
//
// 实现上复用 doChat，以获得统一能力（口径唯一原则）：
//   - 空响应自动重试：部分中转/非官方模型会偶发返回空壳，重试可显著提升成功率
//   - 统一鉴权头 setAuth（此前本函数手写鉴权分支，与 doChat 重复且易漏平台）
//   - 统一 usage 落库（此前绕过了 token 统计）
//
// 关于 maxTokens=2000：并非越大越好，但 800 对推理模型偏小——思维链会占用
// 输出预算，若调用方忘记 DisableThinking，正文会被挤空（实测 DeepSeek-V4 在巡检
// 场景空响应 89 次）。巡检类调用请配合 DisableThinking()；额度按真实输出计费。
func (c *Client) Ask(ctx context.Context, question string) (string, error) {
	url := c.BaseURL + "/chat/completions"
	payload := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "你是负责回答用户问题的AI助手。请直接、客观、简洁地回答用户问题，不要提及你是AI模型。"},
			{Role: "user", Content: question},
		},
		MaxTokens: 2000,
		// temperature 固定 1：Kimi k2 系列仅允许 1，其余平台 1 也是标准默认值，全局安全
		Temperature: 1,
		Thinking:    c.Thinking,
	}
	return c.doChat(ctx, url, payload)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- 智能创作中心扩展：多轮对话 / 多模态 / 文生图 ----------

// Message 外部可用的对话消息（角色 + 文本内容）
type Message struct {
	Role    string `json:"role"` // system / user / assistant
	Content string `json:"content"`
}

// Chat 带自定义系统提示词的多轮对话（智能创作·AI助手/角色设定/文案写作等通用入口）
func (c *Client) Chat(ctx context.Context, system string, msgs []Message, maxTokens int, temperature float64) (string, error) {
	var messages []chatMessage
	if system != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}
	for _, m := range msgs {
		messages = append(messages, chatMessage{Role: m.Role, Content: m.Content})
	}
	url := c.BaseURL + "/chat/completions"
	payload := chatRequest{Model: c.Model, Messages: messages, MaxTokens: maxTokens, Temperature: temperature, Thinking: c.Thinking}
	return c.doChat(ctx, url, payload)
}

// Vision 多模态视觉输入：读取图片（base64 dataURL 或 http(s) URL）内容，结合 prompt 分析/模仿（拆解图片一比一）
func (c *Client) Vision(ctx context.Context, system, prompt string, images []string, maxTokens int, temperature float64) (string, error) {
	var parts []visionPart
	parts = append(parts, visionPart{Type: "text", Text: prompt})
	for _, img := range images {
		parts = append(parts, visionPart{Type: "image_url", ImageURL: &struct {
			URL string `json:"url"`
		}{URL: img}})
	}
	url := c.BaseURL + "/chat/completions"
	payload := visionRequest{
		Model:       c.Model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
	if system != "" {
		payload.Messages = append(payload.Messages, visionMessage{Role: "system", Content: []visionPart{{Type: "text", Text: system}}})
	}
	payload.Messages = append(payload.Messages, visionMessage{Role: "user", Content: parts})
	return c.doChat(ctx, url, payload)
}

// GenerateImage 文生图：调用 OpenAI 兼容 /images/generations
// 返回 (base64, url) 二者至少其一可用
func (c *Client) GenerateImage(ctx context.Context, prompt, size, responseFormat string) (string, string, error) {
	url := c.BaseURL + "/images/generations"
	payload := imageGenRequest{
		Model:          c.Model,
		Prompt:         prompt,
		Size:           size,
		N:              1,
		ResponseFormat: responseFormat,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	var ig imageGenResponse
	if err := json.Unmarshal(data, &ig); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %v", err)
	}
	if ig.Error != nil {
		return "", "", fmt.Errorf("API错误: %s", ig.Error.Message)
	}
	if len(ig.Data) == 0 {
		return "", "", fmt.Errorf("响应为空")
	}
	item := ig.Data[0]
	// base64 可能出现前缀，去掉 data:image/xxx;base64,
	if strings.Contains(item.B64JSON, "base64,") {
		item.B64JSON = item.B64JSON[strings.Index(item.B64JSON, "base64,")+len("base64,"):]
	}
	return item.B64JSON, item.URL, nil
}

// doChat 统一发送 chat 类请求并解析回答。
// 针对部分中转/非官方模型偶发「空响应」问题，遇到空响应自动重试（最多 2 次），提升成功率。
func (c *Client) doChat(ctx context.Context, url string, payload interface{}) (string, error) {
	const maxRetries = 2
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		c.setAuth(req)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("请求失败: %v", err)
			if attempt < maxRetries {
				continue
			}
			return "", lastErr
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取响应失败: %v", err)
			if attempt < maxRetries {
				continue
			}
			return "", lastErr
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
		}
		var cr chatResponse
		if err := json.Unmarshal(data, &cr); err != nil {
			return "", fmt.Errorf("解析响应失败: %v", err)
		}
		if cr.Error != nil {
			return "", fmt.Errorf("API错误: %s", cr.Error.Message)
		}
		if len(cr.Choices) == 0 || cr.Choices[0].Message.Content == "" {
			// 区分两种空：推理模型烧光预算（finish=length，有 reasoning_content）
			// 与平台真的返回空。前者提示调用方考虑 DisableThinking。
			hint := "响应为空"
			if len(cr.Choices) > 0 {
				if fr := cr.Choices[0].FinishReason; fr == "length" {
					hint = "模型输出被 max_tokens 截断且正文为空（推理模型可能把预算消耗在思维链上，结构化任务建议 DisableThinking）"
				}
			}
			lastErr = fmt.Errorf("%s", hint)
			if attempt < maxRetries {
				continue // 空响应自动重试
			}
			return "", lastErr
		}
		// Token 用量统计：解析 usage + 异步落库（不影响主流程）
		c.LastUsage = cr.Usage
		if c.TenantID > 0 && c.PlatformName != "" {
			go recordUsage(c.TenantID, c.PlatformName, c.Model, c.Scene, cr.Usage)
		}
		return cr.Choices[0].Message.Content, nil
	}
	return "", lastErr
}

// recordUsage 异步写用量记录（goroutine 中调用，panic 兜底避免影响主流程）。
func recordUsage(tid uint, platform, model, scene string, u Usage) {
	defer func() { _ = recover() }()
	if u.TotalTokens <= 0 && u.PromptTokens <= 0 && u.CompletionTokens <= 0 {
		return // 平台未返回 usage，跳过
	}
	_ = database.DB.Create(&models.AiUsageRecord{
		TenantID:         tid,
		PlatformName:     platform,
		Model:            model,
		Scene:            scene,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}).Error
}

// setAuth 兼容 Bearer / Api-Key 两种鉴权头
func (c *Client) setAuth(req *http.Request) {
	if c.APIKey == "" {
		return
	}
	if strings.HasPrefix(c.BaseURL, "https://api.hunyuan") || strings.Contains(c.BaseURL, "qianfan") ||
		strings.Contains(c.BaseURL, "bigmodel") {
		req.Header.Set("Authorization", strings.TrimSpace(c.APIKey))
	} else {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.APIKey))
	}
}

// ---------- 多模态 / 文生图请求结构 ----------

type visionPart struct {
	Type     string `json:"type"` // text / image_url
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

type visionMessage struct {
	Role    string       `json:"role"`
	Content []visionPart `json:"content"`
}

type visionRequest struct {
	Model       string          `json:"model"`
	Messages    []visionMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

type imageGenRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size"`
	N              int    `json:"n"`
	ResponseFormat string `json:"response_format"`
}

type imageGenResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
