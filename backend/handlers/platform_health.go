package handlers

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/models"
	"geo-tool/services/ai"
)

// PlatformHealth 单个平台的健康度
type PlatformHealth struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"` // healthy / bad_key / no_key / timeout / empty / error / disabled
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// PlatformHealthCheck 平台对接自检：GET /api/platforms/health
// 对每个平台发一个最小探测请求，判断 Key 有效性、响应是否正常、耗时，输出健康度。
// 供「登录后自检」与「AI 平台」页面展示。
func PlatformHealthCheck(c *gin.Context) {
	tid := TenantID(c)
	var platforms []models.AiPlatform
	// 全局平台 + 分站覆盖层合并，仅取启用平台
	for _, p := range EffectivePlatforms(tid) {
		if p.Enabled {
			platforms = append(platforms, p)
		}
	}

	results := make([]PlatformHealth, len(platforms))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // 最多 4 个平台并发探测，避免同时轰炸

	for i, p := range platforms {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, p models.AiPlatform) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = probePlatform(p)
		}(i, p)
	}
	wg.Wait()

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

// probePlatform 探测单个平台，返回健康度
func probePlatform(p models.AiPlatform) PlatformHealth {
	h := PlatformHealth{ID: p.ID, Name: p.Name, Enabled: p.Enabled}
	if !p.Enabled {
		h.Status = "disabled"
		return h
	}
	if strings.TrimSpace(p.APIKey) == "" {
		h.Status = "no_key"
		h.Error = "未配置 API Key"
		return h
	}

	client := ai.NewClient(p.BaseURL, p.APIKey, p.Model)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	// temperature=1 兼容 Kimi（kimi-k2.6 仅允许 1）；maxTokens=16 避免过小导致空响应误判
	resp, err := client.Chat(ctx, "", []ai.Message{{Role: "user", Content: "hi"}}, 16, 1)
	h.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		msg := err.Error()
		low := strings.ToLower(msg)
		switch {
		case strings.Contains(low, "401") || strings.Contains(low, "incorrect api key") || strings.Contains(low, "invalid api key") || strings.Contains(low, "authentication"):
			h.Status = "bad_key"
		case strings.Contains(low, "timeout") || strings.Contains(low, "deadline"):
			h.Status = "timeout"
		default:
			h.Status = "error"
		}
		h.Error = msg
		return h
	}
	if strings.TrimSpace(resp) == "" {
		h.Status = "empty"
		h.Error = "响应为空"
		return h
	}
	h.Status = "healthy"
	return h
}
