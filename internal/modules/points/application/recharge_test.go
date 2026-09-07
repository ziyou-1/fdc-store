package application

import (
	"testing"
	"time"

	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestResolveRechargePointsOneToOne(t *testing.T) {
	got := ResolveRechargePoints(money.FromDecimal(decimal.NewFromInt(1)), 1)
	if got != 1 {
		t.Fatalf("1.00 * 1 = %d, want 1", got)
	}
	got = ResolveRechargePoints(money.FromDecimal(decimal.RequireFromString("1.99")), 1)
	if got != 1 {
		t.Fatalf("1.99 * 1 floor = %d, want 1", got)
	}
	got = ResolveRechargePoints(money.FromDecimal(decimal.NewFromInt(10)), 1)
	if got != 10 {
		t.Fatalf("10.00 * 1 = %d, want 10", got)
	}
}

func TestDefaultConfigRechargeIsOneToOne(t *testing.T) {
	cfg := pointsdomain.DefaultConfig(time.Now())
	if cfg.RechargePointsPerUnit != 1 {
		t.Fatalf("default recharge rate = %d, want 1", cfg.RechargePointsPerUnit)
	}
}
