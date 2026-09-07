// 微信 Native 扫码支付实现（微信支付 API v3，SDK: wechatpay-go）。
// 下单使用 Native 直连模式，回调使用 RSA 通知处理器验签后解密报文。
package pay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// wechatProvider 微信 Native 支付 Provider
type wechatProvider struct {
	client       *core.Client
	notifyURL    string
	appid        string
	mchid        string
	notifyHandle *notify.Handler
}

// newWechatProvider 初始化微信支付 Provider。
// 依赖全局配置：mch_id / app_id / serial_no / api_v3_key / private_key / notify_base_url。
func newWechatProvider(cfg *SecretConfig) (PayProvider, error) {
	if cfg.WechatPayMchID == "" || cfg.WechatPayAppID == "" ||
		cfg.WechatPaySerialNo == "" || cfg.WechatPayAPIv3Key == "" ||
		cfg.WechatPayPrivateKey == "" || cfg.WechatNotifyBaseURL == "" {
		return nil, errors.New("微信支付配置不完整，请先在总后台「支付设置」补齐商户参数")
	}
	mchPrivateKey, err := utils.LoadPrivateKey(cfg.WechatPayPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("微信商户私钥解析失败: %w", err)
	}
	ctx := context.Background()
	opts := []core.ClientOption{
		option.WithWechatPayAutoAuthCipher(cfg.WechatPayMchID, cfg.WechatPaySerialNo, mchPrivateKey, cfg.WechatPayAPIv3Key),
	}
	client, err := core.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("微信支付客户端初始化失败: %w", err)
	}
	// 验签访问器：自动下载并缓存微信支付平台证书
	visitor := downloader.MgrInstance().GetCertificateVisitor(cfg.WechatPayMchID)
	handle, err := notify.NewRSANotifyHandler(cfg.WechatPayAPIv3Key, verifiers.NewSHA256WithRSAVerifier(visitor))
	if err != nil {
		return nil, fmt.Errorf("微信回调处理器初始化失败: %w", err)
	}
	return &wechatProvider{
		client:       client,
		notifyURL:    cfg.WechatNotifyURL(),
		appid:        cfg.WechatPayAppID,
		mchid:        cfg.WechatPayMchID,
		notifyHandle: handle,
	}, nil
}

// CreateQR 微信 Native 下单，返回二维码链接 code_url。
func (w *wechatProvider) CreateQR(ctx context.Context, orderNo string, amountFen int64, desc string) (string, error) {
	svc := native.NativeApiService{Client: w.client}
	resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(w.appid),
		Mchid:       core.String(w.mchid),
		Description: core.String(desc),
		OutTradeNo:  core.String(orderNo),
		NotifyUrl:   core.String(w.notifyURL),
		Amount: &native.Amount{
			Total: core.Int64(amountFen),
		},
	})
	if err != nil {
		return "", fmt.Errorf("微信下单失败: %w", err)
	}
	if resp == nil || resp.CodeUrl == nil || *resp.CodeUrl == "" {
		return "", errors.New("微信下单返回为空")
	}
	return *resp.CodeUrl, nil
}

// NotifyParse 解析微信支付回调：验签 -> 解密 -> 提取订单号/金额/交易号。
// 注意：本方法仅解析报文，不修改业务状态（幂等入账由 services/recharge.HandlePaid 负责）。
func (w *wechatProvider) NotifyParse(ctx context.Context, body []byte, headers map[string]string) (NotifyResult, error) {
	var result NotifyResult
	req, err := http.NewRequest(http.MethodPost, w.notifyURL, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	// 还原回调请求头（验签依赖 Wechatpay-* 头与平台证书）
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	txn := &payments.Transaction{}
	if _, err := w.notifyHandle.ParseNotifyRequest(ctx, req, txn); err != nil {
		// 验签失败：不确认订单，返回失败结果（调用方会记日志但不会入账）
		return NotifyResult{VerifyOK: false}, nil
	}
	orderNo := ""
	if txn.OutTradeNo != nil {
		orderNo = *txn.OutTradeNo
	}
	var amountFen int64
	if txn.Amount != nil && txn.Amount.Total != nil {
		amountFen = *txn.Amount.Total
	}
	tradeNo := ""
	if txn.TransactionId != nil {
		tradeNo = *txn.TransactionId
	}
	result = NotifyResult{
		VerifyOK:  true,
		OrderNo:   orderNo,
		AmountFen: amountFen,
		TradeNo:   tradeNo,
	}
	// 仅在订单成功时才视为有效支付（SUCCESS）
	if txn.TradeState == nil || *txn.TradeState != "SUCCESS" {
		return NotifyResult{VerifyOK: false}, nil
	}
	return result, nil
}
