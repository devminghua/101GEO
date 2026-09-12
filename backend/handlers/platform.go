package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai"
	"geo-tool/services/ai_platform"
	"geo-tool/services/crypto"
)

// EffectivePlatforms 返回指定租户「实际使用」的平台列表。
//
// 实现已委托给 services/ai_platform（**唯一权威来源**），本函数仅作 HTTP 层的薄封装。
//
// 归属规则（老板 2026-09-12 定）：全部走**分站自己的 Key**。
//   - tid != 0：只返回分站自有平台，不继承总后台全局平台。
//   - tid == 0：返回全局平台池。
//
// 历史备注：本函数旧注释写的是「全局平台 + 分站覆盖层合并」，但旧实现其实是分站独立，
// 而同一时期创作中心走「全局优先」、巡检走「合并覆盖层」——三套并存导致同一分站不同功能
// 从不同账号扣 API 费用。本次重构后所有功能共用本实现，注释与行为已对齐（详见 ai_platform 包注释）。
func EffectivePlatforms(tid uint) []models.AiPlatform {
	return ai_platform.OwnPlatforms(tid)
}

// ListPlatforms 平台列表（当前租户）；API Key 密文解密后返回，便于前端回显与编辑。
// 分站返回「全局平台 + 覆盖层合并」后的实际平台，super 返回全局平台。
func ListPlatforms(c *gin.Context) {
	tid := TenantID(c)
	list := EffectivePlatforms(tid)
	for i := range list {
		if strings.HasPrefix(list[i].APIKey, crypto.PrefixEnc) {
			if dec, err := crypto.Decrypt(list[i].APIKey, config.Load().PayloadSecret()); err == nil {
				list[i].APIKey = dec
			} else {
				list[i].APIKey = ""
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// PlatformTemplates 常见平台模板（国内主流大模型 OpenAI 兼容接口）
func PlatformTemplates(c *gin.Context) {
	templates := []gin.H{
		{"name": "DeepSeek", "base_url": "https://api.deepseek.com", "model": "deepseek-v4-flash"},
		{"name": "豆包（火山方舟）", "base_url": "https://ark.cn-beijing.volces.com/api/v3", "model": "doubao-seed-evolving"},
		{"name": "通义千问（阿里百炼）", "base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1", "model": "qwen3.6-plus"},
		{"name": "智谱 GLM", "base_url": "https://open.bigmodel.cn/api/paas/v4", "model": "glm-4.7-flash"},
		{"name": "Kimi（月之暗面）", "base_url": "https://api.moonshot.cn/v1", "model": "kimi-k2.6"},
		{"name": "腾讯混元", "base_url": "https://tokenhub.tencentmaas.com/v1/", "model": "hy4-preview"},
		{"name": "百度文心（千帆）", "base_url": "https://qianfan.baidubce.com/v2", "model": "ernie-5.0"},
		{"name": "MiniMax（稀宇）", "base_url": "https://api.minimaxi.com/v1", "model": "MiniMax-M2.7"},
		{"name": "讯飞星火", "base_url": "https://spark-api-open.xf-yun.com/v1", "model": "spark-4.0-ultra"},
		{"name": "零一万物", "base_url": "https://api.lingyiwanwu.com/v1", "model": "yi-large"},
		{"name": "百川智能", "base_url": "https://api.baichuan-ai.com/v1", "model": "baichuan4-turbo"},
		{"name": "阶跃星辰", "base_url": "https://api.stepfun.com/v1", "model": "step-2-16k"},
		{"name": "小米 MiMo", "base_url": "https://api.xiaomimimo.com/v1", "model": "mimo-v2.5-pro"},
		{"name": "商汤日日新", "base_url": "https://api.sensenova.cn/v1", "model": "SenseChat-5"},
		{"name": "OpenAI", "base_url": "https://api.openai.com", "model": "gpt-4o-mini"},
		{"name": "Ollama（本地）", "base_url": "http://localhost:11434/v1", "model": "qwen2.5:7b"},
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": templates})
}

type platformReq struct {
	Name       string `json:"name"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
	Enabled    *bool    `json:"enabled"`
	SortOrder  *FlexInt `json:"sort_order"`
	IntervalMs *FlexInt `json:"interval_ms"`
	ConfigJSON string   `json:"config_json"`
}

// FlexInt 兼容数字与数字字符串的 int 解析（如 "0" 或 0）
type FlexInt int

// UnmarshalJSON 接受 JSON number 或 string（含空串），避免参数类型错误
func (f *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

// CreatePlatform 新增平台
func CreatePlatform(c *gin.Context) {
	tid := TenantID(c)
	// 分站可新增自己的平台（存 tenant_id=tid，独立配置自己的 Key）
	var req platformReq
	if !jsonBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Model = strings.TrimSpace(req.Model)
	if req.Name == "" || req.BaseURL == "" || req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "平台名 / Base URL / 模型名 均不能为空"})
		return
	}
	enabled, sort := true, 0
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		sort = int(*req.SortOrder)
	}
	intervalMs := 2000 // 新建平台默认 2000ms 请求间隔（规避限流 429）
	if req.IntervalMs != nil && int(*req.IntervalMs) >= 0 {
		intervalMs = int(*req.IntervalMs)
	}
	// API Key 加密存储（enc:v1:AES-GCM），防止数据库泄露时明文 Key 外泄
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey != "" {
		if enc, err := crypto.Encrypt(apiKey, config.Load().PayloadSecret()); err == nil {
			apiKey = enc
		}
	}
	p := models.AiPlatform{
		TenantID: tid, Name: req.Name, BaseURL: req.BaseURL, APIKey: apiKey,
		Model: req.Model, Enabled: enabled, SortOrder: sort, IntervalMs: intervalMs, ConfigJSON: req.ConfigJSON,
	}
	if err := database.DB.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已新增", "data": p})
}

// UpdatePlatform 更新平台（API Key 留空表示不修改）。
// super 更新全局平台；分站写覆盖层（tenant_platform_overrides，按全局平台 ID 关联）。
func UpdatePlatform(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	if tid != 0 {
		// 分站：只编辑自己的平台（tenant_id=tid）
		var own models.AiPlatform
		if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&own).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "平台不存在"})
			return
		}
		var req struct {
			Name      *string  `json:"name"`
			BaseURL   *string  `json:"base_url"`
			APIKey    *string  `json:"api_key"`
			Model     *string  `json:"model"`
			Enabled   *bool    `json:"enabled"`
			SortOrder *FlexInt `json:"sort_order"`
		}
		if !jsonBody(c, &req) {
			return
		}
		if req.Name != nil {
			own.Name = strings.TrimSpace(*req.Name)
		}
		if req.BaseURL != nil {
			own.BaseURL = strings.TrimSpace(*req.BaseURL)
		}
		if req.Model != nil {
			own.Model = strings.TrimSpace(*req.Model)
		}
		if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
			key := strings.TrimSpace(*req.APIKey)
			if !strings.HasPrefix(key, crypto.PrefixEnc) {
				if enc, err := crypto.Encrypt(key, config.Load().PayloadSecret()); err == nil {
					key = enc
				}
			}
			own.APIKey = key
		}
		if req.Enabled != nil {
			own.Enabled = *req.Enabled
		}
		if req.SortOrder != nil {
			own.SortOrder = int(*req.SortOrder)
		}
		database.DB.Save(&own)
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
		return
	}
	var p models.AiPlatform
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&p).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "平台不存在"})
		return
	}
	var req struct {
		Name       *string   `json:"name"`
		BaseURL    *string   `json:"base_url"`
		APIKey     *string   `json:"api_key"`
		Model      *string   `json:"model"`
		Enabled    *bool     `json:"enabled"`
		SortOrder  *FlexInt  `json:"sort_order"`
		IntervalMs *FlexInt  `json:"interval_ms"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Name != nil {
		p.Name = strings.TrimSpace(*req.Name)
	}
	if req.BaseURL != nil {
		p.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.Model != nil {
		p.Model = strings.TrimSpace(*req.Model)
	}
	if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
		key := strings.TrimSpace(*req.APIKey)
		// 前端回显的是解密明文，正常直接加密保存；若已是 enc:v1 密文则视为未修改跳过
		if !strings.HasPrefix(key, crypto.PrefixEnc) {
			if enc, err := crypto.Encrypt(key, config.Load().PayloadSecret()); err == nil {
				key = enc
			}
		}
		p.APIKey = key
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		p.SortOrder = int(*req.SortOrder)
	}
	if req.IntervalMs != nil && int(*req.IntervalMs) >= 0 {
		p.IntervalMs = int(*req.IntervalMs)
	}
	database.DB.Save(&p)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
}

// DeletePlatform 删除平台。super 删除全局平台；分站删除覆盖层（恢复全局默认）。
func DeletePlatform(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	if tid != 0 {
		// 分站：物理删除自己的平台
		if result := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.AiPlatform{}); result.RowsAffected > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "平台不存在"})
		return
	}
	result := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.AiPlatform{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "平台不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}

// TestPlatform 测试平台连接（返回 AI 回答）
func TestPlatform(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	// 分站用合并后的平台（含覆盖层），super 用全局平台
	var p models.AiPlatform
	found := false
	for _, x := range EffectivePlatforms(tid) {
		if x.ID == uint(id) {
			p = x
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "平台不存在"})
		return
	}
	if p.APIKey == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "请先配置 API Key"})
		return
	}
	client := ai.NewClient(p.BaseURL, p.APIKey, p.Model).WithMeta(TenantID(c), p.Name, "平台测试")
	// 429 限流自动退避重试（3s/8s 两次）：点击「测试」撞上巡检高峰或平台限流时不再直接报错
	var answer string
	var err error
	waits := []time.Duration{3 * time.Second, 8 * time.Second}
	for attempt := 0; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		answer, err = client.Ask(ctx, "你好，请回复“连接成功”")
		cancel()
		if err == nil || attempt >= len(waits) || !strings.Contains(err.Error(), "429") {
			break
		}
		time.Sleep(waits[attempt])
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "连接失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "连接正常", "data": gin.H{"answer": answer}})
}
