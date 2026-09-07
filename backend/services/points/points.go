package points

import (
	"errors"

	"gorm.io/gorm"

	"geo-tool/database"
	"geo-tool/models"
)

// ErrInsufficient 点卡余额不足（AI 调用前置校验失败时统一返回该错误）
var ErrInsufficient = errors.New("点卡余额不足，请联系总后台充值")

// DeductOne 原子扣减 1 点并写入消费流水。
// 通过条件更新（points >= 1 才减）在事务内完成“校验 + 扣减”，
// 条件更新天然具备行级互斥语义；
// 余额不足时不产生任何副作用并返回 ErrInsufficient。
// 注意：tenant_id=0（总后台全局平台运营方）不参与点卡计费，直接放行。
func DeductOne(tenantID uint, remark string) error {
	if tenantID == 0 {
		return nil
	}
	return database.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Tenant{}).
			Where("id = ? AND points >= ?", tenantID, 1).
			Update("points", gorm.Expr("points - 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInsufficient
		}
		var t models.Tenant
		if err := tx.Select("points").First(&t, tenantID).Error; err != nil {
			return err
		}
		return tx.Create(&models.PointRecord{
			TenantID:     tenantID,
			Amount:       -1,
			Type:         "consume",
			Remark:       remark,
			BalanceAfter: t.Points,
		}).Error
	})
}

// Deduct 原子扣减指定点数并写入消费流水（amount 必须为正数）。
// 条件更新（points >= amount 才减）保证余额不足时不产生副作用。
func Deduct(tenantID uint, amount int64, remark string) error {
	if tenantID == 0 {
		return nil
	}
	if amount <= 0 {
		return errors.New("扣减点数必须为正数")
	}
	return database.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Tenant{}).
			Where("id = ? AND points >= ?", tenantID, amount).
			Update("points", gorm.Expr("points - ?", amount))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInsufficient
		}
		var t models.Tenant
		if err := tx.Select("points").First(&t, tenantID).Error; err != nil {
			return err
		}
		return tx.Create(&models.PointRecord{
			TenantID:     tenantID,
			Amount:       -amount,
			Type:         "consume",
			Remark:       remark,
			BalanceAfter: t.Points,
		}).Error
	})
}

// Recharge 充值：增加余额并写入充值流水（amount 必须为正数）。
func Recharge(tenantID uint, amount int64, remark string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		return RechargeTx(tx, tenantID, amount, remark)
	})
}

// RechargeTx 在指定事务内执行充值入账（增加余额 + 写充值流水）。
// 供支付回调等服务在自身单事务内调用，避免“订单标记已支付”与“点数入账”两步分离
// 导致崩溃时点数永久缺失；也保证外部事务整体原子回滚。
func RechargeTx(tx *gorm.DB, tenantID uint, amount int64, remark string) error {
	if amount <= 0 {
		return errors.New("充值点数必须大于 0")
	}
	res := tx.Model(&models.Tenant{}).
		Where("id = ?", tenantID).
		Update("points", gorm.Expr("points + ?", amount))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("租户不存在")
	}
	var t models.Tenant
	if err := tx.Select("points").First(&t, tenantID).Error; err != nil {
		return err
	}
	return tx.Create(&models.PointRecord{
		TenantID:     tenantID,
		Amount:       amount,
		Type:         "recharge",
		Remark:       remark,
		BalanceAfter: t.Points,
	}).Error
}

// Balance 查询租户当前点卡余额
func Balance(tenantID uint) (int64, error) {
	var t models.Tenant
	if err := database.DB.Select("points").First(&t, tenantID).Error; err != nil {
		return 0, err
	}
	return t.Points, nil
}

// ListRecords 查询租户点卡流水（倒序，最多 200 条）
func ListRecords(tenantID uint, limit int) ([]models.PointRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var records []models.PointRecord
	err := database.DB.Where("tenant_id = ?", tenantID).
		Order("id desc").Limit(limit).Find(&records).Error
	return records, err
}
