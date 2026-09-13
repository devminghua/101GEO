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
	svcxhs "geo-tool/services/xhs"
	"geo-tool/services/biztime"
	"geo-tool/services/social"
)

/* ================================================================
 * 小红书获客 · HTTP 处理层（结构与抖音获客模块完全对齐）
 *
 * 合规硬约束（沿用既定半自动模式，全链路体现）：
 *  - 后台不模拟登录任何小红书账号、不自动群发；
 *  - 同步抓取仅面向小红书公开数据且尽力而为，失败降级为结构化估算，
 *    sourced_from=real/estimate 严格区分，严禁把估算冒充真实数据；
 *  - 打招呼动作为「AI 生成话术 → 前置校验 → 复制话术 → 人工确认后
 *    官方客户端发送 → 标记已打招呼」，本模块绝不代发消息；
 *  - 「客户回复有价值数据发给管理员」落实为站内通知（receiver_role=admin）。
 *
 * 所有查询均按当前登录租户 TenantID 隔离，总后台（tid=0）不可见。
 * ================================================================ */

// ---------- 工具辅助（xhs 专用，避免与 douyin 冲突） ----------

// refreshXhsAccountsState 批量刷新小红书账号跨天清零与冷却恢复，有变更则落库
func refreshXhsAccountsState(accounts []models.XhsAccount, f svcxhs.Frequency, now time.Time) {
	db := database.DB
	for i := range accounts {
		a := &accounts[i]
		if svcxhs.RefreshAccountState(a, f, now) {
			db.Model(a).Updates(map[string]interface{}{
				"today_used": a.TodayUsed, "today_date": a.TodayDate,
				"last_action_at": a.LastActionAt, "cooldown_until": a.CooldownUntil, "status": a.Status,
			})
		}
	}
}

// refreshOneXhsAccountState 刷新单个小红书账号并落库（变更时）
func refreshOneXhsAccountState(account *models.XhsAccount, f svcxhs.Frequency, now time.Time) {
	if svcxhs.RefreshAccountState(account, f, now) {
		database.DB.Model(account).Updates(map[string]interface{}{
			"today_used": account.TodayUsed, "today_date": account.TodayDate,
			"last_action_at": account.LastActionAt, "cooldown_until": account.CooldownUntil, "status": account.Status,
		})
	}
}

// syncXhsPeerData 同步一个同行：公开页尽力抓取，失败降级估算（sourced_from 严格区分）
func syncXhsPeerData(link string) (*svcxhs.Profile, []svcxhs.Note) {
	// 抓取链：第三方数据 API → 纯 HTTP → 估算（兜底）
	if social.Enabled() {
		if homeID, e := svcxhs.ResolveHomeID(link); e == nil {
			if u, e2 := social.XhsUserProfile(homeID); e2 == nil {
				profile := &svcxhs.Profile{
					Link: link, HomeID: homeID, Nickname: u.Nickname,
					FansCount: u.FansCount, NoteCount: u.NoteCount, SourcedFrom: models.SourceReal,
				}
				if notes, e3 := social.XhsUserNotes(homeID, 30); e3 == nil {
					out := make([]svcxhs.Note, 0, len(notes))
					for _, n := range notes {
						nn := svcxhs.Note{
							Title: n.Title, LikeCount: n.LikeCount, SaveCount: n.SaveCount,
							CommentCount: n.CommentCount, PublishTime: n.PublishTime, SourcedFrom: models.SourceReal,
						}
						nn.ComputeRate()
						out = append(out, nn)
					}
					if len(out) > 0 {
						return profile, out
					}
				}
				if profile.FansCount > 0 {
					return profile, nil
				}
			}
		}
	}

	profile, err := svcxhs.FetchProfile(link)
	if err != nil {
		homeID, _ := svcxhs.ResolveHomeID(link)
		profile = svcxhs.EstimateProfile(link, homeID)
	}
	var notes []svcxhs.Note
	if profile.SourcedFrom == models.SourceReal {
		notes, err = svcxhs.FetchNotesReal(link, 30)
	}
	if len(notes) == 0 {
		notes = svcxhs.EstimateNotes(profile, 10)
	}
	return profile, notes
}

// persistXhsNotes 覆盖式写入某同行的笔记数据（先删后插，保证刷新干净）
func persistXhsNotes(tid, peerID uint, peerName string, notes []svcxhs.Note) {
	db := database.DB
	db.Where("tenant_id = ? AND peer_id = ?", tid, peerID).Delete(&models.XhsNote{})
	for _, n := range notes {
		db.Create(&models.XhsNote{
			TenantID: tid, PeerID: peerID, PeerName: peerName,
			Title: n.Title, LikeCount: n.LikeCount, SaveCount: n.SaveCount,
			CommentCount: n.CommentCount, InteractionRate: n.InteractionRate,
			PublishTime: n.PublishTime, SourcedFrom: n.SourcedFrom,
		})
	}
}

// xhsLastGreetTime 该账号最近一次打招呼时间（用于间隔校验）
func xhsLastGreetTime(tid, accountID uint) *time.Time {
	var log models.XhsActionLog
	err := database.DB.Where("tenant_id = ? AND account_id = ? AND action_type = ?", tid, accountID, "greet").
		Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	t := log.CreatedAt
	return &t
}

// ---------- 1. 账号管理 XhsAccount ----------

// XhsListAccounts 账号列表（自动刷新跨天清零/冷却恢复）
func XhsListAccounts(c *gin.Context) {
	tid := TenantID(c)
	var list []models.XhsAccount
	database.DB.Where("tenant_id = ?", tid).Order("id asc").Find(&list)
	f := svcxhs.LoadFrequency(tid)
	refreshXhsAccountsState(list, f, biztime.Now())
	dyOK(c, list)
}

// XhsCreateAccount 新增账号
func XhsCreateAccount(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Nickname   string         `json:"nickname"`
		Region     string         `json:"region"`
		DailyLimit int            `json:"daily_limit"`
		Status     string         `json:"status"`
		PeerIDs    models.PeerIDs `json:"peer_ids"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Nickname == "" {
		dyErr(c, http.StatusBadRequest, "请填写账号昵称")
		return
	}
	if req.DailyLimit <= 0 {
		req.DailyLimit = 30
	}
	if req.DailyLimit > 1000 {
		req.DailyLimit = 1000
	}
	status := req.Status
	if status == "" {
		status = models.AccountOnline
	}
	acct := models.XhsAccount{
		TenantID: tid, Nickname: req.Nickname, Region: req.Region,
		DailyLimit: req.DailyLimit, Status: status,
		TodayDate: biztime.Today(), PeerIDs: req.PeerIDs,
	}
	database.DB.Create(&acct)
	dyOK(c, acct)
}

// XhsUpdateAccount 编辑账号（昵称/地区/上限/状态/关联同行/累计获客）
func XhsUpdateAccount(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var req struct {
		Nickname   *string        `json:"nickname"`
		Region     *string        `json:"region"`
		DailyLimit *int           `json:"daily_limit"`
		Status     *string        `json:"status"`
		TotalLeads *int           `json:"total_leads"`
		PeerIDs    models.PeerIDs `json:"peer_ids"`
	}
	if !jsonBody(c, &req) {
		return
	}
	db := database.DB
	var acct models.XhsAccount
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	f := svcxhs.LoadFrequency(tid)
	refreshOneXhsAccountState(&acct, f, biztime.Now())

	if req.Nickname != nil {
		n := strings.TrimSpace(*req.Nickname)
		if n != "" {
			acct.Nickname = n
		}
	}
	if req.Region != nil {
		acct.Region = strings.TrimSpace(*req.Region)
	}
	if req.DailyLimit != nil && *req.DailyLimit > 0 {
		if *req.DailyLimit > 1000 {
			acct.DailyLimit = 1000
		} else {
			acct.DailyLimit = *req.DailyLimit
		}
	}
	if req.Status != nil {
		acct.Status = *req.Status
	}
	if req.TotalLeads != nil && *req.TotalLeads >= 0 {
		acct.TotalLeads = *req.TotalLeads
	}
	if req.PeerIDs != nil {
		acct.PeerIDs = req.PeerIDs
	}
	db.Save(&acct)
	dyOK(c, acct)
}

// XhsDeleteAccount 删除账号
func XhsDeleteAccount(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.XhsAccount{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	dyOK(c, gin.H{"deleted": true})
}

// XhsRefreshAccount 手动刷新账号状态（跨天清零/冷却恢复计算）
func XhsRefreshAccount(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var acct models.XhsAccount
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	f := svcxhs.LoadFrequency(tid)
	refreshOneXhsAccountState(&acct, f, biztime.Now())
	dyOK(c, acct)
}

// ---------- 2. 同行追踪 XhsPeer ----------

// XhsListPeers 同行列表
func XhsListPeers(c *gin.Context) {
	tid := TenantID(c)
	var list []models.XhsPeer
	database.DB.Where("tenant_id = ?", tid).Order("id asc").Find(&list)
	dyOK(c, list)
}

// XhsImportPeers 粘贴批量主页链接加入跟踪（支持主页链接与 xhslink.com 短链）
func XhsImportPeers(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Links string `json:"links"`
	}
	if !jsonBody(c, &req) {
		return
	}
	links := svcxhs.ExtractLinks(req.Links)
	if len(links) == 0 {
		dyErr(c, http.StatusBadRequest, "未识别到有效的小红书主页链接（支持 xiaohongshu.com/user/profile/ 或 xhslink.com 短链）")
		return
	}
	// 短视频查询配额：抖音/小红书/快手共用池，每日 10 次（超限联系官方解锁）
	if !ShortVideoQuotaGuard(c) {
		return
	}
	db := database.DB
	created := []models.XhsPeer{}
	skipped := []string{}
	realCount, estCount := 0, 0
	for _, link := range links {
		var dup int64
		database.DB.Model(&models.XhsPeer{}).Where("tenant_id = ? AND link = ?", tid, link).Count(&dup)
		if dup > 0 {
			skipped = append(skipped, link+"（已在跟踪中）")
			continue
		}
		profile, notes := syncXhsPeerData(link)
		if profile.SourcedFrom == models.SourceReal {
			realCount++
		} else {
			estCount++
		}
		now := time.Now()
		peer := models.XhsPeer{
			TenantID: tid, Link: link, HomeID: profile.HomeID,
			Nickname: profile.Nickname, FansCount: profile.FansCount,
			NoteCount: profile.NoteCount, SourcedFrom: profile.SourcedFrom,
			Status: "tracking", LastSyncAt: &now,
		}
		if peer.HomeID != "" {
			var d int64
			db.Model(&models.XhsPeer{}).Where("tenant_id = ? AND home_id = ?", tid, peer.HomeID).Count(&d)
			if d > 0 {
				skipped = append(skipped, link+"（已在跟踪中）")
				continue
			}
		}
		if err := db.Create(&peer).Error; err != nil {
			skipped = append(skipped, link+"（入库失败："+err.Error()+"）")
			continue
		}
		persistXhsNotes(tid, peer.ID, peer.Nickname, notes)
		created = append(created, peer)
	}
	dyOK(c, gin.H{
		"created":        created,
		"skipped":        skipped,
		"real_count":     realCount,
		"estimate_count": estCount,
		"note":           "同步抓取仅面向小红书公开数据且尽力而为；失败时降级为结构化估算（sourced_from=estimate），估算数据不可作为真实运营依据",
	})
}

// XhsRefreshPeer 刷新某同行（重抓公开数据/估算 + 覆盖笔记）
func XhsRefreshPeer(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	var peer models.XhsPeer
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&peer).Error; err != nil {
		dyErr(c, http.StatusNotFound, "同行不存在")
		return
	}
	// 短视频查询配额（每日 10 次）
	if !ShortVideoQuotaGuard(c) {
		return
	}
	profile, notes := syncXhsPeerData(peer.Link)
	now := time.Now()
	peer.Nickname = profile.Nickname
	peer.HomeID = profile.HomeID
	peer.FansCount = profile.FansCount
	peer.NoteCount = profile.NoteCount
	peer.SourcedFrom = profile.SourcedFrom
	peer.LastSyncAt = &now
	db.Save(&peer)
	persistXhsNotes(tid, peer.ID, peer.Nickname, notes)
	dyOK(c, gin.H{
		"peer":         peer,
		"notes":        notes,
		"sourced_from": profile.SourcedFrom,
	})
}

// XhsDeletePeer 删除同行（级联删除其笔记数据；客户线索保留快照字段）
func XhsDeletePeer(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	var peer models.XhsPeer
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&peer).Error; err != nil {
		dyErr(c, http.StatusNotFound, "同行不存在")
		return
	}
	db.Where("tenant_id = ? AND peer_id = ?", tid, id).Delete(&models.XhsNote{})
	// 级联清理：该同行下的线索（避免删除后残留孤儿线索）
	db.Where("tenant_id = ? AND peer_id = ?", tid, id).Delete(&models.XhsLead{})
	db.Delete(&peer)
	dyOK(c, gin.H{"deleted": true})
}

// ---------- 3. 笔记数据 XhsNote ----------

// XhsListNotes 笔记列表（可按同行过滤，发布时间倒序）
func XhsListNotes(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if peerID := queryUint(c, "peer_id"); peerID > 0 {
		q = q.Where("peer_id = ?", peerID)
	}
	limit := 50
	if v := queryUint(c, "limit"); v > 0 && int(v) <= 200 {
		limit = int(v)
	}
	var list []models.XhsNote
	q.Order("publish_time desc").Limit(limit).Find(&list)
	dyOK(c, list)
}

// XhsAnalysis 数据分析聚合：KPI + Top 爆款笔记 + 近7天按星期发布分布
func XhsAnalysis(c *gin.Context) {
	tid := TenantID(c)
	peerID := queryUint(c, "peer_id")
	db := database.DB

	qp := db.Where("tenant_id = ?", tid)
	if peerID > 0 {
		qp = qp.Where("id = ?", peerID)
	}
	var peers []models.XhsPeer
	qp.Find(&peers)
	totalNotes := 0
	for _, p := range peers {
		totalNotes += int(p.NoteCount)
	}

	qv := db.Where("tenant_id = ?", tid)
	if peerID > 0 {
		qv = qv.Where("peer_id = ?", peerID)
	}
	var notes []models.XhsNote
	qv.Order("interaction_rate desc").Find(&notes)

	totalInteractions := 0
	var sumRate float64
	realN, estN := 0, 0
	for _, n := range notes {
		totalInteractions += int(n.LikeCount + n.SaveCount + n.CommentCount)
		sumRate += n.InteractionRate
		if n.SourcedFrom == models.SourceReal {
			realN++
		} else {
			estN++
		}
	}
	avgRate := 0.0
	if len(notes) > 0 {
		avgRate = sumRate / float64(len(notes))
	}

	top := notes
	if len(top) > 10 {
		top = top[:10]
	}

	dyOK(c, gin.H{
		"kpi": gin.H{
			"peers":              len(peers),
			"total_notes":        totalNotes,
			"total_interactions": totalInteractions,
			"avg_rate":           round2(avgRate),
			"real_notes":         realN,
			"estimate_notes":     estN,
		},
		"top_notes": top,
		"weekly":    buildXhsWeeklyDistribution(notes, biztime.Now()),
		"note":      "同步数据为尽力而为抓取，失败降级为结构化估算，估算数据（estimate）严禁冒充真实数据；互动率为折算参考值，请以小红书官方口径为准",
	})
}

// buildXhsWeeklyDistribution 近7天按星期发布分布（无近7天数据时回落全部笔记）
func buildXhsWeeklyDistribution(notes []models.XhsNote, now time.Time) []gin.H {
	cutoff := now.AddDate(0, 0, -6)
	recent := notes[:0:0]
	for _, n := range notes {
		if n.PublishTime.After(cutoff) {
			recent = append(recent, n)
		}
	}
	if len(recent) == 0 {
		recent = notes
	}
	counts := make([]int, 7)
	var interactions [7]int64
	for _, n := range recent {
		idx := (int(n.PublishTime.Weekday()) + 6) % 7 // 周一=0
		counts[idx]++
		interactions[idx] += n.LikeCount + n.SaveCount + n.CommentCount
	}
	out := make([]gin.H, 0, 7)
	for i := 0; i < 7; i++ {
		out = append(out, gin.H{
			"weekday": weekdayNames[i], "count": counts[i],
			"interactions": interactions[i],
		})
	}
	return out
}

// ---------- 4. 客户获取 XhsLead ----------

// XhsListLeads 线索列表（可按状态/同行/关键词/价值过滤）
func XhsListLeads(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("state"); v != "" {
		q = q.Where("state = ?", v)
	}
	if peerID := queryUint(c, "peer_id"); peerID > 0 {
		q = q.Where("peer_id = ?", peerID)
	}
	if v := c.Query("keyword"); v != "" {
		q = q.Where("nickname LIKE ? OR comment LIKE ?", "%"+v+"%", "%"+v+"%")
	}
	if v := c.Query("valuable"); v == "1" {
		q = q.Where("is_valuable = ?", true)
	}
	var list []models.XhsLead
	q.Order("id desc").Limit(300).Find(&list)
	dyOK(c, list)
}

// XhsParseLeads 从笔记评论解析线索（评论需登录态，公开页无法可靠抓取时降级为结构化估算并明确标记）
func XhsParseLeads(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		NoteID uint `json:"note_id"`
		PeerID uint `json:"peer_id"`
		Count  int  `json:"count"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Count <= 0 || req.Count > 20 {
		req.Count = 8
	}
	db := database.DB

	peerName, noteTitle := "", ""
	if req.PeerID > 0 {
		var peer models.XhsPeer
		if err := db.Where("id = ? AND tenant_id = ?", req.PeerID, tid).First(&peer).Error; err == nil {
			peerName = peer.Nickname
		}
	}
	if req.NoteID > 0 {
		var note models.XhsNote
		if err := db.Where("id = ? AND tenant_id = ?", req.NoteID, tid).First(&note).Error; err == nil {
			noteTitle = note.Title
			if req.PeerID == 0 {
				req.PeerID = note.PeerID
				peerName = note.PeerName
			}
		}
	}
	if req.PeerID == 0 {
		dyErr(c, http.StatusBadRequest, "请先选择来源同行（或对应的笔记）")
		return
	}

	// 评论内容需登录态，公开页无法可靠抓取：按合规约定尽力而为，降级为结构化估算并标记 estimate
	est := svcxhs.EstimateLeads(peerName, noteTitle, req.Count)
	created, skipped := 0, 0
	for _, l := range est {
		var cnt int64
		db.Model(&models.XhsLead{}).Where("tenant_id = ? AND peer_id = ? AND nickname = ?", tid, req.PeerID, l.Nickname).Count(&cnt)
		if cnt > 0 {
			skipped++
			continue
		}
		t := l.CommentTime
		db.Create(&models.XhsLead{
			TenantID: tid, Nickname: l.Nickname, PeerID: req.PeerID, PeerName: peerName,
			NoteID: req.NoteID, NoteTitle: noteTitle, Comment: l.Comment,
			CommentTime: &t, Tag: l.Tag, State: models.LeadStatePending,
			SourceType: models.LeadSourceCommentParse, SourcedFrom: models.SourceEstimate,
		})
		created++
	}
	dyOK(c, gin.H{
		"created":      created,
		"skipped":      skipped,
		"sourced_from": models.SourceEstimate,
		"note":         "评论数据需登录态，当前为结构化估算线索，仅作获客参考；请以官方客户端实际留言为准",
	})
}

// XhsCreateLead 手动添加客户线索
func XhsCreateLead(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Nickname    string     `json:"nickname"`
		PeerID      uint       `json:"peer_id"`
		NoteID      uint       `json:"note_id"`
		Comment     string     `json:"comment"`
		CommentTime *time.Time `json:"comment_time"`
		Tag         string     `json:"tag"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Nickname == "" {
		dyErr(c, http.StatusBadRequest, "请填写客户昵称")
		return
	}
	db := database.DB
	peerName, noteTitle := "", ""
	if req.PeerID > 0 {
		var peer models.XhsPeer
		if err := db.Where("id = ? AND tenant_id = ?", req.PeerID, tid).First(&peer).Error; err == nil {
			peerName = peer.Nickname
		}
	}
	if req.NoteID > 0 {
		var note models.XhsNote
		if err := db.Where("id = ? AND tenant_id = ?", req.NoteID, tid).First(&note).Error; err == nil {
			noteTitle = note.Title
			if req.PeerID == 0 {
				req.PeerID = note.PeerID
				peerName = note.PeerName
			}
		}
	}
	if req.Tag == "" {
		req.Tag = "其他"
	}
	lead := models.XhsLead{
		TenantID: tid, Nickname: req.Nickname, PeerID: req.PeerID, PeerName: peerName,
		NoteID: req.NoteID, NoteTitle: noteTitle, Comment: req.Comment,
		CommentTime: req.CommentTime, Tag: req.Tag, State: models.LeadStatePending,
		SourceType: models.LeadSourceManual, SourcedFrom: models.SourceEstimate,
	}
	db.Create(&lead)
	dyOK(c, lead)
}

// XhsUpdateLead 标记客户状态/标签/价值与意向备注（状态机：待跟进->已打招呼->已回复）
// 客户"已回复 + 有价值"时自动生成站内通知（receiver_role=admin），即「发给管理员」。
func XhsUpdateLead(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var req struct {
		State       *string `json:"state"`
		Tag         *string `json:"tag"`
		IsValuable  *bool   `json:"is_valuable"`
		ValueRemark *string `json:"value_remark"`
	}
	if !jsonBody(c, &req) {
		return
	}
	db := database.DB
	var lead models.XhsLead
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&lead).Error; err != nil {
		dyErr(c, http.StatusNotFound, "线索不存在")
		return
	}
	changed := false
	notifyValuable := false
	if req.State != nil {
		target := strings.TrimSpace(*req.State)
		if cur, okCur := leadStateOrder[lead.State]; okCur {
			if tgt, okTgt := leadStateOrder[target]; !okTgt {
				dyErr(c, http.StatusBadRequest, "非法的客户状态")
				return
			} else if tgt < cur && target != models.LeadStatePending {
				dyErr(c, http.StatusBadRequest, "不能回退状态（当前："+lead.State+"，目标："+target+"）")
				return
			}
		}
		lead.State = target
		if target == models.LeadStateReplied && lead.RepliedAt == nil {
			now := time.Now()
			lead.RepliedAt = &now
		}
		changed = true
	}
	if req.Tag != nil {
		lead.Tag = strings.TrimSpace(*req.Tag)
		changed = true
	}
	if req.IsValuable != nil {
		if *req.IsValuable && !lead.IsValuable {
			// 首次标记有价值
			lead.IsValuable = true
			changed = true
			// 客户已回复 + 有价值 -> 自动通知管理员
			if lead.State == models.LeadStateReplied {
				notifyValuable = true
			}
		} else if !*req.IsValuable && lead.IsValuable {
			lead.IsValuable = false
			changed = true
		}
	} else if lead.IsValuable && lead.State == models.LeadStateReplied && !leadNotified(tid, lead.Nickname) {
		// 兼容：状态已更新为已回复且原本就标记有价值
		notifyValuable = true
	}
	if req.ValueRemark != nil {
		lead.ValueRemark = strings.TrimSpace(*req.ValueRemark)
		changed = true
	}
	if changed {
		db.Save(&lead)
	}
	if notifyValuable {
		createXhsNotification(tid, lead, db)
	}
	dyOK(c, lead)
}

// leadNotified 简单查询：是否已存在指向该客户的 admin 通知（避免重复上报）
func leadNotified(tid uint, nickname string) bool {
	var cnt int64
	database.DB.Model(&models.XhsNotification{}).
		Where("tenant_id = ? AND receiver_role = ? AND content LIKE ?", tid, "admin", "%客户「"+nickname+"」%").
		Count(&cnt)
	return cnt > 0
}

// createXhsNotification 生成"价值客户"站内通知（发给管理员）
func createXhsNotification(tid uint, lead models.XhsLead, db *gorm.DB) {
	remark := strings.TrimSpace(lead.ValueRemark)
	if remark == "" {
		remark = "（未填写意向备注）"
	}
	title := "价值客户提醒"
	content := "客户「" + lead.Nickname + "」（来源同行：" + firstXhsStr(lead.PeerName) + "，标签：" + firstXhsStr(lead.Tag) + "）已回复，被标记为有价值：意向备注：" + remark
	db.Create(&models.XhsNotification{
		TenantID: tid, ReceiverRole: "admin",
		Title: title, Content: content, IsRead: false,
	})
	// 同步写一条动作日志，便于运营追踪
	db.Create(&models.XhsActionLog{
		TenantID: tid, AccountName: "系统", ActionType: "mark",
		Target: lead.Nickname, Result: "有价值客户回复，" + title,
	})
}

func firstXhsStr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "--"
	}
	return s
}

// XhsDeleteLead 删除线索
func XhsDeleteLead(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.XhsLead{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "线索不存在")
		return
	}
	dyOK(c, gin.H{"deleted": true})
}

// ---------- 5. 话术库 XhsSlogan ----------

// XhsListSlogans 话术列表（使用次数倒序）
func XhsListSlogans(c *gin.Context) {
	tid := TenantID(c)
	var list []models.XhsSlogan
	database.DB.Where("tenant_id = ?", tid).Order("used_count desc, id asc").Find(&list)
	dyOK(c, list)
}

// XhsCreateSlogan 新增话术
func XhsCreateSlogan(c *gin.Context) {
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
	if req.Category == "" {
		req.Category = models.SloganCategoryOpen
	}
	s := models.XhsSlogan{TenantID: tid, Category: req.Category, Text: req.Text}
	database.DB.Create(&s)
	dyOK(c, s)
}

// XhsUpdateSlogan 编辑话术
func XhsUpdateSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var req struct {
		Category *string `json:"category"`
		Text     *string `json:"text"`
	}
	if !jsonBody(c, &req) {
		return
	}
	db := database.DB
	var s models.XhsSlogan
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&s).Error; err != nil {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	if req.Category != nil {
		s.Category = strings.TrimSpace(*req.Category)
	}
	if req.Text != nil {
		t := strings.TrimSpace(*req.Text)
		if t != "" {
			s.Text = t
		}
	}
	db.Save(&s)
	dyOK(c, s)
}

// XhsDeleteSlogan 删除话术
func XhsDeleteSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.XhsSlogan{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	dyOK(c, gin.H{"deleted": true})
}

// XhsUseSlogan 话术使用计数 +1
func XhsUseSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	var s models.XhsSlogan
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&s).Error; err != nil {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	s.UsedCount++
	db.Save(&s)
	dyOK(c, s)
}

// ---------- 6. AI 话术生成 ----------

// XhsGenerateSlogan AI 自动生成开场话术（复用统一 AI 客户端；无平台时报「请先配置AI平台」）
func XhsGenerateSlogan(c *gin.Context) {
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
	res, err := svcxhs.GenerateSlogans(c.Request.Context(), tid, svcxhs.GenerateReq{
		Nickname: req.Nickname, Comment: req.Comment, Tag: req.Tag,
		AccountStyle: req.AccountStyle, Count: req.Count,
	})
	if err != nil {
		if strings.Contains(err.Error(), "请先配置 AI 平台") {
			dyErr(c, http.StatusBadRequest, "请先配置 AI 平台")
			return
		}
		dyErr(c, http.StatusInternalServerError, "AI 生成失败："+err.Error())
		return
	}
	// 写动作日志
	database.DB.Create(&models.XhsActionLog{
		TenantID: tid, AccountName: "AI助手", ActionType: "ai_gen",
		Target: req.Nickname,
		Result: "AI 生成 " + strconv.Itoa(len(res.Slogans)) + " 条话术（" + res.Platform + "/" + res.Model + "）",
	})
	dyOK(c, gin.H{
		"slogans": res.Slogans, "platform": res.Platform, "model": res.Model,
		"sourced_from": "ai",
		"notice":       "AI 仅生成话术建议，发送仍由人工在官方小红书客户端完成（本系统不代发消息）",
	})
}

// ---------- 7. 频率与安全设置 + 动作前置校验 ----------

// XhsGetSettings 读取频率与安全设置
func XhsGetSettings(c *gin.Context) {
	tid := TenantID(c)
	f := svcxhs.LoadFrequency(tid)
	dyOK(c, f)
}

// XhsSaveSettings 保存频率与安全设置（保存后立即对动作前置校验生效）
func XhsSaveSettings(c *gin.Context) {
	tid := TenantID(c)
	var f svcxhs.Frequency
	if !jsonBody(c, &f) {
		return
	}
	if f.DailyLimit <= 0 || f.DailyLimit > 1000 {
		f.DailyLimit = 30
	}
	if f.IntervalMin < 0 || f.IntervalMin > 1440 {
		f.IntervalMin = 5
	}
	if f.ActiveStart == "" {
		f.ActiveStart = "09:00"
	}
	if f.ActiveEnd == "" {
		f.ActiveEnd = "22:00"
	}
	if err := svcxhs.SaveFrequency(tid, f); err != nil {
		dyErr(c, http.StatusInternalServerError, "保存失败："+err.Error())
		return
	}
	dyOK(c, gin.H{"saved": true, "settings": f})
}

// XhsPrecheck 动作前置校验：活跃时段/当日上限/间隔/冷却/同日去重
func XhsPrecheck(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		AccountID uint `json:"account_id"`
		LeadID    uint `json:"lead_id"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.AccountID == 0 {
		dyErr(c, http.StatusBadRequest, "请选择账号")
		return
	}
	db := database.DB
	var acct models.XhsAccount
	if err := db.Where("id = ? AND tenant_id = ?", req.AccountID, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	f := svcxhs.LoadFrequency(tid)
	// 活跃时段/冷却判定是业务时间，必须用北京时间（容器时区为 UTC）
	now := biztime.Now()
	pre := svcxhs.Precheck(&acct, f, now, xhsLastGreetTime(tid, acct.ID))
	data := gin.H{
		"can": pre.Can, "reason": pre.Reason,
		"active_window": pre.ActiveWindow, "daily_ok": pre.DailyOK,
		"interval_ok": pre.IntervalOK, "not_cooling": pre.NotCooling,
		"today_used": pre.TodayUsed, "daily_limit": pre.DailyLimit,
		"cooldown_until": pre.CooldownUntil,
	}
	// 同日陌生人去重：同一客户已打招呼则直接提示
	if req.LeadID > 0 && f.RepeatOn {
		var lead models.XhsLead
		if err := db.Where("id = ? AND tenant_id = ?", req.LeadID, tid).First(&lead).Error; err == nil {
			if lead.State == models.LeadStateGreeted || lead.State == models.LeadStateReplied {
				data["can"] = false
				data["reason"] = "该客户已打招呼/已回复（同日陌生人去重开启）"
			}
		}
	}
	dyOK(c, data)
}

// XhsGreet 打招呼动作确认：回写今日计数 + 更新客户状态 + 话术计数 + 写动作日志
// 合规：流程为「AI/话术库生成话术 → 前置校验 → 复制话术 → 人工在官方客户端发送 → 确认标记」，
// 实际发送由人工完成，本系统绝不代发。
func XhsGreet(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		AccountID uint `json:"account_id"`
		LeadID    uint `json:"lead_id"`
		SloganID  uint `json:"slogan_id"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.AccountID == 0 || req.LeadID == 0 {
		dyErr(c, http.StatusBadRequest, "请选择账号与客户")
		return
	}
	db := database.DB
	var acct models.XhsAccount
	if err := db.Where("id = ? AND tenant_id = ?", req.AccountID, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	var lead models.XhsLead
	if err := db.Where("id = ? AND tenant_id = ?", req.LeadID, tid).First(&lead).Error; err != nil {
		dyErr(c, http.StatusNotFound, "客户线索不存在")
		return
	}

	f := svcxhs.LoadFrequency(tid)
	// 打招呼时刻、活跃时段与「同日去重」都是业务时间，必须用北京时间
	now := biztime.Now()

	// 同日陌生人去重
	if f.RepeatOn && (lead.State == models.LeadStateGreeted || lead.State == models.LeadStateReplied) {
		dyErr(c, http.StatusOK, "该客户已打招呼/已回复（同日陌生人去重开启），无需重复操作")
		return
	}

	// 服务端二次前置校验（权威判定）
	pre := svcxhs.Precheck(&acct, f, now, xhsLastGreetTime(tid, acct.ID))
	if !pre.Can {
		dyErr(c, http.StatusOK, pre.Reason)
		return
	}

	// 回写今日计数
	acct.TodayUsed++
	acct.TodayDate = now.Format("2006-01-02")
	acct.LastActionAt = &now
	acct.TotalLeads++
	acct.Status = models.AccountOnline
	// 达上限自动冷却
	if f.CoolOn && acct.TodayUsed >= acct.DailyLimit {
		svcxhs.NextCooldown(&acct, f)
	}
	db.Save(&acct)

	// 更新客户状态
	lead.State = models.LeadStateGreeted
	db.Save(&lead)

	// 话术计数
	sloganText := ""
	if req.SloganID > 0 {
		var slogan models.XhsSlogan
		if err := db.Where("id = ? AND tenant_id = ?", req.SloganID, tid).First(&slogan).Error; err == nil {
			slogan.UsedCount++
			db.Save(&slogan)
			sloganText = slogan.Text
		}
	}

	// 写动作日志
	logEntry := models.XhsActionLog{
		TenantID: tid, AccountID: acct.ID, AccountName: acct.Nickname,
		ActionType: "greet", Target: lead.Nickname,
		Result: "已标记已打招呼，请在官方小红书客户端完成发送",
	}
	db.Create(&logEntry)

	var fresh models.XhsAccount
	_ = db.Where("id = ?", acct.ID).First(&fresh).Error

	dyOK(c, gin.H{
		"account":     fresh,
		"lead":        lead,
		"log":         logEntry,
		"slogan_text": sloganText,
		"notice":      "已复制话术并标记已打招呼，请人工在官方小红书客户端完成发送（本系统不代发消息）",
	})
}

// ---------- 8. 价值统计 + 通知管理员 ----------

// XhsValueStats 价值客户统计：价值客户总数 / 意向分布（按标签） / 按同行 / 按时间（近7天）
func XhsValueStats(c *gin.Context) {
	tid := TenantID(c)
	db := database.DB

	var leads []models.XhsLead
	db.Where("tenant_id = ? AND is_valuable = ?", tid, true).Find(&leads)

	total := len(leads)

	byTag := map[string]int{}
	byPeer := map[string]int{}
	byDate := map[string]int{}
	for _, l := range leads {
		tag := strings.TrimSpace(l.Tag)
		if tag == "" {
			tag = "其他"
		}
		byTag[tag]++

		peer := strings.TrimSpace(l.PeerName)
		if peer == "" {
			peer = "（未关联同行）"
		}
		byPeer[peer]++

		t := l.RepliedAt
		if t == nil {
			t = &l.CreatedAt
		}
		byDate[t.Format("2006-01-02")]++
	}

	// 近7天日期序列（含今天，缺省补 0）
	now := biztime.Now()
	week := make([]gin.H, 0, 7)
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		key := d.Format("2006-01-02")
		week = append(week, gin.H{"date": key, "count": byDate[key]})
	}

	dyOK(c, gin.H{
		"total":   total,
		"by_tag":  mapToSortedSlice(byTag),
		"by_peer": mapToSortedSlice(byPeer),
		"by_date": week,
		"note":    "统计口径：已被标记为「有价值」且客户已回复的线索（is_valuable=true）",
	})
}

// mapToSortedSlice 把统计 map 转成 gin.H 列表（按数值倒序）
func mapToSortedSlice(m map[string]int) []gin.H {
	items := make([]gin.H, 0, len(m))
	for k, v := range m {
		items = append(items, gin.H{"name": k, "count": v})
	}
	// 简单插入排序（数量小，足够）
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j]["count"].(int) > items[j-1]["count"].(int); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	return items
}

// XhsListNotifications 管理员站内通知列表（可选未读过滤）
func XhsListNotifications(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ? AND receiver_role = ?", tid, "admin")
	if v := c.Query("unread"); v == "1" {
		q = q.Where("is_read = ?", false)
	}
	limit := 50
	if v := queryUint(c, "limit"); v > 0 && int(v) <= 200 {
		limit = int(v)
	}
	var list []models.XhsNotification
	q.Order("id desc").Limit(limit).Find(&list)
	var unread int64
	database.DB.Model(&models.XhsNotification{}).Where("tenant_id = ? AND receiver_role = ? AND is_read = ?", tid, "admin", false).Count(&unread)
	dyOK(c, gin.H{"list": list, "unread": unread, "total": len(list)})
}

// XhsMarkNotificationRead 标记通知已读
func XhsMarkNotificationRead(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	var n models.XhsNotification
	if err := db.Where("id = ? AND tenant_id = ? AND receiver_role = ?", id, tid, "admin").First(&n).Error; err != nil {
		dyErr(c, http.StatusNotFound, "通知不存在")
		return
	}
	if !n.IsRead {
		n.IsRead = true
		db.Save(&n)
	}
	dyOK(c, gin.H{"read": true})
}

// ---------- 9. 动作日志 XhsActionLog ----------

// XhsListActionLogs 动作日志分页查询
func XhsListActionLogs(c *gin.Context) {
	tid := TenantID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("account_id"); v != "" {
		q = q.Where("account_id = ?", v)
	}
	if v := c.Query("action_type"); v != "" {
		q = q.Where("action_type = ?", v)
	}
	var total int64
	q.Model(&models.XhsActionLog{}).Count(&total)
	var list []models.XhsActionLog
	q.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	dyOK(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}
