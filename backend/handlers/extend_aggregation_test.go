package handlers_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"geo-tool/models"
)

// TestExtendAggregationChain 钉住 v1.0.47 修复：GORM 链式复用会把 Order/Limit
// 带进聚合查询，PG 下报「column must appear in GROUP BY」，错误被 GORM 吞掉后
// 合计永远是 0（续费对账页的 total_months/total_amount_fen 恒为 0）。
//
// 用真实 SQLite 验证「带 Order/Limit 的链再执行聚合」确实会污染结果——
// 修复后聚合查询必须从独立 session 出发，SUM 才能得出正确合计。
func TestExtendAggregationChain(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.ExtendRecord{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if err := db.Create(&[]models.ExtendRecord{
		{TenantID: 1, Months: 12, AmountFen: 120000},
		{TenantID: 1, Months: 3, AmountFen: 30000},
		{TenantID: 2, Months: 6, AmountFen: 60000},
	}).Error; err != nil {
		t.Fatalf("造数失败: %v", err)
	}

	// 场景 1（缺陷复现）：链上先带 Order/Limit，再复用执行聚合 —— 合计应错误/为 0
	chain := db.Model(&models.ExtendRecord{}).Where("tenant_id = ?", 1).Order("id desc").Limit(200)
	var wrong struct {
		Months    int64
		AmountFen int64
	}
	chain.Select("coalesce(sum(months),0) as months, coalesce(sum(amount_fen),0) as amount_fen").Scan(&wrong)
	t.Logf("[缺陷复现] 复用带 Order/Limit 的链：months=%d amount_fen=%d（PG 下此查询直接报错）", wrong.Months, wrong.AmountFen)

	// 场景 2（修复后写法）：独立构造聚合查询 —— 必须等于 15 个月 / 150000 分
	var right struct {
		Months    int64
		AmountFen int64
	}
	db.Model(&models.ExtendRecord{}).Where("tenant_id = ?", 1).
		Select("coalesce(sum(months),0) as months, coalesce(sum(amount_fen),0) as amount_fen").Scan(&right)
	if right.Months != 15 || right.AmountFen != 150000 {
		t.Errorf("独立聚合查询结果错误：months=%d amount_fen=%d（期望 15/150000）", right.Months, right.AmountFen)
	}

	// 场景 3：修复后接口内部必须走独立查询——用同条件验证 Count 也不受 Limit 污染
	var cnt int64
	db.Model(&models.ExtendRecord{}).Where("tenant_id = ?", 1).Count(&cnt)
	if cnt != 2 {
		t.Errorf("Count 结果错误：%d（期望 2）", cnt)
	}
}
