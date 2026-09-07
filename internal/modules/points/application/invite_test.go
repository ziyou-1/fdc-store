package application

import (
	"errors"
	"testing"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

type stubInviteIssuer struct {
	code     string
	err      error
	calls    int
	lastUser uint
}

func (s *stubInviteIssuer) EnsureInviteCode(userID uint) (string, error) {
	s.calls++
	s.lastUser = userID
	return s.code, s.err
}

func TestBuildInviteDisabledSkipsIssuer(t *testing.T) {
	issuer := &stubInviteIssuer{code: "SHOULDNOT"}
	got, err := buildInvite(16, &pointsdomain.Config{ReferralEnabled: false, ReferralRegisterPoints: 20}, issuer)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Enabled || got.Code != "" {
		t.Fatalf("disabled invite = %+v", got)
	}
	if issuer.calls != 0 {
		t.Fatalf("issuer called %d times", issuer.calls)
	}
}

func TestBuildInviteEnabledReturnsCodeAndRewards(t *testing.T) {
	issuer := &stubInviteIssuer{code: "QZ7MD9LD"}
	got, err := buildInvite(16, &pointsdomain.Config{
		ReferralEnabled:          true,
		ReferralRegisterPoints:   20,
		ReferralFirstOrderPoints: 50,
	}, issuer)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !got.Enabled || got.Code != "QZ7MD9LD" || got.RegisterPoints != 20 || got.FirstOrderPoints != 50 {
		t.Fatalf("invite = %+v", got)
	}
	if issuer.calls != 1 || issuer.lastUser != 16 {
		t.Fatalf("issuer calls=%d user=%d", issuer.calls, issuer.lastUser)
	}
}

func TestBuildInviteNilIssuer(t *testing.T) {
	_, err := buildInvite(16, &pointsdomain.Config{ReferralEnabled: true}, nil)
	if !errors.Is(err, pointscontract.ErrInviteIssuerRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildInviteEmptyCode(t *testing.T) {
	_, err := buildInvite(16, &pointsdomain.Config{ReferralEnabled: true}, &stubInviteIssuer{code: "  "})
	if !errors.Is(err, pointscontract.ErrInviteCodeEmpty) {
		t.Fatalf("err = %v", err)
	}
}

func TestGetInviteZeroUser(t *testing.T) {
	svc := NewService(nil, nil)
	_, err := svc.GetInvite(0)
	if !errors.Is(err, pointscontract.ErrAccountNotFound) {
		t.Fatalf("err = %v", err)
	}
}
