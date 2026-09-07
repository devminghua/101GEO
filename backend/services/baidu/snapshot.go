package baidu

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"geo-tool/database"
	"geo-tool/models"
)

/* ================================================================
 * 百度排名历史快照服务
 *
 * 职责：
 *  1. SaveSnapshot  分析完成后写入/更新当日快照（同关键词+同域名+同日期去重）
 *  2. QueryRankHistory 查询近 N 天按日聚合的排名序列（用于前端趋势折线）
 *  3. TrendSummary  计算某关键词我方最近一个月的排名趋势摘要（上升/下降/持平）
 * 全部按租户 TenantID 隔离。
 * ================================================================ */

// RankPoint 单日聚合点：某域名在某关键词下的当日最佳排名
type RankPoint struct {
	Date        string `json:"date"`         // YYYY-MM-DD
	SiteDomain  string `json:"site_domain"`  // 网站域名
	BestRank    int    `json:"best_rank"`    // 0 = 未上榜
	Occurrences int    `json:"occurrences"`  // 当日出现次数
}

// SaveSnapshot 写入一次分析快照。
// 去重口径：同租户 + 关键词 + 域名 + 抓取日期唯一；存在则更新 best_rank/occurrences，否则插入。
func SaveSnapshot(tid uint, keyword, domain string, bestRank, occurrences int) {
	if tid == 0 || strings.TrimSpace(keyword) == "" || strings.TrimSpace(domain) == "" {
		return
	}
	db := database.DB
	date := time.Now().Format("2006-01-02")
	var snap models.BaiduRankSnapshot
	err := db.Where("tenant_id = ? AND keyword = ? AND site_domain = ? AND date = ?",
		tid, strings.TrimSpace(keyword), strings.TrimSpace(domain), date).First(&snap).Error
	if err == gorm.ErrRecordNotFound {
		// 插入
		db.Create(&models.BaiduRankSnapshot{
			TenantID:    tid,
			Keyword:     strings.TrimSpace(keyword),
			SiteDomain:  strings.TrimSpace(domain),
			BestRank:    bestRank,
			Occurrences: occurrences,
			Date:        date,
		})
		return
	}
	if err != nil {
		return
	}
	// 更新：best_rank 取更靠前（更小）值；occurrences 累加（本日再次抓到按出现次数）
	newBest := snap.BestRank
	if bestRank > 0 && (snap.BestRank == 0 || bestRank < snap.BestRank) {
		newBest = bestRank
	}
	newOcc := snap.Occurrences
	if occurrences > newOcc {
		newOcc = occurrences
	}
	db.Model(&snap).Updates(map[string]interface{}{
		"best_rank":    newBest,
		"occurrences":  newOcc,
	})
}

// QueryRankHistory 查询某关键词近 days 天按日聚合的排名序列。
// 返回按日期升序的数组；每个日期为一条，含该词所有监控域名的当日最佳排名。
func QueryRankHistory(tid uint, keyword string, days int) []RankPoint {
	if tid == 0 {
		return nil
	}
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	from := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	var snaps []models.BaiduRankSnapshot
	database.DB.Where("tenant_id = ? AND keyword = ? AND date >= ?",
		tid, strings.TrimSpace(keyword), from).Find(&snaps)

	// 按 (date, site_domain) 聚合：best_rank 取该日所有记录中最小（最靠前，0 表示未上榜则保留 0）
	type key struct{ date, domain string }
	agg := map[key]RankPoint{}
	for _, s := range snaps {
		k := key{s.Date, s.SiteDomain}
		p, ok := agg[k]
		if !ok {
			p = RankPoint{Date: s.Date, SiteDomain: s.SiteDomain, BestRank: s.BestRank, Occurrences: s.Occurrences}
		} else {
			if s.BestRank > 0 && (p.BestRank == 0 || s.BestRank < p.BestRank) {
				p.BestRank = s.BestRank
			}
			if s.Occurrences > p.Occurrences {
				p.Occurrences = s.Occurrences
			}
		}
		agg[k] = p
	}
	out := make([]RankPoint, 0, len(agg))
	for _, p := range agg {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].SiteDomain < out[j].SiteDomain
	})
	return out
}

// TrendSummary 计算某关键词我方最近一个月的排名趋势摘要。
// 取该词所有监控域名中"可上榜日"的最新一次排名 vs 首次上榜排名：
//   - 新上榜（此前未上榜）→ "上升（新上榜）"
//   - 最新排名比首次更靠前 → 上升
//   - 最新排名比首次更靠后 → 下降
//   - 持平或数据不足 → 持平
func TrendSummary(tid uint, keyword string, days int) string {
	if tid == 0 {
		return ""
	}
	if days <= 0 {
		days = 30
	}
	points := QueryRankHistory(tid, keyword, days)
	if len(points) == 0 {
		return ""
	}
	// 按域名分组
	byDomain := map[string][]RankPoint{}
	for _, p := range points {
		byDomain[p.SiteDomain] = append(byDomain[p.SiteDomain], p)
	}
	bestDomain := ""
	bestFirst := 0
	for d, list := range byDomain {
		// 找首个上榜日排名（best_rank > 0）
		first := 0
		for _, p := range list {
			if p.BestRank > 0 {
				first = p.BestRank
				break
			}
		}
		if first == 0 {
			continue
		}
		if bestDomain == "" || first < bestFirst {
			bestDomain = d
			bestFirst = first
		}
	}
	if bestDomain == "" {
		return "持平（近期未上榜，无排名变化）"
	}
	// 该域名最新上榜排名
	latest := 0
	for i := len(byDomain[bestDomain]) - 1; i >= 0; i-- {
		if byDomain[bestDomain][i].BestRank > 0 {
			latest = byDomain[bestDomain][i].BestRank
			break
		}
	}
	if latest == 0 {
		return fmt.Sprintf("%s：排名波动（最近一次未上榜）", bestDomain)
	}
	switch {
	case latest < bestFirst:
		return fmt.Sprintf("%s：上升（第 %d 名 → 第 %d 名）", bestDomain, bestFirst, latest)
	case latest > bestFirst:
		return fmt.Sprintf("%s：下降（第 %d 名 → 第 %d 名）", bestDomain, bestFirst, latest)
	default:
		return fmt.Sprintf("%s：持平（稳定在第 %d 名）", bestDomain, latest)
	}
}
