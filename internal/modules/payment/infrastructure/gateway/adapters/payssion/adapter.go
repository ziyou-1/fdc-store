package payssionadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/payssion"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// payssionAdapter 是 Payssion 聚合支付的网关适配器。
//
// Payssion 覆盖约 200 种本地支付方式，用 pm_id 区分。这里刻意不为每个 pm_id
// 建一个 channel_type：运营给每种支付方式建一个渠道，把 channel_type 直接写成
// pm_id（如 qris_id、ovo_id），费率/最低额/最高额用渠道自带的四个字段配置。
// 这样费率表可以整表录入，不需要为每种方式写代码。
type payssionAdapter struct{}

// NewPayssionAdapter 实例化 Payssion 适配器。
func NewPayssionAdapter() paymentcontract.GatewayProvider { return &payssionAdapter{} }

var (
	_ paymentcontract.GatewayProvider         = (*payssionAdapter)(nil)
	_ paymentcontract.GatewayCallbackVerifier = (*payssionAdapter)(nil)
	_ paymentcontract.GatewayCapturer         = (*payssionAdapter)(nil)
)

// Type 返回 provider 标识。channel_type 留空表示本适配器接管该 provider 下的所有渠道类型。
func (a *payssionAdapter) Type() string {
	return constants.PaymentProviderPayssion + ":"
}

func (a *payssionAdapter) parseConfig(raw jsonmap.JSON) (*payssion.Config, error) {
	cfg, err := payssion.ParseConfig(raw)
	if err != nil {
		return nil, mapPayssionError(err)
	}
	if err := payssion.ValidateConfig(cfg); err != nil {
		return nil, mapPayssionError(err)
	}
	return cfg, nil
}

// ValidateConfig 校验渠道配置。pm_id 允许留空，此时回落到 channel_type，
// 但两者不能同时为空——否则请求发出去 Payssion 不知道要用哪种支付方式。
func (a *payssionAdapter) ValidateConfig(raw jsonmap.JSON, channelType string) error {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return err
	}
	if cfg.PMID == "" && strings.TrimSpace(channelType) == "" {
		return fmt.Errorf("%w: payssion pm_id required", paymentcontract.ErrGatewayConfigInvalid)
	}
	return nil
}

// CreatePayment 创建支付并返回收银台跳转地址。
func (a *payssionAdapter) CreatePayment(
	ctx context.Context,
	raw jsonmap.JSON,
	input paymentcontract.GatewayCreateInput,
) (*paymentcontract.GatewayCreateResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}
	pmID := cfg.ResolvePMID(input.ChannelType)
	if pmID == "" {
		return nil, fmt.Errorf("%w: payssion pm_id required", paymentcontract.ErrGatewayConfigInvalid)
	}

	// 金额必须先定成字符串再拿去签名：签名算的是字符串本身，
	// 发送值与签名值只要有一位不同（比如 1 与 1.00），网关就会判签名错误。
	originalAmount := input.Amount.Decimal.Round(2).String()
	originalCurrency := strings.ToUpper(strings.TrimSpace(input.Currency))
	amount := originalAmount
	currency := originalCurrency
	exchange := gatewaycommon.ExchangeRateConfig{
		TargetCurrency: strings.TrimSpace(payssion.StringValue(raw["target_currency"])),
		ExchangeRate:   strings.TrimSpace(payssion.StringValue(raw["exchange_rate"])),
	}
	exchange.NormalizeExchangeRate()
	converted := false
	if exchange.NeedsCurrencyConversion() {
		convertedAmount, convertedCurrency, convErr := exchange.ConvertAmount(amount, currency, 2)
		if convErr != nil {
			return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, convErr)
		}
		amount = convertedAmount
		currency = convertedCurrency
		converted = true
	}

	result, err := payssion.Create(ctx, nil, cfg, payssion.CreateInput{
		PMID:        pmID,
		Amount:      amount,
		Currency:    currency,
		OrderID:     input.OrderNo,
		Description: input.Subject,
		ReturnURL:   gatewaycommon.AppendQueryParams(input.ReturnURL, input.ReturnURLQuery),
	})
	if err != nil {
		return nil, mapPayssionError(err)
	}

	payload := jsonmap.JSON{
		"pm_id":     pmID,
		"live_mode": cfg.LiveMode,
		paymentcontract.GatewayPayloadFiatCurrencySent: currency,
	}
	if converted {
		payload["exchange_rate"] = exchange.ExchangeRate
		payload["original_amount"] = originalAmount
		payload["original_currency"] = originalCurrency
	}
	for key, value := range result.Raw {
		payload["response_"+key] = value
	}
	return &paymentcontract.GatewayCreateResult{
		ProviderRef:  result.TransactionID,
		RedirectURL:  result.RedirectURL,
		Payload:      payload,
		AmountSent:   amount,
		CurrencySent: currency,
	}, nil
}

// QueryPayment 主动查询交易状态，用于回调丢失时的兜底对账。
// providerRef 是 Payssion 的 transaction_id；为空时无法查询，由调用方决定是否改用订单号。
func (a *payssionAdapter) QueryPayment(
	ctx context.Context,
	raw jsonmap.JSON,
	providerRef string,
) (*paymentcontract.GatewayQueryResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}
	details, err := payssion.Details(ctx, nil, cfg, strings.TrimSpace(providerRef), "")
	if err != nil {
		return nil, mapPayssionError(err)
	}
	return &paymentcontract.GatewayQueryResult{
		ProviderRef: details.TransactionID,
		Status:      mapState(details.State),
		Amount:      parseAmount(details.Amount),
		Currency:    strings.ToUpper(details.Currency),
		Payload:     details.Raw,
	}, nil
}

// VerifyCallback 校验异步通知。Payssion 既可能发 form POST 也可能发 JSON body，
// 两种都要接住——只解析 form 会让 JSON 通知全部验签失败。
func (a *payssionAdapter) VerifyCallback(
	raw jsonmap.JSON,
	form map[string][]string,
	body []byte,
) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	params := flattenForm(form)
	if len(body) > 0 {
		mergeJSONBody(params, body)
	}

	verified, err := payssion.VerifyCallback(cfg, params)
	if err != nil {
		return nil, mapPayssionError(err)
	}

	payload := jsonmap.JSON{}
	for key, value := range verified.Raw {
		payload[key] = value
	}
	// 记录命中的验签变体：Payssion 两份官方实现的公式不同，
	// 运营需要知道自己账户走的是哪一套，出问题时才有排查起点。
	payload["_signature_variant"] = verified.SignatureVariant

	// 结算货币入账模式下，回调里的 amount/currency 是「换算后入账」的金额，
	// 创建交易时发出去的原值在 amount_local/currency_local。对账必须拿原值比，
	// 否则我们建单记的是 IDR、回调回来的是 USD，每一笔合法支付都会被判币种不符。
	amount, currency := verified.Amount, verified.Currency
	if verified.CurrencySettle != "" && verified.CurrencyLocal != "" && verified.AmountLocal != "" {
		amount, currency = verified.AmountLocal, verified.CurrencyLocal
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     verified.OrderID,
		ProviderRef: verified.TransactionID,
		Status:      mapState(verified.State),
		Amount:      parseAmount(amount),
		Currency:    strings.ToUpper(currency),
		Payload:     payload,
	}, nil
}

// mapState 把 Payssion 状态映射为内部支付状态。
// paid_partial 与 paid_more 映射为 pending 而不是 success：
// 少付多付必须转人工，自动发货会直接造成资损。
func mapState(state string) string {
	switch {
	case payssion.IsSuccessState(state):
		return constants.PaymentStatusSuccess
	case payssion.IsFailedState(state):
		return constants.PaymentStatusFailed
	case payssion.IsExpiredState(state):
		return constants.PaymentStatusExpired
	default:
		return constants.PaymentStatusPending
	}
}

func parseAmount(raw string) money.Amount {
	value := strings.TrimSpace(raw)
	if value == "" {
		return money.Amount{}
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return money.Amount{}
	}
	return money.FromDecimal(parsed)
}

func flattenForm(form map[string][]string) map[string]string {
	params := make(map[string]string, len(form))
	for key, values := range form {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	return params
}

// mergeJSONBody 把 JSON 通知体合并进参数表。已存在的 form 字段不覆盖，
// 避免同一请求里两种来源打架时行为不确定。
func mergeJSONBody(params map[string]string, body []byte) {
	decoded := map[string]interface{}{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return
	}
	for key, value := range decoded {
		if _, exists := params[key]; exists {
			continue
		}
		params[key] = payssion.StringValue(value)
	}
}

func mapPayssionError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, payssion.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, payssion.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	case errors.Is(err, payssion.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, payssion.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	default:
		return err
	}
}
