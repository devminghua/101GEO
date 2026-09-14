package serp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

/* ================================================================
 * 国际收录查询：site:domain 走 SERP 引擎
 *  - Google → Serper（searchInformation.totalResults 为官方口径估算）
 *  - Naver → SerpAPI（返回结果条数近似）
 * ================================================================ */

// IndexItem 收录首页条目
type IndexItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Domain  string `json:"domain"`
	Snippet string `json:"snippet"`
}

// IndexResult 收录查询结果
type IndexResult struct {
	Domain     string      `json:"domain"`
	Engine     string      `json:"engine"`
	Count      int64       `json:"count"`       // 收录总量（官方口径估算，0=未知用条数近似）
	CountExact bool        `json:"count_exact"` // 总数是否来自官方字段
	Items      []IndexItem `json:"items"`
	PageCount  int         `json:"page_count"`
}

// serperIndexResponse Serper 返回（含 searchInformation.totalResults）
type serperIndexResponse struct {
	Organic []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"organic"`
	SearchInformation struct {
		TotalResults int64 `json:"totalResults"`
	} `json:"searchInformation"`
}

// IndexCountGoogle Serper 收录查询
func IndexCountGoogle(domain, key string) (*IndexResult, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("未配置 Serper API Key")
	}
	body, _ := json.Marshal(map[string]interface{}{"q": "site:" + domain, "num": 10})
	req, err := http.NewRequest(http.MethodPost, serperURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", key)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Serper 接口返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var sr serperIndexResponse
	if err := json.Unmarshal(raw, &sr); err != nil {
		return nil, fmt.Errorf("Serper 响应解析失败：%v", err)
	}
	res := &IndexResult{Domain: domain, Engine: "google"}
	for _, o := range sr.Organic {
		res.Items = append(res.Items, IndexItem{Title: o.Title, URL: o.Link, Domain: domainOf(o.Link), Snippet: o.Snippet})
	}
	res.PageCount = len(res.Items)
	res.Count = sr.SearchInformation.TotalResults
	if res.Count > 0 {
		res.CountExact = true
	} else {
		res.Count = int64(len(res.Items))
	}
	return res, nil
}

// IndexCountNaver SerpAPI 收录查询（Naver 无官方总数，返回首页条数近似）
func IndexCountNaver(domain, key string) (*IndexResult, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("未配置 SerpAPI Key")
	}
	u, _ := url.Parse(serpapiURL)
	q := u.Query()
	q.Set("engine", "naver")
	q.Set("query", "site:"+domain)
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
		return nil, fmt.Errorf("SerpAPI 接口返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var sr serpapiNaverResponse
	if err := json.Unmarshal(raw, &sr); err != nil {
		return nil, fmt.Errorf("SerpAPI 响应解析失败：%v", err)
	}
	if sr.Error != "" {
		return nil, fmt.Errorf("SerpAPI 错误：%s", sr.Error)
	}
	res := &IndexResult{Domain: domain, Engine: "naver"}
	for _, o := range sr.OrganicResults {
		res.Items = append(res.Items, IndexItem{Title: o.Title, URL: o.Link, Domain: domainOf(o.Link), Snippet: o.Snippet})
	}
	res.PageCount = len(res.Items)
	res.Count = int64(len(res.Items))
	return res, nil
}
