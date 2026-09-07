// Package payssion 实现 Payssion 聚合支付网关的协议细节。
//
// 协议来源全部取自官方仓库 payssion/payssion-php（lib/PayssionClient.php 与 samples/），
// 而不是文档页面——文档页需要登录商户后台才能看到正文。关键事实：
//   - 生产 https://www.payssion.com/api/v1/ ，沙箱 http://sandbox.payssion.com/api/v1/
//   - 三个端点均为 POST + application/x-www-form-urlencoded，响应为 JSON
//   - result_code == 200 表示成功
//   - 签名是把若干字段值按固定顺序用 | 连接后 md5，末位永远是 secret_key
package payssion

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// 端点。live 与 sandbox 的 scheme 不同，这是官方 SDK 的写法，不是笔误。
const (
	LiveBaseURL    = "https://www.payssion.com/api/v1/"
	SandboxBaseURL = "http://sandbox.payssion.com/api/v1/"

	pathCreate  = "payment/create"
	pathDetails = "payment/details"
	pathRefund  = "refunds"
)

// 网关返回的交易状态。
const (
	StateCompleted   = "completed"
	StatePaidPartial = "paid_partial"
	StatePaidMore    = "paid_more"
	StatePending     = "pending"
	StateFailed      = "failed"
	StateExpired     = "expired"
	StateError       = "error"
	StateCancelled   = "cancelled"
	StateRejected    = "rejected"
	StateBlocked     = "blocked"
)

var (
	ErrConfigInvalid    = errors.New("payssion config invalid")
	ErrRequestFailed    = errors.New("payssion request failed")
	ErrResponseInvalid  = errors.New("payssion response invalid")
	ErrSignatureInvalid = errors.New("payssion signature invalid")
)

// Config 是单个 Payssion 支付渠道的配置。
type Config struct {
	APIKey    string
	SecretKey string
	// PMID 是 Payssion 的支付方式标识（如 qris_id、ovo_id、alipay_cn）。
	// 留空时回落到渠道的 channel_type —— 运营把渠道类型直接填成 pm_id 即可，
	// 不必在同一张表单里把同一个值填两遍。
	PMID string
	// LiveMode 为 false 时走沙箱端点。
	LiveMode bool
	// BaseURL 覆盖端点，仅用于自建代理或联调，留空走上面两个官方地址。
	BaseURL string
}

// ParseConfig 从渠道配置 JSON 解析。
func ParseConfig(raw jsonmap.JSON) (*Config, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: empty config", ErrConfigInvalid)
	}
	cfg := &Config{
		APIKey:    strings.TrimSpace(stringValue(raw["api_key"])),
		SecretKey: strings.TrimSpace(stringValue(raw["secret_key"])),
		PMID:      strings.TrimSpace(stringValue(raw["pm_id"])),
		BaseURL:   strings.TrimSpace(stringValue(raw["api_url"])),
		// 默认生产模式：漏配 live_mode 时宁可打生产（会因密钥不符而失败），
		// 也不要静默打沙箱造成"以为收到钱了"。
		LiveMode: boolValue(raw["live_mode"], true),
	}
	return cfg, nil
}

// ValidateConfig 校验必填项。
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: nil config", ErrConfigInvalid)
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("%w: api_key required", ErrConfigInvalid)
	}
	if cfg.SecretKey == "" {
		return fmt.Errorf("%w: secret_key required", ErrConfigInvalid)
	}
	return nil
}

// endpoint 返回请求地址。
func (c *Config) endpoint(path string) string {
	base := c.BaseURL
	if base == "" {
		if c.LiveMode {
			base = LiveBaseURL
		} else {
			base = SandboxBaseURL
		}
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + path
}

// ResolvePMID 返回本次请求使用的 pm_id：配置优先，留空时回落到渠道类型。
func (c *Config) ResolvePMID(channelType string) string {
	if c.PMID != "" {
		return c.PMID
	}
	return strings.TrimSpace(channelType)
}

// sign 按 Payssion 的规则计算签名：字段值按给定顺序用 | 连接，末位追加 secret_key，再取 md5。
// 缺失字段按空串参与，这与官方 getSig 里 isset 判断的行为一致——
// 例如只用 order_id 查询时，transaction_id 位置会留下一个空段。
func sign(values []string, secretKey string) string {
	parts := append(append([]string{}, values...), secretKey)
	sum := md5.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// CreateInput 是创建支付的入参。
type CreateInput struct {
	PMID        string
	Amount      string
	Currency    string
	OrderID     string
	Description string
	ReturnURL   string
}

// CreateResult 是创建支付的结果。
type CreateResult struct {
	TransactionID string
	RedirectURL   string
	Raw           jsonmap.JSON
}

// Create 调用 payment/create。
func Create(ctx context.Context, client *http.Client, cfg *Config, input CreateInput) (*CreateResult, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if input.PMID == "" {
		return nil, fmt.Errorf("%w: pm_id required", ErrConfigInvalid)
	}
	form := url.Values{}
	form.Set("api_key", cfg.APIKey)
	form.Set("pm_id", input.PMID)
	form.Set("amount", input.Amount)
	form.Set("currency", input.Currency)
	form.Set("order_id", input.OrderID)
	if input.Description != "" {
		form.Set("description", input.Description)
	}
	if input.ReturnURL != "" {
		form.Set("return_url", input.ReturnURL)
	}
	// 签名字段顺序固定为 api_key|pm_id|amount|currency|order_id|secret_key。
	// 参与签名的必须是"实际发出去的那个字符串"，因此这里复用 form 里的值。
	form.Set("api_sig", sign([]string{
		cfg.APIKey, input.PMID, input.Amount, input.Currency, input.OrderID,
	}, cfg.SecretKey))

	raw, err := post(ctx, client, cfg.endpoint(pathCreate), form)
	if err != nil {
		return nil, err
	}
	if err := ensureSuccess(raw); err != nil {
		return nil, err
	}
	redirect := strings.TrimSpace(stringValue(raw["redirect_url"]))
	if redirect == "" {
		return nil, fmt.Errorf("%w: missing redirect_url", ErrResponseInvalid)
	}
	return &CreateResult{
		TransactionID: strings.TrimSpace(stringValue(raw["transaction_id"])),
		RedirectURL:   redirect,
		Raw:           raw,
	}, nil
}

// DetailsResult 是查询交易的结果。
type DetailsResult struct {
	TransactionID string
	OrderID       string
	State         string
	Amount        string
	Currency      string
	Paid          string
	Raw           jsonmap.JSON
}

// Details 调用 payment/details。transactionID 可为空，此时按 orderID 查询——
// 官方示例正是只传 order_id，签名里 transaction_id 的位置留空段。
func Details(ctx context.Context, client *http.Client, cfg *Config, transactionID, orderID string) (*DetailsResult, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if transactionID == "" && orderID == "" {
		return nil, fmt.Errorf("%w: transaction_id or order_id required", ErrConfigInvalid)
	}
	form := url.Values{}
	form.Set("api_key", cfg.APIKey)
	if transactionID != "" {
		form.Set("transaction_id", transactionID)
	}
	if orderID != "" {
		form.Set("order_id", orderID)
	}
	form.Set("api_sig", sign([]string{cfg.APIKey, transactionID, orderID}, cfg.SecretKey))

	raw, err := post(ctx, client, cfg.endpoint(pathDetails), form)
	if err != nil {
		return nil, err
	}
	if err := ensureSuccess(raw); err != nil {
		return nil, err
	}
	transaction, _ := raw["transaction"].(map[string]interface{})
	if transaction == nil {
		return nil, fmt.Errorf("%w: missing transaction", ErrResponseInvalid)
	}
	return &DetailsResult{
		TransactionID: strings.TrimSpace(stringValue(transaction["transaction_id"])),
		OrderID:       strings.TrimSpace(stringValue(transaction["order_id"])),
		State:         strings.TrimSpace(stringValue(transaction["state"])),
		Amount:        strings.TrimSpace(stringValue(transaction["amount"])),
		Currency:      strings.TrimSpace(stringValue(transaction["currency"])),
		Paid:          strings.TrimSpace(stringValue(transaction["paid"])),
		Raw:           raw,
	}, nil
}

// Refund 调用 refunds。
func Refund(ctx context.Context, client *http.Client, cfg *Config, transactionID, amount, currency string) (jsonmap.JSON, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if transactionID == "" {
		return nil, fmt.Errorf("%w: transaction_id required", ErrConfigInvalid)
	}
	form := url.Values{}
	form.Set("api_key", cfg.APIKey)
	form.Set("transaction_id", transactionID)
	form.Set("amount", amount)
	form.Set("currency", currency)
	form.Set("api_sig", sign([]string{cfg.APIKey, transactionID, amount, currency}, cfg.SecretKey))

	raw, err := post(ctx, client, cfg.endpoint(pathRefund), form)
	if err != nil {
		return nil, err
	}
	if err := ensureSuccess(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// CallbackResult 是验签通过后的回调事实。
type CallbackResult struct {
	PMID          string
	Amount        string
	Paid          string
	Currency      string
	OrderID       string
	State         string
	TransactionID string
	Fee           string
	// 结算货币入账模式（Credited In Settlement Currency）下，
	// Payssion 会把 amount/currency 换成入账币种的金额，
	// 创建交易时用的原始金额与币种改放在 amount_local/currency_local。
	CurrencySettle string
	AmountLocal    string
	CurrencyLocal  string
	// SignatureVariant 记录命中的是哪一套验签公式，写进 payload 便于运营核对账户约定。
	SignatureVariant string
	Raw              map[string]string
}

// VerifyCallback 校验异步通知签名。
//
// Payssion 的两份官方实现给出的公式并不一致，且在 sub_track_id 为空时也不等价：
//   - payssion-php samples/sample_postback.php:
//     md5(api_key|pm_id|amount|currency|order_id|state|secret_key)
//   - plugin_whmcs modules/gateways/callback/payssion.php:
//     md5(api_key|pm_id|amount|currency|track_id|sub_track_id|state|secret_key)
//
// 两者都以 secret_key 为密钥，伪造难度相同，因此两式都试、命中即通过，
// 并把命中的变体记录下来。只实现其中一式会让另一类账户的每一个合法回调都被拒。
func VerifyCallback(cfg *Config, params map[string]string) (*CallbackResult, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	got := strings.TrimSpace(params["notify_sig"])
	if got == "" {
		return nil, fmt.Errorf("%w: missing notify_sig", ErrSignatureInvalid)
	}

	pmID := params["pm_id"]
	amount := params["amount"]
	currency := params["currency"]
	state := params["state"]
	orderID := params["order_id"]
	trackID := params["track_id"]
	subTrackID := params["sub_track_id"]

	variants := []struct {
		name   string
		values []string
	}{
		{"sdk", []string{cfg.APIKey, pmID, amount, currency, orderID, state}},
		{"whmcs", []string{cfg.APIKey, pmID, amount, currency, trackID, subTrackID, state}},
	}
	matched := ""
	for _, variant := range variants {
		if equalFold(sign(variant.values, cfg.SecretKey), got) {
			matched = variant.name
			break
		}
	}
	if matched == "" {
		return nil, ErrSignatureInvalid
	}

	// 商户单号：SDK 变体用 order_id，WHMCS 变体用 track_id，两者语义相同。
	businessNo := strings.TrimSpace(orderID)
	if businessNo == "" {
		businessNo = strings.TrimSpace(trackID)
	}
	return &CallbackResult{
		PMID:             pmID,
		Amount:           amount,
		Paid:             params["paid"],
		Currency:         currency,
		OrderID:          businessNo,
		State:            state,
		TransactionID:    strings.TrimSpace(params["transaction_id"]),
		Fee:              params["fee"],
		CurrencySettle:   strings.TrimSpace(params["currency_settle"]),
		AmountLocal:      strings.TrimSpace(params["amount_local"]),
		CurrencyLocal:    strings.TrimSpace(params["currency_local"]),
		SignatureVariant: matched,
		Raw:              params,
	}, nil
}

// IsSuccessState 判断状态是否代表足额到账。
// paid_partial 与 paid_more 刻意不算成功：东南亚的便利店与 ATM 渠道尾差常见，
// 少付多付都必须转人工，不能自动发货。
func IsSuccessState(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), StateCompleted)
}

// IsFailedState 判断状态是否为终态失败。
// rejected（被拒绝）与 blocked（风控阻断）同样是终态，官方状态表里明确列出；
// 漏掉它们会让这两类交易永远停在 pending，订单既不发货也不释放。
func IsFailedState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case StateFailed, StateError, StateCancelled, StateRejected, StateBlocked:
		return true
	default:
		return false
	}
}

// IsExpiredState 判断状态是否为过期。
func IsExpiredState(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), StateExpired)
}

func post(ctx context.Context, client *http.Client, endpoint string, form url.Values) (jsonmap.JSON, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: http %d", ErrRequestFailed, response.StatusCode)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseInvalid, err)
	}
	return jsonmap.JSON(decoded), nil
}

// ensureSuccess 按官方约定判断 result_code == 200。
func ensureSuccess(raw jsonmap.JSON) error {
	code := strings.TrimSpace(stringValue(raw["result_code"]))
	if code == "200" {
		return nil
	}
	description := strings.TrimSpace(stringValue(raw["description"]))
	if description == "" {
		description = strings.TrimSpace(stringValue(raw["message"]))
	}
	return fmt.Errorf("%w: result_code=%s %s", ErrRequestFailed, code, description)
}

// StringValue 把 JSON 值统一转成字符串，供适配器合并 JSON 通知体时复用。
func StringValue(raw interface{}) string {
	return stringValue(raw)
}

func stringValue(raw interface{}) string {
	switch value := raw.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		// JSON 数字统一按去掉多余零的十进制输出，避免 200 变成 2e+02。
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.10f", value), "0"), ".")
	case bool:
		if value {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", value)
	}
}

func boolValue(raw interface{}, fallback bool) bool {
	switch value := raw.(type) {
	case nil:
		return fallback
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	case float64:
		return value != 0
	}
	return fallback
}

func equalFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
