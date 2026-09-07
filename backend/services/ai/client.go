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
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Ask 发送一次对话请求，返回回答文本
func (c *Client) Ask(ctx context.Context, question string) (string, error) {
	url := c.BaseURL + "/chat/completions"
	payload := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "你是负责回答用户问题的AI助手。请直接、客观、简洁地回答用户问题，不要提及你是AI模型。"},
			{Role: "user", Content: question},
		},
		MaxTokens: 800,
		// temperature 固定 1：Kimi k2 系列仅允许 1，其余平台 1 也是标准默认值，全局安全
		Temperature: 1,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// 兼容两种鉴权头：绝大多数平台用 Bearer，部分（如腾讯混元/文心）用 Api-Key
		if strings.HasPrefix(c.BaseURL, "https://api.hunyuan") || strings.Contains(c.BaseURL, "qianfan") ||
			strings.Contains(c.BaseURL, "bigmodel") {
			req.Header.Set("Authorization", strings.TrimSpace(c.APIKey))
		} else {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.APIKey))
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
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
		return "", fmt.Errorf("响应为空")
	}
	return cr.Choices[0].Message.Content, nil
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
	payload := chatRequest{Model: c.Model, Messages: messages, MaxTokens: maxTokens, Temperature: temperature}
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
			lastErr = fmt.Errorf("响应为空")
			if attempt < maxRetries {
				continue // 空响应自动重试
			}
			return "", lastErr
		}
		return cr.Choices[0].Message.Content, nil
	}
	return "", lastErr
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
