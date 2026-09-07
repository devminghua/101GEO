package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai"
	svc "geo-tool/services/ai_creation"
)

// creativeUploadDir 返回创作中心图片落盘目录（随数据根目录解耦）。
func creativeUploadDir() string {
	return filepath.Join(config.Load().UploadsDir, "creative")
}

/* ================================================================
 * 智能创作中心 · HTTP 处理层
 *
 * 功能：AI 助手对话（多轮）、角色设定（CRUD）、文案写作、抖音脚本、
 *      小红书文案、深度学习、洗稿、图片生成。
 *
 * 一律走统一 AI 客户端（services/ai），模型能力基于当前租户 AiPlatform，
 * 未配置时提示"请先配置 AI 平台"。全部按 TenantID 隔离，总后台不可见。
 * ================================================================ */

var creativeSeedOnce sync.Map // tenantID -> 是否已确保内置角色

// ---------- 角色设定 ----------

// builtinRoles 内置预制角色（≥6 个，提示词可控）
var builtinRoles = []models.CreativeRole{
	{Name: "专业文案", Category: "文案", Description: "擅长品牌/营销/公众号各类商业文案",
		SystemPrompt: "你是一位拥有15年经验的资深商业文案策划师。你擅长：1) 广告语与品牌口号；2) 公众号文章；3) 电商详情页文案；4) 促销活动文案。请始终站在目标受众角度，用有温度、有画面感、有行动号召的语言输出，结构清晰，分段明确，避免空洞套话。"},
	{Name: "抖音脚本师", Category: "脚本", Description: "抖音爆款短视频分镜脚本制作",
		SystemPrompt: "你是深谙抖音算法的短视频编导。你掌握爆款公式：3秒强钩子开头、情绪递进、反转/冲突收尾、结尾引导互动。输出分镜脚本时严格包含镜头号/景别/画面/台词/字幕/时长/音乐建议，节奏紧凑、口语化、有记忆点。"},
	{Name: "小红书运营", Category: "运营", Description: "小红书种草笔记文案创作",
		SystemPrompt: "你是小红书资深运营编辑，熟悉平台调性：真实、生活化、有情绪价值、自带种草力。标题要抓眼球（带数字/反差/共鸣），正文要分段清晰、适度emoji、埋搜索关键词，结尾要有互动引导，附话题标签与封面建议。"},
	{Name: "洗稿改写", Category: "改写", Description: "原文深度改写，去重保意",
		SystemPrompt: "你是文字改写专家。改写原则：忠实保留原文核心信息、数据、结论与观点，绝不增删关键事实；通过调整句式、替换词汇、重组段落顺序、转换表达角度降低文字重合度；语言自然流畅，无机翻感。"},
	{Name: "品牌策划", Category: "策划", Description: "品牌定位与营销全案策划",
		SystemPrompt: "你是4A广告公司的品牌策略总监。你精通品牌定位、人群洞察、核心卖点提炼、传播主题与整合营销节奏。输出结构：洞察→定位→卖点→传播主张→落地建议，逻辑严密、可执行、有差异化。"},
	{Name: "抖店带货", Category: "带货", Description: "抖音电商带货话术与直播脚本",
		SystemPrompt: "你是头部带货直播间的内容运营。你擅长：1) 产品卖点种草话术；2) 直播开场/逼单/憋单话术；3) 差异化优势讲解；4) 消除顾虑与促成下单。话术要口语化、有节奏、带紧迫感但不虚假夸大，符合带货合规尺度。"},
}

// ensureBuiltinRoles 租户角色表为空时自动写入内置角色
func ensureBuiltinRoles(tid uint) {
	if _, ok := creativeSeedOnce.Load(tid); ok {
		return
	}
	var cnt int64
	database.DB.Model(&models.CreativeRole{}).Where("tenant_id = ?", tid).Count(&cnt)
	if cnt == 0 {
		for i := range builtinRoles {
			r := builtinRoles[i]
			r.TenantID = tid
			r.Builtin = true
			database.DB.Create(&r)
		}
	}
	creativeSeedOnce.Store(tid, true)
}

// ListRoles 角色列表
func ListRoles(c *gin.Context) {
	tid := TenantID(c)
	ensureBuiltinRoles(tid)
	var list []models.CreativeRole
	database.DB.Where("tenant_id = ?", tid).Order("builtin DESC, id ASC").Find(&list)
	dyOK(c, list)
}

// CreateRole 新建角色
func CreateRole(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Name         string `json:"name"`
		Category     string `json:"category"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		dyErr(c, http.StatusOK, "请填写角色名称")
		return
	}
	if strings.TrimSpace(body.SystemPrompt) == "" {
		dyErr(c, http.StatusOK, "请填写角色提示词")
		return
	}
	r := models.CreativeRole{
		TenantID: tid, Name: strings.TrimSpace(body.Name), Category: body.Category,
		Description: body.Description, SystemPrompt: body.SystemPrompt,
	}
	if err := database.DB.Create(&r).Error; err != nil {
		dyErr(c, http.StatusOK, "保存失败: "+err.Error())
		return
	}
	dyOK(c, r)
}

// UpdateRole 更新角色（内置角色允许编辑提示词）
func UpdateRole(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var r models.CreativeRole
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&r).Error; err != nil {
		dyErr(c, http.StatusOK, "角色不存在")
		return
	}
	var body struct {
		Name         string `json:"name"`
		Category     string `json:"category"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Name) != "" {
		r.Name = strings.TrimSpace(body.Name)
	}
	if strings.TrimSpace(body.SystemPrompt) != "" {
		r.SystemPrompt = body.SystemPrompt
	}
	r.Category = body.Category
	r.Description = body.Description
	if err := database.DB.Save(&r).Error; err != nil {
		dyErr(c, http.StatusOK, "保存失败: "+err.Error())
		return
	}
	dyOK(c, r)
}

// DeleteRole 删除角色（内置角色不可删）
func DeleteRole(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var r models.CreativeRole
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&r).Error; err != nil {
		dyErr(c, http.StatusOK, "角色不存在")
		return
	}
	if r.Builtin {
		dyErr(c, http.StatusOK, "内置角色不可删除，可编辑或停用")
		return
	}
	// 已引用该角色的会话解除关联
	database.DB.Model(&models.ChatSession{}).Where("tenant_id = ? AND role_id = ?", tid, id).
		Updates(map[string]interface{}{"role_id": 0, "role_name": ""})
	database.DB.Delete(&r)
	dyOK(c, gin.H{"id": id})
}

// ---------- AI 助手对话（多轮会话） ----------

// ListSessions 会话列表
func ListSessions(c *gin.Context) {
	tid := TenantID(c)
	var list []models.ChatSession
	database.DB.Where("tenant_id = ?", tid).Order("updated_at DESC").Find(&list)
	dyOK(c, list)
}

// CreateSession 新建会话
func CreateSession(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Title  string `json:"title"`
		RoleID uint   `json:"role_id"`
	}
	if !jsonBody(c, &body) {
		return
	}
	roleName := ""
	if body.RoleID > 0 {
		var r models.CreativeRole
		if database.DB.Where("id = ? AND tenant_id = ?", body.RoleID, tid).First(&r).Error == nil {
			roleName = r.Name
		}
	}
	s := models.ChatSession{
		TenantID: tid, Title: body.Title, RoleID: body.RoleID, RoleName: roleName,
	}
	if strings.TrimSpace(s.Title) == "" {
		s.Title = "新会话"
	}
	if err := database.DB.Create(&s).Error; err != nil {
		dyErr(c, http.StatusOK, "创建失败: "+err.Error())
		return
	}
	dyOK(c, s)
}

// DeleteSession 删除会话（级联删除消息）
func DeleteSession(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.ChatSession{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusOK, "会话不存在")
		return
	}
	database.DB.Where("session_id = ? AND tenant_id = ?", id, tid).Delete(&models.ChatMessage{})
	dyOK(c, gin.H{"id": id})
}

// ListSessionMessages 会话消息历史
func ListSessionMessages(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var msgs []models.ChatMessage
	database.DB.Where("session_id = ? AND tenant_id = ?", id, tid).Order("id ASC").Find(&msgs)
	dyOK(c, msgs)
}

// ChatSend 发送消息并调用 AI 多轮回复
func ChatSend(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		SessionID uint   `json:"session_id"`
		Content   string `json:"content"`
	}
	if !jsonBody(c, &body) {
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		dyErr(c, http.StatusOK, "请输入消息内容")
		return
	}
	var s models.ChatSession
	if err := database.DB.Where("id = ? AND tenant_id = ?", body.SessionID, tid).First(&s).Error; err != nil {
		dyErr(c, http.StatusOK, "会话不存在")
		return
	}
	// 1) 持久化用户消息
	um := models.ChatMessage{TenantID: tid, SessionID: s.ID, Role: "user", Content: content}
	database.DB.Create(&um)

	// 2) 加载历史（含刚写入的用户消息），组装 AI 消息
	var history []models.ChatMessage
	database.DB.Where("session_id = ? AND tenant_id = ?", s.ID, tid).Order("id ASC").Find(&history)
	msgs := make([]ai.Message, 0, len(history))
	for _, h := range history {
		msgs = append(msgs, ai.Message{Role: h.Role, Content: h.Content})
	}
	// 3) 角色系统提示词
	sys := ""
	if s.RoleID > 0 {
		var r models.CreativeRole
		if database.DB.Where("id = ? AND tenant_id = ?", s.RoleID, tid).First(&r).Error == nil {
			sys = r.SystemPrompt
		}
	}
	ctx := context.Background()
	res, err := svc.Chat(ctx, tid, svc.ChatReq{System: sys, Msgs: msgs})
	if err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}
	// 4) 持久化助手回复
	am := models.ChatMessage{TenantID: tid, SessionID: s.ID, Role: "assistant", Content: res.Text}
	database.DB.Create(&am)

	// 5) 更新会话标题与消息数
	newTitle := s.Title
	if history[len(history)-1].Role == "user" && strings.HasPrefix(s.Title, "新会话") {
		runes := []rune(content)
		n := len(runes)
		if n > 14 {
			n = 14
		}
		newTitle = string(runes[:n])
	}
	database.DB.Model(&s).Updates(map[string]interface{}{
		"title": newTitle, "message_count": s.MessageCount + 2, "updated_at": time.Now(),
	})
	dyOK(c, gin.H{"reply": res.Text, "platform": res.Platform, "model": res.Model})
}

// ---------- 生成记录 / 素材库 ----------

// saveRecord 保存一条生成记录
func saveRecord(tid uint, kind, title, prompt, output, platform, model string) (*models.CreativeRecord, error) {
	rec := models.CreativeRecord{
		TenantID: tid, Kind: kind, Title: title, Prompt: prompt,
		Output: output, Platform: platform, Model: model, Status: "done",
	}
	err := database.DB.Create(&rec).Error
	return &rec, err
}

func recordTitle(s string) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) > 40 {
		runes = runes[:40]
	}
	return strings.ReplaceAll(string(runes), "\n", " ")
}

// ---------- 文案写作 ----------

// WriteCopy 文案写作（选角色 + 主题/要求）
func WriteCopy(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		RoleID       uint   `json:"role_id"`
		Theme        string `json:"theme"`
		Requirements string `json:"requirements"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Theme) == "" {
		dyErr(c, http.StatusOK, "请填写创作主题")
		return
	}
	sys := ""
	if body.RoleID > 0 {
		var r models.CreativeRole
		if database.DB.Where("id = ? AND tenant_id = ?", body.RoleID, tid).First(&r).Error == nil {
			sys = r.SystemPrompt
		}
	}
	res, err := svc.WriteCopy(context.Background(), tid, sys, body.Theme, body.Requirements)
	if err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}
	rec, _ := saveRecord(tid, "copy", "文案："+recordTitle(body.Theme), body.Theme, res.Text, res.Platform, res.Model)
	dyOK(c, gin.H{"text": res.Text, "platform": res.Platform, "model": res.Model, "record": rec})
}

// ---------- 抖音热门视频脚本 ----------

// DouyinScript 抖音分镜脚本
func DouyinScript(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Theme string `json:"theme"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Theme) == "" {
		dyErr(c, http.StatusOK, "请填写脚本主题")
		return
	}
	res, err := svc.DouyinScript(context.Background(), tid, body.Theme)
	if err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}
	rec, _ := saveRecord(tid, "script", "抖音脚本："+recordTitle(body.Theme), body.Theme, res.Text, res.Platform, res.Model)
	dyOK(c, gin.H{"text": res.Text, "shots": parseShots(res.Text), "platform": res.Platform, "model": res.Model, "record": rec})
}

// parseShots 解析 AI 返回的分镜 JSON 数组（不强制成功，兼容纯文本）
func parseShots(raw string) []map[string]interface{} {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") {
		return nil
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	return arr
}

// ---------- 小红书热门文案 ----------

// XhsCopy 小红书文案
func XhsCopy(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Theme string `json:"theme"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Theme) == "" {
		dyErr(c, http.StatusOK, "请填写笔记主题")
		return
	}
	res, err := svc.XhsCopy(context.Background(), tid, body.Theme)
	if err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}
	rec, _ := saveRecord(tid, "xhs", "小红书文案："+recordTitle(body.Theme), body.Theme, res.Text, res.Platform, res.Model)
	dyOK(c, gin.H{"text": res.Text, "platform": res.Platform, "model": res.Model, "record": rec})
}

// ---------- 深度学习 ----------

// LearnCopy 深度学习再创作
func LearnCopy(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Reference string `json:"reference"`
		Theme     string `json:"theme"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Reference) == "" {
		dyErr(c, http.StatusOK, "请提供参考文本或链接")
		return
	}
	if strings.TrimSpace(body.Theme) == "" {
		dyErr(c, http.StatusOK, "请填写创作主题")
		return
	}
	res, err := svc.LearnCopy(context.Background(), tid, body.Reference, body.Theme)
	if err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}
	rec, _ := saveRecord(tid, "learn", "深度学习："+recordTitle(body.Theme), body.Reference, res.Text, res.Platform, res.Model)
	dyOK(c, gin.H{"text": res.Text, "platform": res.Platform, "model": res.Model, "record": rec})
}

// ---------- 洗稿 ----------

// Xiegou 洗稿去重
func Xiegou(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Original string `json:"original"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Original) == "" {
		dyErr(c, http.StatusOK, "请输入原文内容")
		return
	}
	res, err := svc.Xiegou(context.Background(), tid, body.Original)
	if err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}
	rec, _ := saveRecord(tid, "xiegou", "洗稿改写："+recordTitle(body.Original), body.Original, res.Text, res.Platform, res.Model)
	dyOK(c, gin.H{"text": res.Text, "platform": res.Platform, "model": res.Model, "record": rec})
}

// ---------- 图片生成 ----------

// GenerateImage 文生图（OpenAI 兼容 /images/generations）
func GenerateImage(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Prompt string `json:"prompt"`
		Size   string `json:"size"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Prompt) == "" {
		dyErr(c, http.StatusOK, "请输入画面描述")
		return
	}
	res, b64, err := svc.ImageGen(context.Background(), tid, body.Prompt, body.Size, "b64_json")
	if err != nil {
		dyErr(c, http.StatusOK, err.Error())
		return
	}
	rec, err := saveRecord(tid, "image", "图片："+recordTitle(body.Prompt), body.Prompt, "", res.Platform, res.Model)
	if err == nil {
		// 保存 base64 到记录，同时落盘为文件供列表展示
		imageURL := saveImageFile(b64, tid, rec.ID)
		updates := map[string]interface{}{"image_b64": b64, "output": ""}
		if imageURL != "" {
			updates["image_url"] = imageURL
		}
		rec.ImageB64 = b64
		rec.ImageURL = imageURL
		database.DB.Model(rec).Updates(updates)
	}
	data := gin.H{
		"image_b64": b64, "image_url": rec.ImageURL,
		"platform": res.Platform, "model": res.Model,
		"record": rec,
	}
	if err != nil {
		data["warn"] = "记录保存失败: " + err.Error()
	}
	dyOK(c, data)
}

// saveImageFile 将 base64 图片落盘到 uploads/creative/
func saveImageFile(b64 string, tid, recordID uint) string {
	if b64 == "" {
		return ""
	}
	raw := b64
	if idx := strings.Index(raw, "base64,"); idx >= 0 {
		raw = raw[idx+len("base64,"):]
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return ""
	}
	dir := filepath.Join(creativeUploadDir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	name := fmt.Sprintf("t%02d_%d.png", tid, recordID)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ""
	}
	return "/uploads/creative/" + name
}

// ---------- 生成记录 / 素材库 ----------

// ListRecords 生成记录列表
func ListRecords(c *gin.Context) {
	tid := TenantID(c)
	kind := c.Query("kind")
	q := database.DB.Where("tenant_id = ?", tid)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var list []models.CreativeRecord
	q.Order("id DESC").Limit(100).Find(&list)
	dyOK(c, list)
}

// DeleteRecord 删除生成记录
func DeleteRecord(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.CreativeRecord{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusOK, "记录不存在")
		return
	}
	dyOK(c, gin.H{"id": id})
}

// ListMaterials 素材库列表
func ListMaterials(c *gin.Context) {
	tid := TenantID(c)
	var list []models.CreativeMaterial
	database.DB.Where("tenant_id = ?", tid).Order("id DESC").Limit(200).Find(&list)
	dyOK(c, list)
}

// SaveMaterial 保存素材
func SaveMaterial(c *gin.Context) {
	tid := TenantID(c)
	var body struct {
		Title   string `json:"title"`
		Kind    string `json:"kind"`
		Content string `json:"content"`
	}
	if !jsonBody(c, &body) {
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		dyErr(c, http.StatusOK, "素材内容为空")
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		body.Title = recordTitle(body.Content)
	}
	m := models.CreativeMaterial{
		TenantID: tid, Title: body.Title, Kind: body.Kind, Content: body.Content,
	}
	if err := database.DB.Create(&m).Error; err != nil {
		dyErr(c, http.StatusOK, "保存失败: "+err.Error())
		return
	}
	dyOK(c, m)
}

// DeleteMaterial 删除素材
func DeleteMaterial(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.CreativeMaterial{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusOK, "素材不存在")
		return
	}
	dyOK(c, gin.H{"id": id})
}
