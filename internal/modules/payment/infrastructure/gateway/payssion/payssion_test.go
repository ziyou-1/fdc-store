package payssion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// 期望值由官方公式独立算出（Python hashlib），不是复算本包的实现——
// 否则测试只能证明代码自洽，证明不了它与 Payssion 一致。
const (
	testAPIKey    = "ak_test"
	testSecretKey = "sk_test"

	wantCreateSig      = "ceb68241bc6b65044bb5b4cc7932d45e"
	wantDetailsByOrder = "b146b04a2c79f6df8ffb0f6b52b647e8"
	wantDetailsByTxn   = "e4428ddb295adbfc02e0c93b112cd952"
	wantRefundSig      = "1aaced9531b0ba2694b4a57b94c097df"
	wantCallbackSDK    = "f82ee058daf829e635f06e26575eca4b"
	wantCallbackWHMCS  = "e5101bf275709ab886c6a88e09046f83"
)

func testConfig() *Config {
	return &Config{APIKey: testAPIKey, SecretKey: testSecretKey, LiveMode: false}
}

func TestSignMatchesOfficialCreateFormula(t *testing.T) {
	got := sign([]string{testAPIKey, "qris_id", "9.90", "USD", "DJ20260902X"}, testSecretKey)
	if got != wantCreateSig {
		t.Fatalf("create sig = %s, want %s", got, wantCreateSig)
	}
}

// 官方 getSig 对缺失字段用空串参与签名，只按 order_id 查询时
// transaction_id 的位置会留下一个空段。这条容易被"跳过空字段"的直觉写错。
func TestSignKeepsEmptySegmentForMissingTransactionID(t *testing.T) {
	got := sign([]string{testAPIKey, "", "DJ20260902X"}, testSecretKey)
	if got != wantDetailsByOrder {
		t.Fatalf("details-by-order sig = %s, want %s", got, wantDetailsByOrder)
	}
	withTxn := sign([]string{testAPIKey, "TX123", "DJ20260902X"}, testSecretKey)
	if withTxn != wantDetailsByTxn {
		t.Fatalf("details-by-txn sig = %s, want %s", withTxn, wantDetailsByTxn)
	}
	if got == withTxn {
		t.Fatal("empty and non-empty transaction_id must not produce the same signature")
	}
}

func TestSignMatchesOfficialRefundFormula(t *testing.T) {
	got := sign([]string{testAPIKey, "TX123", "9.90", "USD"}, testSecretKey)
	if got != wantRefundSig {
		t.Fatalf("refund sig = %s, want %s", got, wantRefundSig)
	}
}

// Payssion 的两份官方实现给出的回调公式不同，两式都必须被接受。
func TestVerifyCallbackAcceptsSDKVariant(t *testing.T) {
	result, err := VerifyCallback(testConfig(), map[string]string{
		"pm_id":          "qris_id",
		"amount":         "9.90",
		"currency":       "USD",
		"order_id":       "DJ20260902X",
		"state":          "completed",
		"transaction_id": "TX123",
		"notify_sig":     wantCallbackSDK,
	})
	if err != nil {
		t.Fatalf("sdk variant rejected: %v", err)
	}
	if result.SignatureVariant != "sdk" {
		t.Fatalf("variant = %s, want sdk", result.SignatureVariant)
	}
	if result.OrderID != "DJ20260902X" {
		t.Fatalf("order id = %s", result.OrderID)
	}
}

func TestVerifyCallbackAcceptsWHMCSVariant(t *testing.T) {
	result, err := VerifyCallback(testConfig(), map[string]string{
		"pm_id":          "qris_id",
		"amount":         "9.90",
		"currency":       "USD",
		"track_id":       "DJ20260902X",
		"sub_track_id":   "",
		"state":          "completed",
		"transaction_id": "TX123",
		"notify_sig":     wantCallbackWHMCS,
	})
	if err != nil {
		t.Fatalf("whmcs variant rejected: %v", err)
	}
	if result.SignatureVariant != "whmcs" {
		t.Fatalf("variant = %s, want whmcs", result.SignatureVariant)
	}
	// WHMCS 变体里商户单号在 track_id，必须能落到同一个字段上。
	if result.OrderID != "DJ20260902X" {
		t.Fatalf("order id = %s, want track_id value", result.OrderID)
	}
}

func TestVerifyCallbackRejectsTamperedAmount(t *testing.T) {
	_, err := VerifyCallback(testConfig(), map[string]string{
		"pm_id":      "qris_id",
		"amount":     "999.00", // 被篡改
		"currency":   "USD",
		"order_id":   "DJ20260902X",
		"state":      "completed",
		"notify_sig": wantCallbackSDK,
	})
	if err == nil {
		t.Fatal("tampered amount must be rejected")
	}
}

func TestVerifyCallbackRejectsMissingSignature(t *testing.T) {
	_, err := VerifyCallback(testConfig(), map[string]string{
		"pm_id":    "qris_id",
		"amount":   "9.90",
		"currency": "USD",
		"order_id": "DJ20260902X",
		"state":    "completed",
	})
	if err == nil {
		t.Fatal("missing notify_sig must be rejected")
	}
}

// 少付多付不能算成功：自动发货会直接造成资损。
func TestStateClassification(t *testing.T) {
	if !IsSuccessState(StateCompleted) {
		t.Fatal("completed must be success")
	}
	for _, state := range []string{StatePaidPartial, StatePaidMore, StatePending} {
		if IsSuccessState(state) {
			t.Fatalf("%s must not be treated as success", state)
		}
	}
	for _, state := range []string{StateFailed, StateError, StateCancelled} {
		if !IsFailedState(state) {
			t.Fatalf("%s must be failed", state)
		}
	}
	if !IsExpiredState(StateExpired) {
		t.Fatal("expired must be expired")
	}
}

// 发出去的 api_sig 必须与官方公式一致，且金额字段与签名用的是同一个字符串。
func TestCreateSendsOfficialSignature(t *testing.T) {
	var gotSig, gotAmount string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotSig = r.PostFormValue("api_sig")
		gotAmount = r.PostFormValue("amount")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result_code":    200,
			"redirect_url":   "https://payssion.example/pay/abc",
			"transaction_id": "TX123",
		})
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.BaseURL = server.URL

	result, err := Create(context.Background(), server.Client(), cfg, CreateInput{
		PMID: "qris_id", Amount: "9.90", Currency: "USD", OrderID: "DJ20260902X",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if gotSig != wantCreateSig {
		t.Fatalf("sent api_sig = %s, want %s", gotSig, wantCreateSig)
	}
	if gotAmount != "9.90" {
		t.Fatalf("sent amount = %s, want the exact string that was signed", gotAmount)
	}
	if result.RedirectURL == "" || result.TransactionID != "TX123" {
		t.Fatalf("unexpected result %+v", result)
	}
}

// result_code 非 200 必须当失败，不能因为 HTTP 200 就认为成功。
func TestCreateRejectsNonSuccessResultCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result_code": 400,
			"description": "invalid pm_id",
		})
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.BaseURL = server.URL
	if _, err := Create(context.Background(), server.Client(), cfg, CreateInput{
		PMID: "bad", Amount: "1", Currency: "USD", OrderID: "X",
	}); err == nil {
		t.Fatal("non-200 result_code must fail")
	}
}

func TestParseConfigDefaultsToLiveMode(t *testing.T) {
	cfg, err := ParseConfig(jsonmap.JSON{"api_key": "a", "secret_key": "b"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	// 漏配 live_mode 时必须打生产：静默走沙箱会让商户以为收到了钱。
	if !cfg.LiveMode {
		t.Fatal("live_mode must default to true")
	}
	if cfg.endpoint(pathCreate) != LiveBaseURL+pathCreate {
		t.Fatalf("endpoint = %s", cfg.endpoint(pathCreate))
	}
}

func TestResolvePMIDFallsBackToChannelType(t *testing.T) {
	cfg := &Config{}
	if got := cfg.ResolvePMID("ovo_id"); got != "ovo_id" {
		t.Fatalf("fallback pm_id = %s", got)
	}
	cfg.PMID = "qris_id"
	if got := cfg.ResolvePMID("ovo_id"); got != "qris_id" {
		t.Fatalf("configured pm_id must win, got %s", got)
	}
}
