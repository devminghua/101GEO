package xhs

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/* ================================================================
 * 小红书页面 SSR 数据抽取（尽力而为、结构不敏感）
 *
 * 小红书网页为纯前端渲染，公开主页会内嵌 __INITIAL_STATE__ / RENDER_DATA
 * 等 JSON 状态。此处采用"通用化 JSON 提取 + 宽容字段匹配"：
 *  - 先定位内嵌 JSON 块并做括号配平抽取；
 *  - 再递归遍历整棵 JSON 树，按字段名宽匹配关键信息
 *    （昵称、粉丝数、笔记数、笔记卡片：标题/点赞/收藏/评论/时间）。
 * 结构变更导致无法匹配时返回 error，由上层降级为估算数据。
 * ================================================================ */

// LoadState 从 HTML 中提取内嵌 JSON 树（可能为 nil）
func LoadState(html string) map[string]interface{} {
	var raw string

	re := regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*(\{.*?\})\s*[;<\/]`)
	if m := re.FindStringSubmatch(html); len(m) == 2 {
		raw = m[1]
	} else {
		raw = ""
	}
	if raw == "" {
		candidates := []string{
			"window.__INITIAL_STATE__=",
			"window.__INITIAL_STATE__ =",
			"RENDER_DATA",
			"window.__NUXT__ =",
		}
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

// findXhsUserInfo 找到包含用户基本信息的节点（昵称/粉丝/笔记数）
func findXhsUserInfo(root map[string]interface{}) (map[string]interface{}, bool) {
	best := map[string]interface{}{}
	found := false
	walk(root, func(m map[string]interface{}) bool {
		// 至少含昵称，且同簇含粉丝/笔记数之一
		hasNick := hasAnyKey(m, "nickname", "nickName")
		_, hasFans := hasKeyBool(m, "fans", "fansCount", "followerCount", "fan_count")
		_, hasNotes := hasKeyBool(m, "showedNotesCount", "notesCount", "noteCount", "notes_count")
		if hasNick && (hasFans || hasNotes) {
			if len(m) > len(best) {
				best = m
				found = true
			}
		}
		return false
	})
	return best, found
}

// findXhsNotes 收集所有候选笔记卡片（含标题 + 点赞/收藏/评论 任意二者的节点）
func findXhsNotes(root map[string]interface{}) ([]map[string]interface{}, bool) {
	var out []map[string]interface{}
	walk(root, func(m map[string]interface{}) bool {
		ids, idOK := hasKeyBool(m, "noteId", "note_id", "id")
		if !idOK {
			return false
		}
		_, hasTitle := hasKeyBool(m, "displayTitle", "title", "desc")
		score := 0
		for _, k := range []string{"likedCount", "liked_count", "collectedCount", "collected_count", "commentCount", "comment_count"} {
			if _, ok := m[k]; ok {
				score++
			}
		}
		if ids && hasTitle && score >= 2 {
			out = append(out, m)
		}
		return false
	})
	return out, len(out) > 0
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

func hasKeyBool(m map[string]interface{}, keys ...string) (bool, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil && v != "" {
			return true, true
		}
	}
	return false, false
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				return strings.TrimSpace(t)
			case float64:
				return formatIntFromInt64(int64(t))
			}
		}
	}
	return ""
}

func firstInt64(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if n := fromCount(v); n > 0 {
				return n
			}
		}
	}
	return 0
}

// fromCount 兼容 JSON 数字与"1.2w/1.2万/9999+"字符串格式
func fromCount(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case bool:
		return 0
	case string:
		s := strings.TrimSpace(t)
		s = strings.TrimRight(s, "+")
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

func formatIntFromInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}

// 笔记发布时间：xhs 使用毫秒时间戳（time）/ 或字符串，兼容处理
func noteTimeFrom(v interface{}) time.Time {
	switch t := v.(type) {
	case float64:
		ms := int64(t)
		if ms > 1e12 { // 毫秒
			return time.Unix(ms/1000, 0)
		}
		if ms > 0 {
			return time.Unix(ms, 0)
		}
	case string:
		if s, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
			if s > 1e12 {
				return time.Unix(s/1000, 0)
			}
			if s > 0 {
				return time.Unix(s, 0)
			}
		}
	}
	// 兜底：近 3 天
	return time.Now().AddDate(0, 0, -3)
}

func trimTitleFrom(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(无标题笔记)"
	}
	r := []rune(s)
	if len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
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
