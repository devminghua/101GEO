package douyin

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

/* ================================================================
 * 抖音主页链接解析
 * 支持形式：
 *  - https://www.douyin.com/user/<sec_uid>
 *  - https://v.douyin.com/<short>/  （短链，需跟随跳转解析）
 *  - 复制分享文本（可能夹杂文字，自动抽取其中的抖音链接）
 * ================================================================ */

var (
	// 抖音主页 ID：兼容 douyin.com/user/<id> 与 iesdouyin.com/share/user/<id> 两种形式
	userShortIDRe = regexp.MustCompile(`(?:iesdouyin\.com/share/user|douyin\.com/user)/([A-Za-z0-9_\-]+)`)
	shortLinkRe   = regexp.MustCompile(`https?://v\.douyin\.com/[A-Za-z0-9_\-/]+`)
	anyLinkRe     = regexp.MustCompile(`https?://[^\s]+`)
)

// HTTP 客户端：短链跟随重定向用，超时保护避免长时间阻塞
var redirectClient = &http.Client{
	Timeout: 6 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("重定向次数过多")
		}
		return nil
	},
}

// ExtractLinks 从粘贴文本中抽取所有抖音主页链接（去重、保序）
func ExtractLinks(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range shortLinkRe.FindAllString(text, -1) {
		k := strings.TrimRight(m, "/")
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, m := range userShortIDRe.FindAllString(text, -1) {
		k := strings.TrimRight(m, "/")
		if !seen[k] {
			seen[k] = true
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		// 兜底：整段视为一个 URL（去掉行尾标点）
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if m := anyLinkRe.FindString(line); m != "" {
				if strings.Contains(m, "douyin.com") {
					k := strings.Trim(m, "，。；;,.")
					if !seen[k] {
						seen[k] = true
						out = append(out, k)
					}
				}
			}
		}
	}
	return out
}

// ExtractSecUID 从链接直接抽取 sec_uid（短链走重定向解析）
func ExtractSecUID(link string) (string, error) {
	l := strings.TrimSpace(link)
	if m := userShortIDRe.FindStringSubmatch(l); len(m) == 2 {
		if m[1] != "" {
			return m[1], nil
		}
	}
	// 短链：跟随重定向拿到最终 URL 再抽取
	if shortLinkRe.MatchString(l) {
		u, err := resolveShort(shortLinkRe.FindString(l))
		if err != nil {
			return "", err
		}
		if m := userShortIDRe.FindStringSubmatch(u); len(m) == 2 && m[1] != "" {
			return m[1], nil
		}
		return "", errors.New("短链未能解析出主页 ID")
	}
	return "", errors.New("无法从链接解析主页 ID")
}

func resolveShort(link string) (string, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	resp, err := redirectClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String(), nil
	}
	return link, nil
}

// LookLikeDouyinLink 粗判是否为疑似抖音主页链接（用于 import 时跳过明显无效行）
func LookLikeDouyinLink(text string) bool {
	return strings.Contains(text, "douyin.com")
}
