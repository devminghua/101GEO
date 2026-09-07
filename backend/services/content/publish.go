package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai_creation"
)

/* ================================================================
 * 内容投放 · 发布服务
 *
 * 发布模式：
 *  - auto：租户配置了第三方发稿平台 API（OpenAI 无关，通用 HTTP POST）时，
 *    提交任务后把 {title, content, media_name} POST 到该接口；成功标记
 *    published 并回填 PublishURL（从响应中解析）；失败标记 failed 并记录
 *    estimate 监控（无法验证）。
 *  - 未配置 API / 配置禁用：任务进入 wait_manual（待人工在媒体后台发布），
 *    由用户在「发布任务」页回填链接。
 * 合规：不模拟登录任何媒体后台、不自动群发；人工环节由用户完成。
 * ================================================================ */

// 发稿平台配置 KV key（租户级，复用 models.Setting）
const (
	KeyPublishEnabled = "content_publish_enabled" // 1/0
	KeyPublishAPI     = "content_publish_api"     // 发稿接口 URL
	KeyPublishKey     = "content_publish_key"     // 鉴权 Key（可选）
)

// PublishConfig 发稿平台配置（Key 脱敏返回，不回传明文）
type PublishConfig struct {
	Enabled   bool   `json:"enabled"`
	API       string `json:"api"`
	HasKey    bool   `json:"has_key"`
	KeyMasked string `json:"key_masked"`
	key       string // 明文仅内部使用（不序列化）
}

// GetPublishConfig 读取发稿平台配置
func GetPublishConfig(tid uint) PublishConfig {
	cfg := PublishConfig{
		Enabled: ai_creation.GetSetting(tid, KeyPublishEnabled) == "1",
		API:     ai_creation.GetSetting(tid, KeyPublishAPI),
		key:     ai_creation.GetSetting(tid, KeyPublishKey),
	}
	cfg.HasKey = cfg.key != ""
	if cfg.HasKey && len(cfg.key) > 4 {
		cfg.KeyMasked = strings.Repeat("*", 6) + cfg.key[len(cfg.key)-4:]
	}
	return cfg
}

// SavePublishConfig 保存发稿平台配置；key 为空时保持原 Key 不变
func SavePublishConfig(tid uint, cfg PublishConfig, key string) error {
	enabled := "0"
	if cfg.Enabled {
		enabled = "1"
	}
	if err := ai_creation.SetSetting(tid, KeyPublishEnabled, enabled); err != nil {
		return err
	}
	if err := ai_creation.SetSetting(tid, KeyPublishAPI, strings.TrimSpace(cfg.API)); err != nil {
		return err
	}
	if strings.TrimSpace(key) != "" {
		return ai_creation.SetSetting(tid, KeyPublishKey, strings.TrimSpace(key))
	}
	return nil
}

// SubmitTask 发布任务：按配置走自动对接或转待人工
func SubmitTask(ctx context.Context, tid, taskID uint) error {
	var task models.CtnTask
	if err := database.DB.Where("id = ? AND tenant_id = ?", taskID, tid).First(&task).Error; err != nil {
		return err
	}
	if task.Status == models.CtnTaskPublished {
		return nil
	}

	database.DB.Model(&task).Update("status", models.CtnTaskPublishing)

	cfg := GetPublishConfig(tid)
	if !cfg.Enabled || strings.TrimSpace(cfg.API) == "" {
		database.DB.Model(&task).Updates(map[string]interface{}{
			"status":         models.CtnTaskWaitManual,
			"publish_result": "未配置发稿平台接口，请到媒体后台人工发布后回填文章链接",
		})
		return nil
	}

	// 自动对接第三方发稿平台
	payload := map[string]string{
		"title":      task.ArticleTitle,
		"content":    loadArticleContent(tid, task.ArticleID),
		"media_name": task.MediaName,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(cfg.API), bytes.NewReader(body))
	if err != nil {
		database.DB.Model(&task).Updates(map[string]interface{}{"status": models.CtnTaskFailed, "publish_result": "构造请求失败: " + err.Error()})
		writeMonitorEstimate(tid, task, "发稿平台请求失败: "+err.Error())
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.key != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.key))
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		database.DB.Model(&task).Updates(map[string]interface{}{"status": models.CtnTaskFailed, "publish_result": "发稿平台请求失败: " + err.Error()})
		writeMonitorEstimate(tid, task, "发稿平台请求失败: "+err.Error())
		return nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		now := time.Now()
		updates := map[string]interface{}{
			"status":         models.CtnTaskPublished,
			"published_at":   &now,
			"publish_result": "已提交发稿平台",
		}
		if u := parseURLFromResponse(data); u != "" {
			updates["publish_url"] = u
		}
		database.DB.Model(&task).Updates(updates)
		return nil
	}

	msg := fmt.Sprintf("发稿平台返回 HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	database.DB.Model(&task).Updates(map[string]interface{}{"status": models.CtnTaskFailed, "publish_result": msg})
	writeMonitorEstimate(tid, task, "发稿平台提交失败: "+msg)
	return nil
}

// loadArticleContent 读取软文正文（按租户隔离）
func loadArticleContent(tid, articleID uint) string {
	if articleID == 0 {
		return ""
	}
	var a models.CtnArticle
	if err := database.DB.Where("id = ? AND tenant_id = ?", articleID, tid).First(&a).Error; err != nil {
		return ""
	}
	return a.Content
}

// parseURLFromResponse 从发稿平台响应中提取文章链接（支持 data.url / url / data.link）
func parseURLFromResponse(data []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if d, ok := m["data"].(map[string]interface{}); ok {
		for _, k := range []string{"url", "link", "article_url", "publish_url"} {
			if v, ok := d[k].(string); ok && strings.HasPrefix(v, "http") {
				return v
			}
		}
	}
	for _, k := range []string{"url", "link", "article_url"} {
		if v, ok := m[k].(string); ok && strings.HasPrefix(v, "http") {
			return v
		}
	}
	return ""
}

// writeMonitorEstimate 记录一条 estimate 监控（无法验证收录）
func writeMonitorEstimate(tid uint, task models.CtnTask, note string) {
	database.DB.Create(&models.CtnMonitor{
		TenantID: tid, TaskID: task.ID, TaskTitle: task.ArticleTitle,
		MediaName: task.MediaName, PublishURL: task.PublishURL,
		CheckMethod: "estimate", SourcedFrom: models.SourceEstimate, Note: note,
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
