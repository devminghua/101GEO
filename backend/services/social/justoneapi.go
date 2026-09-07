// Package social 封装第三方数据 API（Just One API），提供抖音/小红书的稳定数据抓取。
// 认证：token 查询参数，通过环境变量 GEO_DATA_API_TOKEN 配置。
// 用于替代/兜底无头浏览器抓取，解决平台反爬导致的数据缺失。
package social

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"geo-tool/database"
	"geo-tool/models"
)

const (
	// BaseURL Just One API 服务地址（大陆可换 http://47.117.133.51:30015）
	BaseURL = "https://api.justoneapi.com"
	// TokenKey 全局设置 key（SaaS 后台配置的第三方数据 API token）
	TokenKey = "data_api_token"
)

// token 读取 API token：优先 SaaS 后台配置（数据库全局设置），回退环境变量。
func token() string {
	if database.DB != nil {
		var s models.Setting
		if err := database.DB.Where("tenant_id = 0 AND key = ?", TokenKey).First(&s).Error; err == nil {
			if v := strings.TrimSpace(s.Value); v != "" {
				return v
			}
		}
	}
	return strings.TrimSpace(os.Getenv("GEO_DATA_API_TOKEN"))
}

// Token 导出读取 API token（供配置接口展示用）。
func Token() string { return token() }

// Enabled 是否已配置第三方数据 API。
func Enabled() bool { return token() != "" }

// get 发起 GET 请求，返回业务 data 部分（map）。
func get(path string, params map[string]string) (map[string]interface{}, error) {
	if token() == "" {
		return nil, errors.New("未配置 GEO_DATA_API_TOKEN")
	}
	q := url.Values{}
	q.Set("token", token())
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u := BaseURL + path + "?" + q.Encode()
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("API 响应解析失败: %v", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("API 错误(%d): %s", envelope.Code, envelope.Message)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("data 解析失败: %v", err)
	}
	return data, nil
}

/* ============ 抖音 ============ */

// DouyinUser 抖音用户信息。
type DouyinUser struct {
	Nickname   string
	FansCount  int64
	VideoCount int64
}

// DouyinVideo 抖音视频信息。
type DouyinVideo struct {
	VideoID      string
	Title        string
	PlayCount    int64
	LikeCount    int64
	CommentCount int64
	PublishTime  time.Time
}

// DouyinUserDetail 抖音用户详情（粉丝数）。
func DouyinUserDetail(secUID string) (*DouyinUser, error) {
	data, err := get("/api/douyin/get-user-detail/v3", map[string]string{"secUid": secUID})
	if err != nil {
		return nil, err
	}
	// 数据在 data.user 子对象下
	user := pickObject(data, "user")
	if user == nil {
		user = data
	}
	u := &DouyinUser{
		Nickname:   pickString(user, "nickname", "nick_name", "name"),
		FansCount:  pickInt(user, "follower_count", "followerCount", "fans", "mplatform_followers_count"),
		VideoCount: pickInt(user, "aweme_count", "awemeCount", "total_video_count"),
	}
	if u.Nickname == "" && u.FansCount == 0 {
		return nil, errors.New("抖音用户数据为空")
	}
	return u, nil
}

// DouyinVideoList 抖音用户发布视频列表。
func DouyinVideoList(secUID string, maxN int) ([]DouyinVideo, error) {
	data, err := get("/api/douyin/get-user-video-list/v3", map[string]string{"secUid": secUID, "maxCursor": "0"})
	if err != nil {
		return nil, err
	}
	list := pickArray(data, "aweme_list", "video_list", "list", "data")
	if len(list) == 0 {
		// 尝试从 data 本身取数组
		if arr, ok := data["aweme_list"].([]interface{}); ok {
			list = arr
		}
	}
	out := make([]DouyinVideo, 0, maxN)
	for _, item := range list {
		if len(out) >= maxN {
			break
		}
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vid := pickString(m, "aweme_id", "awemeId", "id")
		title := pickString(m, "desc", "title", "caption")
		stats := pickObject(m, "statistics", "stats")
		v := DouyinVideo{
			VideoID:      vid,
			Title:        title,
			PlayCount:    pickInt(stats, "play_count", "playCount"),
			LikeCount:    pickInt(stats, "digg_count", "diggCount", "like_count"),
			CommentCount: pickInt(stats, "comment_count", "commentCount"),
		}
		if ct := pickInt(m, "create_time", "createTime"); ct > 0 {
			v.PublishTime = time.Unix(ct, 0)
		}
		if title == "" && v.LikeCount == 0 {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errors.New("抖音视频列表为空")
	}
	return out, nil
}

/* ============ 小红书 ============ */

// XhsUser 小红书用户信息。
type XhsUser struct {
	Nickname   string
	FansCount  int64
	NoteCount  int64
}

// XhsNote 小红书笔记信息。
type XhsNote struct {
	NoteID       string
	Title        string
	LikeCount    int64
	SaveCount    int64
	CommentCount int64
	PublishTime  time.Time
}

// XhsUserProfile 小红书用户资料（粉丝数）。
func XhsUserProfile(userID string) (*XhsUser, error) {
	data, err := get("/api/xiaohongshu/get-user/v3", map[string]string{"userId": userID})
	if err != nil {
		return nil, err
	}
	u := &XhsUser{
		Nickname:  pickString(data, "nickname", "nick_name", "name"),
		FansCount: pickInt(data, "fans", "fans_count", "follower_count", "followers"),
		NoteCount: pickInt(data, "notes_count", "note_count", "showed_notes_count"),
	}
	if u.Nickname == "" && u.FansCount == 0 {
		return nil, errors.New("小红书用户数据为空")
	}
	return u, nil
}

// XhsUserNotes 小红书用户发布笔记列表。
func XhsUserNotes(userID string, maxN int) ([]XhsNote, error) {
	data, err := get("/api/xiaohongshu/get-user-note-list/v4", map[string]string{"userId": userID, "cursor": "0"})
	if err != nil {
		return nil, err
	}
	list := pickArray(data, "notes", "note_list", "list", "data")
	out := make([]XhsNote, 0, maxN)
	for _, item := range list {
		if len(out) >= maxN {
			break
		}
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		nid := pickString(m, "note_id", "noteId", "id")
		title := pickString(m, "display_title", "displayTitle", "title", "desc")
		// 点赞 likes / 收藏 collected_count / 评论 comments_count 均在笔记顶层
		n := XhsNote{
			NoteID:       nid,
			Title:        title,
			LikeCount:    pickInt(m, "likes", "liked_count", "likedCount", "like_count"),
			SaveCount:    pickInt(m, "collected_count", "collectedCount", "collect_count"),
			CommentCount: pickInt(m, "comments_count", "comment_count", "commentCount"),
		}
		if ct := pickInt(m, "create_time", "createTime", "time"); ct > 0 {
			if ct > 100000000000 { // 毫秒时间戳
				ct = ct / 1000
			}
			n.PublishTime = time.Unix(ct, 0)
		}
		if title == "" && n.LikeCount == 0 {
			continue
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, errors.New("小红书笔记列表为空")
	}
	return out, nil
}

/* ============ 通用字段提取（宽容多字段名） ============ */

// pickString 从 map 中按多个候选 key 取字符串。
func pickString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				return strconv.FormatInt(int64(t), 10)
			case json.Number:
				return t.String()
			}
		}
	}
	return ""
}

// pickInt 从 map 中按多个候选 key 取整数。
func pickInt(m map[string]interface{}, keys ...string) int64 {
	if m == nil {
		return 0
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int64(t)
			case int64:
				return t
			case int:
				return int64(t)
			case string:
				if n, err := parseWan(t); err == nil {
					return n
				}
			case json.Number:
				if n, err := t.Int64(); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

// pickObject 从 map 中取子对象。
func pickObject(m map[string]interface{}, keys ...string) map[string]interface{} {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if mm, ok := v.(map[string]interface{}); ok {
				return mm
			}
		}
	}
	return nil
}

// pickArray 从 map 中取数组。
func pickArray(m map[string]interface{}, keys ...string) []interface{} {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if arr, ok := v.([]interface{}); ok {
				return arr
			}
		}
	}
	return nil
}

// parseWan 解析「9.1万」「91000」「1.2亿」等为整数。
func parseWan(s string) (int64, error) {
	s = strings.TrimSpace(s)
	mult := int64(1)
	switch {
	case strings.Contains(s, "亿"):
		mult = 100000000
	case strings.Contains(s, "万"), strings.Contains(s, "w"), strings.Contains(s, "W"):
		mult = 10000
	}
	s = strings.TrimRight(s, "万wW亿")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * float64(mult)), nil
}
