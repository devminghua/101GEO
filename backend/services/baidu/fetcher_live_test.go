package baidu

import (
	"os"
	"testing"
)

// TestLiveFetch 真实抓取百度 SERP 冒烟：仅当 GEO_LIVE=1 时执行
// 用法: GEO_LIVE=1 go test ./services/baidu/ -run TestLiveFetch -v
func TestLiveFetch(t *testing.T) {
	if os.Getenv("GEO_LIVE") != "1" {
		t.Skip("GEO_LIVE=1 时执行真实抓取")
	}
	html, err := FetchPage("深圳婚恋平台", 1)
	if err != nil {
		t.Skipf("抓取失败（可能反爬）: %v", err)
	}
	pr := ParseSERP(html, "深圳婚恋平台")
	cfg := TaskConfig{PeerLib: DefaultPeerLib}
	analyzePage(&pr, cfg)
	t.Logf("抓取成功 HTML=%d bytes, 结果=%d(广告%d/自然%d), 同行=%d",
		len(html), len(pr.Items), pr.AdCount, pr.OrganicCount, pr.OrganicPeers)
	for _, it := range pr.Items {
		t.Logf("  [%d] %s %s peer=%v suspected=%v title=%s", it.Rank, it.Type, it.Domain, it.Peer, it.Suspected, it.Title)
	}
}
