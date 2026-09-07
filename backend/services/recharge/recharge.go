// Package recharge 点卡自助扫码充值订单服务。
// 负责：创建订单（调支付 Provider 下单）、支付回调幂等入账、轮询查询、关闭超时未支付订单。
// 幂等核心：入账依赖 RechargeOrder.status 条件更新（WHERE status=0 -> 1），
// 条件更新天然原子抢占，重复回调不会重复入账。
package recharge

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/pay"
	"geo-tool/services/points"
)

const (
	// 订单状态
	StatusPending  = 0 // 待支付
	StatusPaid     = 1 // 已支付（已入账）
	StatusClosed   = 2 // 已关闭
	StatusFailed   = 3 // 失败

	// 订单过期时间：创建后 30 分钟未支付自动关闭
	orderTTL = 30 * time.Minute
)

// ErrOrderNotFound 订单不存在
var ErrOrderNotFound = errors.New("订单不存在")

// OrderView 下单成功返回给前端的视图
type OrderView struct {
	OrderNo    string    `json:"order_no"`
	Channel    string    `json:"channel"`
	CodeURL    string    `json:"code_url"`
	AmountFen  int64     `json:"amount_fen"`
	Points     int64     `json:"points"`
	ExpireTime time.Time `json:"expire_time"`
}

// CreateOrder 创建充值订单（按全局单价换算金额）。
// 若第三方下单失败则回滚本地订单，不残留脏数据。
func CreateOrder(ctx context.Context, tenantID uint, channel string, pointsNum int64) (*OrderView, error) {
	if pointsNum < 1 || pointsNum > 999999 {
		return nil, errors.New("充值点数必须在 1-999999 之间")
	}
	cfg, err := pay.LoadSecretConfig()
	if err != nil {
		return nil, err
	}
	amountFen := pointsNum * cfg.PointPrice()
	desc := "点卡充值 " + itoa(pointsNum) + " 点"
	return createOrderWithPrice(ctx, tenantID, channel, pointsNum, amountFen, desc, 0)
}

// CreateOrderByPlan 按套餐下单：用套餐的 token 数与售价（套餐优惠价）创建订单。
func CreateOrderByPlan(ctx context.Context, tenantID uint, channel string, planID uint) (*OrderView, error) {
	var plan models.RechargePlan
	if err := database.DB.Where("id = ? AND enabled = ?", planID, true).First(&plan).Error; err != nil {
		return nil, errors.New("套餐不存在或已下架")
	}
	desc := "套餐充值「" + plan.Name + "」"
	return createOrderWithPrice(ctx, tenantID, channel, plan.Points, plan.PriceFen, desc, plan.DailyQueryLimit)
}

// createOrderWithPrice 按指定点数与金额下单：校验渠道 -> 调支付 Provider 下单 -> 落库。
// dailyLimit 为该套餐解锁的每日查询上限（0=不改变客户当前上限），随订单落库，支付成功后生效。
func createOrderWithPrice(ctx context.Context, tenantID uint, channel string, pointsNum, amountFen int64, desc string, dailyLimit int) (*OrderView, error) {
	provider, err := pay.New(channel)
	if err != nil {
		return nil, err
	}
	orderNo := pay.GenOrderNo()

	codeURL, err := provider.CreateQR(ctx, orderNo, amountFen, desc)
	if err != nil {
		return nil, err
	}

	order := models.RechargeOrder{
		OrderNo:         orderNo,
		TenantID:        tenantID,
		Channel:         channel,
		AmountFen:       amountFen,
		Points:          pointsNum,
		DailyQueryLimit: dailyLimit,
		Status:          StatusPending,
		CodeURL:         codeURL,
		ExpireTime:      time.Now().Add(orderTTL),
	}
	if err := database.DB.Create(&order).Error; err != nil {
		return nil, err
	}
	return &OrderView{
		OrderNo:    order.OrderNo,
		Channel:    order.Channel,
		CodeURL:    order.CodeURL,
		AmountFen:  order.AmountFen,
		Points:     order.Points,
		ExpireTime: order.ExpireTime,
	}, nil
}

// HandlePaid 处理支付成功回调：幂等入账。
// 单事务内先 WHERE status=0 条件更新抢锁，成功后调 points.RechargeTx 入账；
// 任一失败整体回滚；重复回调因 RowsAffected==0 直接安全返回，不重复入账。
// 同时校验回调实付金额与订单金额一致，防止篡改。
func HandlePaid(orderNo, tradeNo string, paidFen int64) (bool, error) {
	handled := false
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var order models.RechargeOrder
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return ErrOrderNotFound
		}
		// 幂等：仅当订单处于待支付时才允许入账
		res := tx.Model(&models.RechargeOrder{}).
			Where("order_no = ? AND status = ?", orderNo, StatusPending).
			Updates(map[string]interface{}{
				"status":   StatusPaid,
				"trade_no": tradeNo,
				"pay_time": time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 已被处理（重复回调/已关闭/已失败），不重复入账
			return nil
		}
		// 金额校验：实付与订单金额必须一致
		if paidFen != order.AmountFen {
			// 金额不符视为异常，回滚状态变更，不入账
			return errors.New("回调金额与订单金额不一致")
		}
		// 入账：点数到租户余额 + 写充值流水
		if err := points.RechargeTx(tx, order.TenantID, order.Points,
			"扫码充值 order="+order.OrderNo); err != nil {
			return err
		}
		// 套餐解锁每日查询上限：支付成功后，若订单携带配额则提升客户每日上限
		if order.DailyQueryLimit > 0 {
			if err := tx.Model(&models.Tenant{}).Where("id = ?", order.TenantID).
				Update("daily_query_limit", order.DailyQueryLimit).Error; err != nil {
				return err
			}
		}
		handled = true
		return nil
	})
	return handled, err
}

// GetByOrderNo 查询订单状态（供前端轮询）。
func GetByOrderNo(orderNo string) (*models.RechargeOrder, error) {
	var order models.RechargeOrder
	if err := database.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, ErrOrderNotFound
	}
	return &order, nil
}

// CloseExpired 关闭所有超时未支付订单，返回关闭数量。
func CloseExpired() (int64, error) {
	res := database.DB.Model(&models.RechargeOrder{}).
		Where("status = ? AND expire_time < ?", StatusPending, time.Now()).
		Update("status", StatusClosed)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
