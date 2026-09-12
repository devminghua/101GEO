package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	svcdouyin "geo-tool/services/douyin"
	"geo-tool/services/biztime"
	"geo-tool/services/social"
)

/* ================================================================
 * 抖音获客 · HTTP 处理层
 *
 * 合规硬约束（沿用既定半自动模式，全链路体现）：
 *  - 后台不模拟登录任何抖音账号、不自动群发；
 *  - 同步抓取仅面向公开数据且尽力而为，失败降级为结构化估算，
 *    sourced_from=real/estimate 严格区分，严禁把估算冒充真实数据；
 *  - 发送动作需人工在官方客户端执行（本模块只做"复制话术 + 标记已打招呼"）。
 *
 * 所有查询均按当前登录租户 TenantID 隔离，总后台（tid=0）不可见。
 * ================================================================ */

// ---------- 工具辅助 ----------

func dyOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data})
}

func dyErr(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": 1, "msg": msg})
}

func parseID(c *gin.Context) uint {
	v, _ := strconv.Atoi(c.Param("id"))
	return uint(v)
}

func queryUint(c *gin.Context, key string) uint {
	v, _ := strconv.Atoi(c.Query(key))
	return uint(v)
}

// refreshAccountsState 批量刷新账号跨天清零与冷却恢复，有变更则落库
func refreshAccountsState(accounts []models.DyAccount, f svcdouyin.Frequency, now time.Time) {
	db := database.DB
	for i := range accounts {
		a := &accounts[i]
		if svcdouyin.RefreshAccountState(a, f, now) {
			db.Model(a).Updates(map[string]interface{}{
				"today_used": a.TodayUsed, "today_date": a.TodayDate,
				"last_action_at": a.LastActionAt, "cooldown_until": a.CooldownUntil, "status": a.Status,
			})
		}
	}
}

// refreshOneAccountState 刷新单个账号并落库（变更时）
func refreshOneAccountState(account *models.DyAccount, f svcdouyin.Frequency, now time.Time) {
	if svcdouyin.RefreshAccountState(account, f, now) {
		database.DB.Model(account).Updates(map[string]interface{}{
			"today_used": account.TodayUsed, "today_date": account.TodayDate,
			"last_action_at": account.LastActionAt, "cooldown_until": account.CooldownUntil, "status": account.Status,
		})
	}
}

// syncPeerData 同步一个同行：公开页尽力抓取，失败降级估算（sourced_from 严格区分）
func syncPeerData(link string) (*svcdouyin.Profile, []svcdouyin.Video) {
	// 抓取链：第三方数据 API → 无头浏览器 → 估算（兜底）
	profile, videos := syncPeerViaAPI(link)
	if profile != nil && len(videos) > 0 {
		return profile, videos
	}

	profile, err := svcdouyin.FetchProfile(link)
	if err != nil {
		if secUID, e2 := svcdouyin.ExtractSecUID(link); e2 == nil {
			if bp, e3 := svcdouyin.FetchProfileBrowser(link, secUID); e3 == nil {
				profile = bp
			}
		}
	}
	if profile == nil || profile.SourcedFrom != models.SourceReal {
		profile = svcdouyin.EstimateProfile(link)
	}
	var vids []svcdouyin.Video
	if profile.SourcedFrom == models.SourceReal {
		vids, err = svcdouyin.FetchVideosReal(link, 30)
		if len(vids) == 0 {
			if bv, e2 := svcdouyin.FetchVideosBrowser(link, 30); e2 == nil && len(bv) > 0 {
				vids = bv
			}
		}
	}
	if len(vids) == 0 {
		vids = svcdouyin.EstimateVideos(profile, 12)
	}
	return profile, vids
}

// syncPeerViaAPI 优先用第三方数据 API 抓取（Just One API）。
func syncPeerViaAPI(link string) (*svcdouyin.Profile, []svcdouyin.Video) {
	if !social.Enabled() {
		return nil, nil
	}
	secUID, err := svcdouyin.ExtractSecUID(link)
	if err != nil {
		return nil, nil
	}
	u, err := social.DouyinUserDetail(secUID)
	if err != nil {
		return nil, nil
	}
	profile := &svcdouyin.Profile{
		Link: link, SecUID: secUID, Nickname: u.Nickname,
		FansCount: u.FansCount, VideoCount: u.VideoCount, SourcedFrom: models.SourceReal,
	}
	list, err := social.DouyinVideoList(secUID, 30)
	if err != nil {
		return profile, nil // 用户信息拿到了，视频列表失败也不降级到估算（至少粉丝真实）
	}
	videos := make([]svcdouyin.Video, 0, len(list))
	for _, v := range list {
		vv := svcdouyin.Video{
			Title:        v.Title,
			PlayCount:    v.PlayCount,
			LikeCount:    v.LikeCount,
			CommentCount: v.CommentCount,
			PublishTime:  v.PublishTime,
			SourcedFrom:  models.SourceReal,
		}
		vv.ComputeRate()
		videos = append(videos, vv)
	}
	return profile, videos
}

// persistVideos 覆盖式写入某同行的视频数据（先删后插，保证刷新干净）
func persistVideos(tid, peerID uint, peerName string, videos []svcdouyin.Video) {
	db := database.DB
	db.Where("tenant_id = ? AND peer_id = ?", tid, peerID).Delete(&models.DyVideo{})
	for _, v := range videos {
		db.Create(&models.DyVideo{
			TenantID: tid, PeerID: peerID, PeerName: peerName,
			Title: v.Title, PlayCount: v.PlayCount, LikeCount: v.LikeCount,
			CommentCount: v.CommentCount, InteractionRate: v.InteractionRate,
			PublishTime: v.PublishTime, SourcedFrom: v.SourcedFrom,
		})
	}
}

// ---------- 1. 账号管理 DyAccount ----------

// DouyinListAccounts 账号列表（自动刷新跨天清零/冷却恢复）
func DouyinListAccounts(c *gin.Context) {
	tid := TenantID(c)
	var list []models.DyAccount
	database.DB.Where("tenant_id = ?", tid).Order("id asc").Find(&list)
	f := svcdouyin.LoadFrequency(tid)
	refreshAccountsState(list, f, biztime.Now())
	dyOK(c, list)
}

// DouyinCreateAccount 新增账号
func DouyinCreateAccount(c *gin.Context) {
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
	acct := models.DyAccount{
		TenantID: tid, Nickname: req.Nickname, Region: req.Region,
		DailyLimit: req.DailyLimit, Status: status,
		TodayDate: biztime.Today(), PeerIDs: req.PeerIDs,
	}
	database.DB.Create(&acct)
	dyOK(c, acct)
}

// DouyinUpdateAccount 编辑账号（昵称/地区/上限/状态/关联同行/累计获客）
func DouyinUpdateAccount(c *gin.Context) {
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
	var acct models.DyAccount
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	f := svcdouyin.LoadFrequency(tid)
	refreshOneAccountState(&acct, f, biztime.Now())

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

// DouyinDeleteAccount 删除账号
func DouyinDeleteAccount(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.DyAccount{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	// 级联清理该账号的动作日志，避免「删掉的数据」残留
	database.DB.Where("tenant_id = ? AND account_id = ?", tid, id).Delete(&models.DyActionLog{})
	dyOK(c, gin.H{"deleted": true})
}

// DouyinRefreshAccount 手动刷新账号状态（跨天清零/冷却恢复计算）
func DouyinRefreshAccount(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var acct models.DyAccount
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	f := svcdouyin.LoadFrequency(tid)
	refreshOneAccountState(&acct, f, biztime.Now())
	dyOK(c, acct)
}

// ---------- 2. 同行追踪 DyPeer ----------

// DouyinListPeers 同行列表
func DouyinListPeers(c *gin.Context) {
	tid := TenantID(c)
	var list []models.DyPeer
	database.DB.Where("tenant_id = ?", tid).Order("id asc").Find(&list)
	dyOK(c, list)
}

// DouyinImportPeers 粘贴批量主页链接加入跟踪（每行一个或混合文本）
func DouyinImportPeers(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Links string `json:"links"`
	}
	if !jsonBody(c, &req) {
		return
	}
	links := svcdouyin.ExtractLinks(req.Links)
	if len(links) == 0 {
		dyErr(c, http.StatusBadRequest, "未识别到有效的抖音主页链接（支持 douyin.com/user/ 或 v.douyin.com 短链）")
		return
	}
	// 每日查询配额：跨百度/抖音/小红书统一计数
	if !QuotaGuard(c) {
		return
	}
	db := database.DB
	created := []models.DyPeer{}
	skipped := []string{}
	realCount, estCount := 0, 0
	for _, link := range links {
		// 按原始链接去重（同一链接重复粘贴跳过）
		var dup int64
		database.DB.Model(&models.DyPeer{}).Where("tenant_id = ? AND link = ?", tid, link).Count(&dup)
		if dup > 0 {
			skipped = append(skipped, link+"（已在跟踪中）")
			continue
		}
		profile, videos := syncPeerData(link)
		if profile.SourcedFrom == models.SourceReal {
			realCount++
		} else {
			estCount++
		}
		now := time.Now()
		peer := models.DyPeer{
			TenantID: tid, Link: link, HomeID: profile.SecUID,
			Nickname: profile.Nickname, FansCount: profile.FansCount,
			VideoCount: profile.VideoCount, SourcedFrom: profile.SourcedFrom,
			Status: "tracking", LastSyncAt: &now,
		}
		// 若同租户已有相同 HomeID 的记录（重复导入），跳过
		if peer.HomeID != "" {
			var d int64
			db.Model(&models.DyPeer{}).Where("tenant_id = ? AND home_id = ?", tid, peer.HomeID).Count(&d)
			if d > 0 {
				skipped = append(skipped, link+"（已在跟踪中）")
				continue
			}
		}
		if err := db.Create(&peer).Error; err != nil {
			skipped = append(skipped, link+"（入库失败："+err.Error()+"）")
			continue
		}
		persistVideos(tid, peer.ID, peer.Nickname, videos)
		created = append(created, peer)
	}
	dyOK(c, gin.H{
		"created":        created,
		"skipped":        skipped,
		"real_count":     realCount,
		"estimate_count": estCount,
		"note":           "同步抓取仅面向抖音公开数据且尽力而为；失败时降级为结构化估算（sourced_from=estimate），请勿将估算数据当作真实数据使用",
	})
}

// DouyinRefreshPeer 刷新某同行（重抓公开数据/估算 + 覆盖视频）
func DouyinRefreshPeer(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	var peer models.DyPeer
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&peer).Error; err != nil {
		dyErr(c, http.StatusNotFound, "同行不存在")
		return
	}
	// 每日查询配额
	if !QuotaGuard(c) {
		return
	}
	profile, videos := syncPeerData(peer.Link)
	now := time.Now()
	peer.Nickname = profile.Nickname
	peer.HomeID = profile.SecUID
	peer.FansCount = profile.FansCount
	peer.VideoCount = profile.VideoCount
	peer.SourcedFrom = profile.SourcedFrom
	peer.LastSyncAt = &now
	db.Save(&peer)
	persistVideos(tid, peer.ID, peer.Nickname, videos)
	dyOK(c, gin.H{
		"peer":         peer,
		"videos":       videos,
		"sourced_from": profile.SourcedFrom,
	})
}

// DouyinRefreshAllPeers 批量刷新全部同行（催更新：逐个重抓公开数据/估算 + 覆盖视频）
func DouyinRefreshAllPeers(c *gin.Context) {
	tid := TenantID(c)
	db := database.DB
	var peers []models.DyPeer
	db.Where("tenant_id = ?", tid).Find(&peers)
	if len(peers) == 0 {
		dyOK(c, gin.H{"updated": 0})
		return
	}
	// 每日查询配额（批量更新算 1 次）
	if !QuotaGuard(c) {
		return
	}
	updated := 0
	for _, p := range peers {
		profile, videos := syncPeerData(p.Link)
		now := time.Now()
		p.Nickname = profile.Nickname
		p.HomeID = profile.SecUID
		p.FansCount = profile.FansCount
		p.VideoCount = profile.VideoCount
		p.SourcedFrom = profile.SourcedFrom
		p.LastSyncAt = &now
		db.Save(&p)
		persistVideos(tid, p.ID, p.Nickname, videos)
		updated++
	}
	dyOK(c, gin.H{"updated": updated})
}

// DouyinDeletePeer 删除同行（级联删除其视频数据；客户线索保留快照字段）
func DouyinDeletePeer(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	var peer models.DyPeer
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&peer).Error; err != nil {
		dyErr(c, http.StatusNotFound, "同行不存在")
		return
	}
	db.Where("tenant_id = ? AND peer_id = ?", tid, id).Delete(&models.DyVideo{})
	// 级联清理：该同行下的线索（否则删除后线索成孤儿数据，仍显示在列表）
	db.Where("tenant_id = ? AND peer_id = ?", tid, id).Delete(&models.DyLead{})
	db.Delete(&peer)
	dyOK(c, gin.H{"deleted": true})
}

// ---------- 3. 视频数据 DyVideo ----------

// DouyinListVideos 视频列表（可按同行过滤，发布时间倒序）
func DouyinListVideos(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if peerID := queryUint(c, "peer_id"); peerID > 0 {
		q = q.Where("peer_id = ?", peerID)
	}
	limit := 50
	if v := queryUint(c, "limit"); v > 0 && int(v) <= 200 {
		limit = int(v)
	}
	var list []models.DyVideo
	q.Order("publish_time desc").Limit(limit).Find(&list)
	dyOK(c, list)
}

var weekdayNames = []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

// DouyinAnalysis 数据分析聚合：KPI + Top 爆款视频 + 近7天按星期发布分布
func DouyinAnalysis(c *gin.Context) {
	tid := TenantID(c)
	peerID := queryUint(c, "peer_id")
	db := database.DB

	qp := db.Where("tenant_id = ?", tid)
	if peerID > 0 {
		qp = qp.Where("id = ?", peerID)
	}
	var peers []models.DyPeer
	qp.Find(&peers)
	totalVideos := 0
	for _, p := range peers {
		totalVideos += int(p.VideoCount)
	}

	qv := db.Where("tenant_id = ?", tid)
	if peerID > 0 {
		qv = qv.Where("peer_id = ?", peerID)
	}
	var videos []models.DyVideo
	qv.Order("interaction_rate desc").Find(&videos)

	totalComments := 0
	var sumRate float64
	realN, estN := 0, 0
	for _, v := range videos {
		totalComments += int(v.CommentCount)
		sumRate += v.InteractionRate
		if v.SourcedFrom == models.SourceReal {
			realN++
		} else {
			estN++
		}
	}
	avgRate := 0.0
	if len(videos) > 0 {
		avgRate = sumRate / float64(len(videos))
	}

	top := videos
	if len(top) > 10 {
		top = top[:10]
	}

	dyOK(c, gin.H{
		"kpi": gin.H{
			"peers":           len(peers),
			"total_videos":    totalVideos,
			"total_comments":  totalComments,
			"avg_rate":        round2(avgRate),
			"real_videos":     realN,
			"estimate_videos": estN,
		},
		"top_videos": top,
			"weekly":     buildWeeklyDistribution(videos, biztime.Now()),
		"note":       "同步数据为尽力而为抓取，失败降级为结构化估算；实时数据请以抖音官方口径为准（estimate 数据严禁冒充真实数据）",
	})
}

// buildWeeklyDistribution 近7天按星期发布分布（无近7天数据时回落全部视频）
func buildWeeklyDistribution(videos []models.DyVideo, now time.Time) []gin.H {
	cutoff := now.AddDate(0, 0, -6)
	recent := videos[:0:0]
	for _, v := range videos {
		if v.PublishTime.After(cutoff) {
			recent = append(recent, v)
		}
	}
	if len(recent) == 0 {
		recent = videos
	}
	counts := make([]int, 7)
	var comments, plays [7]int64
	for _, v := range recent {
		idx := (int(v.PublishTime.Weekday()) + 6) % 7 // 周一=0
		counts[idx]++
		comments[idx] += v.CommentCount
		plays[idx] += v.PlayCount
	}
	out := make([]gin.H, 0, 7)
	for i := 0; i < 7; i++ {
		out = append(out, gin.H{
			"weekday": weekdayNames[i], "count": counts[i],
			"comments": comments[i], "plays": plays[i],
		})
	}
	return out
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// ---------- 4. 客户获取 DyLead ----------

// DouyinListLeads 线索列表（可按状态/同行过滤）
func DouyinListLeads(c *gin.Context) {
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
	var list []models.DyLead
	q.Order("id desc").Limit(300).Find(&list)
	dyOK(c, list)
}

// DouyinParseLeads 从视频评论解析线索（尽力而为；评论需登录态，无法公开抓取时降级为结构化估算并明确标记）
func DouyinParseLeads(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		VideoID uint `json:"video_id"`
		PeerID  uint `json:"peer_id"`
		Count   int  `json:"count"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Count <= 0 || req.Count > 20 {
		req.Count = 8
	}
	db := database.DB

	peerName, videoTitle := "", ""
	if req.PeerID > 0 {
		var peer models.DyPeer
		if err := db.Where("id = ? AND tenant_id = ?", req.PeerID, tid).First(&peer).Error; err == nil {
			peerName = peer.Nickname
		}
	}
	if req.VideoID > 0 {
		var video models.DyVideo
		if err := db.Where("id = ? AND tenant_id = ?", req.VideoID, tid).First(&video).Error; err == nil {
			videoTitle = video.Title
			if req.PeerID == 0 {
				req.PeerID = video.PeerID
				peerName = video.PeerName
			}
		}
	}
	if req.PeerID == 0 {
		dyErr(c, http.StatusBadRequest, "请先选择来源同行（或对应的视频）")
		return
	}

	// 评论内容需登录态，公开页无法可靠抓取：按合规约定尽力而为，降级为结构化估算并标记 estimate
	est := svcdouyin.EstimateLeads(peerName, videoTitle, req.Count)
	created, skipped := 0, 0
	for _, l := range est {
		var cnt int64
		db.Model(&models.DyLead{}).Where("tenant_id = ? AND peer_id = ? AND nickname = ?", tid, req.PeerID, l.Nickname).Count(&cnt)
		if cnt > 0 {
			skipped++
			continue
		}
		t := l.CommentTime
		db.Create(&models.DyLead{
			TenantID: tid, Nickname: l.Nickname, PeerID: req.PeerID, PeerName: peerName,
			VideoID: req.VideoID, VideoTitle: videoTitle, Comment: l.Comment,
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

// DouyinCreateLead 手动添加客户线索
func DouyinCreateLead(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Nickname    string     `json:"nickname"`
		PeerID      uint       `json:"peer_id"`
		VideoID     uint       `json:"video_id"`
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
	peerName, videoTitle := "", ""
	if req.PeerID > 0 {
		var peer models.DyPeer
		if err := db.Where("id = ? AND tenant_id = ?", req.PeerID, tid).First(&peer).Error; err == nil {
			peerName = peer.Nickname
		}
	}
	if req.VideoID > 0 {
		var video models.DyVideo
		if err := db.Where("id = ? AND tenant_id = ?", req.VideoID, tid).First(&video).Error; err == nil {
			videoTitle = video.Title
			if req.PeerID == 0 {
				req.PeerID = video.PeerID
				peerName = video.PeerName
			}
		}
	}
	if req.Tag == "" {
		req.Tag = "其他"
	}
	lead := models.DyLead{
		TenantID: tid, Nickname: req.Nickname, PeerID: req.PeerID, PeerName: peerName,
		VideoID: req.VideoID, VideoTitle: videoTitle, Comment: req.Comment,
		CommentTime: req.CommentTime, Tag: req.Tag, State: models.LeadStatePending,
		SourceType: models.LeadSourceManual, SourcedFrom: models.SourceEstimate,
	}
	db.Create(&lead)
	dyOK(c, lead)
}

var leadStateOrder = map[string]int{models.LeadStatePending: 1, models.LeadStateGreeted: 2, models.LeadStateReplied: 3}

// DouyinUpdateLead 标记客户状态（状态机：待跟进 -> 已打招呼 -> 已回复；允许"已回复"重开为"待跟进"）
func DouyinUpdateLead(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	var req struct {
		State *string `json:"state"`
		Tag   *string `json:"tag"`
	}
	if !jsonBody(c, &req) {
		return
	}
	db := database.DB
	var lead models.DyLead
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&lead).Error; err != nil {
		dyErr(c, http.StatusNotFound, "线索不存在")
		return
	}
	changed := false
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
		changed = true
	}
	if req.Tag != nil {
		lead.Tag = strings.TrimSpace(*req.Tag)
		changed = true
	}
	if changed {
		db.Save(&lead)
	}
	dyOK(c, lead)
}

// DouyinDeleteLead 删除线索
func DouyinDeleteLead(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.DyLead{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "线索不存在")
		return
	}
	dyOK(c, gin.H{"deleted": true})
}

// ---------- 5. 话术库 DySlogan ----------

// DouyinListSlogans 话术列表（使用次数倒序）
func DouyinListSlogans(c *gin.Context) {
	tid := TenantID(c)
	var list []models.DySlogan
	database.DB.Where("tenant_id = ?", tid).Order("used_count desc, id asc").Find(&list)
	dyOK(c, list)
}

// DouyinCreateSlogan 新增话术
func DouyinCreateSlogan(c *gin.Context) {
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
	s := models.DySlogan{TenantID: tid, Category: req.Category, Text: req.Text}
	database.DB.Create(&s)
	dyOK(c, s)
}

// DouyinUpdateSlogan 编辑话术
func DouyinUpdateSlogan(c *gin.Context) {
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
	var s models.DySlogan
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

// DouyinDeleteSlogan 删除话术
func DouyinDeleteSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	res := database.DB.Where("id = ? AND tenant_id = ?", id, tid).Delete(&models.DySlogan{})
	if res.RowsAffected == 0 {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	dyOK(c, gin.H{"deleted": true})
}

// DouyinUseSlogan 话术使用计数 +1
func DouyinUseSlogan(c *gin.Context) {
	tid := TenantID(c)
	id := parseID(c)
	db := database.DB
	var s models.DySlogan
	if err := db.Where("id = ? AND tenant_id = ?", id, tid).First(&s).Error; err != nil {
		dyErr(c, http.StatusNotFound, "话术不存在")
		return
	}
	s.UsedCount++
	db.Save(&s)
	dyOK(c, s)
}

// DouyinGenerateSlogan AI 生成抖音开场话术（对标小红书）：POST /api/douyin/slogans/ai-generate
func DouyinGenerateSlogan(c *gin.Context) {
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
	res, err := svcdouyin.GenerateSlogans(c.Request.Context(), tid, svcdouyin.GenerateReq{
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
	database.DB.Create(&models.DyActionLog{
		TenantID: tid, AccountName: "AI助手", ActionType: "ai_gen",
		Target: req.Nickname,
		Result: "AI 生成 " + strconv.Itoa(len(res.Slogans)) + " 条话术（" + res.Platform + "/" + res.Model + "）",
	})
	dyOK(c, gin.H{
		"slogans": res.Slogans, "platform": res.Platform, "model": res.Model,
		"sourced_from": "ai",
		"notice":       "AI 仅生成话术建议，发送仍由人工在官方抖音客户端完成（本系统不代发消息）",
	})
}

// ---------- 6. 频率与安全设置 + 动作前置校验 ----------

// DouyinGetSettings 读取频率与安全设置
func DouyinGetSettings(c *gin.Context) {
	tid := TenantID(c)
	f := svcdouyin.LoadFrequency(tid)
	dyOK(c, f)
}

// DouyinSaveSettings 保存频率与安全设置（保存后立即对动作前置校验生效）
func DouyinSaveSettings(c *gin.Context) {
	tid := TenantID(c)
	var f svcdouyin.Frequency
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
	if err := svcdouyin.SaveFrequency(tid, f); err != nil {
		dyErr(c, http.StatusInternalServerError, "保存失败："+err.Error())
		return
	}
	dyOK(c, gin.H{"saved": true, "settings": f})
}

// lastGreetTime 该账号最近一次打招呼时间（用于间隔校验）
func lastGreetTime(tid, accountID uint) *time.Time {
	var log models.DyActionLog
	err := database.DB.Where("tenant_id = ? AND account_id = ? AND action_type = ?", tid, accountID, "greet").
		Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	t := log.CreatedAt
	return &t
}

// DouyinPrecheck 动作前置校验：活跃时段/当日上限/间隔/冷却/同日去重
func DouyinPrecheck(c *gin.Context) {
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
	var acct models.DyAccount
	if err := db.Where("id = ? AND tenant_id = ?", req.AccountID, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	f := svcdouyin.LoadFrequency(tid)
	// 活跃时段/冷却判定是业务时间，必须用北京时间（容器时区为 UTC）
	now := biztime.Now()
	pre := svcdouyin.Precheck(&acct, f, now, lastGreetTime(tid, acct.ID))
	data := gin.H{
		"can": pre.Can, "reason": pre.Reason,
		"active_window": pre.ActiveWindow, "daily_ok": pre.DailyOK,
		"interval_ok": pre.IntervalOK, "not_cooling": pre.NotCooling,
		"today_used": pre.TodayUsed, "daily_limit": pre.DailyLimit,
		"cooldown_until": pre.CooldownUntil,
	}
	// 同日陌生人去重：同一客户已打招呼则直接提示
	if req.LeadID > 0 && f.RepeatOn {
		var lead models.DyLead
		if err := db.Where("id = ? AND tenant_id = ?", req.LeadID, tid).First(&lead).Error; err == nil {
			if lead.State == models.LeadStateGreeted || lead.State == models.LeadStateReplied {
				data["can"] = false
				data["reason"] = "该客户已打招呼/已回复（同日陌生人去重开启）"
			}
		}
	}
	dyOK(c, data)
}

// DouyinGreet 打招呼动作确认：回写今日计数 + 更新客户状态 + 话术计数 + 写动作日志
// 合规：仅"复制话术 + 标记已打招呼"，实际发送由人工在官方抖音客户端完成。
func DouyinGreet(c *gin.Context) {
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
	var acct models.DyAccount
	if err := db.Where("id = ? AND tenant_id = ?", req.AccountID, tid).First(&acct).Error; err != nil {
		dyErr(c, http.StatusNotFound, "账号不存在")
		return
	}
	var lead models.DyLead
	if err := db.Where("id = ? AND tenant_id = ?", req.LeadID, tid).First(&lead).Error; err != nil {
		dyErr(c, http.StatusNotFound, "客户线索不存在")
		return
	}

	f := svcdouyin.LoadFrequency(tid)
	// 打招呼时刻、活跃时段与「同日去重」都是业务时间，必须用北京时间
	now := biztime.Now()

	// 同日陌生人去重
	if f.RepeatOn && (lead.State == models.LeadStateGreeted || lead.State == models.LeadStateReplied) {
		dyErr(c, http.StatusOK, "该客户已打招呼/已回复（同日陌生人去重开启），无需重复操作")
		return
	}

	// 服务端二次前置校验（权威判定）
	pre := svcdouyin.Precheck(&acct, f, now, lastGreetTime(tid, acct.ID))
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
		svcdouyin.NextCooldown(&acct, f)
	}
	db.Save(&acct)

	// 更新客户状态
	lead.State = models.LeadStateGreeted
	db.Save(&lead)

	// 话术计数
	sloganText := ""
	if req.SloganID > 0 {
		var slogan models.DySlogan
		if err := db.Where("id = ? AND tenant_id = ?", req.SloganID, tid).First(&slogan).Error; err == nil {
			slogan.UsedCount++
			db.Save(&slogan)
			sloganText = slogan.Text
		}
	}

	// 写动作日志
	logEntry := models.DyActionLog{
		TenantID: tid, AccountID: acct.ID, AccountName: acct.Nickname,
		ActionType: "greet", Target: lead.Nickname,
		Result: "已标记已打招呼，请在官方抖音客户端完成发送",
	}
	db.Create(&logEntry)

	var fresh models.DyAccount
	_ = db.Where("id = ?", acct.ID).First(&fresh).Error

	dyOK(c, gin.H{
		"account":     fresh,
		"lead":        lead,
		"log":         logEntry,
		"slogan_text": sloganText,
		"notice":      "已复制话术并标记已打招呼，请人工在官方抖音客户端完成发送（本系统不代发消息）",
	})
}

// ---------- 7. 动作日志 DyActionLog ----------

// DouyinListActionLogs 动作日志分页查询
func DouyinListActionLogs(c *gin.Context) {
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
	q.Model(&models.DyActionLog{}).Count(&total)
	var list []models.DyActionLog
	q.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	dyOK(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}
