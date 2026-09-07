package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// baiduIndustry 行业定义（代码 / 中文名 / 英文名）
type baiduIndustry struct {
	Code string
	Name string
	En   string
}

// 百度指数行业排行：11 大行业
var baiduIndustries = []baiduIndustry{
	{"automobile", "汽车", "automobile"},
	{"mobile", "手机", "mobile phone"},
	{"computer", "电脑办公", "COMPUTER"},
	{"appliance", "家用电器", "APPLIANCE"},
	{"cosmetic", "化妆品", "cosmetic"},
	{"tourist", "旅游景点", "Tourist Attraction"},
	{"furniture", "家具家居", "FURNITURE"},
	{"decoration", "家装平台", "DECORATION PLATFORM"},
	{"estate", "房产", "Real Estate"},
	{"milk", "婴幼儿奶粉", "Infant Milk Formula"},
	{"university", "高校", "University"},
}

// 品牌指数 TOP5 种子数据（参考百度指数行业排行日榜，指数值已换算为完整数字）
var baiduSeedData = map[string][]struct {
	Brand string
	Value float64
}{
	"automobile": {{"捷达", 16616000}, {"特斯拉", 3660000}, {"比亚迪", 3004000}, {"大众", 2413000}, {"丰田", 2115000}},
	"mobile":     {{"苹果", 7560000}, {"华为", 4881000}, {"小米", 2224000}, {"荣耀", 1385000}, {"vivo", 1118000}},
	"computer":   {{"华为", 1782000}, {"苹果", 1563000}, {"小米", 1373000}, {"联想", 297000}, {"外星人", 279000}},
	"appliance":  {{"华为", 1636000}, {"小米", 1435000}, {"戴森", 364000}, {"海尔", 356000}, {"格力", 320000}},
	"cosmetic":   {{"香奈儿", 356000}, {"迪奥", 166000}, {"圣罗兰", 111000}, {"伊夫.圣罗兰", 77325}, {"水光", 62979}},
	"tourist":    {{"埃及博物馆", 663000}, {"故宫", 644000}, {"北京欢乐谷", 505000}, {"成都欢乐谷", 496000}, {"钱塘江", 427000}},
	"furniture":  {{"索菲亚家居", 22192}, {"索菲亚全屋定制", 21692}, {"宜家", 21216}, {"慕思", 17107}, {"天然", 6658}},
	"decoration": {{"酷家乐", 17368}, {"建e室内设计网", 2387}, {"百安居", 1502}, {"土巴兔", 523}, {"今朝装饰", 377}},
	"estate":     {{"恒大集团", 942000}, {"保利集团", 98948}, {"中海地产", 80900}, {"佳兆业", 70539}, {"万科集团", 67951}},
	"milk":       {{"金领冠-珍护", 87703}, {"雀巢", 47351}, {"太子乐", 37715}, {"金领冠", 36722}, {"雀巢-超启能恩", 33479}},
	"university": {{"武汉大学", 2870000}, {"北京大学", 1494000}, {"北京体育大学", 478000}, {"哈尔滨工业大学", 300000}, {"清华大学", 297000}},
}

// SeedBaiduIndustryRank 初始化行业排行种子数据（表为空时写入品牌指数日榜）
func SeedBaiduIndustryRank() {
	var cnt int64
	database.DB.Model(&models.BaiduIndustryRank{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	today := time.Now().Format("2006-01-02")
	for _, ind := range baiduIndustries {
		rows, ok := baiduSeedData[ind.Code]
		if !ok {
			continue
		}
		for i, r := range rows {
			database.DB.Create(&models.BaiduIndustryRank{
				IndustryCode: ind.Code, IndustryName: ind.Name, IndustryEn: ind.En,
				Metric: "brand", Period: "day", Rank: i + 1,
				BrandName: r.Brand, IndexValue: r.Value, StatDate: today,
			})
		}
	}
	log.Println("[baidu-rank] 行业排行种子数据已初始化")
}

// IndustryRank 行业排行接口：GET /api/baidu/industry-rank?period=day&metric=brand
func IndustryRank(c *gin.Context) {
	period := c.DefaultQuery("period", "day")
	metric := c.DefaultQuery("metric", "brand")
	if period != "week" {
		period = "day"
	}
	if metric == "" {
		metric = "brand"
	}
	var rows []models.BaiduIndustryRank
	database.DB.Where("period = ? AND metric = ?", period, metric).Order("industry_code asc, rank asc").Find(&rows)

	// 按行业分组
	grouped := make([]gin.H, 0, len(baiduIndustries))
	for _, ind := range baiduIndustries {
		items := make([]gin.H, 0, 5)
		statDate := ""
		for _, r := range rows {
			if r.IndustryCode != ind.Code {
				continue
			}
			if statDate == "" {
				statDate = r.StatDate
			}
			items = append(items, gin.H{
				"rank": r.Rank, "brand": r.BrandName, "value": r.IndexValue,
			})
		}
		grouped = append(grouped, gin.H{
			"code": ind.Code, "name": ind.Name, "en": ind.En,
			"items": items, "stat_date": statDate,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"period": period, "metric": metric,
		"industries": grouped,
		"metrics":    []gin.H{{"key": "brand", "label": "品牌指数"}, {"key": "search", "label": "品牌搜索指数"}, {"key": "news", "label": "品牌资讯指数"}, {"key": "interact", "label": "品牌互动指数"}},
		"updated_at": time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"),
	}})
}

// RefreshIndustryRank 更新行业排行（定时/手动触发）。
// 百度指数行业排行存在反爬（登录 Cookie + 参数逆向），纯后端无法稳定抓取；
// 此处保留更新框架：抓取失败则沿用现有数据，确保展示不受影响。
func RefreshIndustryRank() error {
	log.Println("[baidu-rank] 尝试更新行业排行数据…")
	// TODO: 接入可用的数据源（百度指数商业 API / 代理抓取）。当前保留种子数据。
	return nil
}

// IndustryRankRefresh 提交最新行业排行数据（供自动化抓取任务调用）：POST /api/baidu/industry-rank/refresh
// 请求体：{ period, metric, industries: [{ code, items: [{rank, brand, value}] }] }
func IndustryRankRefresh(c *gin.Context) {
	var req struct {
		Period     string `json:"period"`
		Metric     string `json:"metric"`
		Industries []struct {
			Code  string `json:"code"`
			Items []struct {
				Rank  int     `json:"rank"`
				Brand string  `json:"brand"`
				Value float64 `json:"value"`
			} `json:"items"`
		} `json:"industries"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if req.Period == "" {
		req.Period = "day"
	}
	if req.Metric == "" {
		req.Metric = "brand"
	}
	today := time.Now().Format("2006-01-02")
	// 名称映射（code -> 中文/英文）
	nameMap := map[string]baiduIndustry{}
	for _, ind := range baiduIndustries {
		nameMap[ind.Code] = ind
	}
	inserted := 0
	for _, ind := range req.Industries {
		meta, ok := nameMap[ind.Code]
		if !ok {
			continue
		}
		// 删除该行业该 period+metric 的旧数据，写入新数据
		database.DB.Where("industry_code = ? AND period = ? AND metric = ?", ind.Code, req.Period, req.Metric).
			Delete(&models.BaiduIndustryRank{})
		for _, it := range ind.Items {
			if it.Brand == "" || it.Value <= 0 {
				continue
			}
			database.DB.Create(&models.BaiduIndustryRank{
				IndustryCode: ind.Code, IndustryName: meta.Name, IndustryEn: meta.En,
				Metric: req.Metric, Period: req.Period, Rank: it.Rank,
				BrandName: it.Brand, IndexValue: it.Value, StatDate: today,
			})
			inserted++
		}
	}
	log.Printf("[baidu-rank] 刷新完成：%d 条记录，日期 %s", inserted, today)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新", "data": gin.H{"inserted": inserted, "stat_date": today}})
}
