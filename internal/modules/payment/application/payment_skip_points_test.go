package application

import "testing"

func TestSkipPaymentDoesNotApplyWhenUsingPoints(t *testing.T) {
	if skipPaymentApplies(true, true, false) {
		t.Fatal("skip_payment must not swallow a points redeem")
	}
	if !skipPaymentApplies(true, false, false) {
		t.Fatal("skip_payment still applies to cash checkout")
	}
	if skipPaymentApplies(false, true, false) {
		t.Fatal("points redeem is not skip_payment")
	}
}

func TestSkipPaymentDoesNotApplyWhenUsingFreeClaim(t *testing.T) {
	if skipPaymentApplies(true, false, true) {
		t.Fatal("skip_payment must not swallow a free claim")
	}
	if skipPaymentApplies(false, false, true) {
		t.Fatal("free claim is not skip_payment")
	}
}
