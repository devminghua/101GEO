package baidu

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Suggest 拉取百度搜索下拉联想词（sugrec 接口），用于拓词选题。
// 返回与词根相关的真实搜索词候选（不含词根本身）。
func Suggest(keyword string) ([]string, error) {
	kw := strings.TrimSpace(keyword)
	if kw == "" {
		return nil, fmt.Errorf("词根不能为空")
	}
	u := "https://www.baidu.com/sugrec?prod=pc&ie=utf-8&wd=" + url.QueryEscape(kw)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(500+rand.Intn(800)) * time.Millisecond)
		}
		req, err := newReq(u)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("下拉请求失败: %v", err)
			continue
		}
		defer resp.Body.Close()
		var payload struct {
			G []struct {
				Q string `json:"q"`
			} `json:"g"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			lastErr = fmt.Errorf("下拉解析失败: %v", err)
			continue
		}
		out := make([]string, 0, len(payload.G))
		seen := map[string]bool{}
		for _, it := range payload.G {
			q := strings.TrimSpace(it.Q)
			if q == "" || q == kw || seen[q] {
				continue
			}
			seen[q] = true
			out = append(out, q)
		}
		return out, nil
	}
	return nil, fmt.Errorf("下拉获取失败: %v", lastErr)
}

// SuggestGoogle 拉取 Google 搜索补全（suggestqueries），用于海外拓词补充。
// 返回候选词（不含词根本身）。
func SuggestGoogle(keyword string) ([]string, error) {
	kw := strings.TrimSpace(keyword)
	if kw == "" {
		return nil, fmt.Errorf("词根不能为空")
	}
	u := "https://suggestqueries.google.com/complete/search?client=firefox&hl=zh-CN&q=" + url.QueryEscape(kw)
	req, err := newReq(u)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Google 补全请求失败: %v", err)
	}
	defer resp.Body.Close()
	var payload []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("Google 补全解析失败: %v", err)
	}
	out := []string{}
	seen := map[string]bool{}
	if len(payload) >= 2 {
		if arr, ok := payload[1].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					q := strings.TrimSpace(s)
					if q != "" && q != kw && !seen[q] {
						seen[q] = true
						out = append(out, q)
					}
				}
			}
		}
	}
	return out, nil
}

func newReq(u string) (req *http.Request, err error) {
	req, err = http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	return req, nil
}
