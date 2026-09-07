package baidu

import (
	"strings"
	"testing"
)

// 构造一段近似百度 SERP 的 HTML：2 条广告 + 8 条自然（含知名同行/疑似同行/非同行）
const mockHTML = `<html><head><title>biaoti</title></head><body>
<div id="wrapper">
<div id="content_left">
  <div class="result-op c-container" tpl="ad_baokuan" data-tools='{"is_ad":true,"url":"https://www.jiayuan.com/baike.html"}'>
    广告 世纪佳缘 免费找对象
  </div>
  <div class="result-op c-container" tpl="ad_zhanwei">
    广告 百合网婚恋交友平台
  </div>
  <div class="result c-container " id="1">
    <h3 class="c-title"><a href="http://www.baidu.com/link?url=abc123">世纪佳缘_知名婚恋交友平台</a></h3>
    <div class="c-abstract">注册会员超2亿 免费征婚交友服务</div>
    <span class="c-showurl"><cite>www.jiayuan.com</cite></span>
  </div>
  <div class="result c-container " id="2">
    <h3 class="c-title"><a href="http://www.baidu.com/link?url=def">百合网_实名制婚恋网站</a></h3>
    <span class="c-showurl"><cite>www.baihe.com</cite></span>
  </div>
  <div class="result c-container " id="3">
    <h3 class="c-title"><a href="http://www.zhenai.com/">珍爱网_婚恋交友</a></h3>
    <span class="c-showurl"><cite>www.zhenai.com</cite></span>
  </div>
  <div class="result c-container " id="4">
    <h3 class="c-title"><a href="https://www.zhihu.com/question/123">相亲需要注意什么？</a></h3>
    <span class="c-showurl"><cite>www.zhihu.com</cite></span>
  </div>
  <div class="result c-container " id="5">
    <h3 class="c-title"><a href="https://www.marryu.cn/">MarryU高端婚恋</a></h3>
    <span class="c-showurl"><cite>marryu.cn</cite></span>
  </div>
  <div class="result c-container " id="6">
    <h3 class="c-title"><a href="https://some-unknown-site.com/">红娘一对一匹配服务</a></h3>
    <span class="c-showurl"><cite>some-unknown-site.com</cite></span>
  </div>
  <div class="result c-container " id="7">
    <h3 class="c-title"><a href="https://www.xiaohongshu.com/explore">脱单攻略 小红书</a></h3>
    <span class="c-showurl"><cite>www.xiaohongshu.com</cite></span>
  </div>
</div>
</div>
</body></html>`

func TestParseSERP(t *testing.T) {
	pr := ParseSERP(mockHTML, "婚恋平台")
	if pr.ParseErr != "" && !strings.Contains(mockHTML, "") {
		t.Fatalf("unexpected parse err: %s", pr.ParseErr)
	}
	if len(pr.Items) != 9 {
		t.Fatalf("期望 9 个结果（2 广告 + 7 自然），实际 %d", len(pr.Items))
	}
	if pr.AdCount != 2 {
		t.Fatalf("期望 2 条广告，实际 %d", pr.AdCount)
	}
	if pr.OrganicCount != 7 {
		t.Fatalf("期望 7 条自然结果，实际 %d", pr.OrganicCount)
	}
	// 广告位类型识别
	if pr.Items[0].Type != "ad" || pr.Items[1].Type != "ad" {
		t.Errorf("前两条应为广告: %v %v", pr.Items[0].Type, pr.Items[1].Type)
	}
	// 域名抽取（去 www）
	if pr.Items[2].Domain != "jiayuan.com" {
		t.Errorf("条目2 域名应为 jiayuan.com，实际 %q", pr.Items[2].Domain)
	}
	if pr.Items[3].Domain != "baihe.com" {
		t.Errorf("条目3 域名应为 baihe.com，实际 %q", pr.Items[3].Domain)
	}
}

func TestAnalyzePeer(t *testing.T) {
	pr := ParseSERP(mockHTML, "婚恋平台")
	cfg := TaskConfig{PeerLib: DefaultPeerLib}
	analyzePage(&pr, cfg)

	// 广告不计入同行
	if pr.Items[0].Peer {
		t.Error("广告位不应计入同行")
	}
	// 已知同行识别
	if !pr.Items[2].Peer || pr.Items[2].Name != "世纪佳缘" {
		t.Errorf("jiayuan 应识别为同行 世纪佳缘：%+v", pr.Items[2])
	}
	// 非同行（知乎）
	if pr.Items[5].Peer {
		t.Error("zhihu 不应识别为同行")
	}
	// 疑似同行（未知域名 + 命中行业词）
	if !pr.Items[7].Suspected {
		t.Errorf("some-unknown-site.com 应标记疑似同行（命中'红娘'），实际 %+v", pr.Items[7])
	}
	// 记录统计
	if pr.OrganicPeers != 4 {
		t.Errorf("自然同行数应为 4（佳缘/百合/珍爱/MarryU），实际 %d", pr.OrganicPeers)
	}
}

func TestBuildResult(t *testing.T) {
	pr := ParseSERP(mockHTML, "婚恋平台")
	cfg := TaskConfig{PeerLib: DefaultPeerLib}
	analyzePage(&pr, cfg)
	pages := []PageInfo{{
		Page: 1, Ads: pr.AdCount, OrganicPeers: pr.OrganicPeers,
		OrganicOther: countOrganicOther(pr), Density: densityOf(pr), Items: pr.Items,
	}}
	res := BuildResult("婚恋平台", 1, pages, nil, AnalyzeOptions{PeerLib: DefaultPeerLib, MyDomains: nil})
	if res.Heat < 0 || res.Heat > 100 {
		t.Errorf("heat 越界: %d", res.Heat)
	}
	if len(res.Advices) == 0 {
		t.Error("应有优化建议生成")
	}
	if len(res.PeerRows) != 4 {
		t.Errorf("同行排行应有 4 行，实际 %d", len(res.PeerRows))
	}
	if res.PeerRows[0].Domain != "jiayuan.com" {
		t.Errorf("最好排名应为首名，实际 %+v", res.PeerRows[0])
	}
	t.Logf("Heat=%d, advices=%d, peers=%d", res.Heat, len(res.Advices), len(res.PeerRows))
}
