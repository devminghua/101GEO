package ks

import (
	"hash/fnv"
	"math/rand"
	"strings"
	"time"

	"geo-tool/models"
)

/* ================================================================
 * 快手获客 · 估算数据服务（确定性估算，失败降级路径）
 *
 * 快手公开主页 SSR 数据极少、风控严格，抓取失败率高。
 * 与抖音/小红书同一口径：sourced_from=estimate 严格标记，
 * 估算数据仅供运营参考，严禁冒充真实数据。
 * 估算结果由输入链接确定性生成（同链接永远同结果，刷新不跳变）。
 * ================================================================ */

var ksPeerNickPool = []string{
	"红娘李姐", "情感驿站", "同城相亲圈", "脱单情报局",
	"婚恋那些事", "靠谱红娘王姨", "心动信号站", "缘分事务所",
	"婚介老张", "单身青年汇", "红线牵起来", "幸福说媒人",
}

var ksVideoTitlePool = []string{
	"本地单身男生资源多，想脱单的看过来",
	"红娘的一天：安排了 %d 对见面",
	"相亲避坑指南：第一次见面聊什么",
	"为什么现在的年轻人不着急结婚",
	"30+单身女性相亲实录",
	"父母帮孩子把关相亲对象的标准",
	"相亲市场现状：优质男生的条件是什么",
	"婚恋平台怎么选才靠谱",
	"相亲成功案例：认识三个月订婚了",
	"单身社群线下活动花絮",
}

var ksLeadNickPool = []string{
	"小城故事多", "秋天的奶茶", "想脱单的柚子", "北方姑娘在南方",
	"人生半场", "一路向北", "等你下课", "转角遇见爱",
	"慢慢来比较快", "时光不老", "顺其自然", "温柔的坚持",
}

var ksLeadCommentPool = []string{
	"同城求认识，95年",
	"郑州本地，88年，真诚找对象",
	"替儿子了解一下",
	"怎么联系你们？私信发不出去",
	"有35岁以下的吗",
	"这个平台靠谱吗？想了解一下",
	"离异可以聊聊吗",
	"评论区同城的有吗",
	"红娘在哪个区？能上门吗",
	"缘分到了挡不住，求私信",
}

var ksLeadTagPool = []string{"男·单身", "女·单身", "家长", "其他"}

// Profile 同行主页信息
type Profile struct {
	Link        string `json:"link"`
	HomeID      string `json:"home_id"`
	Nickname    string `json:"nickname"`
	FansCount   int64  `json:"fans_count"`
	VideoCount  int64  `json:"video_count"`
	SourcedFrom string `json:"sourced_from"`
}

// Video 同行视频信息
type Video struct {
	Title           string    `json:"title"`
	PlayCount       int64     `json:"play_count"`
	LikeCount       int64     `json:"like_count"`
	CommentCount    int64     `json:"comment_count"`
	InteractionRate float64   `json:"interaction_rate"`
	PublishTime     time.Time `json:"publish_time"`
	SourcedFrom     string    `json:"sourced_from"`
}

// LeadEstimate 估算评论客户
type LeadEstimate struct {
	Nickname    string    `json:"nickname"`
	Comment     string    `json:"comment"`
	CommentTime time.Time `json:"comment_time"`
	Tag         string    `json:"tag"`
}

// seedFrom 用字符串生成稳定种子
func seedFrom(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64())
}

func newRnd(s string) *rand.Rand {
	return rand.New(rand.NewSource(seedFrom(s)))
}

// EstimateProfile 生成同行主页估算信息（确定性）
func EstimateProfile(link, homeID string) *Profile {
	rnd := newRnd(link)
	nick := ksPeerNickPool[rnd.Intn(len(ksPeerNickPool))]
	if rnd.Intn(3) == 0 {
		post := []string{"·同城", "·高端", "·直营"}[rnd.Intn(3)]
		nick = nick + post
	}
	fans := int64(3000 + rnd.Intn(52000))
	videos := int64(60 + rnd.Intn(500))
	return &Profile{
		Link: link, HomeID: homeID, Nickname: nick,
		FansCount: fans, VideoCount: videos, SourcedFrom: models.SourceEstimate,
	}
}

// EstimateVideos 生成确定性估算视频列表
func EstimateVideos(profile *Profile, count int) []Video {
	rnd := newRnd(profile.Link + "#video")
	if count <= 0 || count > 30 {
		count = 10
	}
	now := time.Now()
	out := make([]Video, 0, count)
	for i := 0; i < count; i++ {
		title := ksVideoTitlePool[rnd.Intn(len(ksVideoTitlePool))]
		title = strings.Replace(title, "%d", "7", 1)
		play := int64(5000 + rnd.Int63n(900000))
		like := int64(float64(play) * (0.02 + rnd.Float64()*0.10))
		comment := int64(float64(play) * (0.002 + rnd.Float64()*0.01))
		if like < 1 {
			like = 1
		}
		if comment < 1 {
			comment = 1
		}
		day := now.AddDate(0, 0, -rnd.Intn(7))
		hour := 9 + rnd.Intn(13)
		if hour > 22 {
			hour = 22
		}
		t := time.Date(day.Year(), day.Month(), day.Day(), hour, rnd.Intn(60), rnd.Intn(60), 0, time.Local)
		v := Video{
			Title: title, PlayCount: play, LikeCount: like, CommentCount: comment,
			PublishTime: t, SourcedFrom: models.SourceEstimate,
		}
		v.InteractionRate = float64(like+comment) / float64(play) * 100
		out = append(out, v)
	}
	return out
}

// EstimateLeads 生成确定性估算评论客户
func EstimateLeads(peerName, videoTitle string, count int) []LeadEstimate {
	rnd := newRnd(peerName + "|" + videoTitle)
	if count <= 0 || count > 20 {
		count = 8
	}
	now := time.Now()
	out := make([]LeadEstimate, 0, count)
	used := map[string]bool{}
	for i := 0; i < count; i++ {
		nick := ksLeadNickPool[rnd.Intn(len(ksLeadNickPool))]
		if used[nick] {
			nick = nick + randSuffix(rnd)
		}
		used[nick] = true
		t := now.Add(-time.Duration(rnd.Intn(600)) * time.Minute)
		out = append(out, LeadEstimate{
			Nickname: nick, Comment: ksLeadCommentPool[rnd.Intn(len(ksLeadCommentPool))],
			CommentTime: t, Tag: ksLeadTagPool[rnd.Intn(len(ksLeadTagPool))],
		})
	}
	return out
}

func randSuffix(rnd *rand.Rand) string {
	const digits = "0123456789"
	b := make([]byte, 2)
	for i := range b {
		b[i] = digits[rnd.Intn(len(digits))]
	}
	return string(b)
}
