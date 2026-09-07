package baidu

// 同行域名库：主域名 → 品牌/集团名
// 说明：手工维护 + 自动聚类结果。本库为内置默认，后续可扩展为从 DB(peer_domains) 加载。
var DefaultPeerLib = map[string]string{
	// 婚恋/婚介行业（演示与真实结合）
	"jiayuan.com":        "世纪佳缘",
	"baihe.com":          "百合网",
	"zhenai.com":         "珍爱网",
	"marryu.cn":          "MarryU",
	"wozhuliangyuan.com": "我主良缘",
	"qinglian.com":       "青藤之恋",
	"souhe.com":          "搜合网",
	"tiantianok.com":     "天天恋",
	// 泛婚恋 / 交友
	"tanwanyou.com": "探玩",
	"umsg.com":      "轻芒",
	"huabans.sohu.com": "搜狐婚恋",
}

// 已知非同行（内容/UGC/导航站），不计入同行统计，仅展示区分
var KnownNonPeer = map[string]bool{
	"zhihu.com": true, "xiaohongshu.com": true, "sohu.com": true, "163.com": true,
	"bilibili.com": true, "mafengwo.cn": true, "douban.com": true, "baike.baidu.com": true,
}

// 疑似同行启发式：命中这些行业词且未在已知非同行库中 → 进疑似队列待人工确认
var IndustryHints = []string{"婚恋", "婚介", "相亲", "红娘", "征婚", "脱单", "婚配", "交友", "找对象"}
