package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/points"
)

// 短视频/图片解析代理：仅解析分享链接 → 无水印直链，媒体文件仍由客户端直连平台下载（不占服务器带宽）。
// 支持：抖音、快手、小红书（视频/图片）、以及视频/图片直链。
// 扣费在本接口内完成：解析成功扣 100 Token；解析失败自动退款（不扣客户 Token）。

var (
	noteIDRe  = regexp.MustCompile(`/(?:explore|discovery/item|video|short-video|note|item|aweme)/([A-Za-z0-9_-]{6,})`)
	ogVideoRe = regexp.MustCompile(`<meta[^>]+property=["']og:video["'][^>]+content=["']([^"']+)["']`)
	imageExtRe = regexp.MustCompile(`\.(jpg|jpeg|png|webp|gif|heic)(\?|$)`)
)

// ParseVideo 解析短视频/图片分享链接：POST /api/tools/parse-video
// 前置：Token > 2000；解析成功扣 100 Token；解析失败自动退款。
func ParseVideo(c *gin.Context) {
	tid := TenantID(c)
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "总后台账号无需使用获客工具"})
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if !jsonBody(c, &req) {
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" || !strings.HasPrefix(req.URL, "http") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "请输入有效的分享链接"})
		return
	}
	var t models.Tenant
	if database.DB.First(&t, tid).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "分站不存在"})
		return
	}
	if t.Points <= toolMinBalance {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "获客工具需 Token 余额高于 2000，请先充值"})
		return
	}
	if err := points.Deduct(tid, toolCostPoints, "获客工具·去水印解析"); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "Token 余额不足，请先充值"})
		return
	}
	result, err := parseMediaURL(req.URL)
	if err != nil || (result["video_url"] == "" && len(result["images"].([]string)) == 0) {
		_ = points.Recharge(tid, toolCostPoints, "获客工具解析失败退款")
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "平台接口调整中，本次未扣费，请稍后再试或联系管理员"})
		return
	}
	var fresh models.Tenant
	database.DB.Select("points").First(&fresh, tid)
	result["balance"] = fresh.Points
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// fetchMedia 请求链接并跟随重定向，返回最终 URL 与 HTML 正文。
func fetchMedia(url string) (string, string) {
	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1")
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	return resp.Request.URL.String(), string(body)
}

// parseMediaURL 分发到各平台解析器。
func parseMediaURL(shareURL string) (gin.H, error) {
	finalURL, html := fetchMedia(shareURL)
	if finalURL == "" {
		return nil, &userError{"无法访问该链接，请确认链接有效"}
	}
	out := gin.H{
		"raw_url": finalURL, "type": "video", "video_url": "", "images": []string{},
		"title": "", "cover": "",
	}
	// 图片直链
	if imageExtRe.MatchString(finalURL) {
		out["type"] = "images"
		out["images"] = []string{finalURL}
		return out, nil
	}
	// 视频直链
	if strings.Contains(finalURL, ".mp4") || strings.Contains(finalURL, ".m3u8") {
		out["video_url"] = finalURL
		return out, nil
	}
	switch {
	case strings.Contains(finalURL, "douyin"):
		parseDouyin(finalURL, out)
	case strings.Contains(finalURL, "kuaishou"):
		parseKuaishou(finalURL, html, out)
	case strings.Contains(finalURL, "xiaohongshu") || strings.Contains(finalURL, "xhslink"):
		parseXiaohongshu(finalURL, html, out)
	default:
		out["hint"] = "未能识别平台，请确认粘贴的是抖音/快手/小红书分享链接或媒体直链。"
	}
	return out, nil
}

// parseDouyin 抖音：iesdouyin iteminfo 拿无水印视频（playwm→play）。
func parseDouyin(finalURL string, out gin.H) {
	if m := noteIDRe.FindStringSubmatch(finalURL); len(m) > 1 {
		out["video_id"] = m[1]
		if vu, title, cover := douyinNoWatermark(m[1]); vu != "" {
			out["video_url"] = vu
			out["title"] = title
			out["cover"] = cover
		}
	}
}

// parseKuaishou 快手：从 HTML 提取 og:video 直链。
func parseKuaishou(finalURL, html string, out gin.H) {
	if m := ogVideoRe.FindStringSubmatch(html); len(m) > 1 {
		out["video_url"] = m[1]
		out["type"] = "video"
	}
	if m := noteIDRe.FindStringSubmatch(finalURL); len(m) > 1 {
		out["video_id"] = m[1]
	}
}

// parseXiaohongshu 小红书：从 __INITIAL_STATE__ 提取图片列表或视频直链。
func parseXiaohongshu(finalURL, html string, out gin.H) {
	raw := extractBalancedJSON(html, "window.__INITIAL_STATE__=")
	if raw == "" {
		return
	}
	var state struct {
		Note struct {
			NoteDetailMap map[string]struct {
				Note struct {
					Title     string `json:"title"`
					Desc      string `json:"desc"`
					ImageList []struct {
						URLDefault string `json:"url_default"`
						URL        string `json:"url"`
					} `json:"image_list"`
					Video struct {
						Media struct {
							Stream struct {
								H264 []struct {
									MasterURL string `json:"master_url"`
									URL       string `json:"url"`
								} `json:"h264"`
							} `json:"stream"`
						} `json:"media"`
					} `json:"video"`
				} `json:"note"`
			} `json:"noteDetailMap"`
		} `json:"note"`
	}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return
	}
	for _, nd := range state.Note.NoteDetailMap {
		note := nd.Note
		if note.Title != "" {
			out["title"] = note.Title
		} else if note.Desc != "" {
			out["title"] = note.Desc
		}
		// 视频笔记
		if vu := note.Video.Media.Stream.H264; len(vu) > 0 {
			if vu[0].MasterURL != "" {
				out["video_url"] = vu[0].MasterURL
				out["type"] = "video"
			} else if vu[0].URL != "" {
				out["video_url"] = vu[0].URL
				out["type"] = "video"
			}
			return
		}
		// 图文笔记（多图）
		if len(note.ImageList) > 0 {
			imgs := make([]string, 0, len(note.ImageList))
			for _, img := range note.ImageList {
				u := img.URLDefault
				if u == "" {
					u = img.URL
				}
				if u != "" {
					imgs = append(imgs, u)
				}
			}
			if len(imgs) > 0 {
				out["type"] = "images"
				out["images"] = imgs
				out["cover"] = imgs[0]
			}
		}
	}
}

// extractBalancedJSON 从 HTML 中 marker 之后提取括号平衡的 JSON 对象（用于嵌套 JSON，如 __INITIAL_STATE__）。
func extractBalancedJSON(html, marker string) string {
	idx := strings.Index(html, marker)
	if idx < 0 {
		return ""
	}
	start := strings.Index(html[idx:], "{")
	if start < 0 {
		return ""
	}
	start += idx
	depth := 0
	for i := start; i < len(html); i++ {
		switch html[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[start : i+1]
			}
		}
	}
	return ""
}

// douyinNoWatermark 调抖音公开 iteminfo 接口拿无水印地址（替换 playwm→play 去水印）。
func douyinNoWatermark(vid string) (video, title, cover string) {
	client := &http.Client{Timeout: 15 * time.Second}
	apiURL := "https://www.iesdouyin.com/web/api/v2/aweme/iteminfo/?item_ids=" + vid
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Referer", "https://www.douyin.com/")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	var data struct {
		ItemList []struct {
			Desc  string `json:"desc"`
			Video struct {
				PlayAddr struct {
					URLList []string `json:"url_list"`
				} `json:"play_addr"`
				Cover struct {
					URLList []string `json:"url_list"`
				} `json:"cover"`
			} `json:"video"`
		} `json:"item_list"`
	}
	if err := json.Unmarshal(body, &data); err != nil || len(data.ItemList) == 0 {
		return "", "", ""
	}
	urls := data.ItemList[0].Video.PlayAddr.URLList
	if len(urls) == 0 {
		return "", "", ""
	}
	vu := strings.Replace(urls[0], "playwm", "play", 1)
	cv := ""
	if len(data.ItemList[0].Video.Cover.URLList) > 0 {
		cv = data.ItemList[0].Video.Cover.URLList[0]
	}
	return vu, data.ItemList[0].Desc, cv
}

// userError 用户可读错误（HTTP 200 + code 1）
type userError struct{ msg string }

func (e *userError) Error() string { return e.msg }
