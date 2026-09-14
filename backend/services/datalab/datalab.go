package datalab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

/* ================================================================
 * Naver Datalab（검색어트렌드）官方搜索趋势 API
 *  - 端点 POST https://openapi.naver.com/v1/datalab/search
 *  - 认证：X-Naver-Client-Id / X-Naver-Client-Secret（非登录方式）
 *  - 配额：每日 1000 次（按 Client ID）；免费，无需企业资质
 *  - 能力：最多 5 个关键词组 × 每组 20 词；日/周/月粒度；设备/性别/年龄细分
 *  - 返回归一化 ratio（查询区间内最高点=100）
 * ================================================================ */

const endpoint = "https://openapi.naver.com/v1/datalab/search"

// Point 单期数据点
type Point struct {
	Period string  `json:"period"` // 如 2026-09-01（timeUnit=date 时）
	Value  float64 `json:"value"`  // 归一化相对值（0-100）
}

// Series 一组关键词的趋势序列
type Series struct {
	Name     string  `json:"name"`
	Keywords []string `json:"keywords"`
	Points   []Point `json:"points"`
}

// Query 执行搜索词趋势查询。
// timeUnit: date/week/month；groups: 最多 5 组，每组 Name + 最多 20 个词。
func Query(clientID, clientSecret string, startDate, endDate, timeUnit string, groups []struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}) ([]Series, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return nil, errors.New("未配置 Naver Datalab 凭据（总后台「数据 API」页配置 Client ID / Secret）")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"startDate":     startDate,
		"endDate":       endDate,
		"timeUnit":      timeUnit,
		"keywordGroups": groups,
	})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Naver-Client-Id", clientID)
	req.Header.Set("X-Naver-Client-Secret", clientSecret)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Naver Datalab 返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var res struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		TimeUnit  string `json:"timeUnit"`
		Results   []struct {
			Title    string   `json:"title"`
			Keywords []string `json:"keywords"`
			Data     []struct {
				Period string  `json:"period"`
				Ratio  float64 `json:"ratio"`
			} `json:"data"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("Datalab 响应解析失败：%v", err)
	}
	out := make([]Series, 0, len(res.Results))
	for _, r := range res.Results {
		s := Series{Name: r.Title, Keywords: r.Keywords}
		for _, d := range r.Data {
			s.Points = append(s.Points, Point{Period: d.Period, Value: d.Ratio})
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errors.New("Datalab 未返回数据（关键词可能无足够搜索量）")
	}
	return out, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
