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

	"geo-tool/services/baidu"
)

/* ================================================================
 * Naver SERP 适配器：SerpAPI（engine=naver）
 *  - 实现 baidu.Engine，产出 PageResult 交给共用管线
 *  - SerpAPI 是 Naver 通用网页 SERP 唯一可靠的第三方结构化源
 *  - Key 读取：环境变量 GEO_SERPAPI_KEY 优先，其次总后台 settings（serpapi_key）
 * ================================================================ */

const serpapiURL = "https://serpapi.com/search.json"

// NaverEngine 实现 baidu.Engine
type NaverEngine struct {
	Key string
}

// serpapiNaverResponse SerpAPI engine=naver 返回结构（只取用到的字段）
type serpapiNaverResponse struct {
	OrganicResults []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"organic_results"`
	AdResults []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"ad_results"`
	SearchInformation struct {
		TotalResults string `json:"total_results"`
	} `json:"search_information"`
	Error string `json:"error"`
}

// FetchParse 抓取一页并映射为 PageResult
func (n NaverEngine) FetchParse(keyword string, page int) (baidu.PageResult, error) {
	if strings.TrimSpace(n.Key) == "" {
		return baidu.PageResult{}, errors.New("未配置 SerpAPI Key（总后台「数据 API」页配置）")
	}
	u, _ := url.Parse(serpapiURL)
	q := u.Query()
	q.Set("engine", "naver")
	q.Set("query", keyword)
	q.Set("start", fmt.Sprintf("%d", (page-1)*10+1))
	q.Set("api_key", n.Key)
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return baidu.PageResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return baidu.PageResult{}, fmt.Errorf("SerpAPI 接口返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var sr serpapiNaverResponse
	if err := json.Unmarshal(raw, &sr); err != nil {
		return baidu.PageResult{}, fmt.Errorf("SerpAPI 响应解析失败：%v", err)
	}
	if sr.Error != "" {
		return baidu.PageResult{}, fmt.Errorf("SerpAPI 错误：%s", sr.Error)
	}

	pr := baidu.PageResult{Page: page}
	rank := 0
	for _, o := range sr.OrganicResults {
		rank++
		pr.Items = append(pr.Items, baidu.RankItem{
			Rank: rank, Type: "organic",
			Domain: domainOf(o.Link), Title: o.Title, URL: o.Link, Snippet: o.Snippet,
		})
	}
	pr.OrganicCount = len(sr.OrganicResults)
	pr.AdCount = len(sr.AdResults)
	for _, a := range sr.AdResults {
		pr.Items = append(pr.Items, baidu.RankItem{
			Rank: 0, Type: "ad",
			Domain: domainOf(a.Link), Title: a.Title, URL: a.Link, Snippet: a.Snippet,
		})
	}
	if pr.OrganicCount == 0 && pr.AdCount == 0 {
		return pr, errors.New("SerpAPI 未返回结果（关键词可能无结果或账号额度不足）")
	}
	return pr, nil
}

// RunNaver 执行一次 Naver 关键词分析（复用共用管线）
func RunNaver(cfg baidu.TaskConfig, key string) (*baidu.Result, error) {
	return baidu.RunKeywordEngine(cfg, NaverEngine{Key: key}, nil)
}
