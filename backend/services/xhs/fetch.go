package xhs

import (
	"errors"
	"io"
	"net/http"
	"time"

	"geo-tool/models"
)

/* ================================================================
 * 小红书公开页同步抓取（尽力而为）
 *
 * 合规说明：仅抓取小红书公开主页页面（user/profile 公开页），不调用
 * 任何需要登录态、签名或风控绕过的接口；抓取失败、被风控拦截或页面
 * 无 SSR 数据时降级为估算数据（SourcedFrom=estimate），绝不把估算
 * 冒充真实数据。估算数据仅作运营参考，请以官方客户端实际为准。
 * ================================================================ */

var fetchClient = &http.Client{Timeout: 8 * time.Second}

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

func httpGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP " + resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 上限 8MB
	if err != nil {
		return nil, err
	}
	return b, nil
}

// FetchProfile 尽力抓取同行主页公开信息；失败返回 error 由调用方降级到估算
func FetchProfile(link string) (*Profile, error) {
	homeID, err := ResolveHomeID(link)
	if err != nil {
		return nil, err
	}
	html, err := httpGet("https://www.xiaohongshu.com/user/profile/" + homeID)
	if err != nil {
		return nil, err
	}
	state := LoadState(string(html))
	if state == nil {
		return nil, errors.New("页面无内嵌数据")
	}
	uv, ok := findXhsUserInfo(state)
	if !ok {
		return nil, errors.New("页面未包含可解析的用户信息")
	}
	nick := firstString(uv, "nickname", "nickName")
	fans := firstInt64(uv, "fans", "fansCount", "followerCount", "fan_count")
	notes := firstInt64(uv, "showedNotesCount", "notesCount", "noteCount", "notes_count")
	if nick == "" && fans == 0 && notes == 0 {
		return nil, errors.New("未能识别主页关键字段")
	}
	return &Profile{
		Link: link, HomeID: homeID, Nickname: nick,
		FansCount: fans, NoteCount: notes, SourcedFrom: models.SourceReal,
	}, nil
}

// FetchNotesReal 尽力抓取同行公开笔记列表（最多 maxN 条），失败返回 error
func FetchNotesReal(link string, maxN int) ([]Note, error) {
	homeID, err := ResolveHomeID(link)
	if err != nil {
		return nil, err
	}
	html, err := httpGet("https://www.xiaohongshu.com/user/profile/" + homeID)
	if err != nil {
		return nil, err
	}
	state := LoadState(string(html))
	if state == nil {
		return nil, errors.New("页面无内嵌数据")
	}
	cards, ok := findXhsNotes(state)
	if !ok {
		return nil, errors.New("页面未包含笔记列表")
	}
	out := make([]Note, 0, min2(len(cards), maxN))
	for _, c := range cards {
		if len(out) >= maxN {
			break
		}
		title := firstString(c, "displayTitle", "title", "desc")
		like := firstInt64(c, "likedCount", "liked_count")
		save := firstInt64(c, "collectedCount", "collected_count")
		comment := firstInt64(c, "commentCount", "comment_count")
		if title == "" && like == 0 {
			continue
		}
		// 互动时间：interactInfo 内层的 time 字段可能不在同一节点，
		// 尝试从交互子节点二次提取
		var pub time.Time
		if it, ok := c["interactInfo"].(map[string]interface{}); ok {
			pub = noteTimeFrom(it["time"])
		} else {
			pub = noteTimeFrom(mapVal(c, "time", "publishTime", "releaseTime"))
		}
		n := Note{
			Title: trimTitleFrom(title), LikeCount: like,
			SaveCount: save, CommentCount: comment,
			PublishTime: pub, SourcedFrom: models.SourceReal,
		}
		n.computeRate()
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, errors.New("笔记列表为空")
	}
	return out, nil
}

// mapVal 宽容取键（首个存在的）
func mapVal(m map[string]interface{}, keys ...string) interface{} {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
