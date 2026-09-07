package baidu

import (
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ============ 百度 SERP 抓取（含基础反爬：UA 轮换 / Cookie / 节流 / 备用入口） ============

var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
}

var baiduCookies = []string{
	"BAIDUID=0A1B2C3D4E5F60718293A4B5C6D7E8F9:FG=1; BIDUPSID=0A1B2C3D4E5F60718293A4B5C6D7E8F9; PSTM=1700000000; H_PS_PSSID=39512_39109; BDSVRTM=0",
	"BAIDUID=9F8E7D6C5B4A3120987654ABCDEF1234:FG=1; BIDUPSID=9F8E7D6C5B4A3120987654ABCDEF1234; PSTM=1710000000; H_PS_PSSID=39456_39080; BDSVRTM=0",
}

var httpClient *http.Client

func init() {
	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives:      false,
		MaxIdleConns:           20,
		MaxIdleConnsPerHost:    4,
		IdleConnTimeout:        60 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  15 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
	}
	httpClient = &http.Client{Transport: transport, Timeout: 25 * time.Second}
}

// FetchPage 抓取百度 SERP 某一页（page 从 1 开始），返回 HTML
// 走 https://www.baidu.com/s?wd=...&pn=(page-1)*10；失败时尝试 m.baidu.com 备用入口
func FetchPage(keyword string, page int) (string, error) {
	pn := (page - 1) * 10
	base := []string{
		fmt.Sprintf("https://www.baidu.com/s?ie=utf-8&wd=%s&pn=%d", url.QueryEscape(keyword), pn),
		fmt.Sprintf("https://m.baidu.com/s?ie=utf-8&wd=%s&pn=%d", url.QueryEscape(keyword), pn),
	}
	var lastErr error
	for _, u := range base {
		html, err := fetchOnce(u)
		if err == nil && len(html) > 2000 {
			return html, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("page too short (%d bytes)", len(html))
		}
		// 入口切换间隔
		time.Sleep(time.Duration(800+rand.Intn(1200)) * time.Millisecond)
	}
	return "", fmt.Errorf("抓取失败（www/m 入口均失败）: %v", lastErr)
}

// proxyForward 若配置了 GEO_BAIDU_PROXY（如 http://host.docker.internal:18888），
// 则改走宿主机转发服务抓取，避免容器出口 IP 被百度风控导致整页失败。
// 转发服务约定：GET <proxy>/fetch?url=<urlencoded 目标URL> → 返回 HTML 原文。
// 命中百度安全验证时自动退避重试（最多 2 次），规避连续抓取偶发风控。
func proxyForward(u string) (string, error) {
	proxy := strings.TrimRight(os.Getenv("GEO_BAIDU_PROXY"), "/")
	if proxy == "" {
		return "", nil
	}
	fu := proxy + "/fetch?url=" + url.QueryEscape(u)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1200+rand.Intn(1800)) * time.Millisecond)
		}
		// 复用同一套 UA/Cookie/节流逻辑，只是目标改为转发服务
		ck := baiduCookies[rand.Intn(len(baiduCookies))]
		ua := userAgents[rand.Intn(len(userAgents))]
		req, err := http.NewRequest(http.MethodGet, fu, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
		req.Header.Set("Cookie", ck)
		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("转发抓取失败: %v", err)
		}
		buf, rerr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if rerr != nil {
			return "", rerr
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("转发抓取 HTTP %d", resp.StatusCode)
		}
		s := string(buf)
		if strings.Contains(s, "百度安全验证") || strings.Contains(s, "wappass.baidu.com") || strings.Contains(s, "verify.baidu.com") {
			lastErr = fmt.Errorf("触发百度安全验证（需要人工处理）")
			continue // 退避后重试
		}
		return s, nil
	}
	return "", lastErr
}

func fetchOnce(u string) (string, error) {
	// 优先走宿主机转发出口（若配置了 GEO_BAIDU_PROXY）。
	// 关键：转发服务不可用时（未启动 / 端口不通 / 502）必须回落到容器直连，
	// 否则整条链路会 100% 失败，且错误被误报成「触发反爬」，排查方向完全跑偏。
	if html, err := proxyForward(u); err == nil && html != "" {
		if strings.Contains(html, "百度安全验证") || strings.Contains(html, "wappass.baidu.com") || strings.Contains(html, "verify.baidu.com") {
			return "", fmt.Errorf("触发百度安全验证（需要人工处理）")
		}
		return html, nil
	}
	// 直连兜底（转发未配置或不可用时走这里）
	ck := baiduCookies[rand.Intn(len(baiduCookies))]
	ua := userAgents[rand.Intn(len(userAgents))]
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	// 携带随机 Cookie，尽量规避风控
	req.Header.Set("Cookie", ck)
	// 随机起点 Referer 更像真人
	req.Header.Set("Referer", "https://www.baidu.com/")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 命中安全验证
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	s := string(buf)
	if strings.Contains(s, "百度安全验证") || strings.Contains(s, "wappass.baidu.com") || strings.Contains(s, "verify.baidu.com") {
		return "", fmt.Errorf("触发百度安全验证（需要人工处理）")
	}
	return s, nil
}

// SuggestThrottle 随机节流，避免高频请求触发风控
func SuggestThrottle() {
	time.Sleep(time.Duration(1800+rand.Intn(2200)) * time.Millisecond)
}
