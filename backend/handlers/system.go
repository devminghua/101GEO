package handlers

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
)

// logoUploadDir 返回系统 Logo 上传目录（由 main.go 静态托管 /uploads，随数据根目录解耦）。
func logoUploadDir() string {
	return config.Load().UploadsDir
}

const (
	KeySystemName     = "system_name"      // 系统名称
	KeySystemLogo     = "system_logo"      // 系统 Logo（可访问 URL）
	KeyCopyright      = "copyright"        // 底部版权文案
	KeyServicePhone   = "service_phone"    // 技术服务电话（客户端客服悬浮）
	KeyServiceWechat  = "service_wechat_qr" // 技术服务微信二维码（URL）
)

// isOperator 当前登录用户是否为 AI 优化员（受限角色）
func isOperator(c *gin.Context) bool {
	return CurrentRole(c) == "operator"
}

// AdminOnly 仅 super / admin 可访问，AI 优化员（operator）一律拒绝。
// 用于：平台 API 配置、租户设置读写、密码管理、系统信息写入、优化员账号管理等敏感接口。
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isOperator(c) {
			c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "AI 优化员账号无此权限，请联系管理员操作"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// readSystemInfo 读取品牌配置：分站级优先，回退全局（tenant_id=0）。
// tenantID=0 时只读全局（供登录页/公开接口使用）。
func readSystemInfo(tenantID uint) gin.H {
	return gin.H{
		"system_name":      readSetting(tenantID, KeySystemName),
		"system_logo":      readSetting(tenantID, KeySystemLogo),
		"copyright":        readSetting(tenantID, KeyCopyright),
		"service_phone":    readSetting(0, KeyServicePhone),
		"service_wechat_qr": readSetting(0, KeyServiceWechat),
	}
}

// GetSystemInfo 读取系统名称 / Logo / 版权（公开只读，供登录页、侧边栏、页脚展示）
// 未登录场景只读全局品牌（tenant_id=0）。
func GetSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": readSystemInfo(0)})
}

// MyBrandInfo 登录后读取当前分站的品牌配置（分站级优先，回退全局）：GET /api/system/my-brand
func MyBrandInfo(c *gin.Context) {
	tid := TenantID(c)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"system_name": readSetting(tid, KeySystemName),
		"copyright":   readSetting(tid, KeyCopyright),
	}})
}

type systemInfoReq struct {
	SystemName     string `json:"system_name"`
	SystemLogo     string `json:"system_logo"`
	Copyright      string `json:"copyright"`
	ServicePhone   string `json:"service_phone"`
	ServiceWechatQr string `json:"service_wechat_qr"`
}

// SaveSystemInfo 保存系统名称 / 版权 / 客服电话与二维码（仅 super / admin）。
// 系统名称、版权为品牌定制功能：
//   - super：写入全局（tenant_id=0），影响所有未自定义的分站与登录页；
//   - 分站 admin：写入分站级（tenant_id=分站ID），仅影响自己平台，需 Token ≥ 2000。
func SaveSystemInfo(c *gin.Context) {
	var req systemInfoReq
	if !jsonBody(c, &req) {
		return
	}
	role := CurrentRole(c)
	if role == "super" {
		saveSetting(0, KeySystemName, req.SystemName)
		saveSetting(0, KeyCopyright, req.Copyright)
		saveSetting(0, KeyServicePhone, req.ServicePhone)
		saveSetting(0, KeyServiceWechat, req.ServiceWechatQr)
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "系统信息已保存", "data": readSystemInfo(0)})
		return
	}
	// 分站（admin）自定义品牌名称/版权：需 Token ≥ 2000，写入分站级
	tid := TenantID(c)
	var t models.Tenant
	if database.DB.First(&t, tid).Error == nil && t.Points < 2000 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "自定义品牌名称/版权需 Token ≥ 2000，请先充值"})
		return
	}
	saveSetting(tid, KeySystemName, req.SystemName)
	saveSetting(tid, KeyCopyright, req.Copyright)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "系统信息已保存", "data": readSystemInfo(tid)})
}

// saveSetting 写入指定租户的 KV（tenant_id=0 为全局）。
// 注意：Setting 复合主键为 (tenant_id, key)，其中 tenant_id=0 是零值，
// gorm 的 Save() 会把全零主键当作 INSERT，而 Create() 对已存在行又会触发唯一约束，
// 因此统一走原生 SQL UPSERT（PostgreSQL ON CONFLICT），原子处理插入与更新。
func saveSetting(tenantID uint, key, value string) {
	value = strings.TrimSpace(value)
	// 用 gorm clause.OnConflict 做 upsert（跨 PostgreSQL / SQLite 兼容）
	if err := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&models.Setting{TenantID: tenantID, Key: key, Value: value}).Error; err != nil {
		log.Printf("[system] 保存设置 %s 失败: %v", key, err)
	}
}

// saveGlobalSetting 写入全局 KV（tenant_id=0）。
func saveGlobalSetting(key, value string) { saveSetting(0, key, value) }

// readSetting 读取设置值：分站级优先，回退全局。
func readSetting(tenantID uint, key string) string {
	var s models.Setting
	if tenantID > 0 {
		if database.DB.Where("tenant_id = ? AND key = ?", tenantID, key).First(&s).Error == nil && s.Value != "" {
			return s.Value
		}
	}
	if database.DB.Where("tenant_id = 0 AND key = ?", key).First(&s).Error == nil {
		return s.Value
	}
	return ""
}

// UploadLogo 上传系统 Logo（仅 super / admin），落盘 uploads/ 并返回可访问 URL。
// 前端拿到 URL 后随系统信息一起保存，浏览器可直接访问。
func UploadLogo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请选择要上传的图片"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true,
		".webp": true, ".gif": true,
		// 注意：不支持 .svg —— SVG 可内嵌 <script>，直接访问存在存储型 XSS 风险，已移除。
	}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "仅支持 png/jpg/jpeg/webp/gif 图片"})
		return
	}
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "图片大小不能超过 5MB"})
		return
	}
	if err := os.MkdirAll(logoUploadDir(), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建上传目录失败"})
		return
	}
	name := "system_logo_" + time.Now().Format("20060102150405") + ext
	dst := filepath.Join(logoUploadDir(), name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "保存图片失败：" + err.Error()})
		return
	}
	url := "/uploads/" + name
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "上传成功", "data": gin.H{"url": url}})
}

// UploadImage 通用图片上传（仅 super / admin），form 字段 file + type（文件名前缀，如 service_qr）。
// 用于客服二维码等非 Logo 图片。
func UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请选择要上传的图片"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true,
		".webp": true, ".gif": true,
	}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "仅支持 png/jpg/jpeg/webp/gif 图片"})
		return
	}
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "图片大小不能超过 5MB"})
		return
	}
	if err := os.MkdirAll(logoUploadDir(), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建上传目录失败"})
		return
	}
	prefix := strings.TrimSpace(c.PostForm("type"))
	if prefix == "" || strings.ContainsAny(prefix, "./\\") {
		prefix = "image"
	}
	name := prefix + "_" + time.Now().Format("20060102150405") + ext
	dst := filepath.Join(logoUploadDir(), name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "保存图片失败：" + err.Error()})
		return
	}
	url := "/uploads/" + name
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "上传成功", "data": gin.H{"url": url}})
}
