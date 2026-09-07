package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
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

	// 3) 启用平台（分站优先，未配置继承全局）
	var platforms []models.AiPlatform
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Order("id asc").Find(&platforms)
	if len(platforms) == 0 {
		database.DB.Where("tenant_id = ? AND enabled = ?", 0, true).Order("id asc").Find(&platforms)
	}
	platOK := len(platforms) > 0
	checks = append(checks, gin.H{
		"key": "platforms", "label": "AI 平台", "ok": platOK,
		"hint": func() string {
			if platOK {
				return "已启用 " + strconv.Itoa(len(platforms)) + " 个平台"
			}
			return "未配置启用平台，请到「AI 平台」添加并启用"
		}(),
	})
	if !platOK {
		ready = false
	}

	// 4) 平台 API Key（启用平台中至少一个已填 Key）
	keyOK := false
	for _, p := range platforms {
		if p.APIKey != "" {
			keyOK = true
			break
		}
	}
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
