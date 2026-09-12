package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/auth"
	"geo-tool/services/biztime"
	"geo-tool/services/crypto"
)

// 微信小程序：登录/绑定/效果总览聚合。
// 客户端只读展示，鉴权复用现有 JWT（tenant 隔离）。

// wxCode2Session 用 wx.login 的 code 换 openid。
// 未配置 GEO_WX_APPID/GEO_WX_SECRET 时进入测试模式：用 code 生成确定性 openid，便于本地联调。
func wxCode2Session(code string) (string, error) {
	if code == "" {
		return "", errors.New("缺少登录 code")
	}
	cfg := config.Load()
	if cfg.WxAppID == "" || cfg.WxSecret == "" {
		// 测试模式：未配置小程序 AppID
		return "wx-test-" + code, nil
	}
	u := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(cfg.WxAppID), url.QueryEscape(cfg.WxSecret), url.QueryEscape(code))
	resp, err := http.Get(u)
	if err != nil {
		return "", errors.New("微信登录服务不可用")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var out struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.OpenID == "" {
		return "", errors.New("微信登录失败：" + out.ErrMsg)
	}
	return out.OpenID, nil
}

// MiniappLogin 微信登录：POST /miniapp/login { code }
// 已绑定 → 直接签 JWT；未绑定 → 返回 need_bind=true。
func MiniappLogin(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if !jsonBody(c, &req) {
		return
	}
	openID, err := wxCode2Session(req.Code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	var binding models.MiniappBinding
	if err := database.DB.Where("open_id = ?", openID).First(&binding).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"need_bind": true, "open_id": openID}})
		return
	}
	// 校验绑定账号仍有效
	var user models.User
	if err := database.DB.Where("id = ? AND tenant_id = ?", binding.UserID, binding.TenantID).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"need_bind": true, "open_id": openID}})
		return
	}
	token, _ := auth.Sign(user.ID, user.Username, user.TenantID, user.Role)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"token": token, "user": miniappUser(user)}})
}

// MiniappBind 绑定分站账号：POST /miniapp/bind { code, username, password }
// 用分站账号密码验证身份，成功后绑定 openid 并签 JWT。
func MiniappBind(c *gin.Context) {
	var req struct {
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !jsonBody(c, &req) {
		return
	}
	openID, err := wxCode2Session(req.Code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "请输入账号和密码"})
		return
	}
	var user models.User
	if err := database.DB.Where("username = ? AND role = ? AND status = ?", req.Username, "admin", 1).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "账号不存在或已停用"})
		return
	}
	if !crypto.Verify(user.Password, req.Password, config.Load().PayloadSecret()) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "账号或密码错误"})
		return
	}
	// 绑定（已存在则更新）
	var binding models.MiniappBinding
	if err := database.DB.Where("open_id = ?", openID).First(&binding).Error; err == nil {
		binding.TenantID = user.TenantID
		binding.UserID = user.ID
		database.DB.Save(&binding)
	} else {
		database.DB.Create(&models.MiniappBinding{OpenID: openID, TenantID: user.TenantID, UserID: user.ID})
	}
	token, _ := auth.Sign(user.ID, user.Username, user.TenantID, user.Role)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"token": token, "user": miniappUser(user)}})
}

// MiniappHome 效果总览聚合：GET /miniapp/home
// 一次返回六项指标 + 近 7 天趋势 + 竞品声量 + 未读站内信 + 最近巡检时间。
func MiniappHome(c *gin.Context) {
	tid := TenantID(c)
	since := biztime.Since(6)

	var results []models.CheckResult
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).Find(&results)

	total, hit, top3, cited := 0, 0, 0, 0
	mentionSum := 0
	var lastRun time.Time
	dayMap := map[string]*miniTrend{}
	for _, r := range results {
		if r.ErrorMsg != "" {
			continue
		}
		total++
		if r.Hit {
			hit++
			mentionSum += r.MentionCount
			if r.HitPosition <= 3 {
				top3++
			}
		}
		if r.CreatedAt.After(lastRun) {
			lastRun = r.CreatedAt
		}
		d := r.CreatedAt.Format("2006-01-02")
		td := dayMap[d]
		if td == nil {
			td = &miniTrend{}
			dayMap[d] = td
		}
		td.Total++
		if r.Hit {
			td.Hit++
		}
	}

	// 引用率（近 7 天有引用的结果占比）
	if len(results) > 0 {
		var citedIDs []uint
		var cites []models.Citation
		database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).Find(&cites)
		seen := map[uint]bool{}
		for _, ct := range cites {
			if !seen[ct.ResultID] {
				seen[ct.ResultID] = true
				citedIDs = append(citedIDs, ct.ResultID)
			}
		}
		idSet := map[uint]bool{}
		for _, id := range citedIDs {
			idSet[id] = true
		}
		for _, r := range results {
			if r.ErrorMsg == "" && idSet[r.ID] {
				cited++
			}
		}
	}

	pct := func(n int) float64 {
		if total == 0 {
			return 0
		}
		return float64(int(float64(n)/float64(total)*1000+0.5)) / 10
	}
	brandRate := pct(hit)
	top3Rate := pct(top3)
	citeRate := pct(cited)

	// 竞品声量（近 7 天竞品命中占比，简化：用 CompetitorSov 相同口径）
	compRate := 0.0
	if total > 0 {
		var comps []models.Competitor
		database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&comps)
		compWords := make([]string, 0)
		for _, cm := range comps {
			for _, w := range strings.Split(cm.Name, ",") {
				if w = strings.TrimSpace(w); w != "" {
					compWords = append(compWords, w)
				}
			}
		}
		compHit := 0
		for _, r := range results {
			if r.ErrorMsg != "" {
				continue
			}
			low := strings.ToLower(r.Response)
			for _, w := range compWords {
				if strings.Contains(low, strings.ToLower(w)) {
					compHit++
					break
				}
			}
		}
		compRate = pct(compHit)
	}

	// 近 7 天趋势（补空天）
	trend := make([]float64, 0, 7)
	for i := 6; i >= 0; i-- {
		d := biztime.Day(-i)
		td := dayMap[d]
		if td == nil || td.Total == 0 {
			trend = append(trend, 0)
		} else {
			trend = append(trend, float64(int(float64(td.Hit)/float64(td.Total)*1000+0.5))/10)
		}
	}

	// 未读站内信
	var unread int64
	database.DB.Model(&models.Notification{}).Where("tenant_id = ? AND read = ?", tid, false).Count(&unread)

	// 平台分布（环形图/水平堆叠柱/饼图用）：各平台可见度 + TOP3 率 + 覆盖情况
	type platAgg struct {
		Name       string
		Total      int
		Hit        int
		Top3       int
	}
	platMap := map[string]*platAgg{}
	for _, r := range results {
		if r.ErrorMsg != "" {
			continue
		}
		p := platMap[r.PlatformName]
		if p == nil {
			p = &platAgg{Name: r.PlatformName}
			platMap[r.PlatformName] = p
		}
		p.Total++
		if r.Hit {
			p.Hit++
			if r.HitPosition <= 3 {
				p.Top3++
			}
		}
	}
	platforms := make([]gin.H, 0, len(platMap))
	for _, p := range platMap {
		vis := 0.0
		top3 := 0.0
		if p.Total > 0 {
			vis = float64(int(float64(p.Hit)/float64(p.Total)*1000+0.5)) / 10
			top3 = float64(int(float64(p.Top3)/float64(p.Total)*1000+0.5)) / 10
		}
		platforms = append(platforms, gin.H{"name": p.Name, "visibility": vis, "top3_rate": top3, "total": p.Total})
	}

	lastRunStr := ""
	if !lastRun.IsZero() {
		lastRunStr = lastRun.Format("2006-01-02 15:04")
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"brand_rate":   brandRate,
		"top3_rate":    top3Rate,
		"cite_rate":    citeRate,
		"comp_sov":     compRate,
		"brand_sov":    float64(mentionSum),
		"trend":        trend,
		"platforms":    platforms,
		"unread":       unread,
		"total_checks": total,
		"last_run_at":  lastRunStr,
	}})
}

type miniTrend struct {
	Total int
	Hit   int
}

func miniappUser(u models.User) gin.H {
	return gin.H{
		"id": u.ID, "username": u.Username, "nickname": u.Nickname,
		"tenant_id": u.TenantID, "role": u.Role,
	}
}
