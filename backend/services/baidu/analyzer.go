package baidu

import (
	"regexp"
	"sort"
	"strings"
)

// ============ 同行识别 / 归因分析 / 标题套路 / 优化建议生成 ============

type PeerRow struct {
	Domain   string   `json:"domain"`
	Name     string   `json:"name"`
	Count    int      `json:"count"`
	Pages    []int    `json:"pages"`
	BestRank int      `json:"bestRank"`
	AvgRank  float64  `json:"avgRank"`
	Pattern  string   `json:"pattern"`
	Feats    []string `json:"feats"`
	Hits     []string `json:"hits"` // 标题样本
}

type SuspiciousPeer struct {
	Domain string `json:"domain"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Pages  []int  `json:"pages"`
	Hints  string `json:"hints"`
}

type Advice struct {
	Level string `json:"level"` // P0/P1/P2
	Tag   string `json:"tag"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Basis string `json:"basis"`
}

type PageInfo struct {
	Page          int        `json:"page"`
	Density       float64    `json:"density"` // 同行密度 0-1
	Ads           int        `json:"ads"`
	OrganicPeers  int        `json:"organicPeers"`
	OrganicOther  int        `json:"organicOther"`
	Items         []RankItem `json:"items"`
	ParseErrTag   string     `json:"parseErrTag,omitempty"` // 该页抓取失败原因
}

type Result struct {
	Keyword      string           `json:"keyword"`
	Depth        int              `json:"depth"`
	Peers        int              `json:"peers"`
	MyBest       int              `json:"myBest"`
	Heat         int              `json:"heat"`
	Pages        []PageInfo       `json:"pages"`
	PeerRows     []PeerRow        `json:"peerRows"`
	Suspicious   []SuspiciousPeer `json:"suspicious"`
	Advices      []Advice         `json:"advices"`
	TotalItems   int              `json:"totalItems"`
	MyDomains    []string         `json:"myDomains"`
	CostMs       int              `json:"costMs"`
	PartialFail  string           `json:"partialFail,omitempty"` // 有页面抓取失败时的说明
	TrendSummary string           `json:"trendSummary,omitempty"` // 我方该词最近一个月排名趋势摘要（上升/下降/持平）
}

type AnalyzeOptions struct {
	MyDomains []string // 我方站点域名（最好排名统计用）
	PeerLib   map[string]string
}

var (
	reSuccess   = regexp.MustCompile(`\d{2,4}\s*(?:万|%|人|位|对|位会员)?|成功率|成功率\s*\d+%|会员[^\s，。]{0,6}\d+万|牵手\d+`)
	reQuestion  = regexp.MustCompile(`[?？]`)
	reRegion    = regexp.MustCompile(`(深圳|北京|上海|广州|成都|杭州|武汉|南京|西安|天津|重庆|同城|本地)`)
	reTrust     = regexp.MustCompile(`(实名|认证|备案|资质|门店|直营|正规|口碑|十年|10年|承诺)`)
	reCta       = regexp.MustCompile(`(免费|咨询|领取|下载|注册|预约|报名|获取)`)
	reExclam    = regexp.MustCompile(`[!！]`)
	reDigit     = regexp.MustCompile(`\d+`)
)

// BuildResult 汇总解析后的多页数据 → 归因 + 建议
func BuildResult(keyword string, depth int, pages []PageInfo, suspicious []SuspiciousPeer, opts AnalyzeOptions) *Result {
	mySet := map[string]bool{}
	for _, d := range opts.MyDomains {
		mySet[d] = true
	}

	peerCount := map[string]int{}
	peerPages := map[string][]int{}
	peerBest := map[string]int{}
	peerRanks := map[string][]int{}
	peerTitles := map[string][]string{}
	peerFeatSet := map[string]map[string]bool{}
	var totalItems int
	myBest := 0

	for _, pg := range pages {
		totalItems += len(pg.Items)
		for _, it := range pg.Items {
			if mySet[it.Domain] && it.Type == "organic" {
				if myBest == 0 || it.Rank < myBest {
					myBest = it.Rank
				}
			}
			if !it.Peer || it.Type == "ad" {
				continue
			}
			peerCount[it.Domain]++
			if !containsInt(peerPages[it.Domain], pg.Page) {
				peerPages[it.Domain] = append(peerPages[it.Domain], pg.Page)
			}
			if peerBest[it.Domain] == 0 || it.Rank < peerBest[it.Domain] {
				peerBest[it.Domain] = it.Rank
			}
			peerRanks[it.Domain] = append(peerRanks[it.Domain], it.Rank)
			if len(peerTitles[it.Domain]) < 5 {
				peerTitles[it.Domain] = append(peerTitles[it.Domain], it.Title)
			}
			if peerFeatSet[it.Domain] == nil {
				peerFeatSet[it.Domain] = map[string]bool{}
			}
			for _, f := range titleFeatures(it.Title) {
				peerFeatSet[it.Domain][f] = true
			}
		}
	}

	rows := make([]PeerRow, 0, len(peerCount))
	for d, cnt := range peerCount {
		name := opts.PeerLib[d]
		if name == "" {
			name = d
		}
		var sum int
		for _, r := range peerRanks[d] {
			sum += r
		}
		avg := float64(sum) / float64(len(peerRanks[d]))
		feats := sortedKeys(peerFeatSet[d])
		pattern := summarizePattern(feats)
		rows = append(rows, PeerRow{
			Domain: d, Name: name, Count: cnt, Pages: sortedInts(peerPages[d]),
			BestRank: peerBest[d], AvgRank: round1(avg), Pattern: pattern, Feats: feats, Hits: peerTitles[d],
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].BestRank < rows[j].BestRank
	})

	// 竞争热度 0-100：同行密度 + 我方劣势 + 广告竞争
	density := 0.0
	for _, pg := range pages {
		density += pg.Density
	}
	if len(pages) > 0 {
		density /= float64(len(pages))
	}
	totalAds := 0
	for _, pg := range pages {
		totalAds += pg.Ads
	}
	avgAds := float64(totalAds) / float64(maxInt(1, len(pages)))
	heat := int(density*45 + minFloat(avgAds*4, 30) + float64(scoreMyBad(myBest))*8)
	if heat > 100 {
		heat = 100
	}
	if heat < 15 {
		heat = 15
	}

	advices := GenAdvice(keyword, depth, rows, myBest, density, totalAds, len(pages), opts)

	return &Result{
		Keyword: keyword, Depth: depth, Peers: len(rows), MyBest: myBest, Heat: heat,
		Pages: pages, PeerRows: rows, Suspicious: suspicious, Advices: advices,
		TotalItems: totalItems, MyDomains: opts.MyDomains,
	}
}

func scoreMyBad(myBest int) int {
	if myBest == 0 {
		return 1
	}
	if myBest <= 3 {
		return 0
	}
	if myBest <= 5 {
		return 1
	}
	if myBest <= 10 {
		return 2
	}
	return 3
}

// GenAdvice 对比我方与前排同行，输出 P0/P1/P2 建议
func GenAdvice(keyword string, depth int, rows []PeerRow, myBest int, density float64, totalAds, pageCount int, opts AnalyzeOptions) []Advice {
	var adv []Advice
	top3 := rows
	if len(top3) > 3 {
		top3 = rows[:3]
	}
	names := make([]string, 0, len(top3))
	for _, r := range top3 {
		names = append(names, r.Name)
	}
	dominant := "（暂无明确霸屏同行）"
	dominantCount := 0
	if len(rows) > 0 {
		dominant = rows[0].Name
		dominantCount = rows[0].Count
	}

	// P0-1 TDK 信任要素
	if len(rows) > 0 && myBest > 3 {
		adv = append(adv, Advice{
			Level: "P0", Tag: "TDK 重写",
			Title:   "标题缺失「数字信任」要素，前排名被同行霸占",
			Desc:    "前排同行（" + strings.Join(names, " / ") + "）标题普遍带「成功率」「会员 X 万」「实名认证」等硬背书。我方当前最佳排名第 " + orDash(myBest) + " 名，标题缺乏数字与信任词，点击率被压制。",
			Basis:   "前排标题高频特征：" + strings.Join(flattenFeats(top3), "、"),
		})
	}
	// P0-2 铺位密度
	if dominantCount >= 4 {
		adv = append(adv, Advice{
			Level: "P0", Tag: "自然排名",
			Title:   "自然排名铺位不足，霸屏同行「" + dominant + "」连续占位",
			Desc:    "霸屏同行在前 " + itoa(depth) + " 页自然结果中出现 " + itoa(dominantCount) + " 次，覆盖页码 " + joinInts(rows[0].Pages) + "。建议围绕同义长尾重写落地页，按「主词-长尾-地域变体」三层布局补位。",
			Basis:   "最好排名第 " + itoa(rows[0].BestRank) + " 位",
		})
	}
	// P1-1 长尾词
	adv = append(adv, Advice{
		Level: "P1", Tag: "长尾词切入",
		Title:   "低竞位长尾机会：地域 + 服务修饰词组合",
		Desc:    "同行在高意图修饰词（本地/1对1/高端/门店）组合上覆盖密度偏低，该类词搜索意图强、转化率高。建议以品牌名 + 地域 + 服务词新开 3-5 个长尾落地页，避开头部正面竞争。",
		Basis:   "前 5 页同行密度 " + pct(density) + "，存在卡位空间",
	})
	// P1-2 描述要素
	adv = append(adv, Advice{
		Level: "P1", Tag: "描述要素",
		Title:   "Description 缺少地域定位与行动号召（CTA）",
		Desc:    "同行描述普遍含地域词 + 「免费咨询/免费注册」CTA。建议补地域定位与低门槛 CTA，提升自然排名点击率。",
		Basis:   "地域词出现率：同行显著高于我方（基于标题特征统计）",
	})
	// P2-1 落地页信任
	adv = append(adv, Advice{
		Level: "P2", Tag: "信任元素",
		Title:   "落地页信任背书不足",
		Desc:    "前排同行落地页普遍具备「实名认证、成功案例、线下门店、备案资质」等信任元素。建议首屏补齐认证标识、真实案例、服务资质，形成转化闭环。",
		Basis:   "同行信任类特征明显（实名/认证/门店/口碑）",
	})
	// P2-2 广告观察
	if totalAds > 0 {
		adv = append(adv, Advice{
			Level: "P2", Tag: "广告位观察",
			Title:   "广告位由头部平台垄断，自然位是差异化主战场",
			Desc:    "前几页共出现 " + itoa(totalAds) + " 条广告，基本由头部平台投放。预算有限时建议集中优化自然排名，避免与头部正面对抗。",
			Basis:   "广告位以头部同行投放为主",
		})
	}
	// SEO 优化建议：结合同行密度与我方排名，给出落地页标题/TDK/外链等落地动作
	adv = append(adv, genSeoAdvice(keyword, rows, myBest, density, pageCount))
	return adv
}

// genSeoAdvice 生成「SEO 优化建议」类目条目：
// 结合同行密度（density）与我方最佳排名（myBest），输出落地页标题/TDK/外链等可执行动作。
func genSeoAdvice(keyword string, rows []PeerRow, myBest int, density float64, pageCount int) Advice {
	// 判定阶段：未上榜(0) / 排名靠后(>10) / 中游(6-10) / 前排(1-5)
	stage := "中游"
	switch {
	case myBest == 0:
		stage = "未上榜"
	case myBest > 10:
		stage = "靠后"
	case myBest <= 5:
		stage = "前排"
	}

	var desc, basis string
	switch stage {
	case "未上榜":
		desc = "当前「" + keyword + "」未进入前 " + itoa(pageCount*10) + " 名。建议：① 落地页 Title 直接含主词 + 地域词，TDK 全部重写并保证每页唯一；② 页面 H1 只保留主词，正文首段自然嵌入 2-3 次关键词变体；③ 从首页/栏目页内链至该落地页，锚文本用「" + keyword + "」；④ 申请百度收录并提交站点地图，配合外链冷启动（行业目录、问答平台、新闻源）。"
		basis = "同行密度 " + pct(density) + "，我方尚未上榜，需从收录与内链基础做起"
	case "靠后":
		desc = "当前「" + keyword + "」最佳排名第 " + orDash(myBest) + " 名，距首页较远。建议：① 对比前排同行 " + topNames(rows, 3) + " 的 TDK，逐项补齐数字/信任/地域要素；② 内容加厚：补充 FAQ、案例、数据表格，提升页面停留与相关性评分；③ 为落地页建设 3-5 条高质量外链（行业站/百科/论坛），锚文本自然多样；④ 优化移动端首屏加载与内链权重集中。"
		basis = "同行密度 " + pct(density) + "，前排头部占位密集，需长尾 + 内容加厚双管齐下"
	case "中游":
		desc = "当前「" + keyword + "」最佳排名第 " + orDash(myBest) + " 名，处于第二梯队。建议：① 主攻首屏前 3 名的缺口：拆分前排标题高频特征（" + strings.Join(flattenFeats(limitRows(rows, 3)), "、") + "）逐条对照；② 增加同义长尾页面做围剿，覆盖「" + keyword + "+ 地域/服务修饰词」组合；③ 外链策略向权威信源倾斜，提升整域权重带动该词上浮。"
		basis = "同行密度 " + pct(density) + "，向上突破需依赖内容相关性与权威外链的加权"
	default: // 前排
		desc = "当前「" + keyword + "」最佳排名第 " + orDash(myBest) + " 名，已进入前排。建议：① 巩固排名：保持更新频率，TDK 不做大改以防波动，仅小幅微调标题；② 扩词布局：在已上词基础上横向扩展 5-10 个同义词/长尾词页面，放大整域在该话题的话语权；③ 监控竞品动态：前排同行 " + topNames(rows, 3) + " 一旦加厚内容，需及时跟进；④ 保持外链持续供给，防止权重流失。"
		basis = "同行密度 " + pct(density) + "，前排位置需持续维护防竞品反超"
	}
	return Advice{
		Level: "SEO", Tag: "SEO 优化建议",
		Title:   "「" + keyword + "」SEO 落地动作（当前" + stage + "）",
		Desc:    desc,
		Basis:   basis,
	}
}

func limitRows(rows []PeerRow, n int) []PeerRow {
	if len(rows) > n {
		return rows[:n]
	}
	return rows
}

func topNames(rows []PeerRow, n int) string {
	names := make([]string, 0, n)
	for _, r := range limitRows(rows, n) {
		names = append(names, r.Name)
	}
	return strings.Join(names, " / ")
}

// titleFeatures 从标题抽取高频特征（数字/疑问/地域/信任/CTA/感叹/品牌）
func titleFeatures(title string) []string {
	var f []string
	add := func(s string) {
		for _, x := range f {
			if x == s {
				return
			}
		}
		f = append(f, s)
	}
	if reSuccess.MatchString(title) {
		add("数字/成功率背书")
	}
	if reQuestion.MatchString(title) {
		add("疑问句式")
	}
	if reRegion.MatchString(title) {
		add("地域词")
	}
	if reTrust.MatchString(title) {
		add("信任背书")
	}
	if reCta.MatchString(title) {
		add("行动号召CTA")
	}
	if reExclam.MatchString(title) {
		add("感叹引关注")
	}
	if len(f) == 0 {
		add("品牌词直出")
	}
	return f
}

func summarizePattern(feats []string) string {
	if len(feats) == 0 {
		return "「品牌词 + 通用简介」"
	}
	return "「" + strings.Join(feats, " + ") + "」"
}

// ============ 辅助 ============

func containsInt(a []int, v int) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}

func sortedInts(a []int) []int {
	b := append([]int{}, a...)
	sort.Ints(b)
	return b
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if m[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

func flattenFeats(rows []PeerRow) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range rows {
		for _, f := range r.Feats {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func orDash(v int) string {
	if v == 0 {
		return "-"
	}
	return itoa(v)
}

func itoa(v int) string {
	return strconvIt(v)
}

func strconvIt(v int) string {
	b := make([]byte, 0, 8)
	if v == 0 {
		return "0"
	}
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}

func joinInts(a []int) string {
	s := make([]string, 0, len(a))
	for _, v := range a {
		s = append(s, strconvIt(v))
	}
	return strings.Join(s, "、")
}

func pct(f float64) string {
	return strconvIt(int(f*100)) + "%"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

var _ = reDigit
