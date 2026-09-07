package application

import (
	"errors"
	"testing"

	freeclaimcontract "github.com/dujiao-next/internal/modules/freeclaim/contract"
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"
)

type stubMembers struct {
	paid    bool
	levelID uint
}

func (s stubMembers) CurrentMemberLevelID(uint) (uint, error) { return s.levelID, nil }
func (s stubMembers) HasPaidMembership(uint) (bool, error)     { return s.paid, nil }

type stubOrders struct {
	calls int
}

func (s *stubOrders) CreateFreeClaimOrder(uint, uint, uint, string) (string, uint, error) {
	s.calls++
	return "DJTEST", 99, nil
}

type stubRepo struct {
	cfg      *freeclaimdomain.Config
	items    []freeclaimdomain.PoolItem
	existing *freeclaimdomain.Claim
}

func (r *stubRepo) ListActivePoolItems() ([]freeclaimdomain.PoolItem, error) { return r.items, nil }
func (r *stubRepo) ListPoolItems() ([]freeclaimdomain.PoolItem, error)       { return nil, errUnused }
func (r *stubRepo) CreatePoolItem(*freeclaimdomain.PoolItem) error           { return errUnused }
func (r *stubRepo) UpdatePoolItem(*freeclaimdomain.PoolItem) error           { return errUnused }
func (r *stubRepo) DeletePoolItem(uint) error                               { return errUnused }
func (r *stubRepo) GetPoolItemByID(uint) (*freeclaimdomain.PoolItem, error)  { return nil, errUnused }
func (r *stubRepo) CreateClaim(*freeclaimdomain.Claim) error                { return errUnused }
func (r *stubRepo) GetClaimByDate(uint, string) (*freeclaimdomain.Claim, error) {
	return r.existing, nil
}
func (r *stubRepo) ListClaimsByUser(uint, int, int) ([]freeclaimdomain.Claim, int64, error) {
	return nil, 0, errUnused
}
func (r *stubRepo) GetConfig() (*freeclaimdomain.Config, error) { return r.cfg, nil }
func (r *stubRepo) SaveConfig(*freeclaimdomain.Config) error    { return errUnused }

var errUnused = errors.New("unused")

func TestMembershipAllowsClaim(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		only     bool
		minLevel uint
		paid     bool
		levelID  uint
		want     bool
	}{
		{name: "unpaid blocked when member-only", only: true, paid: false, levelID: 1, want: false},
		{name: "paid allowed when member-only", only: true, paid: true, want: true},
		{name: "open to any login when not member-only", only: false, paid: false, want: true},
		{name: "paid still needs min level", only: true, minLevel: 5, paid: true, levelID: 3, want: false},
		{name: "paid meets min level", only: true, minLevel: 5, paid: true, levelID: 5, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := membershipAllowsClaim(tc.only, tc.minLevel, tc.paid, tc.levelID)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestClaimMemberOnlyRejectsUnpaidMembership(t *testing.T) {
	orders := &stubOrders{}
	svc := NewService(&stubRepo{cfg: &freeclaimdomain.Config{Enabled: true, MemberOnly: true}})
	svc.SetCollaborators(orders, stubMembers{paid: false, levelID: 1})
	_, err := svc.Claim(16)
	if !errors.Is(err, freeclaimcontract.ErrMemberOnly) {
		t.Fatalf("err = %v", err)
	}
	if orders.calls != 0 {
		t.Fatalf("orders called %d times", orders.calls)
	}
}

func TestClaimMemberOnlyAllowsPaidMembership(t *testing.T) {
	orders := &stubOrders{}
	svc := NewService(&stubRepo{
		cfg:   &freeclaimdomain.Config{Enabled: true, MemberOnly: true},
		items: []freeclaimdomain.PoolItem{{ID: 1, ProductID: 2, SKUID: 2, IsActive: true}},
	})
	svc.SetCollaborators(orders, stubMembers{paid: true, levelID: 0})
	got, err := svc.Claim(18)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got == nil || got.OrderNo != "DJTEST" {
		t.Fatalf("result = %+v", got)
	}
	if orders.calls != 1 {
		t.Fatalf("orders called %d times", orders.calls)
	}
}

func TestTodayItemEligibleOnlyForPaidMember(t *testing.T) {
	svc := NewService(&stubRepo{
		cfg:   &freeclaimdomain.Config{Enabled: true, MemberOnly: true},
		items: []freeclaimdomain.PoolItem{{ID: 1, ProductID: 2, SKUID: 2, IsActive: true}},
	})
	svc.SetCollaborators(&stubOrders{}, stubMembers{paid: false, levelID: 1})
	guest, err := svc.TodayItem(0)
	if err != nil {
		t.Fatalf("guest err = %v", err)
	}
	if !guest.MemberOnly || guest.Eligible {
		t.Fatalf("guest today = %+v", guest)
	}
	unpaid, err := svc.TodayItem(16)
	if err != nil {
		t.Fatalf("unpaid err = %v", err)
	}
	if unpaid.Eligible {
		t.Fatalf("unpaid today = %+v", unpaid)
	}
	svc.SetCollaborators(&stubOrders{}, stubMembers{paid: true})
	paid, err := svc.TodayItem(18)
	if err != nil {
		t.Fatalf("paid err = %v", err)
	}
	if !paid.Eligible {
		t.Fatalf("paid today = %+v", paid)
	}
}
