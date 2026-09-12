package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
)

// DashboardSummary 仪表盘汇总：今日 / 累计指标（当前租户）
func DashboardSummary(c *gin.Context) {
	tid := TenantID(c)
	db := database.DB
	// 今日零点必须取**北京时间**零点：容器时区为 UTC，
	// 用 time.Now() 得到的「本地零点」实际是 UTC 零点（= 北京时间 8 点），
	// 会让「今日巡检次数」等当日指标在北京时间 8 点才清零。
	now := biztime.Now()
	midnight := biztime.DayStart(now)

	// 今日指标（取最近一次运行中的最新任务聚合结果）
	var todayChecked, todayHit, todayMiss, todayErr, todayCoverage int64
	db.Model(&models.CheckResult{}).
		Where("tenant_id = ? AND created_at >= ?", tid, midnight).
		Count(&todayChecked)
	db.Model(&models.CheckResult{}).
		Where("tenant_id = ? AND created_at >= ? AND hit = ?", tid, midnight, true).
		Count(&todayHit)
	// 仅统计真实错误：error_msg 非 NULL 且非空（原 OR 写法恒为真，导致 error=总数、miss 为负）
	db.Model(&models.CheckResult{}).
		Where("tenant_id = ? AND created_at >= ? AND error_msg IS NOT NULL AND error_msg <> ''", tid, midnight).
		Count(&todayErr)
	todayMiss = todayChecked - todayHit - todayErr
	db.Model(&models.CheckResult{}).
		Where("tenant_id = ? AND created_at >= ? AND hit = ?", tid, midnight, true).
		Distinct("platform_name").Count(&todayCoverage)

	// 累计指标
	var totalChecked, totalHit int64
	db.Model(&models.CheckResult{}).Where("tenant_id = ?", tid).Count(&totalChecked)
	db.Model(&models.CheckResult{}).Where("tenant_id = ? AND hit = ?", tid, true).Count(&totalHit)
	todayRate := 0.0
	if todayChecked > 0 {
		todayRate = float64(todayHit) / float64(todayChecked) * 100
	}
	totalRate := 0.0
	if totalChecked > 0 {
		totalRate = float64(totalHit) / float64(totalChecked) * 100
	}

	// 当前是否有任务运行
	var running int64
	db.Model(&models.CheckTask{}).Where("tenant_id = ? AND status = ?", tid, "running").Count(&running)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"today": gin.H{
			"total_queries": todayChecked, "hit": todayHit, "miss": todayMiss,
			"error": todayErr, "coverage": todayCoverage, "rate": round1(todayRate),
		},
		"total": gin.H{"total_queries": totalChecked, "hit": totalHit, "rate": round1(totalRate)},
		"running": running > 0,
	}})
}

// DashboardTrend 品牌出现率趋势（按天）
func DashboardTrend(c *gin.Context) {
	tid := TenantID(c)
	days := 30
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}
	since := biztime.Since(days-1)
	type row struct {
		Day   string
		Hit   int64
		Total int64
	}
	var rows []row
	database.DB.Model(&models.CheckResult{}).
		Select("" + dayExpr() + " as day, sum(case when hit then 1 else 0 end) as hit, count(*) as total").
		Where("tenant_id = ? AND created_at >= ?", tid, since).
		Group("" + dayExpr() + "").Order("day asc").Scan(&rows)

	// 补齐空天
	list := make([]gin.H, 0, days)
	byDay := map[string]row{}
	for _, r := range rows {
		byDay[r.Day] = r
	}
	for i := days - 1; i >= 0; i-- {
		d := biztime.Day(-i)
		r, ok := byDay[d]
		if !ok {
			r.Day = d
		}
		list = append(list, gin.H{"Day": r.Day, "Hit": r.Hit, "Total": r.Total})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"list": list}})
}

// DashboardPlatforms 各平台出现率
func DashboardPlatforms(c *gin.Context) {
	tid := TenantID(c)
	type row struct {
		Platform string
		Hit      int64
		Total    int64
	}
	var rows []row
	database.DB.Model(&models.CheckResult{}).
		Select("platform_name as platform, sum(case when hit then 1 else 0 end) as hit, count(*) as total").
		Where("tenant_id = ?", tid).
		Group("platform_name").Order("total desc").Scan(&rows)
	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		v := 0.0
		if r.Total > 0 {
			v = round1(float64(r.Hit) / float64(r.Total) * 100)
		}
		list = append(list, gin.H{"platform": r.Platform, "rate": v})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// DashboardKeywords 关键词出现率排行
func DashboardKeywords(c *gin.Context) {
	tid := TenantID(c)
	type row struct {
		Question string
		Hit      int64
		Total    int64
	}
	var rows []row
	database.DB.Model(&models.CheckResult{}).
		Select("question, sum(case when hit then 1 else 0 end) as hit, count(*) as total").
		Where("tenant_id = ?", tid).
		Group("question").Order("total desc").Limit(20).Scan(&rows)
	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		v := 0.0
		if r.Total > 0 {
			v = round1(float64(r.Hit) / float64(r.Total) * 100)
		}
		list = append(list, gin.H{"question": r.Question, "rate": v})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// dayExpr 按天分组的 SQL 表达式（PostgreSQL to_char）。
func dayExpr() string { return database.DaySQL("created_at") }

// DashboardOverview 客户端驾驶舱（对标行业标杆）：
// 平台覆盖横幅、五大 KPI、各平台表现表（含趋势/变化）、场景分布、关键词排名 TOP10 与排名分布。
// 所有指标均基于当前租户的 CheckResult / GeoKeyword / AiPlatform 聚合。
func DashboardOverview(c *gin.Context) {
	tid := TenantID(c)
	db := database.DB

	// 1) 启用的平台列表（全局平台 + 分站覆盖层合并，与巡检引擎保持一致）
	platsAll := EffectivePlatforms(tid)
	plats := make([]models.AiPlatform, 0, len(platsAll))
	for _, p := range platsAll {
		if p.Enabled {
			plats = append(plats, p)
		}
	}
	totalPlatforms := len(plats)

	// 2) 按平台聚合（可见度/TOP3/曝光/排名档位）
	type agg struct {
		Platform   string
		HitOK      int64
		OKCount    int64
		Total      int64
		Top1       int64
		Top23      int64
		Top410     int64
		Exposure   int64
	}
	var aggs []agg
	// 只统计启用平台的数据；按 platform_name 关联（避免分站复制平台后 platform_id 跨租户冲突）
	platNames := make([]string, 0, len(plats))
	for _, p := range plats {
		platNames = append(platNames, p.Name)
	}
	aggQ := db.Model(&models.CheckResult{}).
		Select(`platform_name as platform,
			sum(case when hit then 1 else 0 end) as hit_ok,
			sum(case when error_msg is null or error_msg = '' then 1 else 0 end) as ok_count,
			count(*) as total,
			sum(case when hit and hit_position = 1 then 1 else 0 end) as top1,
			sum(case when hit and hit_position between 2 and 3 then 1 else 0 end) as top23,
			sum(case when hit and hit_position between 4 and 10 then 1 else 0 end) as top410,
			sum(case when coalesce(mention_count,0) > 0 then mention_count else 0 end) as exposure`).
		Where("tenant_id = ?", tid)
	if len(platNames) > 0 {
		aggQ = aggQ.Where("platform_name IN ?", platNames)
	}
	aggQ.Group("platform_name").Scan(&aggs)

	// 3) 近 30 天按平台按天的趋势（用于增长/趋势线）
	type trendRow struct {
		Platform string
		Day      string
		Hit      int64
		Total    int64
	}
	var trendRows []trendRow
	db.Model(&models.CheckResult{}).
		Select("platform_name as platform, " + dayExpr() + " as day, sum(case when hit then 1 else 0 end) as hit, count(*) as total").
		Where("tenant_id = ? AND created_at >= ?", tid, biztime.Since(29)).
		Group("platform_name, " + dayExpr() + "").
		Scan(&trendRows)
	trendMap := map[string]map[string]trendRow{}
	for _, r := range trendRows {
		mm := trendMap[r.Platform]
		if mm == nil {
			mm = map[string]trendRow{}
			trendMap[r.Platform] = mm
		}
		mm[r.Day] = r
	}
	rateOf := func(r trendRow) float64 {
		if r.Total == 0 {
			return 0
		}
		return round1(float64(r.Hit) / float64(r.Total) * 100)
	}
	// 填充近 30 天完整序列 + 最近两个有数据日的差值
	fillTrend := func(platform string) ([]float64, float64) {
		mm := trendMap[platform]
		seq := make([]float64, 0, 30)
		var lastDay, prevDay string
		var has bool
		for i := 29; i >= 0; i-- {
			d := biztime.Day(-i)
			if mm != nil {
				if r, ok := mm[d]; ok && r.Total > 0 {
					seq = append(seq, rateOf(r))
					if !has {
						lastDay, has = d, true
					} else if prevDay == "" {
						prevDay = d
					}
					continue
				}
			}
			seq = append(seq, 0)
		}
		delta := 0.0
		if has && prevDay != "" {
			delta = round1(rateOf((trendMap[platform])[lastDay]) - rateOf((trendMap[platform])[prevDay]))
		}
		return seq, delta
	}

	// 4) 各平台表现表
	type platRow struct {
		Platform   string    `json:"platform"`
		Enabled    bool      `json:"enabled"`
		Visibility float64   `json:"visibility"` // 可见度 %
		Top3Rate   float64   `json:"top3_rate"`  // TOP3 占比 %
		Exposure   int64     `json:"exposure"`   // 曝光次数
		Delta      float64   `json:"delta"`      // 较上次变化（百分点）
		Trend      []float64 `json:"trend"`      // 近30天趋势
	}
	platformTable := make([]platRow, 0, len(plats))
	aggMap := map[string]agg{}
	for _, a := range aggs {
		aggMap[a.Platform] = a
	}
	var sumHit, sumTotal, sumTop3, sumExposure int64
	covered := 0
	for _, p := range plats {
		a, ok := aggMap[p.Name]
		if !ok {
			continue
		}
		sumHit += a.HitOK
		sumTotal += a.Total
		sumTop3 += a.Top1 + a.Top23
		sumExposure += a.Exposure
		if a.HitOK > 0 {
			covered++
		}
		seq, delta := fillTrend(p.Name)
		row := platRow{
			Platform:   p.Name,
			Enabled:    p.Enabled,
			Visibility: round1(float64(a.HitOK) / float64(a.Total) * 100),
			Top3Rate:   round1(float64(a.Top1+a.Top23) / float64(a.Total) * 100),
			Exposure:   a.Exposure,
			Delta:      delta,
			Trend:      seq,
		}
		platformTable = append(platformTable, row)
	}
	sort.Slice(platformTable, func(i, j int) bool {
		return platformTable[i].Visibility > platformTable[j].Visibility
	})

	// 5) KPI（sumTotal=0 时避免除零产生 NaN）
	avgVis, top3Rate := 0.0, 0.0
	if sumTotal > 0 {
		avgVis = round1(float64(sumHit) / float64(sumTotal) * 100)
		top3Rate = round1(float64(sumTop3) / float64(sumTotal) * 100)
	}
	kpi := gin.H{
		"avg_visibility": avgVis,       // 整体可见度（平均）
		"top3_rate":      top3Rate,     // TOP3 推荐占比
		"exposure":       sumExposure,  // 品牌曝光次数
		"covered":        covered,      // 覆盖平台数
		"total_platform": totalPlatforms, // 监控平台数
	}
	var kwCount int64
	db.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND enabled = ?", tid, true).Count(&kwCount)
	kpi["keyword_count"] = kwCount
	kpi["has_data"] = sumTotal > 0

	// 5.1) 全局近30天逐日指标（KPI 迷你趋势线）
	// 独立按天聚合：总查询/命中/TOP3/曝光/覆盖平台数/涉及关键词数
	type dayRow struct {
		Day     string
		Hit     int64
		Total   int64
		Top3    int64
		Exp     int64
		PlatCnt int64
		KwCnt   int64
	}
	var dayRows []dayRow
	db.Model(&models.CheckResult{}).
		Select(dayExpr() + ` as day,
			sum(case when hit then 1 else 0 end) as hit,
			count(*) as total,
			sum(case when hit and hit_position between 1 and 3 then 1 else 0 end) as top3,
			sum(case when coalesce(mention_count,0) > 0 then mention_count else 0 end) as exp,
			count(distinct platform_name) as plat_cnt,
			count(distinct question) as kw_cnt`).
		Where("tenant_id = ? AND created_at >= ?", tid, biztime.Since(29)).
		Group(dayExpr()).Scan(&dayRows)
	dayHit := map[string]int64{}
	dayTotal := map[string]int64{}
	dayTop3 := map[string]int64{}
	dayExp := map[string]int64{}
	dayPlat := map[string]int64{}
	dayKw := map[string]int64{}
	for _, r := range dayRows {
		dayHit[r.Day] = r.Hit
		dayTotal[r.Day] = r.Total
		dayTop3[r.Day] = r.Top3
		dayExp[r.Day] = r.Exp
		dayPlat[r.Day] = r.PlatCnt
		dayKw[r.Day] = r.KwCnt
	}
	fillDay := func(pick func(dayHit, dayTotal, dayTop3, dayExp, dayPlat, dayKw int64) (float64, bool)) []float64 {
		seq := make([]float64, 0, 30)
		for i := 29; i >= 0; i-- {
			d := biztime.Day(-i)
			v, ok := pick(dayHit[d], dayTotal[d], dayTop3[d], dayExp[d], dayPlat[d], dayKw[d])
			if !ok || v < 0 {
				v = 0
			}
			seq = append(seq, v)
		}
		return seq
	}
	visibilityTrend := fillDay(func(h, total, top3, exp, plat, kw int64) (float64, bool) {
		if total == 0 {
			return 0, false
		}
		return round1(float64(h) / float64(total) * 100), true
	})
	top3Trend := fillDay(func(h, total, top3, exp, plat, kw int64) (float64, bool) {
		if total == 0 {
			return 0, false
		}
		return round1(float64(top3) / float64(total) * 100), true
	})
	exposureTrend := fillDay(func(h, total, top3, exp, plat, kw int64) (float64, bool) {
		return float64(exp), true
	})
	coveredTrend := fillDay(func(h, total, top3, exp, plat, kw int64) (float64, bool) {
		return float64(plat), true
	})
	keywordTrend := fillDay(func(h, total, top3, exp, plat, kw int64) (float64, bool) {
		return float64(kw), true
	})
	kpi["trend"] = visibilityTrend
	kpi["visibility_trend"] = visibilityTrend
	kpi["top3_trend"] = top3Trend
	kpi["exposure_trend"] = exposureTrend
	kpi["covered_trend"] = coveredTrend
	kpi["keyword_trend"] = keywordTrend

	// 6) 场景覆盖分布（按关键词分类）
	type sceneRow struct {
		Category string
		Cnt      int64
	}
	var scenes []sceneRow
	db.Model(&models.GeoKeyword{}).
		Select("coalesce(nullif(category,''),'未分类') as category, count(*) as cnt").
		Where("tenant_id = ?", tid).
		Group("category").Scan(&scenes)
	sceneDist := make([]gin.H, 0, len(scenes))
	for _, s := range scenes {
		sceneDist = append(sceneDist, gin.H{"name": s.Category, "value": s.Cnt})
	}

	// 7) 最近一次已完成任务 → 关键词排名监测 TOP10 + 排名分布
	var lastTaskID uint
	db.Model(&models.CheckTask{}).
		Where("tenant_id = ? AND status IN ?", tid, []string{"success", "partial"}).
		Order("id desc").Limit(1).Pluck("id", &lastTaskID)
	kwRankTop := make([]gin.H, 0)
	rankDist := gin.H{"top1": 0, "top23": 0, "top410": 0, "miss": 0}
	if lastTaskID > 0 {
		// 排名分布（基于最近任务全部结果）
		type rd struct {
			Top1   int64
			Top23  int64
			Top410 int64
			Miss   int64
		}
		var rdv rd
		db.Model(&models.CheckResult{}).
			Select(`sum(case when hit and hit_position = 1 then 1 else 0 end) as top1,
				sum(case when hit and hit_position between 2 and 3 then 1 else 0 end) as top23,
				sum(case when hit and hit_position between 4 and 10 then 1 else 0 end) as top410,
				sum(case when not hit then 1 else 0 end) as miss`).
			Where("tenant_id = ? AND task_id = ?", tid, lastTaskID).
			Scan(&rdv)
		rankDist = gin.H{"top1": rdv.Top1, "top23": rdv.Top23, "top410": rdv.Top410, "miss": rdv.Miss}

		// 排名监测 TOP10：枚举最近任务全部 关键词×平台 组合，取各自最优位置（含未命中，未命中排后）
		type rr struct {
			Keyword      string
			PlatformName string
			BestPos      int
			CreatedAt    string // max(created_at) 用 string 接收，兼容时间戳多格式解析
		}
		var rows []rr
		db.Model(&models.CheckResult{}).
			Select(`question as keyword, platform_name,
				min(case when hit then hit_position else 999 end) as best_pos,
				max(created_at) as created_at`).
			Where("tenant_id = ? AND task_id = ?", tid, lastTaskID).
			Group("question, platform_name").
			Order("best_pos asc, created_at desc").
			Limit(10).Scan(&rows)
		// 上一任务（用于排名变化）
		var prevTaskID uint
		db.Model(&models.CheckTask{}).
			Where("tenant_id = ? AND status IN ? AND id < ?", tid, []string{"success", "partial"}, lastTaskID).
			Order("id desc").Limit(1).Pluck("id", &prevTaskID)
		prevPos := map[string]int{}
		if prevTaskID > 0 {
			type pr struct {
				Keyword      string
				PlatformName string
				BestPos      int
			}
			var prevs []pr
			db.Model(&models.CheckResult{}).
				Select(`question as keyword, platform_name,
					min(case when hit then hit_position else 999 end) as best_pos`).
				Where("tenant_id = ? AND task_id = ?", tid, prevTaskID).
				Group("question, platform_name").
				Scan(&prevs)
			for _, p := range prevs {
				prevPos[p.Keyword+"|"+p.PlatformName] = p.BestPos
			}
		}
		for _, r := range rows {
			delta := 0
			if old, ok := prevPos[r.Keyword+"|"+r.PlatformName]; ok && old < 999 && r.BestPos < 999 {
				delta = old - r.BestPos
			}
			// created_at 兼容多种时间格式解析，失败则回退为字符串截取（取前 16 字符，T 转空格）
			checkTime := ""
			if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", r.CreatedAt); err == nil {
				checkTime = t.Format("2006-01-02 15:04")
			} else if len(r.CreatedAt) >= 16 {
				checkTime = r.CreatedAt[:16]
				if checkTime[10] == 'T' {
					checkTime = checkTime[:10] + " " + checkTime[11:]
				}
			}
			kwRankTop = append(kwRankTop, gin.H{
				"keyword":    r.Keyword,
				"platform":   r.PlatformName,
				"position":   r.BestPos,
				"top3":       r.BestPos >= 1 && r.BestPos <= 3,
				"miss":       r.BestPos >= 999,
				"delta":      delta,
				"check_time": checkTime,
			})
		}
	}

	// 8) 平台覆盖横幅数据
	coverage := make([]gin.H, 0, len(plats))
	for _, p := range plats {
		coverage = append(coverage, gin.H{
			"name": p.Name, "enabled": p.Enabled,
			// 覆盖 = 该平台存在成功响应的巡检记录（API 已打通），而非仅品牌词命中
			"covered": aggMap[p.Name].OKCount > 0,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"kpi":            kpi,
		"coverage":       coverage,
		"platform_table": platformTable,
		"scene_dist":     sceneDist,
		"kw_rank_top":    kwRankTop,
		"rank_dist":      rankDist,
		// 收口到 biztime（原为本地 CST 硬编码，属于第二套时区实现）
		"generated_at":   biztime.DateTimeSec(),
	}})
}
