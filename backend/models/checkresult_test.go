package models_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"geo-tool/models"
)

// TestCheckResultZeroSampleCountPersisted 钉住 v1.0.45 修复的真实缺陷：
//
// GORM 在 Create 时会把**零值字段**从 INSERT 语句中省略，让数据库填默认值。
// 曾经 SampleCount 上带 `gorm:"default:1"`，导致熔断跳过的记录（显式赋 0，表示
// 未调用、未扣费）被数据库默认值 1 覆盖，工作日志据此算出「消耗 329 点」，
// 而真实扣费只有 149 点 —— 展示口径虚增 202 点，等于对客户的对账单撒谎。
//
// 这个测试用真实 SQLite 落库，不是纯内存断言，确保 GORM 行为被真实覆盖。
func TestCheckResultZeroSampleCountPersisted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.CheckResult{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}

	// 场景 1：熔断跳过 —— 未调用、未扣费，SampleCount 必须落库为 0
	skipped := models.CheckResult{
		TenantID: 1, TaskID: 481, PlatformID: 2, PlatformName: "豆包（火山方舟）",
		Question: "婚恋系统哪家好", ErrorMsg: "平台已熔断（连续失败达阈值），本轮跳过",
		SampleCount: 0,
	}
	if err := db.Create(&skipped).Error; err != nil {
		t.Fatalf("写入熔断记录失败: %v", err)
	}

	var got models.CheckResult
	if err := db.First(&got, skipped.ID).Error; err != nil {
		t.Fatalf("读回熔断记录失败: %v", err)
	}
	if got.SampleCount != 0 {
		t.Errorf("熔断跳过记录 SampleCount 应为 0（未扣费），实际落库为 %d —— "+
			"说明 default 标签又把零值吞了，工作日志会虚增消耗", got.SampleCount)
	}
	if got.RefundedPoints != 0 {
		t.Errorf("熔断跳过记录 RefundedPoints 应为 0，实际为 %d", got.RefundedPoints)
	}

	// 场景 2：正常命中 —— SampleCount 必须如实落库（非零值不受影响）
	ok := models.CheckResult{
		TenantID: 1, TaskID: 481, PlatformID: 1, PlatformName: "DeepSeek",
		Question: "婚恋系统哪家好", SampleCount: 3, SampleHits: 2, RefundedPoints: 1,
	}
	if err := db.Create(&ok).Error; err != nil {
		t.Fatalf("写入正常记录失败: %v", err)
	}
	var got2 models.CheckResult
	if err := db.First(&got2, ok.ID).Error; err != nil {
		t.Fatalf("读回正常记录失败: %v", err)
	}
	if got2.SampleCount != 3 || got2.SampleHits != 2 || got2.RefundedPoints != 1 {
		t.Errorf("正常记录字段被篡改: SampleCount=%d SampleHits=%d RefundedPoints=%d（期望 3/2/1）",
			got2.SampleCount, got2.SampleHits, got2.RefundedPoints)
	}

	// 场景 3：净消耗口径 —— SUM(sample_count) - SUM(refunded_points) 必须等于真实支出
	// 期望：(0 + 3) - (0 + 1) = 2
	var net int
	db.Model(&models.CheckResult{}).
		Select("COALESCE(SUM(sample_count),0) - COALESCE(SUM(refunded_points),0)").
		Scan(&net)
	if net != 2 {
		t.Errorf("净消耗口径错位：期望 2，实际 %d（熔断记录被计入消耗会导致虚增）", net)
	}
}
