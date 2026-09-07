// 支付宝当面付（扫码支付）实现（SDK: smartwalle/alipay v3）。
// 下单使用 alipay.trade.precreate，回调按支付宝异步通知规范验签并解析。
package pay

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/smartwalle/alipay/v3"
)

// alipayProvider 支付宝当面付 Provider
type alipayProvider struct {
	client       *alipay.Client
	notifyURL    string
	appID        string
}

// newAlipayProvider 初始化支付宝 Provider。
// 依赖全局配置：app_id / private_key / public_key / notify_base_url。
func newAlipayProvider(cfg *SecretConfig) (PayProvider, error) {
	if cfg.AlipayPayAppID == "" || cfg.AlipayPayPrivateKey == "" ||
		cfg.AlipayPayPublicKey == "" || cfg.AlipayNotifyBaseURL == "" {
		return nil, errors.New("支付宝配置不完整，请先在总后台「支付设置」补齐商户参数")
	}
	client, err := alipay.New(cfg.AlipayPayAppID, cfg.AlipayPayPrivateKey, true)
	if err != nil {
		return nil, fmt.Errorf("支付宝客户端初始化失败: %w", err)
	}
	if err := client.LoadAliPayPublicKey(cfg.AlipayPayPublicKey); err != nil {
		return nil, fmt.Errorf("加载支付宝公钥失败: %w", err)
	}
	return &alipayProvider{
		client:    client,
		notifyURL: cfg.AlipayNotifyURL(),
		appID:     cfg.AlipayPayAppID,
	}, nil
}

// CreateQR 支付宝当面付预下单，返回二维码码串 qr_code。
func (a *alipayProvider) CreateQR(ctx context.Context, orderNo string, amountFen int64, desc string) (string, error) {
	resp, err := a.client.TradePreCreate(ctx, alipay.TradePreCreate{
		Trade: alipay.Trade{
			NotifyURL:    a.notifyURL,
			Subject:      desc,
			OutTradeNo:   orderNo,
			TotalAmount:  fenToYuan(amountFen),
			ProductCode:  "FACE_TO_FACE_PAYMENT",
			TimeoutExpress: "30m",
		},
	})
	if err != nil {
		return "", fmt.Errorf("支付宝下单失败: %w", err)
	}
	if resp.IsFailure() {
		return "", fmt.Errorf("支付宝下单失败: %s", resp.SubMsg)
	}
	if resp.QRCode == "" {
		return "", errors.New("支付宝下单返回二维码为空")
	}
	return resp.QRCode, nil
}

// NotifyParse 解析支付宝异步通知：验签 -> 提取订单号/金额/交易号。
// 验签失败或交易未成功时返回 VerifyOK=false（不触发入账）。
func (a *alipayProvider) NotifyParse(ctx context.Context, body []byte, headers map[string]string) (NotifyResult, error) {
	var result NotifyResult
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return result, err
	}
	noti, err := a.client.DecodeNotification(ctx, values)
	if err != nil {
		// 验签失败：不确认订单
		return NotifyResult{VerifyOK: false}, nil
	}
	// 当面付成功状态：TRADE_SUCCESS
	if noti.TradeStatus != "TRADE_SUCCESS" {
		return NotifyResult{VerifyOK: false}, nil
	}
	amountFen, _ := yuanToFen(noti.TotalAmount)
	return NotifyResult{
		VerifyOK:  true,
		OrderNo:   noti.OutTradeNo,
		AmountFen: amountFen,
		TradeNo:   noti.TradeNo,
	}, nil
}

// fenToYuan 分 -> 元（两位小数），如 100 -> "1.00"
func fenToYuan(fen int64) string {
	neg := false
	if fen < 0 {
		neg = true
		fen = -fen
	}
	s := strconv.FormatInt(fen, 10)
	for len(s) < 3 {
		s = "0" + s
	}
	intPart, decPart := s[:len(s)-2], s[len(s)-2:]
	if neg {
		return "-" + intPart + "." + decPart
	}
	return intPart + "." + decPart
}

// yuanToFen 元（字符串，两位小数）-> 分
func yuanToFen(yuan string) (int64, error) {
	y := strings.TrimSpace(yuan)
	if y == "" {
		return 0, errors.New("金额为空")
	}
	f, err := strconv.ParseFloat(y, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f * 100)), nil
}
