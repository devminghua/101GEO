package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
	svcks "geo-tool/services/ks"
)

/* ================================================================
 * 快手获客 · HTTP 处理层（结构与抖音/小红书获客模块对齐）
 *
 * 合规硬约束（延续既定半自动模式）：
 *  - 后台不模拟登录任何快手账号、不自动群发；
 *  - 快手公开主页 SSR 数据极少、风控严格，同步以结构化估算为主，
 *    sourced_from=real/estimate 严格区分，严禁把估算冒充真实数据；
 *  - 打招呼动作为「AI 生成话术 → 前置校验 → 复制话术 → 人工确认后
 *    官方快手客户端发送 → 标记已打招呼」，本模块绝不代发消息。
 *
 * 所有查询均按当前登录租户 TenantID 隔离，总后台（tid=0）不可见。
 * ================================================================ */

// refreshKsAccountsState 批量刷新快手账号跨天清零与冷却恢复
func refreshKsAccountsState(accounts []models.KsAccount, f svcks.Frequency, now time.Time) {
	db := database.DB
	for i := range accounts {
		a := &accounts[i]
		if svcks.RefreshAccountState(a, f, now) {
			db.Model(a).Updates(map[string]interface{}{
				"today_used": a.TodayUsed, "today_date": a.TodayDate,
				"last_action_at": a.LastActionAt, "cooldown_until": a.CooldownUntil, "status": a.Status,
			})
		}
	}
}

// refreshOneKsAccountState 刷新单个快手账号并落库
func refreshOneKsAccountState(account *models.KsAccount, f svcks.Frequency, now time.Time) {
	if svcks.RefreshAccountState(account, f, now) {
		database.DB.Model(account).Updates(map[string]interface{}{
			"today_used": account.TodayUsed, "today_date": account.TodayDate,
			"last_action_at": account.LastActionAt, "cooldown_until": account.CooldownUntil, "status": account.Status,
		})
	}
}

// ---------- 账号管理 ----------

// KsListAccounts GET /api/ks/accounts
func KsListAccounts(c *gin.Context) {
	tid := TenantID(c)
	f := svcks.LoadFrequency(tid)
	now := biztime.Now()
	var list []models.KsAccount
	database.DB.Where("tenant_id = ?", tid).Order("id asc").Find(&list)
	refreshKsAccountsState(list, f, now)
	dyOK(c, list)
}

// KsCreateAccount POST /api/ks/accounts
func KsCreateAccount(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Nickname   string `json:"nickname"`
		Region     string `json:"region"`
		DailyLimit int    `json:"daily_limit"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Nickname == "" {
		dyErr(c, http.StatusBadRequest, "请填写账号昵称")
		return
	}
	limit := req.DailyLimit
	if limit <= 0 {
		limit = 30
	}
	acc := models.KsAccount{
		TenantID: tid, Nickname: req.Nickname, Region: strings.TrimSpace(req.Region),
		DailyLimit: limit, Status: "在线", TodayDate: biztime.Today(),
	}
	if err := database.DB.Create(&acc).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "创建失败："+err.Error())
		return
	}
	dyOK(c, acc)
}

// KsUpdateAccount PUT /api/ks/accounts/:id
func KsUpdateAccount(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var acc models.KsAccount
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&acc).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	var req struct {
		Nickname   string `json:"nickname"`
		Region     string `json:"region"`
		DailyLimit int    `json:"daily_limit"`
	}
	if !jsonBody(c, &req) {
		return
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Nickname); v != "" {
		updates["nickname"] = v
	}
	updates["region"] = strings.TrimSpace(req.Region)
	if req.DailyLimit > 0 {
		updates["daily_limit"] = req.DailyLimit
	}
	if len(updates) > 0 {
		database.DB.Model(&acc).Updates(updates)
	}
	dyOK(c, acc)
}

// KsDeleteAccount DELETE /api/ks/accounts/:id
func KsDeleteAccount(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.KsAccount{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	dyOK(c, gin.H{"deleted": id})
}

// ---------- 同行追踪 ----------

// syncKsPeerData 同步一个同行：快手公开页抓取失败率高，直接确定性估算
func syncKsPeerData(link string) *svcks.Profile {
	homeID := linkIDOf(link)
	return svcks.EstimateProfile(link, homeID)
}

// linkIDOf 从链接中提取稳定 ID 片段（无 ID 时用链接哈希）
func linkIDOf(link string) string {
	if i := strings.LastIndex(link, "/"); i >= 0 && i < len(link)-1 {
		seg := strings.Trim(link[i+1:], "?#")
		if seg != "" && len(seg) <= 128 {
			return seg
		}
	}
	return "ks-" + link
}

// KsListPeers GET /api/ks/peers
func KsListPeers(c *gin.Context) {
	tid := TenantID(c)
	var list []models.KsPeer
	database.DB.Where("tenant_id = ?", tid).Order("id desc").Find(&list)
	if list == nil {
		list = []models.KsPeer{}
	}
	dyOK(c, list)
}

// KsImportPeers POST /api/ks/peers/import
func KsImportPeers(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Links string `json:"links"`
	}
	if !jsonBody(c, &req) {
		return
	}
	links := extractKSLinks(req.Links)
	if len(links) == 0 {
		dyErr(c, http.StatusBadRequest, "未识别到有效的快手主页链接（支持 kuaishou.com/profile/ 或 www.kuaishou.com 短链）")
		return
	}
	// 短视频查询配额：抖音/小红书/快手共用池，每日 10 次（超限联系官方解锁）
	if !ShortVideoQuotaGuard(c) {
		return
	}
	db := database.DB
	created := []models.KsPeer{}
	skipped := []string{}
	for _, link := range links {
		var dup int64
		db.Model(&models.KsPeer{}).Where("tenant_id = ? AND link = ?", tid, link).Count(&dup)
		if dup > 0 {
			skipped = append(skipped, link+"（已在跟踪中）")
			continue
		}
		profile := syncKsPeerData(link)
		now := time.Now()
		peer := models.KsPeer{
			TenantID: tid, Link: link, HomeID: profile.HomeID,
			Nickname: profile.Nickname, FansCount: profile.FansCount,
			VideoCount: profile.VideoCount, SourcedFrom: profile.SourcedFrom,
			Status: "tracking", LastSyncAt: &now,
		}
		if peer.HomeID != "" {
			var d int64
			db.Model(&models.KsPeer{}).Where("tenant_id = ? AND home_id = ?", tid, peer.HomeID).Count(&d)
			if d > 0 {
				skipped = append(skipped, link+"（已在跟踪中）")
				continue
			}
		}
		if err := db.Create(&peer).Error; err != nil {
			skipped = append(skipped, link+"（入库失败）")
			continue
		}
		persistKsVideos(tid, peer.ID, peer.Nickname, svcks.EstimateVideos(profile, 10))
		created = append(created, peer)
	}
	dyOK(c, gin.H{
		"created": created, "skipped": skipped,
		"note": "快手公开主页数据极少，同步以结构化估算为主（sourced_from=estimate），估算数据不可作为真实运营依据",
	})
}

// extractKSLinks 从多行文本提取快手链接
func extractKSLinks(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ' ' || r == '，' || r == ',' || r == '；' || r == ';' }) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "kuaishou.com") {
			continue
		}
		if !seen[line] {
			seen[line] = true
			out = append(out, line)
		}
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

// KsRefreshPeer POST /api/ks/peers/:id/refresh
func KsRefreshPeer(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var peer models.KsPeer
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&peer).Error; err != nil {
		dyErr(c, http.StatusNotFound, "同行不存在")
		return
	}
	// 快手公开页重抓成功率低，刷新保持确定性估算（数据稳定不跳变）
	now := time.Now()
	database.DB.Model(&peer).Update("last_sync_at", &now)
	dyOK(c, gin.H{"refreshed": true, "last_sync_at": now})
}

// KsDeletePeer DELETE /api/ks/peers/:id
func KsDeletePeer(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.KsPeer{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "同行不存在")
		return
	}
	database.DB.Where("tenant_id = ? AND peer_id = ?", tid, id).Delete(&models.KsVideo{})
	dyOK(c, gin.H{"deleted": id})
}

func persistKsVideos(tid, peerID uint, peerName string, videos []svcks.Video) {
	db := database.DB
	for _, v := range videos {
		db.Create(&models.KsVideo{
			TenantID: tid, PeerID: peerID, PeerName: peerName,
			Title: v.Title, PlayCount: v.PlayCount, LikeCount: v.LikeCount,
			CommentCount: v.CommentCount, InteractionRate: v.InteractionRate,
			PublishTime: v.PublishTime, SourcedFrom: v.SourcedFrom,
		})
	}
}

// KsListVideos GET /api/ks/videos
func KsListVideos(c *gin.Context) {
	tid := TenantID(c)
	var list []models.KsVideo
	q := database.DB.Where("tenant_id = ?", tid)
	if pid := queryUint(c, "peer_id"); pid > 0 {
		q = q.Where("peer_id = ?", pid)
	}
	q.Order("publish_time desc").Limit(200).Find(&list)
	if list == nil {
		list = []models.KsVideo{}
	}
	dyOK(c, list)
}

// KsAnalysis GET /api/ks/analysis
func KsAnalysis(c *gin.Context) {
	tid := TenantID(c)
	var peers int64
	var videos int64
	database.DB.Model(&models.KsPeer{}).Where("tenant_id = ?", tid).Count(&peers)
	database.DB.Model(&models.KsVideo{}).Where("tenant_id = ?", tid).Count(&videos)
	var avgRate float64
	database.DB.Model(&models.KsVideo{}).Where("tenant_id = ?", tid).Select("COALESCE(AVG(interaction_rate),0)").Scan(&avgRate)
	var top []models.KsVideo
	database.DB.Where("tenant_id = ?", tid).Order("like_count desc").Limit(10).Find(&top)
	if top == nil {
		top = []models.KsVideo{}
	}
	dyOK(c, gin.H{
		"peer_count": peers, "video_count": videos,
		"avg_interaction_rate": avgRate, "top_videos": top,
	})
}

// ---------- 线索 ----------

// KsListLeads GET /api/ks/leads
func KsListLeads(c *gin.Context) {
	tid := TenantID(c)
	var list []models.KsLead
	q := database.DB.Where("tenant_id = ?", tid)
	if st := c.Query("state"); st != "" {
		q = q.Where("state = ?", st)
	}
	q.Order("id desc").Limit(500).Find(&list)
	if list == nil {
		list = []models.KsLead{}
	}
	dyOK(c, list)
}

// KsParseLeads POST /api/ks/leads/parse
func KsParseLeads(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		PeerID   uint   `json:"peer_id"`
		VideoID  uint   `json:"video_id"`
		Text     string `json:"text"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.PeerID == 0 && strings.TrimSpace(req.Text) == "" {
		dyErr(c, http.StatusBadRequest, "请选择来源同行或粘贴评论文本")
		return
	}
	// 短视频查询配额（每日 10 次）
	if !ShortVideoQuotaGuard(c) {
		return
	}
	db := database.DB
	var peer models.KsPeer
	if req.PeerID > 0 {
		db.Where("id = ? AND tenant_id = ?", req.PeerID, tid).First(&peer)
	}
	videoTitle := ""
	if req.VideoID > 0 {
		var v models.KsVideo
		if err := db.Where("id = ? AND tenant_id = ?", req.VideoID, tid).First(&v).Error; err == nil {
			videoTitle = v.Title
		}
	}
	estimates := svcks.EstimateLeads(peer.Nickname, videoTitle, 8)
	created := []models.KsLead{}
	for _, e := range estimates {
		lead := models.KsLead{
			TenantID: tid, Nickname: e.Nickname, PeerID: req.PeerID, PeerName: peer.Nickname,
			VideoID: req.VideoID, VideoTitle: videoTitle,
			Comment: e.Comment, CommentTime: &e.CommentTime, Tag: e.Tag,
			State: "待跟进", SourceType: "comment_parse", SourcedFrom: models.SourceEstimate,
		}
		if err := db.Create(&lead).Error; err == nil {
			created = append(created, lead)
		}
	}
	dyOK(c, gin.H{
		"created": created,
		"note":    "评论解析以估算为主，请人工核对快手客户端真实评论后跟进",
	})
}

// KsCreateLead POST /api/ks/leads
func KsCreateLead(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Nickname string `json:"nickname"`
		Comment  string `json:"comment"`
		Tag      string `json:"tag"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Nickname == "" {
		dyErr(c, http.StatusBadRequest, "请填写客户昵称")
		return
	}
	lead := models.KsLead{
		TenantID: tid, Nickname: req.Nickname, Comment: strings.TrimSpace(req.Comment),
		Tag: req.Tag, State: "待跟进", SourceType: "manual",
	}
	if err := database.DB.Create(&lead).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "创建失败")
		return
	}
	dyOK(c, lead)
}

// KsUpdateLead PUT /api/ks/leads/:id
func KsUpdateLead(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var lead models.KsLead
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&lead).Error; err != nil {
		dyErr(c, http.StatusNotFound, "客户不存在")
		return
	}
	var req struct {
		State       string `json:"state"`
		Tag         string `json:"tag"`
		IsValuable  *bool  `json:"is_valuable"`
		ValueRemark string `json:"value_remark"`
	}
	if !jsonBody(c, &req) {
		return
	}
	updates := map[string]interface{}{}
	if req.State != "" {
		updates["state"] = req.State
	}
	if req.Tag != "" {
		updates["tag"] = req.Tag
	}
	if req.IsValuable != nil {
		updates["is_valuable"] = *req.IsValuable
	}
	if req.ValueRemark != "" {
		updates["value_remark"] = req.ValueRemark
	}
	if len(updates) > 0 {
		database.DB.Model(&lead).Updates(updates)
	}
	dyOK(c, lead)
}

// KsDeleteLead DELETE /api/ks/leads/:id
func KsDeleteLead(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.KsLead{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "客户不存在")
		return
	}
	dyOK(c, gin.H{"deleted": id})
}

// ---------- 话术 ----------

// KsListSlogans GET /api/ks/slogans
func KsListSlogans(c *gin.Context) {
	tid := TenantID(c)
	var list []models.KsSlogan
	database.DB.Where("tenant_id = ?", tid).Order("id desc").Limit(200).Find(&list)
	if list == nil {
		list = []models.KsSlogan{}
	}
	dyOK(c, list)
}

// KsCreateSlogan POST /api/ks/slogans
func KsCreateSlogan(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Category string `json:"category"`
		Text     string `json:"text"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		dyErr(c, http.StatusBadRequest, "请填写话术内容")
		return
	}
	cat := strings.TrimSpace(req.Category)
	if cat == "" {
		cat = "开场白"
	}
	s := models.KsSlogan{TenantID: tid, Category: cat, Text: req.Text}
	if err := database.DB.Create(&s).Error; err != nil {
		dyErr(c, http.StatusInternalServerError, "保存失败")
		return
	}
	dyOK(c, s)
}

// KsUpdateSlogan PUT /api/ks/slogans/:id
func KsUpdateSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var s models.KsSlogan
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&s).Error; err != nil {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	var req struct {
		Category string `json:"category"`
		Text     string `json:"text"`
	}
	if !jsonBody(c, &req) {
		return
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Text); v != "" {
		updates["text"] = v
	}
	if v := strings.TrimSpace(req.Category); v != "" {
		updates["category"] = v
	}
	if len(updates) > 0 {
		database.DB.Model(&s).Updates(updates)
	}
	dyOK(c, s)
}

// KsDeleteSlogan DELETE /api/ks/slogans/:id
func KsDeleteSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.KsSlogan{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	dyOK(c, gin.H{"deleted": id})
}

// KsUseSlogan POST /api/ks/slogans/:id/use
func KsUseSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var s models.KsSlogan
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&s).Error; err != nil {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	database.DB.Model(&s).UpdateColumn("used_count", gorm.Expr("used_count + 1"))
	dyOK(c, gin.H{"used": true})
}

// KsGenerateSlogan POST /api/ks/slogans/ai-generate
func KsGenerateSlogan(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Nickname     string `json:"nickname"`
		Comment      string `json:"comment"`
		Tag          string `json:"tag"`
		AccountStyle string `json:"account_style"`
		Count        int    `json:"count"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Nickname == "" {
		dyErr(c, http.StatusBadRequest, "请提供客户昵称")
		return
	}
	res, err := svcks.GenerateSlogans(c.Request.Context(), tid, svcks.GenerateReq{
		Nickname: req.Nickname, Comment: req.Comment, Tag: req.Tag,
		AccountStyle: req.AccountStyle, Count: req.Count,
	})
	if err != nil {
		if strings.Contains(err.Error(), "请先配置 AI 平台") {
			dyErr(c, http.StatusBadRequest, "请先配置 AI 平台")
			return
		}
		dyErr(c, http.StatusInternalServerError, "生成失败："+err.Error())
		return
	}
	dyOK(c, res)
}

// ---------- 设置 / 前置校验 / 打招呼 / 日志 ----------

// KsGetSettings GET /api/ks/settings
func KsGetSettings(c *gin.Context) {
	tid := TenantID(c)
	dyOK(c, svcks.LoadFrequency(tid))
}

// KsSaveSettings POST /api/ks/settings
func KsSaveSettings(c *gin.Context) {
	tid := TenantID(c)
	var f svcks.Frequency
	if !jsonBody(c, &f) {
		return
	}
	if err := svcks.SaveFrequency(tid, f); err != nil {
		dyErr(c, http.StatusBadRequest, err.Error())
		return
	}
	dyOK(c, gin.H{"saved": true})
}

// KsPrecheck POST /api/ks/precheck
func KsPrecheck(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		AccountID uint `json:"account_id"`
	}
	if !jsonBody(c, &req) {
		return
	}
	f := svcks.LoadFrequency(tid)
	now := biztime.Now()
	var acc models.KsAccount
	if err := database.DB.Where("id = ? AND tenant_id = ?", req.AccountID, tid).First(&acc).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	refreshOneKsAccountState(&acc, f, now)
	// 活跃时段校验
	hhmm := now.Format("15:04")
	if hhmm < f.ActiveStart || hhmm > f.ActiveEnd {
		dyOK(c, gin.H{"ok": false, "reason": "当前不在活跃时段（" + f.ActiveStart + "~" + f.ActiveEnd + "）"})
		return
	}
	// 冷却校验
	if acc.CooldownUntil != nil && now.Before(*acc.CooldownUntil) {
		dyOK(c, gin.H{"ok": false, "reason": "账号冷却中，剩余 " + strconv.Itoa(int(acc.CooldownUntil.Sub(now).Minutes())+1) + " 分钟"})
		return
	}
	// 上限校验
	if acc.TodayUsed >= acc.DailyLimit {
		dyOK(c, gin.H{"ok": false, "reason": "今日打招呼已达上限（" + strconv.Itoa(acc.TodayUsed) + "/" + strconv.Itoa(acc.DailyLimit) + "）"})
		return
	}
	dyOK(c, gin.H{"ok": true, "remaining": acc.DailyLimit - acc.TodayUsed})
}

// KsGreet POST /api/ks/greet
func KsGreet(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		AccountID uint   `json:"account_id"`
		LeadID    uint   `json:"lead_id"`
		SloganID  uint   `json:"slogan_id"`
		Note      string `json:"note"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.AccountID == 0 || req.LeadID == 0 {
		dyErr(c, http.StatusBadRequest, "请选择打招呼账号与目标客户")
		return
	}
	f := svcks.LoadFrequency(tid)
	now := biztime.Now()
	db := database.DB
	var acc models.KsAccount
	if err := db.Where("id = ? AND tenant_id = ?", req.AccountID, tid).First(&acc).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	refreshOneKsAccountState(&acc, f, now)
	if acc.CooldownUntil != nil && now.Before(*acc.CooldownUntil) {
		dyErr(c, http.StatusConflict, "账号冷却中，无法打招呼")
		return
	}
	if acc.TodayUsed >= acc.DailyLimit {
		dyErr(c, http.StatusConflict, "今日打招呼已达上限")
		return
	}
	// 间隔校验
	if acc.LastActionAt != nil && now.Sub(*acc.LastActionAt) < time.Duration(f.IntervalMin)*time.Minute {
		dyErr(c, http.StatusConflict, "操作过于频繁，请间隔 "+strconv.Itoa(f.IntervalMin)+" 分钟")
		return
	}
	// 同日重复校验
	if f.RepeatOn {
		var dup int64
		todayStart := biztime.DayStart(now)
		db.Model(&models.KsActionLog{}).
			Where("tenant_id = ? AND account_id = ? AND action_type = ? AND target = ? AND created_at >= ?",
				tid, acc.ID, "greet", req.LeadID, todayStart).Count(&dup)
		if dup > 0 {
			dyErr(c, http.StatusConflict, "今日已向该客户打过招呼，不重复操作")
			return
		}
	}
	var lead models.KsLead
	if err := db.Where("id = ? AND tenant_id = ?", req.LeadID, tid).First(&lead).Error; err != nil {
		dyErr(c, http.StatusNotFound, "客户不存在")
		return
	}
	sloganText := ""
	if req.SloganID > 0 {
		var s models.KsSlogan
		if err := db.Where("id = ? AND tenant_id = ?", req.SloganID, tid).First(&s).Error; err == nil {
			sloganText = s.Text
			db.Model(&s).UpdateColumn("used_count", gorm.Expr("used_count + 1"))
		}
	}
	// 更新账号计数 + 冷却判定
	acc.TodayUsed++
	acc.LastActionAt = &now
	if acc.TodayUsed >= acc.DailyLimit && f.CoolOn {
		cool := svcks.CoolUntil(now, f)
		acc.CooldownUntil = &cool
		acc.Status = "冷却中"
	}
	db.Model(&acc).Updates(map[string]interface{}{
		"today_used": acc.TodayUsed, "last_action_at": acc.LastActionAt,
		"cooldown_until": acc.CooldownUntil, "status": acc.Status,
	})
	// 更新客户状态
	db.Model(&lead).Update("state", "已打招呼")
	// 动作日志
	db.Create(&models.KsActionLog{
		TenantID: tid, AccountID: acc.ID, AccountName: acc.Nickname,
		ActionType: "greet", Target: lead.Nickname,
		Result: "已标记打招呼（话术：" + strings.TrimSpace(sloganText) + "｜备注：" + strings.TrimSpace(req.Note) + "）",
	})
	dyOK(c, gin.H{"ok": true, "today_used": acc.TodayUsed, "slogan": sloganText})
}

// KsListActionLogs GET /api/ks/logs
func KsListActionLogs(c *gin.Context) {
	tid := TenantID(c)
	var list []models.KsActionLog
	database.DB.Where("tenant_id = ?", tid).Order("id desc").Limit(100).Find(&list)
	if list == nil {
		list = []models.KsActionLog{}
	}
	dyOK(c, list)
}
