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

	"geo-tool/services/baidu"
)

/* ================================================================
 * Google SERP 适配器：Serper.dev（第三方结构化 SERP API）
 *  - 实现 baidu.Engine，产出 PageResult 交给共用管线（同行识别/归因/建议全复用）
 *  - 无需站点验证；2500 次免费额度，之后 $0.30/千次
 *  - Key 读取：环境变量 GEO_SERPER_KEY 优先，其次总后台 settings（serper_api_key）
 * ================================================================ */

const serperURL = "https://google.serper.dev/search"

// GoogleEngine 实现 baidu.Engine
type GoogleEngine struct {
	Key string
}

// serperResponse Serper.dev 返回结构（只取用到的字段）
type serperResponse struct {
	Organic []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
		Domain  string `json:"domain"` // Serper 直接给域名（部分账号支持）
	} `json:"organic"`
	Ads []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"ads"`
}

// FetchParse 抓取一页（10 条）并映射为 PageResult
func (g GoogleEngine) FetchParse(keyword string, page int) (baidu.PageResult, error) {
	if strings.TrimSpace(g.Key) == "" {
		return baidu.PageResult{}, errors.New("未配置 Serper API Key（总后台「数据 API」页配置）")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"q":    keyword,
		"gl":   "cn", // 国内用户默认中国区结果；后续可配置
		"num":  10,
		"page": page,
	})
	req, err := http.NewRequest(http.MethodPost, serperURL, bytes.NewReader(body))
	if err != nil {
		return baidu.PageResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", g.Key)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return baidu.PageResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return baidu.PageResult{}, fmt.Errorf("Serper 接口返回 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var sr serperResponse
	if err := json.Unmarshal(raw, &sr); err != nil {
		return baidu.PageResult{}, fmt.Errorf("Serper 响应解析失败：%v", err)
	}

	pr := baidu.PageResult{Page: page}
	rank := 0
	for _, o := range sr.Organic {
		rank++
		domain := o.Domain
		if domain == "" {
			domain = domainOf(o.Link)
		}
		pr.Items = append(pr.Items, baidu.RankItem{
			Rank: rank, Type: "organic",
			Domain: domain, Title: o.Title, URL: o.Link, Snippet: o.Snippet,
		})
	}
	pr.OrganicCount = len(sr.Organic)
	pr.AdCount = len(sr.Ads)
	for _, a := range sr.Ads {
		pr.Items = append(pr.Items, baidu.RankItem{
			Rank: 0, Type: "ad",
			Domain: domainOf(a.Link), Title: a.Title, URL: a.Link, Snippet: a.Snippet,
		})
	}
	if pr.OrganicCount == 0 && pr.AdCount == 0 {
		return pr, errors.New("Serper 未返回结果（关键词可能无结果或账号额度不足）")
	}
	return pr, nil
}

// RunGoogle 执行一次 Google 关键词分析（复用共用管线）
func RunGoogle(cfg baidu.TaskConfig, key string) (*baidu.Result, error) {
	return baidu.RunKeywordEngine(cfg, GoogleEngine{Key: key}, nil)
}

// domainOf 从 URL 提取裸域名（host:port 去掉端口）
func domainOf(link string) string {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return link
	}
	return strings.ToLower(u.Hostname())
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
