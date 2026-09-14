package serp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"geo-tool/services/datalab"
)

/* ================================================================
 * Google Trends：走 SerpAPI engine=google_trends
 *  - 复用 SerpAPI Key（与 Naver SERP 同 Key）
 *  - 返回 timeline_data → 统一 datalab.Series 结构（与 Naver Datalab 对齐）
 * ================================================================ */

// serpapiTrendsResponse SerpAPI google_trends 返回（timeline 部分）
type serpapiTrendsResponse struct {
	Error       string `json:"error"`
	InterestByRegion []struct {
		Title string `json:"title"`
	} `json:"interest_by_region"`
	InterestOverTime []struct {
		Date      string `json:"date"`
		Values    []struct {
			Query           string  `json:"query"`
			Value           float64 `json:"value"`
			ExtractedValue  float64 `json:"extracted_value"`
		} `json:"values"`
	} `json:"interest_over_time"`
}

// TrendsGoogle 查询 Google Trends（SerpAPI），返回统一趋势序列。
// keywords: 最多 5 个关键词；date 格式 "today 3-m"（近3月）/"today 12-m"/"today 1-m"。
func TrendsGoogle(key, date string, keywords []string) ([]datalab.Series, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("未配置 SerpAPI Key")
	}
	if len(keywords) == 0 {
		return nil, errors.New("请至少填写一个关键词")
	}
	u, _ := url.Parse(serpapiURL)
	q := u.Query()
	q.Set("engine", "google_trends")
	q.Set("q", strings.Join(keywords, ","))
	q.Set("date", date)
	q.Set("api_key", key)
	u.RawQuery = q.Encode()
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SerpAPI Trends 返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var sr serpapiTrendsResponse
	if err := json.Unmarshal(raw, &sr); err != nil {
		return nil, fmt.Errorf("Trends 响应解析失败：%v", err)
	}
	if sr.Error != "" {
		return nil, fmt.Errorf("SerpAPI 错误：%s", sr.Error)
	}
	if len(sr.InterestOverTime) == 0 {
		return nil, errors.New("Trends 未返回数据（关键词可能无足够搜索量）")
	}
	// 转置：timeline[date].values[query] → series[query].points[date]
	order := keywords
	seriesMap := map[string]*datalab.Series{}
	for _, kw := range keywords {
		seriesMap[kw] = &datalab.Series{Name: kw, Keywords: []string{kw}}
	}
	for _, t := range sr.InterestOverTime {
		for _, v := range t.Values {
			if s, ok := seriesMap[v.Query]; ok {
				val := v.ExtractedValue
				if val == 0 {
					val = v.Value
				}
				s.Points = append(s.Points, datalab.Point{Period: t.Date, Value: val})
			}
		}
	}
	out := make([]datalab.Series, 0, len(order))
	for _, kw := range order {
		if s, ok := seriesMap[kw]; ok && len(s.Points) > 0 {
			out = append(out, *s)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("Trends 数据转换失败")
	}
	return out, nil
}
