package content

import (

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/biztime"
)

/* ================================================================
 * 内容投放 · 效果分析服务
 *
 * 聚合 发布任务/软文库/监控记录，产出：发布成功率、收录率（仅按
 * SourcedFrom=real 统计）、平均质量分、媒体分布、场景分布、质量分布、
 * 近 30 天发布与收录趋势。
 * ================================================================ */

// MediaDistItem 媒体分布
type MediaDistItem struct {
	Name    string `json:"name"`
	Count   int    `json:"count"`
	Indexed int    `json:"indexed"`
}

// CatDistItem 场景分布（软文库）
type CatDistItem struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// QualityDistItem 质量分布
type QualityDistItem struct {
	Range string `json:"range"`
	Count int    `json:"count"`
}

// TrendItem 趋势点
type TrendItem struct {
	Day     string `json:"day"`
	Published int  `json:"published"`
	Indexed   int  `json:"indexed"`
}

// AnalysisResp 分析结果
type AnalysisResp struct {
	TotalTasks      int              `json:"total_tasks"`
	Published       int              `json:"published"`
	WaitManual      int              `json:"wait_manual"`
	Failed          int              `json:"failed"`
	PublishRate     float64          `json:"publish_rate"` // 发布成功率（已发布/全部任务）
	TotalMonitors   int              `json:"total_monitors"`
	IndexedReal     int              `json:"indexed_real"`
	IndexedEstimate int              `json:"indexed_estimate"`
	IndexedRate     float64          `json:"indexed_rate"` // 收录率（仅按 real 统计）
	AvgQuality      float64          `json:"avg_quality"`
	MediaDist       []MediaDistItem  `json:"media_dist"`
	CategoryDist    []CatDistItem    `json:"category_dist"`
	QualityDist     []QualityDistItem `json:"quality_dist"`
	Trend           []TrendItem      `json:"trend"`
}

// Analysis 执行效果分析
func Analysis(tid uint) (*AnalysisResp, error) {
	var tasks []models.CtnTask
	database.DB.Where("tenant_id = ?", tid).Find(&tasks)

	var articles []models.CtnArticle
	database.DB.Where("tenant_id = ?", tid).Find(&articles)

	cut := biztime.Since(30)
	var monitors []models.CtnMonitor
	database.DB.Where("tenant_id = ? AND created_at >= ?", tid, cut).Find(&monitors)

	resp := &AnalysisResp{
		MediaDist:    []MediaDistItem{},
		CategoryDist: []CatDistItem{},
		QualityDist:  []QualityDistItem{},
		Trend:        []TrendItem{},
	}

	resp.TotalTasks = len(tasks)
	qualitySum := 0
	mediaMap := map[string]*MediaDistItem{}
	for i := range tasks {
		t := &tasks[i]
		switch t.Status {
		case models.CtnTaskPublished:
			resp.Published++
		case models.CtnTaskWaitManual:
			resp.WaitManual++
		case models.CtnTaskFailed:
			resp.Failed++
		}
		qualitySum += t.QualityScore

		key := t.MediaName
		if key == "" {
			key = "未知"
		}
		m, ok := mediaMap[key]
		if !ok {
			m = &MediaDistItem{Name: key}
			mediaMap[key] = m
		}
		m.Count++
		if t.SyncState == models.CtnSyncIndexed {
			m.Indexed++
		}
	}
	if resp.TotalTasks > 0 {
		resp.PublishRate = round2(float64(resp.Published) / float64(resp.TotalTasks) * 100)
	}
	if resp.TotalTasks > 0 {
		resp.AvgQuality = round2(float64(qualitySum) / float64(resp.TotalTasks))
	}
	for _, m := range mediaMap {
		resp.MediaDist = append(resp.MediaDist, *m)
	}
	sortMedia(resp.MediaDist)

	// 场景分布
	catMap := map[string]int{}
	for i := range articles {
		cat := articles[i].Category
		if cat == "" {
			cat = "未分类"
		}
		catMap[cat]++
	}
	for c, n := range catMap {
		resp.CategoryDist = append(resp.CategoryDist, CatDistItem{Category: c, Count: n})
	}
	sortCat(resp.CategoryDist)

	// 质量分布
	qDist := map[string]int{"0-59": 0, "60-79": 0, "80-100": 0}
	for i := range tasks {
		s := tasks[i].QualityScore
		switch {
		case s <= 59:
			qDist["0-59"]++
		case s <= 79:
			qDist["60-79"]++
		default:
			qDist["80-100"]++
		}
	}
	for r, n := range qDist {
		resp.QualityDist = append(resp.QualityDist, QualityDistItem{Range: r, Count: n})
	}
	sortQuality(resp.QualityDist)

	// 监控与收录
	resp.TotalMonitors = len(monitors)
	realChecked := 0
	for i := range monitors {
		m := &monitors[i]
		if m.SourcedFrom == models.SourceEstimate {
			resp.IndexedEstimate++
		} else {
			realChecked++
			if m.BaiduIndexed != nil && *m.BaiduIndexed {
				resp.IndexedReal++
			}
		}
	}
	if realChecked > 0 {
		resp.IndexedRate = round2(float64(resp.IndexedReal) / float64(realChecked) * 100)
	}

	// 近 30 天趋势
	dayIdx := map[string]*TrendItem{}
	for d := 0; d < 30; d++ {
		day := biztime.Day(-d)
		ti := &TrendItem{Day: day}
		dayIdx[day] = ti
		resp.Trend = append(resp.Trend, *ti)
	}
	for i := range tasks {
		t := &tasks[i]
		if t.PublishedAt != nil && t.PublishedAt.After(cut) {
			key := t.PublishedAt.Format("2006-01-02")
			if ti, ok := dayIdx[key]; ok {
				ti.Published++
			}
		}
	}
	for i := range monitors {
		m := &monitors[i]
		if m.IndexedAt != nil && m.IndexedAt.After(cut) {
			key := m.IndexedAt.Format("2006-01-02")
			if ti, ok := dayIdx[key]; ok {
				ti.Indexed++
			}
		}
	}
	for d := 0; d < 30; d++ {
		day := biztime.Day(-d)
		resp.Trend[d] = *dayIdx[day]
	}

	return resp, nil
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func sortMedia(items []MediaDistItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].Count > items[j-1].Count; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func sortCat(items []CatDistItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].Count > items[j-1].Count; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func sortQuality(items []QualityDistItem) {
	order := map[string]int{"80-100": 0, "60-79": 1, "0-59": 2}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && order[items[j].Range] < order[items[j-1].Range]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
