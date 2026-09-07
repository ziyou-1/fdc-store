package payssionadapter

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestCreatePaymentConvertsCurrencyBeforeSigning(t *testing.T) {
	var gotAmount, gotCurrency, gotPMID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotAmount = r.PostFormValue("amount")
		gotCurrency = r.PostFormValue("currency")
		gotPMID = r.PostFormValue("pm_id")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result_code":    200,
			"redirect_url":   "https://www.payssion.com/pay/TX1",
			"transaction_id": "TX1",
		})
	}))
	defer server.Close()

	result, err := NewPayssionAdapter().CreatePayment(context.Background(), jsonmap.JSON{
		"api_key":         "k",
		"secret_key":      "s",
		"api_url":         server.URL,
		"live_mode":       false,
		"target_currency": "IDR",
		"exchange_rate":   "16000",
	}, paymentcontract.GatewayCreateInput{
		OrderNo:     "DJ20260905X",
		Amount:      money.FromDecimal(decimal.RequireFromString("1.00")),
		Currency:    "USD",
		ChannelType: "qris_id",
		Subject:     "QRIS",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if gotPMID != "qris_id" || gotAmount != "16000" || gotCurrency != "IDR" {
		t.Fatalf("sent pm_id=%s amount=%s currency=%s", gotPMID, gotAmount, gotCurrency)
	}
	if result.AmountSent != "16000" || result.CurrencySent != "IDR" {
		t.Fatalf("sent snapshot %+v", result)
	}
	if result.Payload["original_amount"] != "1" || result.Payload["original_currency"] != "USD" {
		t.Fatalf("conversion audit %+v", result.Payload)
	}
}

func TestCreatePaymentRejectsInvalidExchangeRate(t *testing.T) {
	_, err := NewPayssionAdapter().CreatePayment(context.Background(), jsonmap.JSON{
		"api_key":         "k",
		"secret_key":      "s",
		"target_currency": "IDR",
		"exchange_rate":   "0",
	}, paymentcontract.GatewayCreateInput{
		OrderNo:     "DJ20260905X",
		Amount:      money.FromDecimal(decimal.RequireFromString("1.00")),
		Currency:    "USD",
		ChannelType: "ovo_id",
	})
	if err == nil {
		t.Fatal("zero exchange_rate must fail")
	}
}

// 结算货币入账模式：Payssion 把 amount/currency 换成入账币种，
// 建单时发出去的原值放在 amount_local/currency_local。
// 对账必须回到原值，否则合法回调会被判币种不符而全部拒收。
func TestVerifyCallbackReconcilesAgainstLocalAmountWhenSettled(t *testing.T) {
	const apiKey, secret = "k", "s"
	raw := jsonmap.JSON{"api_key": apiKey, "secret_key": secret, "live_mode": false}

	sig := md5Hex(strings.Join([]string{apiKey, "qris_id", "1.71", "USD", "DJ20260905X", "completed", secret}, "|"))
	form := map[string][]string{
		"pm_id":           {"qris_id"},
		"amount":          {"1.71"},
		"currency":        {"USD"},
		"order_id":        {"DJ20260905X"},
		"state":           {"completed"},
		"transaction_id":  {"TX9"},
		"currency_settle": {"USD"},
		"amount_local":    {"26329.74"},
		"currency_local":  {"IDR"},
		"notify_sig":      {sig},
	}

	verifier := NewPayssionAdapter().(paymentcontract.GatewayCallbackVerifier)
	result, err := verifier.VerifyCallback(raw, form, nil)
	if err != nil {
		t.Fatalf("VerifyCallback: %v", err)
	}
	if result.Currency != "IDR" || result.Amount.Decimal.String() != "26329.74" {
		t.Fatalf("reconcile against local value, got %s %s", result.Amount.Decimal, result.Currency)
	}
	// 入账事实不能丢，运营对账需要看到结算币种与净额。
	if result.Payload["currency_settle"] != "USD" || result.Payload["amount"] != "1.71" {
		t.Fatalf("settlement facts lost: %+v", result.Payload)
	}
}

// 标准通知没有 currency_settle，必须原样用 amount/currency。
func TestVerifyCallbackKeepsAmountWhenNotSettled(t *testing.T) {
	const apiKey, secret = "k", "s"
	sig := md5Hex(strings.Join([]string{apiKey, "ovo_id", "26329.74", "IDR", "DJ20260905Y", "completed", secret}, "|"))
	verifier := NewPayssionAdapter().(paymentcontract.GatewayCallbackVerifier)
	result, err := verifier.VerifyCallback(
		jsonmap.JSON{"api_key": apiKey, "secret_key": secret, "live_mode": false},
		map[string][]string{
			"pm_id": {"ovo_id"}, "amount": {"26329.74"}, "currency": {"IDR"},
			"order_id": {"DJ20260905Y"}, "state": {"completed"}, "notify_sig": {sig},
		}, nil)
	if err != nil {
		t.Fatalf("VerifyCallback: %v", err)
	}
	if result.Currency != "IDR" || result.Amount.Decimal.String() != "26329.74" {
		t.Fatalf("got %s %s", result.Amount.Decimal, result.Currency)
	}
}

// rejected / blocked 是官方状态表里的终态失败，必须落到 failed，
// 停在 pending 会让订单既不发货也不释放。
func TestMapStateTreatsRejectedAndBlockedAsFailed(t *testing.T) {
	for _, state := range []string{"rejected", "blocked", "failed", "error", "cancelled"} {
		if got := mapState(state); got != constants.PaymentStatusFailed {
			t.Fatalf("state %s mapped to %s", state, got)
		}
	}
	for _, state := range []string{"pending", "awaiting_confirm", "paid_partial", "paid_more"} {
		if got := mapState(state); got != constants.PaymentStatusPending {
			t.Fatalf("state %s mapped to %s", state, got)
		}
	}
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
