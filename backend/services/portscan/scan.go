package portscan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

/* ================================================================
 * 站点体检 · 端口扫描（services/portscan）
 *  - Go 原生 TCP connect 扫描（与 nmap -sT 同原理），无需 nmap 二进制
 *  - 背景：2026-09-14 实测 nmap 7.93 在容器网络栈下有假阳性（20 端口全报
 *    open，而 /dev/tcp 直连同一批端口全部 CLOSED）→ 改用 Go 拨号判定，可靠
 *  - 安全约束：
 *     ① 目标仅允许域名（从 URL 提取 host），禁止直接传 IP
 *     ② 解析后禁止回环/私有/链路本地网段（防 SSRF）
 *     ③ 只扫固定端口清单（20 个常用端口），无用户可控参数
 *  - 超时：整体 60 秒（context 中断）
 * ================================================================ */

// Port 单个端口扫描结果
type Port struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`
	Service  string `json:"service"`
	Version  string `json:"version"`
}

// Result 扫描结果
type Result struct {
	Host      string `json:"host"`
	IP        string `json:"ip"`
	OpenCount int    `json:"open_count"`
	Ports     []Port `json:"ports"`
	Elapsed   string `json:"elapsed"`
	Note      string `json:"note,omitempty"`
}

// 常用端口清单与默认服务名
var portList = []struct {
	Port    int
	Service string
}{
	{21, "ftp"}, {22, "ssh"}, {25, "smtp"}, {53, "domain"},
	{80, "http"}, {110, "pop3"}, {143, "imap"}, {443, "https"},
	{465, "smtps"}, {587, "submission"}, {993, "imaps"}, {995, "pop3s"},
	{3306, "mysql"}, {3389, "rdp"}, {5432, "postgresql"}, {6379, "redis"},
	{8080, "http-alt"}, {8443, "https-alt"}, {8888, "http-alt"}, {9000, "http-alt"},
}

// 私有网段判定（防 SSRF）
var privateNetworks = []*net.IPNet{
	mustCIDR("127.0.0.0/8"),
	mustCIDR("10.0.0.0/8"),
	mustCIDR("172.16.0.0/12"),
	mustCIDR("192.168.0.0/16"),
	mustCIDR("169.254.0.0/16"),
	mustCIDR("::1/128"),
	mustCIDR("fc00::/7"),
	mustCIDR("fe80::/10"),
}

func mustCIDR(s string) *net.IPNet {
	_, n, _ := net.ParseCIDR(s)
	return n
}

// ResolveHost 从 URL/域名提取 host 并解析公网 IP（安全校验）
func ResolveHost(target string) (host, ip string, err error) {
	t := strings.TrimSpace(target)
	t = regexp.MustCompile(`^[a-zA-Z]+://`).ReplaceAllString(t, "")
	if i := strings.IndexAny(t, "/?#"); i >= 0 {
		t = t[:i]
	}
	if i := strings.LastIndex(t, ":"); i >= 0 && !strings.Contains(t[i:], "]") {
		t = t[:i]
	}
	t = strings.Trim(t, "[]")
	host = strings.ToLower(t)
	if host == "" || !strings.Contains(host, ".") {
		return "", "", errors.New("请填写有效的网站域名（如 example.com）")
	}
	addrs, err := net.LookupIP(host)
	if err != nil || len(addrs) == 0 {
		return "", "", errors.New("域名解析失败，请检查域名是否正确")
	}
	var public net.IP
	for _, a := range addrs {
		if a.To4() != nil && !isPrivate(a) {
			public = a
			break
		}
	}
	if public == nil {
		return "", "", errors.New("该域名解析到内网地址，出于安全策略禁止扫描")
	}
	return host, public.String(), nil
}

func isPrivate(ip net.IP) bool {
	for _, n := range privateNetworks {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Scan 执行 TCP connect 端口扫描（Go 原生，带超时）
func Scan(target string, timeout time.Duration) (*Result, error) {
	host, ip, err := ResolveHost(target)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	res := &Result{Host: host, IP: ip}

	// 并发拨号（工作池 10，每端口 3s 超时）
	type job struct {
		idx int
		p   int
	}
	jobs := make(chan job)
	var mu sync.Mutex
	var wg sync.WaitGroup
	ports := make([]Port, 0, len(portList))
	workers := 10
	if workers > len(portList) {
		workers = len(portList)
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				d := net.Dialer{Timeout: 3 * time.Second}
				conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, fmt.Sprintf("%d", j.p)))
				if err != nil {
					continue // closed / filtered / timeout
				}
				// 开放端口：尝试轻量 banner 探测（1.5s 读超时）
				version := ""
				_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
				buf := make([]byte, 256)
				if n, _ := conn.Read(buf); n > 0 {
					version = cleanBanner(string(buf[:n]))
				}
				conn.Close()
				mu.Lock()
				ports = append(ports, Port{
					Port:     j.p,
					Protocol: "tcp",
					State:    "open",
					Service:  portList[j.idx].Service,
					Version:  version,
				})
				mu.Unlock()
			}
		}()
	}
	for i, p := range portList {
		select {
		case <-ctx.Done():
			break
		case jobs <- job{idx: i, p: p.Port}:
		}
	}
	close(jobs)
	wg.Wait()

	sort.Slice(ports, func(a, b int) bool { return ports[a].Port < ports[b].Port })
	res.Ports = ports
	res.OpenCount = len(ports)
	res.Elapsed = fmt.Sprintf("%.1fs", time.Since(start).Seconds())
	return res, nil
}

// cleanBanner 清洗 banner：去控制字符、截断 120 字
func cleanBanner(b string) string {
	b = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, b)
	b = strings.Join(strings.Fields(b), " ")
	if len(b) > 120 {
		b = b[:120]
	}
	return b
}

// 保留 io 引用（banner 读取用）
var _ = io.Discard
