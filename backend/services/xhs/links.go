package xhs

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

/* ================================================================
 * 小红书链接解析
 * 支持形式：
 *  - https://www.xiaohongshu.com/user/profile/<hex_id>   （主页链接）
 *  - https://www.xiaohongshu.com/explore/<note_id>       （笔记链接）
 *  - https://xhslink.com/<short>                         （短链，需跟随跳转解析）
 *  - 复制分享文本（自动抽取其中的小红书链接）
 * ================================================================ */

var (
	profileRe  = regexp.MustCompile(`xiaohongshu\.com/user/profile/([0-9a-zA-Z]+)`)
	noteRe     = regexp.MustCompile(`xiaohongshu\.com/(?:explore|discovery/item)/([0-9a-zA-Z]+)`)
	xhsShortRe = regexp.MustCompile(`https?://xhslink\.com/[A-Za-z0-9_\-/]+`)
	anyLinkRe  = regexp.MustCompile(`https?://[^\s]+`)
)

// HTTP 客户端：短链跟随重定向用，超时保护避免长时间阻塞
var redirectClient = &http.Client{
	Timeout: 6 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 {
			return errors.New("重定向次数过多")
		}
		return nil
	},
}

// ExtractLinks 从粘贴文本中抽取所有小红书链接（去重、保序，先短链后主页）
func ExtractLinks(text string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(k string) {
		k = strings.TrimRight(strings.TrimSpace(k), "/")
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, m := range xhsShortRe.FindAllString(text, -1) {
		add(m)
	}
	for _, m := range profileRe.FindAllString(text, -1) {
		add(m)
	}
	for _, m := range noteRe.FindAllString(text, -1) {
		add(m)
	}
	if len(out) == 0 {
		// 兜底：整段视为一个 URL（去掉行尾标点）
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if m := anyLinkRe.FindString(line); m != "" && strings.Contains(m, "xiaohongshu.com") {
				add(strings.Trim(m, "，。；;,."))
			}
		}
	}
	return out
}

// ResolveHomeID 从链接解析出小红书主页 ID（含 xhslink 短链重定向解析）。
// 仅接受 user/profile 主页链接；短链解析后若落到笔记页则返回错误。
func ResolveHomeID(link string) (string, error) {
	l := strings.TrimSpace(link)
	if m := profileRe.FindStringSubmatch(l); len(m) == 2 && m[1] != "" {
		return m[1], nil
	}
	if noteRe.MatchString(l) {
		return "", errors.New("这是笔记链接而非主页链接，请粘贴同行主页链接（user/profile）")
	}
	if xhsShortRe.MatchString(l) {
		u, err := resolveShort(xhsShortRe.FindString(l))
		if err != nil {
			return "", err
		}
		if m := profileRe.FindStringSubmatch(u); len(m) == 2 && m[1] != "" {
			return m[1], nil
		}
		if noteRe.MatchString(u) {
			return "", errors.New("短链落到了笔记页，请粘贴同行主页分享短链")
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
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
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

// LookLikeXhsLink 粗判是否为疑似小红书链接（用于 import 时跳过明显无效行）
func LookLikeXhsLink(text string) bool {
	return strings.Contains(text, "xiaohongshu.com") || strings.Contains(text, "xhslink.com")
}
