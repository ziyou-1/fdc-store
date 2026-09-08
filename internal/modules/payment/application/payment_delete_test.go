package application

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/constants"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func seedPaymentWithStatus(t *testing.T, svc *PaymentService, status string) *paymentdomain.Payment {
	t.Helper()
	p := &paymentdomain.Payment{
		OrderID:  1,
		Amount:   money.FromDecimal(decimal.RequireFromString("9.90")),
		Currency: "USD",
		Status:   status,
	}
	if err := svc.paymentRepo.Create(p); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	return p
}

// 成功的支付是买家真付过钱的凭证，删掉订单的已付状态就失去依据，
// 也没法再跟网关对账，任何角色都不许删。
func TestDeletePaymentRejectsSuccessfulPayment(t *testing.T) {
	svc, _ := setupPaymentServiceWalletTest(t)
	p := seedPaymentWithStatus(t, svc, constants.PaymentStatusSuccess)

	err := svc.DeletePayment(p.ID)
	if !errors.Is(err, ErrPaymentDeleteNotAllowed) {
		t.Fatalf("成功支付必须拒绝删除，实得 %v", err)
	}
	// 拒绝之后记录必须还在
	got, err := svc.GetPayment(p.ID)
	if err != nil || got == nil {
		t.Fatalf("被拒绝后记录不该消失: %v", err)
	}
}

// 弃单留下的噪音（initiated/pending/failed/expired）允许清理。
func TestDeletePaymentAllowsNonSuccessStates(t *testing.T) {
	for _, status := range []string{
		constants.PaymentStatusInitiated,
		constants.PaymentStatusPending,
		constants.PaymentStatusFailed,
		constants.PaymentStatusExpired,
	} {
		svc, _ := setupPaymentServiceWalletTest(t)
		p := seedPaymentWithStatus(t, svc, status)

		if err := svc.DeletePayment(p.ID); err != nil {
			t.Fatalf("状态 %s 应允许删除，实得 %v", status, err)
		}
		// 软删除后列表与详情都查不到
		if _, err := svc.GetPayment(p.ID); !errors.Is(err, ErrPaymentNotFound) {
			t.Fatalf("状态 %s 删除后应查不到，实得 %v", status, err)
		}
	}
}

func TestDeletePaymentRejectsUnknownID(t *testing.T) {
	svc, _ := setupPaymentServiceWalletTest(t)
	if err := svc.DeletePayment(0); !errors.Is(err, ErrPaymentInvalid) {
		t.Fatalf("id=0 应判非法，实得 %v", err)
	}
	if err := svc.DeletePayment(99999); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("不存在的 id 应报 not found，实得 %v", err)
	}
}
