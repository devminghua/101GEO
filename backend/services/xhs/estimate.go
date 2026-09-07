package xhs

import (
	"hash/fnv"
	"math/rand"
	"strings"
	"time"

	"geo-tool/models"
)

/* ================================================================
 * 小红书获客 · 结构化解算估值生成器
 *
 * 用途：同步抓取"尽力而为"，当公开页抓取失败/被拦截/无网络/页面无
 * SSR 数据时，降级为确定性的结构化估算数据（基于链接哈希做种子，
 * 保证同一链接多次估算结果一致）。
 *
 * 重要：该类数据一律标记 SourcedFrom=estimate，严禁冒充真实数据。
 * 真实抓取成功的数据标记 SourcedFrom=real，两者在响应中并存区分。
 * 估算数据仅供参考，不可作为真实运营依据。
 * ================================================================ */

// Profile 解析出的同行主页信息（real 或 estimate 通用载体）
type Profile struct {
	Link        string `json:"link"`
	HomeID      string `json:"home_id"`
	Nickname    string `json:"nickname"`
	FansCount   int64  `json:"fans_count"`   // 粉丝数
	NoteCount   int64  `json:"note_count"`   // 笔记数
	SourcedFrom string `json:"sourced_from"` // real / estimate
}

// Note 笔记数据载体（real 或 estimate）
type Note struct {
	PeerID          uint      `json:"peer_id"`
	PeerName        string    `json:"peer_name"`
	Title           string    `json:"title"`
	LikeCount       int64     `json:"like_count"`
	SaveCount       int64     `json:"save_count"`
	CommentCount    int64     `json:"comment_count"`
	InteractionRate float64   `json:"interaction_rate"`
	PublishTime     time.Time `json:"publish_time"`
	SourcedFrom     string    `json:"sourced_from"` // real / estimate
}

// computeRate 互动率（折算参考值）：
// 小红书公开卡片无阅读量，此处以 (点赞+收藏+评论) 互动总量按 100 折算一
// 个百分点，上限 100，仅作横向对比参考，不作为运营真实指标。
func (n *Note) computeRate() {
	total := float64(n.LikeCount + n.SaveCount + n.CommentCount)
	rate := total / 100
	if rate > 100 {
		rate = 100
	}
	rate = float64(int(rate*10+0.5)) / 10
	n.InteractionRate = rate
}

// ComputeRate 导出互动率计算（供外部包调用）。
func (n *Note) ComputeRate() { n.computeRate() }

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
	"深圳同城恋爱研究所", "相亲帮·坐标深圳", "脱单情报站·珠三角",
	"红娘文姐说情感", "幸福补给站·婚恋", "遇见爱·同城脱单",
	"缘份天空婚恋馆", "恋爱小助手·深圳",
}

var noteTitlePool = []string{
	"深圳95后女孩的择偶标准，太真实了",
	"婚介会员费到底值得花吗？从业5年说真话",
	"红娘的一天：我成功撮合了21对",
	"30岁还单身，深圳女生的真实心态",
	"相亲角实录：深圳家长们最看重什么",
	"第一次见面聊什么最加分？红娘经验谈",
	"脱单先别急：先避开这3个坑",
	"同城优质单身资源盘点（第%d期）",
	"帮孩子征婚的家长，最看重对方哪三点",
	"异地恋和同城恋，哪个更容易成？",
	"为什么条件不错却总脱不了单",
	"单身女孩子怎么提升认识男生的渠道",
}

var leadNickPool = []string{
	"miumiu酱", "在深圳的Andrea", "小饼干要脱单", "阿凯同学",
	"奶茶三分糖", "Yuki_yuki", "北纬30度", "一颗栗子呀",
	"南山不加班", "时光机乘客", "小满不是小满", "夜风习习",
	"深海里的鲸", "草莓味夏天", "荔枝妹妹",
}

var leadCommentPool = []string{
	"同城姐妹求认识，98年",
	"深圳本地，89年，想脱单",
	"替女儿看看情况",
	"怎么联系你们呀？私信发不出去",
	"有30岁以下的吗",
	"这个平台靠谱吗？想报名",
	"离异带娃可以聊聊吗",
	"评论区姐妹有同城的吗",
	"红娘在哪个区？可以上门吗",
	"刷到就是缘分，求私信",
	"爸妈催得紧，先来了解下",
}

var leadTagPool = []string{"男·单身", "女·单身", "家长", "其他"}

// EstimateProfile 生成同行主页估算信息（确定性）
func EstimateProfile(link, homeID string) *Profile {
	rnd := newRnd(link)
	nick := peerNickPool[rnd.Intn(len(peerNickPool))]
	if rnd.Intn(3) == 0 {
		post := []string{"·同城", "·高端", "·直营"}[rnd.Intn(3)]
		nick = nick + post
	}
	fans := int64(3000 + rnd.Intn(52000))
	notes := int64(60 + rnd.Intn(500))
	return &Profile{
		Link: link, HomeID: homeID, Nickname: nick,
		FansCount: fans, NoteCount: notes, SourcedFrom: models.SourceEstimate,
	}
}

// EstimateNotes 生成确定性估算笔记列表（数量取 min(count, 合理上限)）
func EstimateNotes(profile *Profile, count int) []Note {
	rnd := newRnd(profile.Link + "#note")
	if count <= 0 || count > 30 {
		count = 10
	}
	now := time.Now()
	out := make([]Note, 0, count)
	for i := 0; i < count; i++ {
		title := noteTitlePool[rnd.Intn(len(noteTitlePool))]
		title = strings.Replace(title, "%d", "7", 1)
		like := int64(200 + rnd.Int63n(120000))
		save := int64(float64(like) * (0.15 + rnd.Float64()*0.35))
		comment := int64(float64(like) * (0.03 + rnd.Float64()*0.08))
		if save < 1 {
			save = 1
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
		n := Note{
			Title:        title,
			LikeCount:    like,
			SaveCount:    save,
			CommentCount: comment,
			PublishTime:  t,
			SourcedFrom:  models.SourceEstimate,
		}
		n.computeRate()
		out = append(out, n)
	}
	return out
}

// EstimateLeads 生成确定性估算评论留言客户（评论解析降级）
func EstimateLeads(peerName, noteTitle string, count int) []LeadEstimate {
	rnd := newRnd(peerName + "|" + noteTitle)
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
