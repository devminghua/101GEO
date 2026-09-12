package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai_platform"
)

// GeoSetupCheck GEO 配置完整性自检：GET /api/geo/setup-check
// 检查品牌词 / 关键词 / 启用平台 / 平台 API Key 是否就绪，
// 返回逐项检查结果 + 引导文案，供客户端首页「配置引导」提示。
func GeoSetupCheck(c *gin.Context) {
	tid := TenantID(c)
	checks := make([]gin.H, 0, 4)
	ready := true

	// 1) 品牌词
	brand := BrandOf(c)
	brandOK := brand != ""
	checks = append(checks, gin.H{
		"key": "brand", "label": "品牌词", "ok": brandOK,
		"hint": func() string {
			if brandOK {
				return "已配置：" + brand
			}
			return "未配置品牌词，请到「系统设置 → 默认品牌词」填写"
		}(),
	})
	if !brandOK {
		ready = false
	}

	// 2) 关键词
	var kwCount int64
	database.DB.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND enabled = ?", tid, true).Count(&kwCount)
	kwOK := kwCount > 0
	checks = append(checks, gin.H{
		"key": "keywords", "label": "监控关键词", "ok": kwOK,
		"hint": func() string {
			if kwOK {
				return "已配置 " + strconv.Itoa(int(kwCount)) + " 个启用关键词"
			}
			return "未配置监控关键词，请到「关键词监控」添加"
		}(),
	})
	if !kwOK {
		ready = false
	}

	// 3) 启用平台（统一走 services/ai_platform：分站只用自己的平台，不继承全局）
	//
	// 历史实现是「分站优先，未配置继承全局」，会给客户一个错误的体检结论——
	// 提示「AI 平台已就绪」，但实际功能全部不可用（因为统一后分站不继承全局平台）。
	// 老板 2026-09-12 统一规则后，此处必须与实际取平台逻辑保持一致，否则体检会误报。
	var platCount int
	for _, p := range ai_platform.OwnPlatforms(tid) {
		if ai_platform.Usable(&p) {
			platCount++
		}
	}
	platOK := platCount > 0
	checks = append(checks, gin.H{
		"key": "platforms", "label": "AI 平台", "ok": platOK,
		"hint": func() string {
			if platOK {
				return "已启用 " + strconv.Itoa(platCount) + " 个平台"
			}
			return "未配置启用平台，请到「AI 平台」添加并启用"
		}(),
	})
	if !platOK {
		ready = false
	}

	// 4) 平台 API Key
	//
	// 注意：ai_platform.Usable 已经把「Key 为空且非本地自托管」的平台排除在外，
	// 所以上面的 platCount 能算出非零，就意味着至少有一个平台 Key 是齐的，
	// 无需再单列一遍 Key 检查。这里保留该体检项仅用于向客户展示这一层信息。
	keyOK := platOK
	checks = append(checks, gin.H{
		"key": "platform_key", "label": "平台 API Key", "ok": keyOK,
		"hint": func() string {
			if keyOK {
				return "已配置 API Key"
			}
			if !platOK {
				return "无启用平台，无需配置"
			}
			return "启用平台未填 API Key，请到「AI 平台」填写"
		}(),
	})
	if platOK && !keyOK {
		ready = false
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"ready": ready, "checks": checks}})
}
