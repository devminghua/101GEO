// Package ai_platform 是「用哪个 AI 平台」的**唯一权威来源**。
//
// 为什么必须收拢到一处（2026-09-12 重构）：
// 重构前系统里并存三套互相冲突的策略——
//   A) handlers.EffectivePlatforms  → 只读分站自有平台（AI 助手、话题簇）
//   B) ai_creation.FirstEnabledPlatform → 全局优先、分站兜底（创作中心、抖音/小红书话术、文章生成）
//   C) services/geo/checker         → 全局平台打底 + 分站覆盖层合并（GEO 巡检）
//
// 后果是同一个分站的不同功能会从两个不同账号扣 API 费用：
// 实测分站 linkcrm 用「AI 助手」走自己的 Key（sk-0f535…），
// 用「创作中心」却走总后台的 Key（sk-87b63…）。
// 且未配置平台的分站会出现「AI 平台页显示 0 个，但 GEO 巡检照样能跑」的体验割裂。
//
// 老板 2026-09-12 拍板：**统一为「全部走分站自己的 Key」**——
// 各分站客户自备 Key、自付 API 费用，平台方只收软件费。这样零 API 成本、无欠费风险，
// 且客户的用量与账单在自己账号里一目了然。
//
// 因此本包是唯一入口，任何需要「取 AI 平台」的代码都必须调用这里，
// 不要再各写一套（同 v1.0.33 站点审计口径统一、v1.0.35 GEO 指标口径统一的同一个原则）。
package ai_platform

import (
	"strings"

	"geo-tool/database"
	"geo-tool/models"
)

// isLocalBaseURL 判断是否自托管本地服务（如 Ollama / 本地 vLLM）。
// 本地服务通常不需要 API Key，因此这类平台即使 Key 为空也视为可用。
//
// 注意：原先有两份各自为政的同类函数（ai_creation.isLocalBaseURL 少判 "::1"，
// handlers.isLocalURL 也少判 "::1"），这里统一补齐，避免同一台机器两种判定。
func isLocalBaseURL(u string) bool {
	l := strings.ToLower(strings.TrimSpace(u))
	return strings.Contains(l, "localhost") ||
		strings.Contains(l, "127.0.0.1") ||
		strings.Contains(l, "0.0.0.0") ||
		strings.Contains(l, "::1")
}

// IsLocalBaseURL 导出版本，供其他包（如 ai_creation）复用同一判定，避免再各写一套。
func IsLocalBaseURL(u string) bool { return isLocalBaseURL(u) }

// Usable 判断单个平台是否「真的能用来发请求」。
//
// 可用 = 已启用 && base_url 非空 && （有 API Key || 是本地自托管服务）。
// 最后一条很关键：Key 为空却去请求厂商接口只会拿到 401，
// 不如提前跳过，让上层给出「请先配置 AI 平台」的明确提示。
func Usable(p *models.AiPlatform) bool {
	if p == nil || !p.Enabled {
		return false
	}
	if strings.TrimSpace(p.BaseURL) == "" {
		return false
	}
	if strings.TrimSpace(p.APIKey) == "" && !isLocalBaseURL(p.BaseURL) {
		return false
	}
	return true
}

// OwnPlatforms 返回该租户「实际会使用」的全部平台。
//
// 规则（老板 2026-09-12 定）：
//   - tenant_id != 0（分站/客户端）：**只返回分站自有平台**，不继承总后台全局平台。
//     分站之间互不可见，各自用各自的 Key。
//   - tenant_id == 0（总后台 super）：返回全局平台池（tenant_id=0 的记录）。
//
// 返回值已按 sort_order, id 排序，调用方可直接取第一个。
func OwnPlatforms(tid uint) []models.AiPlatform {
	owner := tid
	if tid == 0 {
		owner = 0
	}
	var list []models.AiPlatform
	database.DB.Where("tenant_id = ?", owner).
		Order("sort_order asc, id asc").
		Find(&list)
	return list
}

// FirstUsable 返回该租户第一个「可用」的平台，没有则返回 nil。
//
// 用途：单平台场景（话术生成、文章生成等只需一个模型的功能）。
// 调用方拿到 nil 应当返回「请先配置 AI 平台」之类的明确提示，而不是继续调用。
func FirstUsable(tid uint) *models.AiPlatform {
	for _, p := range OwnPlatforms(tid) {
		if Usable(&p) {
			cp := p
			return &cp
		}
	}
	return nil
}

// PickPreferred 返回该租户「优先匹配 prefer 关键字」的可用平台。
//
// 匹配范围：平台 Name 或 Model 包含 prefer（大小写不敏感）。
// 例如 prefer="deepseek" 会优先选中 DeepSeek，用于「AI 数据分析助手优先用 DeepSeek」这类需求。
//
// 若没有任何平台匹配 prefer，则回落到第一个可用平台——保证助手不会因单一平台缺失而不可用。
// 若该租户一个可用平台都没有，返回 nil。
func PickPreferred(tid uint, prefer string) *models.AiPlatform {
	prefer = strings.ToLower(strings.TrimSpace(prefer))

	var matched, fallback *models.AiPlatform
	for _, p := range OwnPlatforms(tid) {
		if !Usable(&p) {
			continue
		}
		cp := p
		if prefer != "" {
			name := strings.ToLower(cp.Name)
			model := strings.ToLower(cp.Model)
			if strings.Contains(name, prefer) || strings.Contains(model, prefer) {
				if matched == nil {
					matched = &cp
				}
				continue
			}
		}
		if fallback == nil {
			fallback = &cp
		}
	}
	if matched != nil {
		return matched
	}
	return fallback
}
