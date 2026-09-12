package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// ============================================================
// GEO 智能中心：品牌事实库 / 竞品库 / 引用溯源 / 六项指标 /
// 缺口分析 / 优化行动清单 / 网站审计 / llms.txt & Schema 生成
// ============================================================

// ---------- 品牌事实库 ----------

// ListFacts 品牌事实库列表（可按分类筛选）
func ListFacts(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("category"); v != "" {
		q = q.Where("category = ?", v)
	}
	var list []models.FactItem
	q.Order("id asc").Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func CreateFact(c *gin.Context) {
	var body models.FactItem
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	body.TenantID = TenantID(c)
	body.Category = strings.TrimSpace(body.Category)
	if body.Category == "" {
		body.Category = "其他"
	}
	if err := database.DB.Create(&body).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "保存失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": body})
}

func UpdateFact(c *gin.Context) {
	id := c.Param("id")
	var body models.FactItem
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	var f models.FactItem
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, TenantID(c)).First(&f).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "记录不存在"})
		return
	}
	body.ID = f.ID
	body.TenantID = f.TenantID
	database.DB.Model(&f).Updates(map[string]interface{}{
		"category": body.Category, "question": body.Question, "fact": body.Fact,
		"not_fact": body.NotFact, "enabled": body.Enabled,
	})
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": f})
}

func DeleteFact(c *gin.Context) {
	database.DB.Where("id = ? AND tenant_id = ?", c.Param("id"), TenantID(c)).Delete(&models.FactItem{})
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// ---------- 竞品库 ----------

func ListCompetitors(c *gin.Context) {
	var list []models.Competitor
	database.DB.Where("tenant_id = ?", TenantID(c)).Order("id asc").Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func CreateCompetitor(c *gin.Context) {
	var body models.Competitor
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	body.TenantID = TenantID(c)
	if strings.TrimSpace(body.Name) == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "竞品名称不能为空"})
		return
	}
	database.DB.Create(&body)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": body})
}

func UpdateCompetitor(c *gin.Context) {
	var body models.Competitor
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	var f models.Competitor
	if err := database.DB.Where("id = ? AND tenant_id = ?", c.Param("id"), TenantID(c)).First(&f).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "记录不存在"})
		return
	}
	database.DB.Model(&f).Updates(map[string]interface{}{"name": body.Name, "remark": body.Remark, "enabled": body.Enabled})
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": f})
}

func DeleteCompetitor(c *gin.Context) {
	database.DB.Where("id = ? AND tenant_id = ?", c.Param("id"), TenantID(c)).Delete(&models.Competitor{})
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// ---------- 风险词库 ----------

func ListRiskWords(c *gin.Context) {
	var list []models.RiskWord
	database.DB.Where("tenant_id = ?", TenantID(c)).Order("id asc").Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func CreateRiskWord(c *gin.Context) {
	var body models.RiskWord
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	body.TenantID = TenantID(c)
	if strings.TrimSpace(body.Word) == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "风险词不能为空"})
		return
	}
	database.DB.Create(&body)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": body})
}

func DeleteRiskWord(c *gin.Context) {
	database.DB.Where("id = ? AND tenant_id = ?", c.Param("id"), TenantID(c)).Delete(&models.RiskWord{})
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// ---------- 引用溯源 ----------

// ListCitations 引用溯源列表：可按任务 / 平台 / 关键词 / 域名筛选
func ListCitations(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("task_id"); v != "" {
		q = q.Where("task_id = ?", v)
	}
	if v := c.Query("platform"); v != "" {
		q = q.Where("platform_name = ?", v)
	}
	if v := c.Query("domain"); v != "" {
		q = q.Where("domain = ?", v)
	}
	if v := c.Query("keyword"); v != "" {
		q = q.Where("question LIKE ?", "%"+v+"%")
	}
	page, size := pageArgs(c)
	var total int64
	q.Model(&models.Citation{}).Count(&total)
	var list []models.Citation
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"list": list, "total": total, "page": page, "size": size}})
}

// CitationDomains 引用来源域名排行（近 N 天）
func CitationDomains(c *gin.Context) {
	tid := TenantID(c)
	days := parseDay(c.DefaultQuery("days", "30"))
	since := time.Now().AddDate(0, 0, -days)
	rows := []struct {
		Domain string `json:"domain"`
		Cnt    int64  `json:"cnt"`
	}{}
	database.DB.Model(&models.Citation{}).
		Select("domain, count(*) as cnt").
		Where("tenant_id = ? AND created_at >= ? AND domain != ''", tid, since).
		Group("domain").Order("cnt desc").Limit(20).Scan(&rows)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rows})
}

// ---------- 信源图谱：品牌 / 竞品信源对比 + 缺口清单 ----------

type domainCnt struct {
	Domain string `json:"domain"`
	Cnt    int64  `json:"cnt"`
}

type sourceGapResp struct {
	TopDomains        []domainCnt `json:"top_domains"`        // 高频引用域名
	BrandDomains      []domainCnt `json:"brand_domains"`      // 品牌被引用时的信源
	CompetitorDomains []domainCnt `json:"competitor_domains"` // 竞品被引用时的信源
	GapDomains        []domainCnt `json:"gap_domains"`        // 信源缺口（竞品有、品牌没有）
}

// SourceGaps 信源图谱：AI 回答中引用的站点，区分「品牌信源 vs 竞品信源」，
// 并给出「竞品有、品牌没有」的信源缺口清单（按竞品引用频次排序）。
func SourceGaps(c *gin.Context) {
	tid := TenantID(c)
	days := parseDay(c.DefaultQuery("days", "30"))
	since := time.Now().AddDate(0, 0, -days)

	var comps []models.Competitor
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&comps)
	compWords := []string{}
	for _, cm := range comps {
		for _, w := range strings.Split(cm.Name, ",") {
			if w = strings.TrimSpace(w); w != "" {
				compWords = append(compWords, w)
			}
		}
	}

	var cites []models.Citation
	database.DB.Where("tenant_id = ? AND created_at >= ? AND domain != ''", tid, since).Find(&cites)

	resultMap := map[uint]models.CheckResult{}
	if len(cites) > 0 {
		ids := make([]uint, 0, len(cites))
		for _, ct := range cites {
			ids = append(ids, ct.ResultID)
		}
		var rs []models.CheckResult
		database.DB.Where("tenant_id = ? AND id IN ?", tid, ids).Find(&rs)
		for _, r := range rs {
			resultMap[r.ID] = r
		}
	}

	allCnt := map[string]int64{}
	brandCnt := map[string]int64{}
	compCnt := map[string]int64{}
	for _, ct := range cites {
		allCnt[ct.Domain]++
		r, ok := resultMap[ct.ResultID]
		if !ok {
			continue
		}
		if r.Hit {
			brandCnt[ct.Domain]++
		}
		low := strings.ToLower(r.Response)
		for _, w := range compWords {
			if strings.Contains(low, strings.ToLower(w)) {
				compCnt[ct.Domain]++
				break
			}
		}
	}

	resp := &sourceGapResp{}
	resp.TopDomains = sortDomains(allCnt, 20)
	resp.BrandDomains = sortDomains(brandCnt, 20)
	resp.CompetitorDomains = sortDomains(compCnt, 20)
	// 缺口：竞品被引用、品牌未被引用的域名
	for d, c := range compCnt {
		if brandCnt[d] == 0 {
			resp.GapDomains = append(resp.GapDomains, domainCnt{Domain: d, Cnt: c})
		}
	}
	sort.Slice(resp.GapDomains, func(i, j int) bool { return resp.GapDomains[i].Cnt > resp.GapDomains[j].Cnt })
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func sortDomains(m map[string]int64, limit int) []domainCnt {
	out := make([]domainCnt, 0, len(m))
	for d, c := range m {
		out = append(out, domainCnt{Domain: d, Cnt: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cnt > out[j].Cnt })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ---------- 六项核心指标 ----------

// geoIndicator 六项指标 + 平台/趋势明细
type geoIndicator struct {
	Days          int                `json:"days"`
	Period        string             `json:"period"`
	BrandRate     float64            `json:"brand_rate"`     // ① 品牌出现率
	Top3Rate      float64            `json:"top3_rate"`      // ② 推荐率（TOP3 覆盖）
	CitationRate  float64            `json:"citation_rate"`  // ③ 引用率（有引用链接的回答占比）
	AccuracyRate  float64            `json:"accuracy_rate"`  // ④ 事实一致率（与品牌事实库无冲突）
	BrandSov      float64            `json:"brand_sov"`      // ⑤a 品牌 AI 声量（出现率）
	CompetitorSov float64            `json:"competitor_sov"` // ⑤b 竞品声量（提及率）
	RiskRate      float64            `json:"risk_rate"`      // ⑥ 风险回答率（命中风险词）
	AvgMention    float64            `json:"avg_mention"`
	AvgCitation   float64            `json:"avg_citation"`
	Platforms     []geoPlatform      `json:"platforms"`
	Trend         []TrendPoint       `json:"trend"`
	Deltas        map[string]float64 `json:"deltas"` // 各核心指标「较前期」变化（百分点）
	// AI 可见度评分（0~100 综合分 + 四维拆解 + 行业基准 + 改进建议）
	Visibility visibilityScore `json:"visibility"`
	// 结果样本量（用于评分置信度提示）
	SampleCount int `json:"sample_count"`
}

type geoPlatform struct {
	Name          string  `json:"name"`
	Queries       int     `json:"queries"`
	BrandRate     float64 `json:"brand_rate"`
	CitationRate  float64 `json:"citation_rate"`
	AccuracyRate  float64 `json:"accuracy_rate"`
	CompetitorSov float64 `json:"competitor_sov"`
	RiskRate      float64 `json:"risk_rate"`
}

// GeoIntel 六项核心指标聚合（HTTP 入口，薄封装）
func GeoIntel(c *gin.Context) {
	results, days, since := queryResultsInRange(c, 7)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": computeGeoIndicator(TenantID(c), results, since, days)})
}

// computeGeoIndicator 六项核心指标聚合的**唯一权威实现**。
//
// 为什么抽成纯函数：AI 数据分析助手需要把同样的指标喂给大模型做解读。
// 若助手另写一套算法，就会出现「仪表盘显示 A、AI 说 B」的口径分叉——
// 这正是 v1.0.33 站点审计两套口径并存踩过的坑。此处统一为单一来源。
func computeGeoIndicator(tid uint, results []models.CheckResult, since time.Time, days int) *geoIndicator {
	// 加载事实库 / 风险词 / 竞品（启用态）
	var facts []models.FactItem
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&facts)
	var risks []models.RiskWord
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&risks)
	var comps []models.Competitor
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&comps)

	// 引用：构建 result_id -> 引用数
	citeCnt := map[uint]int{}
	if len(results) > 0 {
		var cites []models.Citation
		database.DB.Where("tenant_id = ? AND created_at >= ?", tid, since).Find(&cites)
		for _, ct := range cites {
			citeCnt[ct.ResultID]++
		}
	}

	ind := &geoIndicator{Days: days, Period: fmt.Sprintf("%s ~ %s", since.Format("2006-01-02"), time.Now().Format("2006-01-02"))}
	pm := map[string]*geoPlatform{}
	dayMap := map[string]*TrendPoint{}

	success, hit, top3 := 0, 0, 0
	cited, accurate, riskHit := 0, 0, 0
	compHit := 0
	mentionSum, citeSum := 0, 0
	scoreDelta := 0.0

	// 竞品关键词（逗号分隔同义名展开）
	compWords := make([]string, 0)
	for _, cm := range comps {
		for _, w := range strings.Split(cm.Name, ",") {
			if w = strings.TrimSpace(w); w != "" {
				compWords = append(compWords, w)
			}
		}
	}

	for _, r := range results {
		if r.ErrorMsg != "" {
			continue
		}
		success++
		low := strings.ToLower(r.Response)
		hitThis := r.Hit
		if hitThis {
			hit++
			if r.HitPosition <= 3 {
				top3++
			}
			mentionSum += r.MentionCount
		}
		n := citeCnt[r.ID]
		if n > 0 {
			cited++
			citeSum += n
		}
		// 事实冲突检测：命中任一 not_fact 表述视为与事实库冲突
		conflict := false
		for _, f := range facts {
			if strings.TrimSpace(f.NotFact) == "" {
				continue
			}
			if strings.Contains(low, strings.ToLower(f.NotFact)) {
				conflict = true
				break
			}
		}
		if !conflict {
			accurate++
		}
		// 风险词命中
		risky := false
		for _, rw := range risks {
			if strings.Contains(low, strings.ToLower(rw.Word)) {
				risky = true
				break
			}
		}
		if risky {
			riskHit++
		}
		// 竞品提及
		compPresent := false
		for _, w := range compWords {
			if strings.Contains(low, strings.ToLower(w)) {
				compPresent = true
				break
			}
		}
		if compPresent {
			compHit++
		}

		// 平台聚合
		p, ok := pm[r.PlatformName]
		if !ok {
			p = &geoPlatform{Name: r.PlatformName}
			pm[r.PlatformName] = p
		}
		p.Queries++
		if hitThis {
			p.BrandRate += 1
		}
		if n > 0 {
			p.CitationRate += 1
		}
		if !conflict {
			p.AccuracyRate += 1
		}
		if compPresent {
			p.CompetitorSov += 1
		}
		if risky {
			p.RiskRate += 1
		}
		// 趋势
		day := r.CreatedAt.Format("01-02")
		tp, ok := dayMap[day]
		if !ok {
			tp = &TrendPoint{Day: day}
			dayMap[day] = tp
		}
		tp.Queries++
		if hitThis {
			tp.Hit++
		}
	}

	denom := func(n int) float64 {
		if success == 0 {
			return 0
		}
		return round1(float64(n) / float64(success) * 100)
	}
	ind.BrandRate = denom(hit)
	ind.Top3Rate = denom(top3)
	ind.CitationRate = denom(cited)
	ind.AccuracyRate = denom(accurate)
	ind.BrandSov = ind.BrandRate
	ind.CompetitorSov = denom(compHit)
	ind.RiskRate = denom(riskHit)
	if hit > 0 {
		ind.AvgMention = round1(float64(mentionSum) / float64(hit))
	}
	if cited > 0 {
		ind.AvgCitation = round1(float64(citeSum) / float64(cited))
	}

	// 平台明细换算为百分比
	for _, p := range pm {
		if p.Queries > 0 {
			conv := func(n float64) float64 { return round1(n / float64(p.Queries) * 100) }
			p.BrandRate = conv(p.BrandRate)
			p.CitationRate = conv(p.CitationRate)
			p.AccuracyRate = conv(p.AccuracyRate)
			p.CompetitorSov = conv(p.CompetitorSov)
			p.RiskRate = conv(p.RiskRate)
		}
		ind.Platforms = append(ind.Platforms, *p)
	}
	sort.Slice(ind.Platforms, func(i, j int) bool { return ind.Platforms[i].Queries > ind.Platforms[j].Queries })

	// 趋势升序
	daysL := make([]string, 0, len(dayMap))
	for d := range dayMap {
		daysL = append(daysL, d)
	}
	sort.Strings(daysL)
	for _, d := range daysL {
		tp := dayMap[d]
		if tp.Queries > 0 {
			tp.Rate = round1(float64(tp.Hit) / float64(tp.Queries) * 100)
		}
		ind.Trend = append(ind.Trend, *tp)
	}

	// 前后期对比：把结果按时间升序分成前后两半，算核心指标变化（较前期 ±百分点）
	calcSeg := func(rs []models.CheckResult) (brand, top3, cite, acc, risk, comp float64) {
		n := 0
		for _, r := range rs {
			if r.ErrorMsg != "" {
				continue
			}
			n++
			if r.Hit {
				brand++
				if r.HitPosition <= 3 {
					top3++
				}
			}
			if citeCnt[r.ID] > 0 {
				cite++
			}
			conflict := false
			for _, f := range facts {
				if strings.TrimSpace(f.NotFact) != "" && strings.Contains(strings.ToLower(r.Response), strings.ToLower(f.NotFact)) {
					conflict = true
					break
				}
			}
			if !conflict {
				acc++
			}
			for _, rw := range risks {
				if strings.Contains(strings.ToLower(r.Response), strings.ToLower(rw.Word)) {
					risk++
					break
				}
			}
			for _, w := range compWords {
				if strings.Contains(strings.ToLower(r.Response), strings.ToLower(w)) {
					comp++
					break
				}
			}
		}
		pct := func(x float64) float64 {
			if n == 0 {
				return 0
			}
			return round1(x / float64(n) * 100)
		}
		return pct(brand), pct(top3), pct(cite), pct(acc), pct(risk), pct(comp)
	}
	if len(results) >= 2 {
		// results 按 created_at desc（最新在前）：[:mid]=后期（较新），[mid:]=前期（较早）
		mid := len(results) / 2
		lateB, lateT, lateC, lateA, lateR, lateComp := calcSeg(results[:mid])
		earlyB, earlyT, earlyC, earlyA, earlyR, earlyComp := calcSeg(results[mid:])
		ind.Deltas = map[string]float64{
			"brand_rate":     round1(lateB - earlyB),
			"top3_rate":      round1(lateT - earlyT),
			"citation_rate":  round1(lateC - earlyC),
			"accuracy_rate":  round1(lateA - earlyA),
			"risk_rate":      round1(lateR - earlyR),
			"competitor_sov": round1(lateComp - earlyComp),
		}
		// 评分环比：用同一套评分函数分别复算前后期，得到分数变化（避免口径不一致）
		lateScore := scoreOfResults(results[:mid], citeCnt, facts, risks, compWords)
		earlyScore := scoreOfResults(results[mid:], citeCnt, facts, risks, compWords)
		scoreDelta = round1(lateScore - earlyScore)
	}

	// AI 可见度评分：由六项原始指标合成总分 + 四维拆解 + 行业基准 + 改进建议
	ind.Visibility = buildVisibilityScore(
		ind.BrandRate, ind.Top3Rate, ind.CitationRate, ind.AccuracyRate,
		ind.RiskRate, ind.AvgMention, success, scoreDelta,
	)
	ind.SampleCount = success

	return ind
}

// ---------- 缺口分析 ----------

type gapItem struct {
	Question          string   `json:"question"`
	Misses            int      `json:"misses"`
	CompetitorMention int      `json:"competitor_mention"`
	CompetitorNames   []string `json:"competitor_names"`
	Level             string   `json:"level"` // high / mid / low
}

type gapResp struct {
	Total             int       `json:"total"`
	HighGap           int       `json:"high_gap"`
	Gaps              []gapItem `json:"gaps"`
	CompetitorSOV     []compSOV `json:"competitor_sov"`
	UncoveredKeywords []string  `json:"uncovered_keywords"`
}

type compSOV struct {
	Name          string   `json:"name"`
	Mentions      int      `json:"mentions"`
	FirstMentions int      `json:"first_mentions"` // 首推次数（第一个被提及的品牌即该竞品）
	Questions     []string `json:"questions"`
}

// GeoGaps 竞品对比 + 品牌缺口分析（HTTP 入口，薄封装）
func GeoGaps(c *gin.Context) {
	results, _, _ := queryResultsInRange(c, 7)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": computeGapResp(TenantID(c), results)})
}

// computeGapResp 缺口/竞品分析的**唯一权威实现**（「差距诊断」页与 AI 助手共用，
// 避免助手另算一套导致"页面说 A、AI 说 B"）。
func computeGapResp(tid uint, results []models.CheckResult) *gapResp {
	var comps []models.Competitor
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&comps)

	compWords := []string{}
	for _, cm := range comps {
		for _, w := range strings.Split(cm.Name, ",") {
			if w = strings.TrimSpace(w); w != "" {
				compWords = append(compWords, w)
			}
		}
	}

	qMiss := map[string]int{}
	qTotal := map[string]int{}
	qComp := map[string]int{}
	qCompNames := map[string][]string{}
	compStats := map[string]*compSOV{}

	for _, r := range results {
		if r.ErrorMsg != "" {
			continue
		}
		qTotal[r.Question]++
		if !r.Hit {
			qMiss[r.Question]++
		}
		low := strings.ToLower(r.Response)

		// 首推检测：回答中「第一个出现的品牌词」是谁（品牌首推则竞品不计首推）
		firstWord, firstPos := "", len(low)+1
		scan := func(w string) {
			if p := strings.Index(low, strings.ToLower(w)); p >= 0 && p < firstPos {
				firstPos, firstWord = p, w
			}
		}
		for _, bw := range strings.Split(r.BrandKeywords, ",") {
			if bw = strings.TrimSpace(bw); bw != "" {
				scan(bw)
			}
		}
		for _, w := range compWords {
			scan(w)
		}

		for _, w := range compWords {
			if strings.Contains(low, strings.ToLower(w)) {
				qComp[r.Question]++
				if !containsStr(qCompNames[r.Question], w) {
					qCompNames[r.Question] = append(qCompNames[r.Question], w)
				}
				cs, ok := compStats[w]
				if !ok {
					cs = &compSOV{Name: w}
					compStats[w] = cs
				}
				cs.Mentions++
				if !containsStr(cs.Questions, r.Question) {
					cs.Questions = append(cs.Questions, r.Question)
				}
				// 首推：第一个被提及的品牌即该竞品
				if firstWord != "" && strings.EqualFold(firstWord, w) {
					cs.FirstMentions++
				}
			}
		}
	}

	resp := &gapResp{}
	seen := map[string]bool{}
	for q := range qTotal {
		if qTotal[q] < 1 {
			continue
		}
		miss := qMiss[q]
		cm := qComp[q]
		item := gapItem{Question: q, Misses: miss, CompetitorMention: cm, CompetitorNames: qCompNames[q]}
		item.Level = "low"
		if miss >= 1 {
			item.Level = "mid"
		}
		if miss >= 1 && cm >= 1 {
			item.Level = "high" // 品牌缺席且竞品在场 → 最高优先级
		}
		if miss >= 2 {
			item.Level = "high"
		}
		resp.Gaps = append(resp.Gaps, item)
		if item.Level == "high" {
			resp.HighGap++
		}
		if miss >= 1 {
			seen[q] = true
		}
	}
	sort.Slice(resp.Gaps, func(i, j int) bool {
		if resp.Gaps[i].Level != resp.Gaps[j].Level {
			return resp.Gaps[i].Level < resp.Gaps[j].Level
		}
		return resp.Gaps[i].Misses > resp.Gaps[j].Misses
	})
	resp.Total = len(resp.Gaps)
	// 品牌完全缺席的关键词
	for q := range qTotal {
		if !seen[q] && qMiss[q] == 0 && qTotal[q] > 0 && qComp[q] == 0 {
			// 有数据但既未命中品牌、也未出现竞品 → 中性
		}
	}
	for q := range qMiss {
		if qMiss[q] >= 1 {
			resp.UncoveredKeywords = append(resp.UncoveredKeywords, q)
		}
	}
	sort.Strings(resp.UncoveredKeywords)

	for _, cs := range compStats {
		resp.CompetitorSOV = append(resp.CompetitorSOV, *cs)
	}
	sort.Slice(resp.CompetitorSOV, func(i, j int) bool { return resp.CompetitorSOV[i].Mentions > resp.CompetitorSOV[j].Mentions })

	return resp
}

// ---------- 效果归因：前后期对比 ----------

type compareQ struct {
	Question    string  `json:"question"`
	BeforeRate  float64 `json:"before_rate"`
	AfterRate   float64 `json:"after_rate"`
	Delta       float64 `json:"delta"`
	BeforeHits  int     `json:"before_hits"`
	BeforeTotal int     `json:"before_total"`
	AfterHits   int     `json:"after_hits"`
	AfterTotal  int     `json:"after_total"`
}

type compareResp struct {
	BeforeDays int        `json:"before_days"`
	AfterDays  int        `json:"after_days"`
	BrandRate  [2]float64 `json:"brand_rate"` // [前期, 后期] 品牌出现率
	FirstRate  [2]float64 `json:"first_rate"` // [前期, 后期] 首推率
	Questions  []compareQ `json:"questions"`  // 逐题前后期对比
}

// CompareGeoIntel 效果归因：对比前后两个时间段的提及率变化。
// 前期 = [now-before_days-after_days, now-after_days]，后期 = [now-after_days, now]。
func CompareGeoIntel(c *gin.Context) {
	tid := TenantID(c)
	beforeDays := parseDay(c.DefaultQuery("before_days", "30"))
	afterDays := parseDay(c.DefaultQuery("after_days", "7"))
	if beforeDays <= 0 {
		beforeDays = 30
	}
	if afterDays <= 0 {
		afterDays = 7
	}
	now := time.Now()
	beforeStart := now.AddDate(0, 0, -(beforeDays + afterDays))
	afterStart := now.AddDate(0, 0, -afterDays)

	var before []models.CheckResult
	database.DB.Where("tenant_id = ? AND created_at >= ? AND created_at < ?", tid, beforeStart, afterStart).Find(&before)
	var after []models.CheckResult
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, afterStart).Find(&after)

	resp := &compareResp{BeforeDays: beforeDays, AfterDays: afterDays}
	resp.BrandRate = [2]float64{hitRate(before), hitRate(after)}
	resp.FirstRate = [2]float64{firstRate(before), firstRate(after)}

	type qAgg struct{ hits, total int }
	agg := func(rs []models.CheckResult) map[string]qAgg {
		m := map[string]qAgg{}
		for _, r := range rs {
			if r.ErrorMsg != "" {
				continue
			}
			a := m[r.Question]
			a.total++
			if r.Hit {
				a.hits++
			}
			m[r.Question] = a
		}
		return m
	}
	bq, aq := agg(before), agg(after)
	allQ := map[string]bool{}
	for q := range bq {
		allQ[q] = true
	}
	for q := range aq {
		allQ[q] = true
	}
	for q := range allQ {
		b, a := bq[q], aq[q]
		br, ar := 0.0, 0.0
		if b.total > 0 {
			br = float64(b.hits) / float64(b.total) * 100
		}
		if a.total > 0 {
			ar = float64(a.hits) / float64(a.total) * 100
		}
		resp.Questions = append(resp.Questions, compareQ{
			Question: q, BeforeRate: br, AfterRate: ar, Delta: ar - br,
			BeforeHits: b.hits, BeforeTotal: b.total, AfterHits: a.hits, AfterTotal: a.total,
		})
	}
	sort.Slice(resp.Questions, func(i, j int) bool { return resp.Questions[i].Delta > resp.Questions[j].Delta })
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func hitRate(rs []models.CheckResult) float64 {
	hit, total := 0, 0
	for _, r := range rs {
		if r.ErrorMsg != "" {
			continue
		}
		total++
		if r.Hit {
			hit++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total) * 100
}

func firstRate(rs []models.CheckResult) float64 {
	first, total := 0, 0
	for _, r := range rs {
		if r.ErrorMsg != "" {
			continue
		}
		total++
		if r.Hit && r.HitPosition == 1 {
			first++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(first) / float64(total) * 100
}

func ListOptTasks(c *gin.Context) {
	tid := TenantID(c)
	q := database.DB.Where("tenant_id = ?", tid)
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := c.Query("type"); v != "" {
		q = q.Where("type = ?", v)
	}
	var list []models.OptTask
	q.Order("status asc, priority asc, id desc").Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func UpdateOptTask(c *gin.Context) {
	var body struct {
		Status   string `json:"status"`
		Priority int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	var t models.OptTask
	if err := database.DB.Where("id = ? AND tenant_id = ?", c.Param("id"), TenantID(c)).First(&t).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "记录不存在"})
		return
	}
	updates := map[string]interface{}{}
	if body.Status != "" {
		updates["status"] = body.Status
		if body.Status == "done" {
			now := time.Now()
			updates["done_at"] = &now
		} else {
			// 重开任务时清掉复测结论，避免残留旧结论误导
			updates["done_at"] = nil
			updates["verify_status"] = ""
			updates["verify_note"] = ""
			updates["verified_at"] = nil
		}
	}
	if body.Priority > 0 {
		updates["priority"] = body.Priority
	}
	if len(updates) > 0 {
		database.DB.Model(&t).Updates(updates)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": t})
}

func DeleteOptTask(c *gin.Context) {
	database.DB.Where("id = ? AND tenant_id = ?", c.Param("id"), TenantID(c)).Delete(&models.OptTask{})
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// VerifyOptTaskLoop 闭环复测：对已完成的行动项回测效果。
// 做法是「任务完成时间点前 7 天 vs 后 7 天」对比，用同口径指标验证改动是否真的生效，
// 并把结论写回任务（improved / unchanged / worse）。
// 为什么需要：只有「完成」没有「验证」的行动清单是假闭环——运营会习惯性点完成，
// 但没人知道内容到底有没有被 AI 采纳。复测把这条链路闭合。
func VerifyOptTaskLoop(c *gin.Context) {
	tid := TenantID(c)
	var t models.OptTask
	if err := database.DB.Where("id = ? AND tenant_id = ?", c.Param("id"), tid).First(&t).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "行动项不存在"})
		return
	}
	if t.Status != "done" || t.DoneAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请先标记该行动项为「已完成」，再做复测"})
		return
	}

	pivot := *t.DoneAt
	var before, after []models.CheckResult
	database.DB.Where("tenant_id = ? AND created_at >= ? AND created_at < ? AND error_msg = ''",
		tid, pivot.AddDate(0, 0, -7), pivot).Find(&before)
	database.DB.Where("tenant_id = ? AND created_at >= ? AND created_at < ? AND error_msg = ''",
		tid, pivot, pivot.AddDate(0, 0, 7)).Find(&after)

	if len(after) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "完成后还没有新的巡检数据，无法复测。请先执行一次巡检"})
		return
	}
	if len(before) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "完成前 7 天没有巡检基线数据，无法对比"})
		return
	}

	// 按任务类型选取最能反映该任务效果的指标
	metricOf := func(rs []models.CheckResult) float64 {
		if len(rs) == 0 {
			return 0
		}
		switch t.Type {
		case "citation":
			// 引用率：按有引用的结果占比
			ids := make([]uint, 0, len(rs))
			for _, r := range rs {
				ids = append(ids, r.ID)
			}
			var cites []models.Citation
			database.DB.Where("tenant_id = ? AND result_id IN ? AND domain != ''", tid, ids).Find(&cites)
			seen := map[uint]bool{}
			for _, ct := range cites {
				seen[ct.ResultID] = true
			}
			return round1(float64(len(seen)) / float64(len(rs)) * 100)
		case "risk":
			// 风险回答率（越低越好）
			var risks []models.RiskWord
			database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&risks)
			hit := 0
			for _, r := range rs {
				low := strings.ToLower(r.Response)
				for _, rw := range risks {
					if rw.Word != "" && strings.Contains(low, strings.ToLower(rw.Word)) {
						hit++
						break
					}
				}
			}
			return round1(float64(hit) / float64(len(rs)) * 100)
		default:
			// gap / competitor / audit 等都看品牌出现率
			h := 0
			for _, r := range rs {
				if r.Hit {
					h++
				}
			}
			return round1(float64(h) / float64(len(rs)) * 100)
		}
	}

	bv := metricOf(before)
	av := metricOf(after)
	diff := round1(av - bv)

	// 风险类任务指标越低越好，判定方向相反
	metricName := "品牌出现率"
	improved := diff >= 5
	worse := diff <= -5
	if t.Type == "citation" {
		metricName = "引用率"
	} else if t.Type == "risk" {
		metricName = "风险回答率"
		improved = diff <= -5
		worse = diff >= 5
	}

	status := "unchanged"
	label := "无变化"
	if improved {
		status, label = "improved", "已改善"
	} else if worse {
		status, label = "worse", "变差"
	}
	note := fmt.Sprintf("完成前 7 天 %s %.1f%%（%d 条样本），完成后 7 天 %.1f%%（%d 条样本），变化 %+.1f 个百分点 → %s",
		metricName, bv, len(before), av, len(after), diff, label)

	now := time.Now()
	database.DB.Model(&t).Updates(map[string]interface{}{
		"verify_status": status, "verify_note": note, "verified_at": &now,
	})

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"verify_status": status, "metric": metricName,
		"before": bv, "after": av, "delta": diff,
		"before_samples": len(before), "after_samples": len(after),
		"note": note,
	}})
}

// LoopSummary 闭环概览：一眼看清「监测 → 诊断 → 行动 → 复测」四段各有多少积压。
func LoopSummary(c *gin.Context) {
	tid := TenantID(c)

	var open, doing, done, verified, improved int64
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND status = ?", tid, "open").Count(&open)
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND status = ?", tid, "doing").Count(&doing)
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND status = ?", tid, "done").Count(&done)
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND verify_status <> ''", tid).Count(&verified)
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND verify_status = ?", tid, "improved").Count(&improved)

	// 监测：近 7 天是否有巡检数据
	since := time.Now().AddDate(0, 0, -7)
	var recent int64
	database.DB.Model(&models.CheckResult{}).Where("tenant_id = ? AND created_at >= ?", tid, since).Count(&recent)

	// 诊断：关键词 / 话题簇
	var kwCnt, clCnt, unclassified int64
	database.DB.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND enabled = ?", tid, true).Count(&kwCnt)
	database.DB.Model(&models.KeywordCluster{}).Where("tenant_id = ?", tid).Count(&clCnt)
	database.DB.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND enabled = ? AND cluster_id = 0", tid, true).Count(&unclassified)

	steps := []gin.H{
		{"key": "monitor", "name": "监测", "value": recent, "unit": "条回答（近 7 天）",
			"done": recent > 0, "hint": "在「巡检任务」中执行巡检，或在 AI 平台配置后等待自动巡检"},
		{"key": "diagnose", "name": "诊断", "value": clCnt, "unit": "个话题簇",
			"done": clCnt > 0 && unclassified < kwCnt, "hint": "用「话题簇」的 AI 一键聚类，把关键词按搜索意图分组后看结构性缺口"},
		{"key": "action", "name": "行动", "value": open + doing, "unit": "项待办",
			"done": open+doing == 0, "hint": "点「一键生成行动清单」，系统会从巡检数据推导具体要做的事"},
		{"key": "verify", "name": "复测", "value": verified, "unit": "项已验证",
			"done": verified > 0 && improved > 0, "hint": "完成行动项后点「复测」，用完成前后 7 天数据验证是否真的生效"},
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"steps": steps,
		"counts": gin.H{
			"open": open, "doing": doing, "done": done,
			"verified": verified, "improved": improved, "pending_verify": done - verified,
			"keywords": kwCnt, "clusters": clCnt, "unclassified": unclassified,
			"recent_results": recent,
		},
	}})
}

// GenerateOptTasks 基于最新巡检数据重新生成优化行动清单（幂等：已存在的 open 任务不重复）
func GenerateOptTasks(c *gin.Context) {
	results, _, _ := queryResultsInRange(c, 30)
	tid := TenantID(c)
	created := 0

	var risks []models.RiskWord
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&risks)
	var facts []models.FactItem
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&facts)
	var comps []models.Competitor
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&comps)
	compWords := []string{}
	for _, cm := range comps {
		for _, w := range strings.Split(cm.Name, ",") {
			if w = strings.TrimSpace(w); w != "" {
				compWords = append(compWords, w)
			}
		}
	}

	// 品牌缺口任务
	qMiss := map[string]int{}
	qComp := map[string]int{}
	qTotal := map[string]int{}
	qRisk := map[string][]string{}
	qCompNames := map[string][]string{}
	for _, r := range results {
		if r.ErrorMsg != "" {
			continue
		}
		qTotal[r.Question]++
		if !r.Hit {
			qMiss[r.Question]++
		}
		low := strings.ToLower(r.Response)
		for _, w := range compWords {
			if strings.Contains(low, strings.ToLower(w)) {
				qComp[r.Question]++
				if !containsStr(qCompNames[r.Question], w) {
					qCompNames[r.Question] = append(qCompNames[r.Question], w)
				}
			}
		}
		for _, rw := range risks {
			if strings.Contains(low, strings.ToLower(rw.Word)) {
				qRisk[r.Question] = append(qRisk[r.Question], rw.Word)
			}
		}
	}
	for q := range qTotal {
		if qMiss[q] >= 2 {
			typ := "gap"
			title := fmt.Sprintf("补齐内容缺口：%s", q)
			detail := fmt.Sprintf("近 30 天该问题共查询 %d 次，品牌缺席 %d 次（出现率 %.0f%%）。请围绕该问题铺设含品牌词的自然回答，覆盖用户真实问法。", qTotal[q], qMiss[q], float64(qTotal[q]-qMiss[q])/float64(qTotal[q])*100)
			pri := 3
			if qComp[q] >= 1 {
				typ = "competitor"
				title = fmt.Sprintf("竞品抢占需反制：%s", q)
				detail = fmt.Sprintf("该问题品牌缺席 %d 次，且回答中出现了竞品（%s）。建议补齐对比内容，突出 %s 的差异化优势与适用场景。", qMiss[q], strings.Join(qCompNames[q], "、"), BrandOf(c))
				pri = 1
			}
			if createOptTaskIfMissing(tid, typ, title, detail, pri, q) {
				created++
			}
		}
	}

	// 风险任务（按风险词聚合，最多 5 条）
	riskWordCnt := map[string]int{}
	for _, words := range qRisk {
		for _, w := range words {
			riskWordCnt[w]++
		}
	}
	for w, cnt := range riskWordCnt {
		if cnt < 1 {
			continue
		}
		title := fmt.Sprintf("修复回答风险用语：「%s」", w)
		detail := fmt.Sprintf("有 %d 条 AI 回答命中了风险词「%s」，存在过度承诺/合规风险。请在品牌事实库中补充「不适用/边界」事实，并在内容投放中避免此类表述。", cnt, w)
		if createOptTaskIfMissing(tid, "risk", title, detail, 1, "risk:"+w) {
			created++
		}
	}

	// 事实库冲突任务
	for _, f := range facts {
		if strings.TrimSpace(f.NotFact) == "" {
			continue
		}
		cnt := 0
		for _, r := range results {
			if r.ErrorMsg == "" && strings.Contains(strings.ToLower(r.Response), strings.ToLower(f.NotFact)) {
				cnt++
			}
		}
		if cnt >= 1 {
			title := fmt.Sprintf("人工校正：%s 表述被 AI 错误引用", f.NotFact)
			detail := fmt.Sprintf("「%s」作为事实库中的边界/禁区表述，仍有 %d 条回答命中。请核实回答上下文并校正事实库，必要时向平台提交纠错。", f.NotFact, cnt)
			if createOptTaskIfMissing(tid, "risk", title, detail, 2, "fact:"+f.NotFact, "observe") {
				created++
			}
		}
	}

	// 引用提升任务：引用率 < 30% 的平台
	citeByPlatform := map[string]int{}
	totalByPlatform := map[string]int{}
	if len(results) > 0 {
		var cites []models.Citation
		database.DB.Where("tenant_id = ?", tid).Find(&cites)
		for _, ct := range cites {
			citeByPlatform[ct.PlatformName]++
		}
	}
	for _, r := range results {
		if r.ErrorMsg == "" {
			totalByPlatform[r.PlatformName]++
		}
	}
	for pname, tot := range totalByPlatform {
		if tot >= 5 {
			rate := float64(citeByPlatform[pname]) / float64(tot)
			if rate < 0.3 {
				title := fmt.Sprintf("提升「%s」引用率", pname)
				detail := fmt.Sprintf("「%s」近 30 天 %d 条成功回答中仅 %d 条带引用（%.0f%%）。建议在该平台铺设可被引用的权威内容（官网文章/百科/媒体报道）。", pname, tot, citeByPlatform[pname], rate*100)
				if createOptTaskIfMissing(tid, "citation", title, detail, 2, "citation:"+pname) {
					created++
				}
			}
		}
	}

	// ---- 话题簇驱动的闭环信号（GEO 管线第⑧步「强化闭环」）----
	// 簇是从「AI 如何组织话题」的视角切的，因此簇级信号比词级信号更能命中
	// 结构性缺口：某个话题簇整体 0 覆盖，说明这一类问题我们完全没有内容资产。
	created += generateClusterDrivenTasks(tid, results)

	// 信源建设任务：竞品被引用而品牌未被引用的域名
	created += generateSourceTasks(tid, results)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"created": created}})
}

// generateClusterDrivenTasks 从话题簇派生行动任务：
//  1. 某个簇「有词但一条都没测」→ 提示去巡检（数据缺口）
//  2. 某个簇覆盖率 < 20% → 高优内容缺口（按簇批量处理，比逐词精准）
//  3. 某个簇「未归类」占比过高 → 提示先做聚类
func generateClusterDrivenTasks(tid uint, results []models.CheckResult) int {
	created := 0

	var kws []models.GeoKeyword
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&kws)
	if len(kws) == 0 {
		return 0
	}
	var clusters []models.KeywordCluster
	database.DB.Where("tenant_id = ?", tid).Find(&clusters)
	clName := map[uint]string{}
	for _, cl := range clusters {
		clName[cl.ID] = cl.Name
	}

	kwCluster := map[uint]uint{}
	byCluster := map[uint][]uint{}
	for _, k := range kws {
		kwCluster[k.ID] = k.ClusterID
		byCluster[k.ClusterID] = append(byCluster[k.ClusterID], k.ID)
	}

	// 簇级聚合（仅统计成功结果）
	type cAgg struct {
		total, hits int
		questions   map[string]int // 问题 → 缺席次数
	}
	agg := map[uint]*cAgg{}
	for _, r := range results {
		if r.ErrorMsg != "" {
			continue
		}
		cl := kwCluster[r.KeywordID]
		a, ok := agg[cl]
		if !ok {
			a = &cAgg{questions: map[string]int{}}
			agg[cl] = a
		}
		a.total++
		if r.Hit {
			a.hits++
		} else {
			a.questions[r.Question]++
		}
	}

	for clID, kwIDs := range byCluster {
		if clID == 0 {
			continue // 未归类单独处理
		}
		name := clName[clID]
		if name == "" {
			continue
		}
		a := agg[clID]
		if a == nil || a.total == 0 {
			// ① 该簇从未被巡检过
			if len(kwIDs) >= 3 {
				detail := fmt.Sprintf("话题簇「%s」下已有 %d 个问题，但一条巡检记录都没有。建议先执行巡检，拿到该话题的可见度基线，再决定内容投入方向。", name, len(kwIDs))
				if createOptTaskIfMissing(tid, "gap", fmt.Sprintf("话题簇未测：%s", name), detail, 3, "cluster-untested:"+fmt.Sprint(clID)) {
					created++
				}
			}
			continue
		}
		cov := float64(a.hits) / float64(a.total) * 100
		if cov < 20 {
			// ② 簇整体覆盖不足：列出缺席最多的问题，形成批量选题
			top := topMissed(a.questions, 5)
			detail := fmt.Sprintf("话题簇「%s」共 %d 个问题、%d 条回答，品牌出现率仅 %.0f%%（%d 条命中）。这是结构性缺口——整个话题缺少能引用你的内容资产。\n\n优先攻克以下高频缺席问题：\n%s\n\n建议：针对该话题产出 1~2 篇体系化长文（覆盖多个子问题），并在「内容投放」中分发。",
				name, len(kwIDs), a.total, cov, a.hits, strings.Join(top, "\n"))
			pri := 2
			if cov == 0 {
				pri = 1
			}
			if createOptTaskIfMissing(tid, "gap", fmt.Sprintf("话题簇覆盖不足：%s（%.0f%%）", name, cov), detail, pri, "cluster-low:"+fmt.Sprint(clID)) {
				created++
			}
		}
	}

	// ③ 未归类占比过高 → 先聚类
	if n := len(byCluster[0]); n >= 10 {
		pct := float64(n) / float64(len(kws)) * 100
		if pct >= 30 {
			detail := fmt.Sprintf("当前 %d/%d（%.0f%%）的问题尚未归入任何话题簇，簇级诊断会失真。建议在「GEO 智能 → 话题簇」中执行「AI 一键聚类」，让系统按搜索意图自动分组。", n, len(kws), pct)
			if createOptTaskIfMissing(tid, "gap", "关键词尚未聚类", detail, 3, "cluster-unclassified") {
				created++
			}
		}
	}
	return created
}

// topMissed 取缺席次数最高的 N 个问题，格式化为可读列表
func topMissed(m map[string]int, n int) []string {
	type kv struct {
		q string
		c int
	}
	list := make([]kv, 0, len(m))
	for q, c := range m {
		list = append(list, kv{q, c})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].c > list[j].c })
	out := make([]string, 0, n)
	for i := 0; i < len(list) && i < n; i++ {
		out = append(out, fmt.Sprintf("%d. %s（缺席 %d 次）", i+1, list[i].q, list[i].c))
	}
	return out
}

// generateSourceTasks 信源建设任务：竞品被 AI 引用、而品牌从未被引用的域名。
// 依据「AI 引用偏好」的反向工程：竞品能拿到引用，说明该信源被引擎信任，
// 我们把品牌内容铺到同一批域名上，是最高效的引用提升路径。
func generateSourceTasks(tid uint, results []models.CheckResult) int {
	created := 0

	var comps []models.Competitor
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&comps)
	if len(comps) == 0 {
		return 0
	}
	compWords := []string{}
	for _, cm := range comps {
		for _, w := range strings.Split(cm.Name, ",") {
			if w = strings.TrimSpace(w); w != "" {
				compWords = append(compWords, strings.ToLower(w))
			}
		}
	}
	if len(compWords) == 0 {
		return 0
	}

	var cites []models.Citation
	database.DB.Where("tenant_id = ? AND domain != ''", tid).Find(&cites)
	if len(cites) == 0 {
		return 0
	}

	resultMap := map[uint]models.CheckResult{}
	ids := make([]uint, 0, len(cites))
	for _, ct := range cites {
		ids = append(ids, ct.ResultID)
	}
	var rs []models.CheckResult
	database.DB.Where("tenant_id = ? AND id IN ?", tid, ids).Find(&rs)
	for _, r := range rs {
		resultMap[r.ID] = r
	}

	compDomain := map[string]int{}
	brandDomain := map[string]int{}
	for _, ct := range cites {
		r, ok := resultMap[ct.ResultID]
		if !ok {
			continue
		}
		low := strings.ToLower(r.Response)
		isComp := false
		for _, w := range compWords {
			if strings.Contains(low, w) {
				isComp = true
				break
			}
		}
		if isComp {
			compDomain[ct.Domain]++
		}
		if r.Hit {
			brandDomain[ct.Domain]++
		}
	}

	// 只取竞品有、品牌无的域名，按竞品被引用次数降序，最多 5 条
	type kv struct {
		d string
		c int
	}
	gaps := make([]kv, 0, len(compDomain))
	for d, c := range compDomain {
		if brandDomain[d] == 0 && c >= 2 {
			gaps = append(gaps, kv{d, c})
		}
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].c > gaps[j].c })
	if len(gaps) > 5 {
		gaps = gaps[:5]
	}
	for _, g := range gaps {
		detail := fmt.Sprintf("域名「%s」在近 30 天有 %d 条含竞品的 AI 回答引用，但从未引用过你的品牌。\n\n说明该信源已被 AI 引擎信任，是最省力的引用突破口。建议：\n1. 查看该域名上的相关页面，确认收录偏好（如百科式条目、榜单、对比评测）；\n2. 用相同体裁产出品牌相关内容并提交（投稿/PR/合作）；\n3. 复测观察该域名的引用情况。", g.d, g.c)
		if createOptTaskIfMissing(tid, "citation", fmt.Sprintf("信源建设：%s", g.d), detail, 2, "source-gap:"+g.d) {
			created++
		}
	}
	return created
}

// createOptTaskIfMissing 幂等创建行动任务
func createOptTaskIfMissing(tid uint, typ, title, detail string, pri int, source string, riskLevel ...string) bool {
	var cnt int64
	database.DB.Model(&models.OptTask{}).Where("tenant_id = ? AND type = ? AND source = ? AND status = ?", tid, typ, source, "open").Count(&cnt)
	if cnt > 0 {
		return false
	}
	rl := "low"
	if len(riskLevel) > 0 && riskLevel[0] != "" {
		rl = riskLevel[0]
	} else {
		switch typ {
		case "risk":
			rl = "high"
		case "audit":
			rl = "observe"
		}
	}
	t := models.OptTask{TenantID: tid, Type: typ, Title: title, Detail: detail, Priority: pri, RiskLevel: rl, Status: "open", Source: source}
	if err := database.DB.Create(&t).Error; err == nil {
		return true
	}
	return false
}

// ---------- 阵地地图 ----------

type geoChannel struct {
	Name     string `json:"name"`     // 阵地名
	Market   string `json:"market"`   // cn / global
	Weight   string `json:"weight"`   // AI 引用权重：high / mid / low
	Priority string `json:"priority"` // 建设优先级：high / mid / low
	What     string `json:"what"`     // 建什么
	Pace     string `json:"pace"`     // 节奏
}

// ListChannels 阵地地图：预置 19 个 GEO 建设阵地（按 AI 真实引用语料标定的中文/海外阵地）。
func ListChannels(c *gin.Context) {
	channels := []geoChannel{
		{Name: "百度百科", Market: "cn", Weight: "high", Priority: "high", What: "品牌词条 + 产品词条，含可验证事实与时间", Pace: "1-2 周建词条，持续更新"},
		{Name: "知乎", Market: "cn", Weight: "high", Priority: "high", What: "品牌相关问题专业回答 + 行业洞察文章", Pace: "每周 1-2 条高质量回答"},
		{Name: "微信公众号", Market: "cn", Weight: "high", Priority: "high", What: "品牌深度文章 + 案例，便于被引用", Pace: "每周 1 篇深度文"},
		{Name: "行业垂直评测站", Market: "cn", Weight: "high", Priority: "high", What: "产品评测条目 + 对比榜单", Pace: "1 次建条目，季度更新"},
		{Name: "权威媒体报道", Market: "cn", Weight: "high", Priority: "mid", What: "以真实新闻事件为前提的媒体稿件", Pace: "按事件节奏，不可购买"},
		{Name: "今日头条", Market: "cn", Weight: "mid", Priority: "mid", What: "品牌资讯 + 行业解读", Pace: "每周 2-3 条"},
		{Name: "百家号", Market: "cn", Weight: "mid", Priority: "mid", What: "品牌文章 + 百度生态内容", Pace: "每周 1-2 篇"},
		{Name: "36氪 / 虎嗅", Market: "cn", Weight: "mid", Priority: "mid", What: "行业报道 + 深度分析", Pace: "季度 1 次行业投稿"},
		{Name: "小红书", Market: "cn", Weight: "mid", Priority: "mid", What: "种草笔记 + 真实使用体验", Pace: "每周 2-3 篇"},
		{Name: "抖音", Market: "cn", Weight: "mid", Priority: "low", What: "品牌短视频 + 口播", Pace: "每周 1-2 条"},
		{Name: "B站", Market: "cn", Weight: "low", Priority: "low", What: "评测视频 + 教程", Pace: "月度 1 条"},
		{Name: "搜狐号 / 网易号", Market: "cn", Weight: "low", Priority: "low", What: "新闻稿分发", Pace: "月度 1-2 篇"},
		{Name: "腾讯新闻", Market: "cn", Weight: "low", Priority: "low", What: "媒体报道转载", Pace: "按需"},
		{Name: "百度知道 / 问答", Market: "cn", Weight: "low", Priority: "low", What: "问答口碑内容", Pace: "持续维护"},
		{Name: "Wikipedia", Market: "global", Weight: "high", Priority: "mid", What: "品牌词条（需第三方可靠来源）", Pace: "按可验证来源建立"},
		{Name: "G2 / Capterra", Market: "global", Weight: "mid", Priority: "mid", What: "产品评价页 + 用户评价", Pace: "1 次建页，持续积累评价"},
		{Name: "Reddit", Market: "global", Weight: "mid", Priority: "low", What: "社区真实讨论 + AMA", Pace: "按需参与"},
		{Name: "YouTube", Market: "global", Weight: "low", Priority: "low", What: "品牌视频 + 教程", Pace: "月度 1 条"},
		{Name: "Product Hunt", Market: "global", Weight: "low", Priority: "low", What: "产品发布页", Pace: "发布节点 1 次"},
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": channels})
}

// ---------- 网站 GEO 审计 ----------

type auditDims struct {
	Label string `json:"label"`
	Score int    `json:"score"`
	Pass  bool   `json:"pass"`
	Note  string `json:"note"`
}

// RunAudit 网站 GEO 审计 —— v1.0.33 起与「百度分析 → 站点体检」共用同一套四层加权评分核心
// （handlers.runSiteAuditCore），彻底消除历史遗留的双口径问题：
// 同一站点无论在哪个入口跑，分数、等级、待优化项、行动工单都完全一致，且写同一张 audit_results 表。
func RunAudit(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "请提供要审计的站点 URL"})
		return
	}
	target := normalizeAuditURL(body.URL)
	if target == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "URL 格式错误"})
		return
	}

	tid := TenantID(c)
	client := &http.Client{Timeout: 20 * time.Second}
	rep := runSiteAuditCore(client, target, BrandOf(c))
	id := saveAuditResult(tid, target, rep)
	spawnAuditTasks(tid, target, rep)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"id": id, "url": rep.URL, "score": rep.Score, "level": rep.Level,
		"dimensions": rep.Dimensions, "findings": rep.Findings,
		"layers": rep.Layers, "grade_dist": rep.GradeDist,
		"overall_note": rep.OverallNote, "host": rep.Host,
	}})
}

func ListAudits(c *gin.Context) {
	var list []models.AuditResult
	database.DB.Where("tenant_id = ?", TenantID(c)).Order("id desc").Limit(50).Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// fetchHeadOK 请求目标 URL 并判断是否 200
func fetchHeadOK(client *http.Client, u string) bool {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GeoAudit/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	return resp.StatusCode == 200
}

// fetchAsBot 以指定爬虫 UA 请求 URL，返回状态码与是否拿到响应。
// 用于「AI 爬虫实测」：站点可能对普通浏览器放行、却对 AI 爬虫返回 403，
// 必须用真实 UA 逐个验证，才知道 AI 到底有没有能力抓到这个站。
func fetchAsBot(client *http.Client, u string, botUA string) (int, bool) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("User-Agent", botUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	return resp.StatusCode, true
}

// ---------- llms.txt / Schema 生成 ----------

// GenerateLLMS 基于品牌事实库生成 llms.txt 内容
func GenerateLLMS(c *gin.Context) {
	tid := TenantID(c)
	brand := BrandOf(c)
	var facts []models.FactItem
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Order("id asc").Find(&facts)
	var b strings.Builder
	b.WriteString("# " + brand + "\n\n")
	b.WriteString("> 本文件为供大语言模型（LLM）阅读的品牌介绍与事实说明，便于 AI 在回答中准确引用。\n\n")
	for _, f := range facts {
		b.WriteString("## " + f.Category + "\n")
		if f.Question != "" {
			b.WriteString("- 适用问题：" + f.Question + "\n")
		}
		b.WriteString("- " + f.Fact + "\n")
		if f.NotFact != "" {
			b.WriteString("- 边界/禁区：" + f.NotFact + "\n")
		}
		b.WriteString("\n")
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"content": b.String(), "filename": "llms.txt"}})
}

// GenerateSchema 基于品牌事实库生成 schema.org JSON-LD。
// 生产三类最被 AI 引擎采信的结构化数据：
//
//	① Organization —— 品牌实体本身（含 sameAs 实体锚定，帮助 AI 消歧）
//	② FAQPage      —— 事实库中的问答对，直接对应「用户会怎么问 AI」
//	③ HowTo/ItemList —— 事实条目归类后的结构化呈现
//
// 返回 multiple 数组，前端可分文件写入站点。
func GenerateSchema(c *gin.Context) {
	tid := TenantID(c)
	brand := BrandOf(c)
	var facts []models.FactItem
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Order("id asc").Find(&facts)

	// 站点信息（用于 sameAs / url 实体锚定）
	siteURL := readSetting(tid, "site_url")
	if siteURL == "" {
		siteURL = readSetting(0, "site_url")
	}
	phone := readSetting(tid, KeyServicePhone)

	// ---- ① Organization：品牌实体 ----
	org := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     brand,
	}
	if siteURL != "" {
		org["url"] = siteURL
		// sameAs 是实体锚定的关键：告诉 AI 「这个官网、这个百科、这个公众号是同一个人/机构」
		org["sameAs"] = []string{siteURL}
	}
	if phone != "" {
		org["contactPoint"] = map[string]interface{}{
			"@type": "ContactPoint", "telephone": phone, "contactType": "customer service",
		}
	}
	// 按事实分类归纳到 knowsAbout（比 JSON 塞 hasCredential 语义正确）
	cats := map[string]bool{}
	for _, f := range facts {
		if c := strings.TrimSpace(f.Category); c != "" {
			cats[c] = true
		}
	}
	if len(cats) > 0 {
		known := make([]string, 0, len(cats))
		for c := range cats {
			known = append(known, c)
		}
		sort.Strings(known)
		org["knowsAbout"] = known
	}
	// description 取「品类」或「介绍/简介」类事实
	for _, f := range facts {
		if c := f.Category; c == "品类" || c == "简介" || c == "介绍" || c == "品牌介绍" {
			if d, _ := org["description"].(string); d == "" {
				org["description"] = f.Fact
			}
		}
	}
	if _, ok := org["description"]; !ok {
		org["description"] = brand
	}

	// ---- ② FAQPage：事实库中的问答对（AI 引擎最喜欢直接引用的形态）----
	type qaPair struct{ Q, A string }
	pairs := make([]qaPair, 0)
	for _, f := range facts {
		q := strings.TrimSpace(f.Question)
		if q == "" {
			continue
		}
		pairs = append(pairs, qaPair{Q: q, A: f.Fact})
	}
	faq := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "FAQPage",
		"mainEntity": func() []map[string]interface{} {
			out := make([]map[string]interface{}, 0, len(pairs))
			for _, p := range pairs {
				out = append(out, map[string]interface{}{
					"@type": "Question",
					"name":  p.Q,
					"acceptedAnswer": map[string]interface{}{
						"@type": "Answer",
						"text":  p.A,
					},
				})
			}
			return out
		}(),
	}

	// ---- ③ ItemList：事实清单的结构化呈现（无问答时的兜底形态）----
	type factOut struct {
		Type string `json:"@type"`
		Name string `json:"name"`
		Text string `json:"text"`
	}
	listItems := make([]map[string]interface{}, 0, len(facts))
	for i, f := range facts {
		name := strings.TrimSpace(f.Category)
		if name == "" {
			name = brand + " 品牌事实"
		}
		listItems = append(listItems, map[string]interface{}{
			"@type":    "ListItem",
			"position": i + 1,
			"item": factOut{
				Type: "DefinedTerm",
				Name: name,
				Text: f.Fact,
			},
		})
	}
	itemList := map[string]interface{}{
		"@context":        "https://schema.org",
		"@type":           "ItemList",
		"name":            brand + " 品牌事实清单",
		"numberOfItems":   len(listItems),
		"itemListElement": listItems,
	}

	blocks := []map[string]interface{}{org}
	if len(pairs) > 0 {
		blocks = append(blocks, faq)
	}
	if len(listItems) > 0 {
		blocks = append(blocks, itemList)
	}

	// 兼容旧版：保留 content 字段（输出 Organization 主块），新增 blocks 数组供分文件写入
	mainOut, _ := json.MarshalIndent(org, "", "  ")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"content":  string(mainOut),
		"filename": "schema.jsonld",
		"blocks":   blocks,
		"summary": gin.H{
			"organization": 1,
			"faq":          len(pairs),
			"itemlist":     len(listItems),
		},
	}})
}

// ---------- 工具函数 ----------

func pageArgs(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func firstMatch(s, pattern string) string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func truncateCN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}
