package douyin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"geo-tool/models"
)

/* ================================================================
 * 公开页同步抓取（尽力而为）
 *
 * 合规说明：仅抓取抖音公开页面数据（主页/公开作品列表），不调用
 * 任何需要登录态、签名或风控绕过的接口；抓取失败或页面结构变更时
 * 降级为估算数据（SourcedFrom=estimate），绝不把估算冒充真实数据。
 *
 * 抓取目标：
 *  - https://www.douyin.com/user/<sec_uid>        （网页版主页）
 *  - https://www.iesdouyin.com/share/user/<sec_uid>（轻量分享页，含 SSR 数据）
 * ================================================================ */

var fetchClient = &http.Client{Timeout: 8 * time.Second}

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func httpGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
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
	secUID, err := ExtractSecUID(link)
	if err != nil {
		return nil, err
	}
	html, err := httpGet("https://www.iesdouyin.com/share/user/" + secUID)
	if err != nil {
		return nil, err
	}
	store := LoadShareData(string(html))
	pv, ok := findAweStore(store)
	if !ok {
		return nil, errors.New("页面未包含可解析的主页数据")
	}
	nick := firstString(pv, "nickname", "nick_name")
	fans := firstInt64(pv, "follower_count", "followerCount")
	awemes := firstInt64(pv, "aweme_count", "awemeCount")
	if nick == "" && fans == 0 && awemes == 0 {
		return nil, errors.New("未能识别主页关键字段")
	}
	return &Profile{
		Link: link, SecUID: secUID, Nickname: nick,
		FansCount: fans, VideoCount: awemes, SourcedFrom: models.SourceReal,
	}, nil
}

// FetchVideosReal 尽力抓取同行公开作品列表（最多 maxN 条），失败返回 error
func FetchVideosReal(link string, maxN int) ([]Video, error) {
	secUID, err := ExtractSecUID(link)
	if err != nil {
		return nil, err
	}
	html, err := httpGet("https://www.iesdouyin.com/share/user/" + secUID)
	if err != nil {
		return nil, err
	}
	store := LoadShareData(string(html))
	awemes, ok := findAweLists(store)
	if !ok {
		return nil, errors.New("页面未包含作品列表")
	}
	out := make([]Video, 0, min2(len(awemes), maxN))
	for _, a := range awemes {
		if len(out) >= maxN {
			break
		}
		desc := firstString(a, "desc")
		stats, _ := a["statistics"].(map[string]interface{})
		play := fromNumber(mapVal(stats, "play_count", "digg_play_count"))
		digg := fromNumber(mapVal(stats, "digg_count"))
		comment := fromNumber(mapVal(stats, "comment_count"))
		ct := fromNumber(mapVal(a, "create_time"))
		if desc == "" && play == 0 {
			continue
		}
		v := Video{
			Title: trimTitle(desc), PlayCount: play, LikeCount: digg,
			CommentCount: comment,
			PublishTime:  timeFromUnix(ct),
			SourcedFrom:  models.SourceReal,
		}
		v.computeRate()
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errors.New("作品列表为空")
	}
	return out, nil
}

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

/* ================================================================
 * 无头浏览器抓取（绕过抖音 JSVM 反爬）
 *
 * 抖音网页版现已对 SSR 数据做 JS 混淆（_$jsvmprt），纯 HTTP 抓取只能拿到
 * 空壳页面；用无头浏览器（chromium --headless --dump-dom）执行 JS 后可拿到
 * 渲染后的真实数据（昵称 / 粉丝数 / 视频标题）。
 * 浏览器路径优先取环境变量 GEO_BROWSER_BIN，否则探测常见二进制名。
 * ================================================================ */

var browserBins = []string{"chromium", "chromium-browser", "google-chrome", "chrome", "chrome-headless-shell"}

// browserPath 探测无头浏览器可执行文件路径。
func browserPath() string {
	if bin := os.Getenv("GEO_BROWSER_BIN"); bin != "" {
		return bin
	}
	for _, b := range browserBins {
		if _, err := exec.LookPath(b); err == nil {
			return b
		}
	}
	return ""
}

// fetchWithBrowser 用无头浏览器抓取渲染后的 DOM（HTML）。
func fetchWithBrowser(url string) (string, error) {
	bin := browserPath()
	if bin == "" {
		return "", errors.New("无头浏览器不可用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"--headless=new", "--disable-gpu", "--no-sandbox", "--dump-dom",
		"--virtual-time-budget=15000",
		"--user-agent="+userAgent,
		"--window-size=1280,3000",
		url,
	)
	// 忽略 stderr（chromium 会输出大量调试信息）
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// fetchWithScroll 用 chromedp 滚动页面触发懒加载，返回滚动后的完整 HTML（含视频标题）。
func fetchWithScroll(url string) (string, error) {
	bin := browserPath()
	if bin == "" {
		return "", errors.New("无头浏览器不可用")
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(bin),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent(userAgent),
		chromedp.WindowSize(1280, 3000),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(string, ...interface{}) {}))
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(4*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 多次滚动触发视频列表懒加载
			var ignored interface{}
			for i := 0; i < 8; i++ {
				if err := chromedp.Evaluate(`window.scrollBy(0, 3000)`, &ignored).Do(ctx); err != nil {
					return err
				}
				if err := chromedp.Sleep(1200 * time.Millisecond).Do(ctx); err != nil {
					return err
				}
			}
			_ = chromedp.Evaluate(`window.scrollTo(0, 0)`, &ignored).Do(ctx)
			return nil
		}),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return "", err
	}
	return html, nil
}

var (
	nickRe       = regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`)
	titleRe      = regexp.MustCompile(`<title>([^<]+)</title>`)
	fansRe       = regexp.MustCompile(`粉丝</div><div[^>]*>([0-9]+(?:\.[0-9]+)?\s*[万wW亿]?)`)
	tagRe        = regexp.MustCompile(`<[^>]+>`)
	videoTitleRe = regexp.MustCompile(`<a[^>]+href="[^"]*/video/[0-9]+[^"]*"[^>]*>([^<]{1,120})</a>`)
)

// FetchProfileBrowser 无头浏览器抓取同行主页真实信息（昵称/粉丝/获赞）。
func FetchProfileBrowser(link, secUID string) (*Profile, error) {
	html, err := fetchWithBrowser("https://www.douyin.com/user/" + secUID)
	if err != nil {
		return nil, err
	}
	nick := ""
	if m := nickRe.FindStringSubmatch(html); len(m) > 1 {
		nick = strings.TrimSpace(stripTags(m[1]))
	}
	if nick == "" {
		// 兜底：从 <title>XXX的抖音 - 抖音</title> 提取
		if m := titleRe.FindStringSubmatch(html); len(m) > 1 {
			t := strings.TrimSpace(m[1])
			t = strings.TrimSuffix(t, "- 抖音")
			t = strings.TrimSuffix(t, "的抖音")
			t = strings.TrimSpace(t)
			if len(t) > 0 && len(t) < 64 {
				nick = t
			}
		}
	}
	fans := parseWanCount(fansRe.FindStringSubmatch(html))
	if nick == "" && fans == 0 {
		return nil, errors.New("浏览器抓取未识别到主页信息")
	}
	return &Profile{
		Link: link, SecUID: secUID, Nickname: nick,
		FansCount: fans, VideoCount: 0, SourcedFrom: models.SourceReal,
	}, nil
}

// FetchVideosBrowser 无头浏览器滚动抓取同行作品标题列表（触发懒加载）。
func FetchVideosBrowser(link string, maxN int) ([]Video, error) {
	secUID, err := ExtractSecUID(link)
	if err != nil {
		return nil, err
	}
	html, err := fetchWithScroll("https://www.douyin.com/user/" + secUID)
	if err != nil {
		return nil, err
	}
	titles := videoTitleRe.FindAllStringSubmatch(html, -1)
	out := make([]Video, 0, min2(len(titles), maxN))
	seen := map[string]bool{}
	for _, m := range titles {
		if len(out) >= maxN {
			break
		}
		t := strings.TrimSpace(m[1])
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, Video{
			Title: trimTitle(t), SourcedFrom: models.SourceReal,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("浏览器抓取未识别到作品标题")
	}
	return out, nil
}

// stripTags 去除 HTML 标签，返回纯文本。
func stripTags(s string) string {
	return strings.TrimSpace(tagRe.ReplaceAllString(s, ""))
}

// parseWanCount 解析「9.1万」「197.4万」「1.2亿」等中文计数为整数。
func parseWanCount(m []string) int64 {
	if len(m) < 2 {
		return 0
	}
	num := strings.TrimSpace(m[1])
	mult := int64(1)
	switch {
	case strings.Contains(num, "亿"):
		mult = 100000000
	case strings.Contains(num, "w"), strings.Contains(num, "W"):
		mult = 10000
	case strings.Contains(num, "万"):
		mult = 10000
	}
	num = strings.TrimRight(num, "万wW亿")
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	return int64(f * float64(mult))
}
