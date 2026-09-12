package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/config"
	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/ai"
	"geo-tool/services/ai_platform"
	"geo-tool/services/points"
)

// ============================================================
// 话题簇（提示词聚类）：GEO 八阶段管线的第③步
//
// 为什么需要：AI 引擎按「话题」组织知识，而不是按「词」。若把几百个关键词
// 平铺统计，整体覆盖率会被少数品牌词拉高，掩盖「行业词完全缺席」这类
// 结构性缺口。按簇聚合后，才能回答「哪一类问题我们颗粒无收」。
//
// 簇维度 = 搜索意图（认知/对比/决策/品牌），与主流 GEO 平台的
// Prompt Clustering 口径一致，直接对接后续的覆盖率-引用率-准确率三元组。
// ============================================================

// intentLabel 意图中文名
func intentLabel(k string) string {
	switch k {
	case models.IntentInformational:
		return "认知型"
	case models.IntentCommercial:
		return "对比型"
	case models.IntentTransactional:
		return "决策型"
	case models.IntentNavigational:
		return "品牌型"
	}
	return "未分类"
}

// validIntent 校验意图值，非法则回退认知型
func validIntent(k string) string {
	switch k {
	case models.IntentInformational, models.IntentCommercial,
		models.IntentTransactional, models.IntentNavigational:
		return k
	}
	return models.IntentInformational
}

// defaultClusterColor 按意图给默认色，前端不传色时使用
func defaultIntentColor(k string) string {
	switch k {
	case models.IntentInformational:
		return "#165DFF"
	case models.IntentCommercial:
		return "#722ED1"
	case models.IntentTransactional:
		return "#00B42A"
	case models.IntentNavigational:
		return "#FF7D00"
	}
	return "#86909C"
}

// ClusterStat 簇统计（含覆盖/引用/命中三元组）
type ClusterStat struct {
	models.KeywordCluster
	KeywordCount int     `json:"keyword_count"` // 簇内关键词数
	CrawledCount int     `json:"crawled_count"` // 有巡检结果的关键词数
	HitCount     int     `json:"hit_count"`     // 命中品牌的关键词数
	Coverage     float64 `json:"coverage"`      // 品牌出现率（%）
	CitationRate float64 `json:"citation_rate"` // 引用率（%）
	AccuracyRate float64 `json:"accuracy_rate"` // 事实一致率（%）
	AvgMention   float64 `json:"avg_mention"`   // 平均提及次数
	SampleCount  int     `json:"sample_count"`  // 样本量（结果条数）
	IntentLabel  string  `json:"intent_label"`
	Status       string  `json:"status"` // empty(无词)/untested(未测)/weak(偏弱)/normal(正常)/strong(领先)
}

// ListClusters 话题簇列表（带统计）
// GET /api/clusters?days=30
func ListClusters(c *gin.Context) {
	tid := TenantID(c)
	days := parseDay(c.DefaultQuery("days", "30"))
	since := time.Now().AddDate(0, 0, -days)

	var clusters []models.KeywordCluster
	database.DB.Where("tenant_id = ?", tid).Order("sort_order asc, id asc").Find(&clusters)

	// 关键词 → 簇归属
	var kws []models.GeoKeyword
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&kws)
	kwCluster := map[uint]uint{}
	kwByCluster := map[uint][]uint{}
	for _, k := range kws {
		kwCluster[k.ID] = k.ClusterID
		kwByCluster[k.ClusterID] = append(kwByCluster[k.ClusterID], k.ID)
	}

	// 近 N 天结果，按关键词 → 簇聚合
	var results []models.CheckResult
	database.DB.Where("tenant_id = ? AND created_at >= ? AND error_msg = ''", tid, since).Find(&results)

	type agg struct {
		total, hits, cites int // 结果条数 / 命中条数 / 有引用的条数
		facts, factsOK     int // 参与一致性判定的条数 / 未冲突条数
		mentions           int // 提及次数合计
	}
	byCluster := map[uint]*agg{}
	for _, r := range results {
		cl := kwCluster[r.KeywordID]
		a, ok := byCluster[cl]
		if !ok {
			a = &agg{}
			byCluster[cl] = a
		}
		a.total++
		a.mentions += r.MentionCount
		if r.Hit {
			a.hits++
		}
	}

	// 引用率：同一条结果可能有多条引用，去重后按「有引用的结果占比」计
	if len(results) > 0 {
		ids := make([]uint, 0, len(results))
		kwOfResult := map[uint]uint{}
		for _, r := range results {
			ids = append(ids, r.ID)
			kwOfResult[r.ID] = r.KeywordID
		}
		var cites []models.Citation
		database.DB.Where("tenant_id = ? AND result_id IN ? AND domain != ''", tid, ids).Find(&cites)
		seen := map[uint]bool{}
		for _, ct := range cites {
			if seen[ct.ResultID] {
				continue
			}
			seen[ct.ResultID] = true
			if a, ok := byCluster[kwCluster[kwOfResult[ct.ResultID]]]; ok {
				a.cites++
			}
		}
	}

	// 事实一致率：仅统计命中的回答（未命中无从比对），口径与指标总览一致
	var facts []models.FactItem
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Find(&facts)
	if len(facts) > 0 {
		for _, r := range results {
			if !r.Hit {
				continue
			}
			a, ok := byCluster[kwCluster[r.KeywordID]]
			if !ok {
				continue
			}
			a.facts++
			if !factConflict(r.Response, facts) {
				a.factsOK++
			}
		}
	}

	out := make([]ClusterStat, 0, len(clusters)+1)
	build := func(id uint, cl models.KeywordCluster, kwCount int) ClusterStat {
		a := byCluster[id]
		if a == nil {
			a = &agg{}
		}
		st := ClusterStat{
			KeywordCluster: cl,
			KeywordCount:   kwCount,
			CrawledCount:   a.total,
			HitCount:       a.hits,
			SampleCount:    a.total,
			IntentLabel:    intentLabel(cl.Intent),
		}
		if cl.Color == "" {
			st.Color = defaultIntentColor(cl.Intent)
		}
		if a.total > 0 {
			st.Coverage = round1(float64(a.hits) / float64(a.total) * 100)
			st.CitationRate = round1(float64(a.cites) / float64(a.total) * 100)
			st.AvgMention = round1(float64(a.mentions) / float64(a.total))
		}
		if a.facts > 0 {
			st.AccuracyRate = round1(float64(a.factsOK) / float64(a.facts) * 100)
		}
		switch {
		case kwCount == 0:
			st.Status = "empty"
		case a.total == 0:
			st.Status = "untested"
		case st.Coverage >= 60:
			st.Status = "strong"
		case st.Coverage >= 30:
			st.Status = "normal"
		default:
			st.Status = "weak"
		}
		return st
	}

	for _, cl := range clusters {
		kwCount := len(kwByCluster[cl.ID])
		out = append(out, build(cl.ID, cl, kwCount))
	}

	// 未归类（cluster_id=0）作为一个虚拟簇展示，便于把散词归拢
	if n := len(kwByCluster[0]); n > 0 {
		un := models.KeywordCluster{ID: 0, TenantID: tid, Name: "未归类", Intent: models.IntentInformational,
			Description: "尚未归入任何话题簇的关键词", Color: "#86909C"}
		out = append(out, build(0, un, n))
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out, "days": days})
}

// CreateCluster 新建话题簇
func CreateCluster(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Name        string `json:"name"`
		Intent      string `json:"intent"`
		Description string `json:"description"`
		Color       string `json:"color"`
		SortOrder   int    `json:"sort_order"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请填写簇名称"})
		return
	}
	if w, reason, hit := checkBannedWord(tid, req.Name+" "+req.Description); hit {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "包含违禁词「" + w + "」(" + reason + ")，请修改后重试"})
		return
	}
	cl := models.KeywordCluster{
		TenantID: tid, Name: req.Name, Intent: validIntent(req.Intent),
		Description: strings.TrimSpace(req.Description), SortOrder: req.SortOrder,
		Color: req.Color,
	}
	if cl.Color == "" {
		cl.Color = defaultIntentColor(cl.Intent)
	}
	if err := database.DB.Create(&cl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已创建", "data": cl})
}

// UpdateCluster 更新话题簇
func UpdateCluster(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var cl models.KeywordCluster
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&cl).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "话题簇不存在"})
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Intent      *string `json:"intent"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
		SortOrder   *int    `json:"sort_order"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		cl.Name = strings.TrimSpace(*req.Name)
	}
	if req.Intent != nil {
		cl.Intent = validIntent(*req.Intent)
	}
	if req.Description != nil {
		cl.Description = strings.TrimSpace(*req.Description)
	}
	if req.Color != nil && *req.Color != "" {
		cl.Color = *req.Color
	}
	if req.SortOrder != nil {
		cl.SortOrder = *req.SortOrder
	}
	database.DB.Save(&cl)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
}

// DeleteCluster 删除话题簇（簇内关键词自动退回未归类，不删词）
func DeleteCluster(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var cl models.KeywordCluster
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&cl).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "话题簇不存在"})
		return
	}
	database.DB.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND cluster_id = ?", tid, cl.ID).
		Update("cluster_id", 0)
	database.DB.Delete(&cl)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除，簇内关键词已退回未归类"})
}

// AssignCluster 批量调整关键词归属（把词归入某簇 / 移出）
// POST /api/clusters/assign  {cluster_id: 3, keyword_ids: [1,2,3]}
func AssignCluster(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		ClusterID  uint   `json:"cluster_id"`
		KeywordIDs []uint `json:"keyword_ids"`
		Question   string `json:"question"` // 或用问题文本直接归入（前端从建议里点选时用）
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.ClusterID > 0 {
		var cnt int64
		database.DB.Model(&models.KeywordCluster{}).
			Where("id = ? AND tenant_id = ?", req.ClusterID, tid).Count(&cnt)
		if cnt == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "目标话题簇不存在"})
			return
		}
	}
	// 按问题文本归档：不存在则新建关键词（AI 聚类采纳时常用）
	if q := strings.TrimSpace(req.Question); q != "" {
		var k models.GeoKeyword
		if err := database.DB.Where("tenant_id = ? AND question = ?", tid, q).First(&k).Error; err != nil {
			k = models.GeoKeyword{TenantID: tid, Question: q, Enabled: true, ClusterID: req.ClusterID}
			database.DB.Create(&k)
		} else if k.ClusterID != req.ClusterID {
			k.ClusterID = req.ClusterID
			database.DB.Save(&k)
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已归入"})
		return
	}
	if len(req.KeywordIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请选择要归类的问题"})
		return
	}
	res := database.DB.Model(&models.GeoKeyword{}).
		Where("tenant_id = ? AND id IN ?", tid, req.KeywordIDs).
		Update("cluster_id", req.ClusterID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已调整 " + strconv.Itoa(int(res.RowsAffected)) + " 条"})
}

// ---------- AI 一键聚类 ----------

// clusterSuggestion AI 返回的单个簇建议。
// 用编号（ids）而非问题原文回传：让 AI 逐字复制上百条问题会撑爆输出 token，
// 导致响应被截断（实测 117 条问题在 3000 max_tokens 下返回空内容）。
// 编号方案把输出压到原来的 1/10 左右，且不受改写/标点差异影响。
type clusterSuggestion struct {
	Name        string   `json:"name"`
	Intent      string   `json:"intent"`
	Description string   `json:"description"`
	IDs         []int    `json:"ids"`
	Questions   []string `json:"questions"` // 兼容：AI 若仍返回原文，走文本匹配
}

// aiClusterSchema AI 应返回的 JSON 结构
type aiClusterSchema struct {
	Clusters []clusterSuggestion `json:"clusters"`
}

// GenClusters AI 一键聚类：把扁平关键词按搜索意图自动分簇。
// POST /api/clusters/generate { overwrite?: bool, model?: "platformID" }
func GenClusters(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Overwrite  bool   `json:"overwrite"`   // true=清空现有簇重建（关键词保留）
		PlatformID uint   `json:"platform_id"` // 指定平台，0=自动选第一个可用
		Extra      string `json:"extra"`       // 补充业务背景，帮助 AI 理解行业
	}
	_ = jsonBody(c, &req)

	var kws []models.GeoKeyword
	database.DB.Where("tenant_id = ? AND enabled = ?", tid, true).Order("id asc").Find(&kws)
	if len(kws) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "还没有关键词，请先在「关键词管理」中添加"})
		return
	}

	platforms := EffectivePlatforms(tid)
	var chosen *models.AiPlatform
	for i := range platforms {
		// 可用性判定统一走 ai_platform.Usable（唯一权威来源）：
		// 已启用 && base_url 非空 && （有 Key || 本地自托管服务）。
		// 原来手写「Enabled && APIKey != ""」会把本地自托管平台（Ollama 等无 Key）误判为不可用，
		// 与巡检/AI 助手/AI 平台页的判定口径不一致。
		if !ai_platform.Usable(&platforms[i]) {
			continue
		}
		if req.PlatformID > 0 {
			if platforms[i].ID == req.PlatformID {
				chosen = &platforms[i]
				break
			}
			continue
		}
		chosen = &platforms[i]
		break
	}
	if chosen == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "没有可用的 AI 平台，请先在「AI 平台」中配置密钥并启用"})
		return
	}
	if err := points.DeductOne(tid, "GEO 话题簇聚类"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	// 关键词列表（限 120 条，控制 token）
	limit := len(kws)
	if limit > 120 {
		limit = 120
	}
	qs := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		qs = append(qs, strconv.Itoa(i+1)+". "+kws[i].Question)
	}
	brand := brandNameOf(tid)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	sys := "你是 GEO（生成式引擎优化）提示词策略专家，擅长把用户的关键词/问题按搜索意图聚成话题簇。" +
		"搜索意图分四类：informational(认知型，还在了解行业与问题)、commercial(对比型，正在选型比较)、" +
		"transactional(决策型，明确要购买或联系)、navigational(品牌型，已经在找特定品牌)。" +
		"只输出 JSON，不要任何解释、不要 markdown 代码块。"

	user := "以下是「" + brand + "」这个品牌在 AI 搜索场景下需要覆盖的问题清单：\n" + strings.Join(qs, "\n") + "\n"
	if ex := strings.TrimSpace(req.Extra); ex != "" {
		user += "\n补充业务背景：" + ex + "\n"
	}
	user += "\n请把它们聚成 3~8 个话题簇。要求：\n" +
		"1. 同一簇内的问题应属于同一搜索意图、同一业务主题；\n" +
		"2. 簇名用 4~8 个汉字的短语（如「价格与预算」「服务对比」「城市选择」）；\n" +
		"3. intent 字段只能取 informational / commercial / transactional / navigational；\n" +
		"4. ids 字段填该簇包含的问题编号（整数数组，如 [1,5,9]），**每个编号只能出现在一个簇里，且必须覆盖全部 " + strconv.Itoa(limit) + " 个编号**；\n" +
		"5. 只回编号，不要回问题原文，不要遗漏任何编号。\n\n" +
		"严格按此 JSON 格式输出：\n" +
		"{\"clusters\":[{\"name\":\"簇名\",\"intent\":\"informational\",\"description\":\"该簇覆盖什么问题\",\"ids\":[1,5,9]}]}"

	// DisableThinking：聚类是结构化抽取任务，不需要推理。
	// 不关闭的话，推理模型的思维链会吃光 max_tokens，返回空 content（详见 ai.Client.Thinking 注释）。
	client := ai.NewClient(chosen.BaseURL, chosen.APIKey, chosen.Model).
		WithMeta(tid, chosen.Name, "话题簇聚类").DisableThinking()
	maxTok := 1200 + limit*12
	if maxTok > 8000 {
		maxTok = 8000
	}
	answer, err := client.Chat(ctx, sys, []ai.Message{{Role: "user", Content: user}}, maxTok, 0.3)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "AI 聚类失败：" + err.Error()})
		return
	}

	var parsed aiClusterSchema
	if err := json.Unmarshal([]byte(extractJSON(answer)), &parsed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "AI 返回格式无法解析，请重试。原始片段：" + truncateCN(answer, 200)})
		return
	}
	if len(parsed.Clusters) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "AI 未返回有效话题簇，请重试"})
		return
	}

	// 编号 → 关键词 ID；同时保留文本匹配作为兼容路径
	norm := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.TrimLeft(s, "0123456789.、) ")
		repl := strings.NewReplacer("？", "?", "！", "!", "，", ",", "。", ".", " ", "", "　", "", "\t", "")
		return strings.ToLower(repl.Replace(s))
	}
	idByNo := map[int]uint{}
	kwIdx := map[string]uint{}
	for i := 0; i < limit; i++ {
		idByNo[i+1] = kws[i].ID
		kwIdx[norm(kws[i].Question)] = kws[i].ID
	}
	// 已归位集合：防止 AI 重复编号把同一个词抢到两个簇
	placed := map[uint]bool{}

	created, assigned, unmatched := 0, 0, 0
	tx := database.DB.Begin()
	if req.Overwrite {
		var old []models.KeywordCluster
		tx.Where("tenant_id = ?", tid).Find(&old)
		tx.Model(&models.GeoKeyword{}).Where("tenant_id = ?", tid).Update("cluster_id", 0)
		if len(old) > 0 {
			ids := make([]uint, 0, len(old))
			for _, o := range old {
				ids = append(ids, o.ID)
			}
			tx.Where("tenant_id = ? AND id IN ?", tid, ids).Delete(&models.KeywordCluster{})
		}
	}
	for i, cs := range parsed.Clusters {
		name := strings.TrimSpace(cs.Name)
		if name == "" {
			continue
		}
		intent := validIntent(strings.ToLower(strings.TrimSpace(cs.Intent)))
		var exist models.KeywordCluster
		var clID uint
		// 同名簇复用（非 overwrite 时不产生重复簇）
		if err := tx.Where("tenant_id = ? AND name = ?", tid, name).First(&exist).Error; err == nil {
			clID = exist.ID
			if exist.Color == "" {
				exist.Color = defaultIntentColor(intent)
				tx.Save(&exist)
			}
		} else {
			nc := models.KeywordCluster{
				TenantID: tid, Name: name, Intent: intent,
				Description: strings.TrimSpace(cs.Description),
				Color:       defaultIntentColor(intent), SortOrder: (i + 1) * 10,
			}
			if err := tx.Create(&nc).Error; err != nil {
				continue
			}
			clID = nc.ID
			created++
		}

		// 优先用编号归位
		kwIDs := make([]uint, 0, len(cs.IDs))
		for _, no := range cs.IDs {
			if id, ok := idByNo[no]; ok && !placed[id] {
				kwIDs = append(kwIDs, id)
			}
		}
		// 兼容：AI 仍返回问题原文时按文本匹配
		if len(kwIDs) == 0 {
			for _, q := range cs.Questions {
				if id, ok := kwIdx[norm(q)]; ok && !placed[id] {
					kwIDs = append(kwIDs, id)
				}
			}
		}
		for _, id := range kwIDs {
			if placed[id] {
				continue
			}
			placed[id] = true
			tx.Model(&models.GeoKeyword{}).Where("tenant_id = ? AND id = ?", tid, id).
				Update("cluster_id", clID)
			assigned++
		}
		unmatched += len(cs.IDs) - len(kwIDs)
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"clusters": len(parsed.Clusters), "created": created,
			"assigned": assigned, "unmatched": unmatched,
			"platform": chosen.Name, "total_keywords": len(kws),
		},
		"msg": "聚类完成：新建 " + strconv.Itoa(created) + " 个簇，归类 " + strconv.Itoa(assigned) + " 个问题",
	})
}

// extractJSON 从 AI 回答中提取 JSON 主体（容忍 markdown 代码块与前后废话）
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		s = strings.TrimPrefix(s, "json")
		s = strings.TrimPrefix(s, "JSON")
		if j := strings.Index(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}

// factConflict 判断回答文本是否与事实库的「禁区表述」冲突。
// 口径与指标总览 / 行动清单保持一致：整条 NotFact 作为子串匹配（不拆分）。
func factConflict(response string, facts []models.FactItem) bool {
	low := strings.ToLower(response)
	for _, f := range facts {
		nf := strings.ToLower(strings.TrimSpace(f.NotFact))
		if nf == "" {
			continue
		}
		if strings.Contains(low, nf) {
			return true
		}
	}
	return false
}

// brandNameOf 取租户品牌名（用于 AI 提示词上下文），无则回退系统默认品牌
func brandNameOf(tid uint) string {
	if tid > 0 {
		var s models.Setting
		if err := database.DB.Where("tenant_id = ? AND key = ?", tid, "default_brand").First(&s).Error; err == nil {
			if v := strings.TrimSpace(s.Value); v != "" {
				return v
			}
		}
	}
	return config.Load().DefaultBrand
}
