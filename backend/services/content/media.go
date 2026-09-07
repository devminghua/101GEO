package content

import (
	"geo-tool/database"
	"geo-tool/models"
)

/* ================================================================
 * 内容投放 · 媒体库服务
 *
 * 全国主流媒体库 = 内置预设（22 家，首次进入时按租户写入）
 *  + 用户自定义（custom）。支持按分类/权重/关键字筛选。
 * ================================================================ */

// MediaPreset 媒体预设
type MediaPreset struct {
	Name     string // 媒体名称
	Category string // 分类
	Level    string // 权重
	URL      string // 发稿入口
	Remark   string // 说明
}

// BuiltinMedia 内置主流媒体预设（新闻门户 / 商业门户 / 自媒体 / 行业垂直）
var BuiltinMedia = []MediaPreset{
	// 新闻门户
	{Name: "新浪新闻", Category: "新闻门户", Level: "权威", URL: "https://news.sina.com.cn", Remark: "综合门户，权重高"},
	{Name: "网易新闻", Category: "新闻门户", Level: "权威", URL: "https://news.163.com", Remark: "综合门户，权重高"},
	{Name: "腾讯新闻", Category: "新闻门户", Level: "权威", URL: "https://news.qq.com", Remark: "综合门户，权重高"},
	{Name: "搜狐新闻", Category: "新闻门户", Level: "权威", URL: "https://news.sohu.com", Remark: "综合门户，权重高"},
	{Name: "凤凰网", Category: "新闻门户", Level: "权威", URL: "https://www.ifeng.com", Remark: "新闻门户，权重高"},
	// 商业门户
	{Name: "百度百家号", Category: "商业门户", Level: "权威", URL: "https://baijiahao.baidu.com", Remark: "百度系，利于百度收录"},
	{Name: "今日头条", Category: "商业门户", Level: "高", URL: "https://www.toutiao.com", Remark: "算法分发，曝光量大"},
	{Name: "知乎", Category: "商业门户", Level: "权威", URL: "https://www.zhihu.com", Remark: "高权重问答社区"},
	{Name: "CSDN", Category: "商业门户", Level: "高", URL: "https://www.csdn.net", Remark: "IT 技术社区"},
	{Name: "太平洋电脑网", Category: "商业门户", Level: "高", URL: "https://www.pconline.com.cn", Remark: "科技数码垂直门户"},
	{Name: "中关村在线", Category: "商业门户", Level: "高", URL: "https://www.zol.com.cn", Remark: "科技数码垂直门户"},
	// 自媒体
	{Name: "微信公众号", Category: "自媒体", Level: "权威", URL: "https://mp.weixin.qq.com", Remark: "私域粉丝沉淀"},
	{Name: "小红书", Category: "自媒体", Level: "高", URL: "https://www.xiaohongshu.com", Remark: "种草平台"},
	{Name: "抖音", Category: "自媒体", Level: "高", URL: "https://www.douyin.com", Remark: "短视频平台"},
	{Name: "微博", Category: "自媒体", Level: "高", URL: "https://weibo.com", Remark: "社交平台"},
	{Name: "哔哩哔哩", Category: "自媒体", Level: "高", URL: "https://www.bilibili.com", Remark: "视频社区"},
	{Name: "百家号号外", Category: "自媒体", Level: "中", URL: "https://baijiahao.baidu.com", Remark: "自媒体分发"},
	// 行业垂直
	{Name: "汽车之家", Category: "行业垂直", Level: "权威", URL: "https://www.autohome.com.cn", Remark: "汽车行业垂直媒体"},
	{Name: "什么值得买", Category: "行业垂直", Level: "高", URL: "https://www.smzdm.com", Remark: "消费决策平台"},
	{Name: "36氪", Category: "行业垂直", Level: "权威", URL: "https://36kr.com", Remark: "创投科技媒体"},
	{Name: "虎嗅网", Category: "行业垂直", Level: "权威", URL: "https://www.huxiu.com", Remark: "商业科技媒体"},
	{Name: "大众点评", Category: "行业垂直", Level: "高", URL: "https://www.dianping.com", Remark: "本地生活点评"},
}

// EnsureBuiltin 租户媒体库为空时写入内置预设
func EnsureBuiltin(tid uint) {
	var cnt int64
	database.DB.Model(&models.CtnMedia{}).Where("tenant_id = ?", tid).Count(&cnt)
	if cnt > 0 {
		return
	}
	for _, p := range BuiltinMedia {
		database.DB.Create(&models.CtnMedia{
			TenantID: tid, Name: p.Name, Category: p.Category, Level: p.Level,
			URL: p.URL, SourceType: "builtin", Status: 1, Remark: p.Remark,
		})
	}
}
