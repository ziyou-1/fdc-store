package presenter

import (
	"testing"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

func TestNewPublicConfigRespIncludesReferral(t *testing.T) {
	got := NewPublicConfigResp(&pointsdomain.Config{
		ReferralEnabled:          true,
		ReferralRegisterPoints:   20,
		ReferralFirstOrderPoints: 50,
		CheckinEnabled:           true,
		CheckinPoints:            5,
	})
	if !got.ReferralEnabled || got.ReferralRegisterPoints != 20 || got.ReferralFirstOrderPoints != 50 {
		t.Fatalf("public config = %+v", got)
	}
}

func TestNewInviteResp(t *testing.T) {
	got := NewInviteResp(&pointscontract.Invite{
		Enabled:          true,
		Code:             "QZ7MD9LD",
		RegisterPoints:   20,
		FirstOrderPoints: 50,
	})
	if !got.Enabled || got.Code != "QZ7MD9LD" || got.RegisterPoints != 20 || got.FirstOrderPoints != 50 {
		t.Fatalf("invite resp = %+v", got)
	}
}
