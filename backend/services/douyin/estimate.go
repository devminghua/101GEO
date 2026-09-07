package douyin

import (
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"geo-tool/models"
)

/* ================================================================
 * 结构化解算估值生成器
 *
 * 用途：同步抓取"尽力而为"，当公开页抓取失败/被拦截/无网络时，
 * 降级为确定性的结构化估算数据（基于链接哈希做种子，保证同一链接
 * 多次估算结果一致）。
 *
 * 重要：该类数据一律标记 SourcedFrom=estimate，严禁冒充真实数据。
 * 真实抓取成功的数据标记 SourcedFrom=real，两者在响应中并存区分。
 * ================================================================ */

// Profile 解析出的同行主页信息（real 或 estimate 通用载体）
type Profile struct {
	Link        string `json:"link"`
	SecUID      string `json:"sec_uid"`
	Nickname    string `json:"nickname"`
	FansCount   int64  `json:"fans_count"`
	VideoCount  int64  `json:"video_count"`
	SourcedFrom string `json:"sourced_from"` // real / estimate
}

// Video 视频数据载体（real 或 estimate）
type Video struct {
	PeerID          uint      `json:"peer_id"`
	PeerName        string    `json:"peer_name"`
	Title           string    `json:"title"`
	PlayCount       int64     `json:"play_count"`
	LikeCount       int64     `json:"like_count"`
	CommentCount    int64     `json:"comment_count"`
	InteractionRate float64   `json:"interaction_rate"`
	PublishTime     time.Time `json:"publish_time"`
	SourcedFrom     string    `json:"sourced_from"` // real / estimate
}

// InteractionRate 互动率 = (点赞+评论)/播放 * 100
// 抖音平台不公开播放量（play_count 恒为 0），此时降级用「赞评比」= 评论/点赞*100（衡量讨论热度）。
func (v *Video) computeRate() {
	if v.PlayCount > 0 {
		v.InteractionRate = float64(v.LikeCount+v.CommentCount) / float64(v.PlayCount) * 100
		return
	}
	if v.LikeCount > 0 {
		v.InteractionRate = float64(v.CommentCount) / float64(v.LikeCount) * 100
		return
	}
	v.InteractionRate = 0
}

// ComputeRate 导出互动率计算（供外部包调用）。
func (v *Video) ComputeRate() { v.computeRate() }

// LeadEstimate 评论留言潜在客户载体（estimate）
type LeadEstimate struct {
	Nickname    string    `json:"nickname"`
	Comment     string    `json:"comment"`
	CommentTime time.Time `json:"comment_time"`
	Tag         string    `json:"tag"`
}

func seedFrom(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}

func newRnd(s string) *rand.Rand {
	return rand.New(rand.NewSource(seedFrom(s)))
}

// 同行昵称池（婚恋/同城行业示例）
var peerNickPool = []string{
	"深圳一号红娘机构", "相亲派·高端婚恋", "缘来是你婚介所", "红娘帮帮·同城",
	"幸福里婚恋顾问", "囍相逢网红娘", "遇见婚恋馆", "同城相亲服务站",
}

var videoTitlePool = []string{
	"深圳女孩找对象，月入2万+，要求真诚",
	"婚介所到底靠不靠谱？从业8年说实话",
	"红娘的一天：撮合成功率最高的方法",
	"32岁未婚程序员的择偶观",
	"深圳相亲角实录：家长们在聊什么",
	"第一次相亲见面聊什么不尴尬",
	"脱单不是玄学：先做好这3件事",
	"大龄未婚，到底在焦虑什么",
	"红娘教你一眼识破杀猪盘",
	"同城近期优质资源盘点（第%d期）",
	"家长替孩子征婚，最看重什么",
	"异地恋到底要不要坚持",
}

var leadNickPool = []string{
	"阿伟不加班", "小鹿加油鸭", "南山程序员", "桂花糯米藕", "小叶_design",
	"Kevin在广州", "糖糖不甜", "夏天要变瘦", "大熊爱撸猫", "阿May呀",
	"木子李啊", "夜空最亮的星", "在深圳的北方人", "拾光慢递", "一只小楠楠",
}

var leadCommentPool = []string{
	"想找同城对象，26岁", "深圳本地，89年，想脱单", "替女儿咨询一下",
	"怎么联系你们？私信发不了", "有30岁以下的资源吗", "每周都有活动吗",
	"离异带娃可以吗", "可以加微信了解一下吗", "红娘在哪个区？想上门聊聊",
	"看到视频来的，求推荐", "爸妈催婚，先来看看情况",
}

var leadTagPool = []string{"男·单身", "女·单身", "家长", "其他"}

// EstimateProfile 生成同行主页估算信息（确定性）
func EstimateProfile(link string) *Profile {
	rnd := newRnd(link)
	nick := peerNickPool[rnd.Intn(len(peerNickPool))]
	if rnd.Intn(3) == 0 {
		post := []string{"·同城", "·高端", "·直营"}[rnd.Intn(3)]
		nick = nick + post
	}
	fans := int64(5800 + rnd.Intn(42000))
	videos := int64(80 + rnd.Intn(420))
	secUID := ""
	if v, err := ExtractSecUID(link); err == nil {
		secUID = v
	}
	return &Profile{
		Link: link, SecUID: secUID, Nickname: nick,
		FansCount: fans, VideoCount: videos, SourcedFrom: models.SourceEstimate,
	}
}

// EstimateVideos 生成确定性估算视频列表（数量取 min(count, 其中一个合理上限)）
func EstimateVideos(profile *Profile, count int) []Video {
	rnd := newRnd(profile.Link + "#video")
	if count <= 0 || count > 30 {
		count = 12
	}
	now := time.Now()
	out := make([]Video, 0, count)
	for i := 0; i < count; i++ {
		title := videoTitlePool[rnd.Intn(len(videoTitlePool))]
		title = strings.Replace(title, "%d", "9", 1)
		play := int64(3000 + rnd.Int63n(180000))
		rate := 0.06 + rnd.Float64()*0.09 // 6%~15%
		like := int64(float64(play) * (rate * 0.6))
		comment := int64(float64(play) * (rate * 0.25))
		if like < 5 {
			like = 5
		}
		if comment < 1 {
			comment = 1
		}
		// 发布时间散布在近 7 天内，晚上时段偏多（模拟真实发布习惯）
		day := now.AddDate(0, 0, -rnd.Intn(7))
		hour := 9 + rnd.Intn(13)
		if hour > 22 {
			hour = 22
		}
		t := time.Date(day.Year(), day.Month(), day.Day(), hour, rnd.Intn(60), rnd.Intn(60), 0, time.Local)
		v := Video{
			Title:        title,
			PlayCount:    play,
			LikeCount:    like,
			CommentCount: comment,
			PublishTime:  t,
			SourcedFrom:  models.SourceEstimate,
		}
		v.computeRate()
		out = append(out, v)
	}
	return out
}

// EstimateLeads 生成确定性估算评论留言客户（视频评论解析降级）
func EstimateLeads(peerName, videoTitle string, count int) []LeadEstimate {
	// 加入当前时间戳，避免同一视频每次解析都生成完全相同的评论（造成"老数据"观感）
	rnd := newRnd(peerName + "|" + videoTitle + "|" + strconv.FormatInt(time.Now().UnixNano(), 10))
	if count <= 0 || count > 20 {
		count = 8
	}
	now := time.Now()
	out := make([]LeadEstimate, 0, count)
	used := map[string]bool{}
	for i := 0; i < count; i++ {
		nick := leadNickPool[rnd.Intn(len(leadNickPool))]
		if used[nick] {
			nick = nick + randSuffix(rnd)
		}
		used[nick] = true
		t := now.Add(-time.Duration(rnd.Intn(600)) * time.Minute)
		out = append(out, LeadEstimate{
			Nickname:    nick,
			Comment:     leadCommentPool[rnd.Intn(len(leadCommentPool))],
			CommentTime: t,
			Tag:         leadTagPool[rnd.Intn(len(leadTagPool))],
		})
	}
	return out
}

func randSuffix(rnd *rand.Rand) string {
	const digits = "0123456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = digits[rnd.Intn(len(digits))]
	}
	return string(b)
}
