package douyin

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/* ================================================================
 * 抖音分享页 SSR 数据抽取（尽力而为、结构不敏感）
 *
 * 网页结构会随版本变化，因此采用"通用化 JSON 提取 + 宽容字段匹配"：
 *  - 先定位 _ROUTER_DATA / RENDER_DATA 等内嵌 JSON 块；
 *  - 再递归遍历整棵 JSON 树，按字段名宽匹配关键信息（nickname、
 *    follower_count、aweme_count、作品对象含 desc+statistics）。
 * 结构变更导致无法匹配时返回 error，由上层降级为估算数据。
 * ================================================================ */

// LoadShareData 从 HTML 中提取内嵌 JSON 树（可能为 nil）
func LoadShareData(html string) map[string]interface{} {
	re := regexp.MustCompile(`window\._ROUTER_DATA(?:_SAVED)?\s*=\s*(\{.*?\})\s*;`)
	var raw string
	if m := re.FindStringSubmatch(html); len(m) == 2 {
		raw = m[1]
	} else {
		// 退而求其次：定位任意一个已知标记后的 JSON 对象
		candidates := []string{"RENDER_DATA", "window.__INITIAL_STATE__ ="}
		idx := -1
		var marker string
		for _, c := range candidates {
			if i := strings.Index(html, c); i >= 0 {
				idx = i
				marker = c
				break
			}
		}
		if idx < 0 {
			return nil
		}
		rest := html[idx+len(marker):]
		if j := strings.Index(rest, "{"); j >= 0 {
			rest = rest[j:]
		}
		raw = extractJSONObject(rest)
	}
	if raw == "" {
		return nil
	}
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil
	}
	return root
}

// extractJSONObject 从片段开头截取一个括号配平的 JSON 对象文本
func extractJSONObject(s string) string {
	depth := 0
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

// findAweStore 找到包含用户信息与作品列表的"创作者数据节点"
func findAweStore(root map[string]interface{}) (map[string]interface{}, bool) {
	best := map[string]interface{}{}
	found := false
	walk(root, func(m map[string]interface{}) bool {
		if hasAnyKey(m, "follower_count", "followerCount", "aweme_count", "awemeCount") {
			// 优先带用户昵称的同级节点
			if _, ok := m["nickname"]; ok {
				best = m
				found = true
				return true
			}
			// 否则先记录，继续寻找更优节点
			if len(m) > len(best) {
				best = m
				found = true
			}
		}
		return false
	})
	return best, found
}

// findAweLists 收集所有候选作品对象（含 desc 且含播放统计的节点）
func findAweLists(root map[string]interface{}) ([]map[string]interface{}, bool) {
	var out []map[string]interface{}
	walk(root, func(m map[string]interface{}) bool {
		_, hasDesc := m["desc"]
		_, hasStats := m["statistics"]
		_, hasPlay := m["play_count"]
		if hasDesc && (hasStats || hasPlay) {
			out = append(out, m)
		}
		return false
	})
	return out, len(out) > 0
}

// walk 深度优先遍历 JSON 树，回调返回 true 时停止遍历
func walk(node interface{}, fn func(map[string]interface{}) bool) bool {
	switch t := node.(type) {
	case map[string]interface{}:
		if fn(t) {
			return true
		}
		for _, v := range t {
			if walk(v, fn) {
				return true
			}
		}
	case []interface{}:
		for _, v := range t {
			if walk(v, fn) {
				return true
			}
		}
	}
	return false
}

// ---- 宽容取值工具 ----

func hasAnyKey(m map[string]interface{}, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				return strings.TrimSpace(t)
			case float64:
				return trimTitle(formatInt(int64(t)))
			}
		}
	}
	return ""
}

func firstInt64(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if n := fromNumber(v); n > 0 {
				return n
			}
		}
	}
	return 0
}

// fromNumber 兼容 JSON 数字与个别的"1.2w/1.2万"字符串格式
func fromNumber(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		s := strings.TrimSpace(t)
		multi := int64(1)
		if strings.HasSuffix(s, "w") || strings.HasSuffix(s, "W") {
			multi = 10000
			s = strings.TrimRight(s, "wW")
		} else if strings.Contains(s, "万") {
			multi = 10000
			s = strings.ReplaceAll(s, "万", "")
		}
		s = strings.ReplaceAll(s, ",", "")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int64(f * float64(multi))
	}
	return 0
}

func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

func trimTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(无标题视频)"
	}
	r := []rune(s)
	if len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}

func timeFromUnix(sec int64) time.Time {
	if sec <= 0 {
		return time.Now().AddDate(0, 0, -3)
	}
	return time.Unix(sec, 0)
}
