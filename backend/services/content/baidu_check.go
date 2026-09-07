package content

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"geo-tool/database"
	"geo-tool/models"
)

/* ================================================================
 * 内容投放 · 百度收录检测服务
 *
 * 尽力而为：请求百度公开搜索页（site:链接 或 直达链接）判断是否收录。
 *  - 明确出现"没有找到该URL / 很抱歉没有找到 / 未收录"等未收录标记 -> real, false
 *  - 页面出现"找到相关结果"等结果计数 -> real, true
 *  - 网络错误 / 非 200 / 页面正常但无明确标记（可能反爬）-> estimate（indexed=nil）
 * sourced_from=real/estimate 严格区分，严禁把估算冒充真实收录。
 * ================================================================ */

// CheckBaidu 检测单个链接是否被百度收录
// 返回 indexed（nil=无法验证）、method、note、err
func CheckBaidu(ctx context.Context, publishURL string) (indexed *bool, method, note string, err error) {
	publishURL = strings.TrimSpace(publishURL)
	if publishURL == "" {
		return nil, "estimate", "任务无发布链接，无法检测", nil
	}

	// 优先用 site: 精确检索
	query := "site:" + publishURL
	searchURL := "https://www.baidu.com/s?wd=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, "estimate", "构造检测请求失败: " + err.Error(), err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "estimate", "联网检测失败（网络错误），请手动在百度搜索 site:" + publishURL + " 确认", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "estimate", "读取百度响应失败，请手动在百度确认收录情况", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "estimate", "百度返回 HTTP "+itoa(resp.StatusCode)+"，请手动在百度确认收录情况", nil
	}

	html := string(data)
	// 未收录标记
	if strings.Contains(html, "没有找到该URL") ||
		strings.Contains(html, "很抱歉，没有找到") ||
		strings.Contains(html, "您要查找的页面不存在") ||
		strings.Contains(html, "未收录") {
		b := false
		return &b, "url_check", "百度未收录该链接（页面返回未找到标记）", nil
	}
	// 结果计数
	if strings.Contains(html, "找到相关结果") ||
		strings.Contains(html, "百度为您找到相关结果") ||
		strings.Contains(html, "找到相关结果数") {
		b := true
		return &b, "site_query", "百度已收录（site 检索命中）", nil
	}
	// 页面正常返回但无明确标记：可能被反爬或结果页结构变化，保守降级 estimate
	return nil, "estimate", "百度返回页面但未识别到明确收录标记（可能被反爬），请手动确认", nil
}

// CheckTask 对某发布任务执行一次收录检测，写入监控记录并更新任务质量字段
func CheckTask(ctx context.Context, tid, taskID uint) (*models.CtnMonitor, error) {
	var task models.CtnTask
	if err := database.DB.Where("id = ? AND tenant_id = ?", taskID, tid).First(&task).Error; err != nil {
		return nil, err
	}

	indexed, method, note, _ := CheckBaidu(ctx, task.PublishURL)

	source := models.SourceReal
	if method == "estimate" {
		source = models.SourceEstimate
	}

	mon := models.CtnMonitor{
		TenantID: tid, TaskID: task.ID, TaskTitle: task.ArticleTitle,
		MediaName: task.MediaName, PublishURL: task.PublishURL,
		BaiduIndexed: indexed, CheckMethod: method, SourcedFrom: source, Note: note,
	}
	if indexed != nil && *indexed && source == models.SourceReal {
		now := time.Now()
		mon.IndexedAt = &now
	}

	// 质量分：真实已收录 85+；真实未收录 30；无法验证 0 并说明
	q := 0
	qmsg := ""
	switch {
	case source == models.SourceReal && indexed != nil && *indexed:
		q = 85
		qmsg = "已确认被百度收录"
	case source == models.SourceReal && indexed != nil && !*indexed:
		q = 30
		qmsg = "百度未收录，建议优化标题/正文关键词或补充外链后重检"
	default:
		q = 0
		qmsg = "无法验证，请手动在百度搜索确认收录情况"
	}
	mon.QualityScore = q

	if err := database.DB.Create(&mon).Error; err != nil {
		return nil, err
	}

	// 更新任务：CheckCount / SyncState / QualityScore
	syncState := models.CtnSyncNotIndexed
	if source == models.SourceEstimate {
		syncState = models.CtnSyncUnknown
	} else if indexed != nil && *indexed {
		syncState = models.CtnSyncIndexed
	}
	database.DB.Model(&task).Updates(map[string]interface{}{
		"check_count":   task.CheckCount + 1,
		"sync_state":    syncState,
		"quality_score": q,
		"quality_msg":   qmsg,
	})

	return &mon, nil
}
