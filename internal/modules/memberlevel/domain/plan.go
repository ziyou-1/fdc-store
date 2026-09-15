package domain

import (
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

const (
	PeriodMonth   = "month"
	PeriodQuarter = "quarter"
	PeriodYear    = "year"
)

// PeriodSpec is the fixed membership term: month 9折 / quarter 8.5折 / year 8折.
type PeriodSpec struct {
	Period       string
	DiscountRate decimal.Decimal
	DurationDays int
}

// Plan is an admin-priced, purchasable membership term.
// Discount and duration come from Period, not from this row.
type Plan struct {
	Period          string       `gorm:"primaryKey;type:varchar(16)" json:"period"`
	Price           money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price"`
	NameJSON        jsonmap.JSON `gorm:"type:json;not null" json:"name"`
	DescriptionJSON jsonmap.JSON `gorm:"type:json;not null" json:"description"`
	IsActive        bool         `gorm:"not null;default:true" json:"is_active"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

func (Plan) TableName() string {
	return "membership_plans"
}

// SpecForPeriod returns the hardcoded term for a period.
func SpecForPeriod(period string) (PeriodSpec, bool) {
	switch period {
	case PeriodMonth:
		return PeriodSpec{Period: PeriodMonth, DiscountRate: decimal.NewFromInt(90), DurationDays: 30}, true
	case PeriodQuarter:
		return PeriodSpec{Period: PeriodQuarter, DiscountRate: decimal.NewFromInt(85), DurationDays: 90}, true
	case PeriodYear:
		return PeriodSpec{Period: PeriodYear, DiscountRate: decimal.NewFromInt(80), DurationDays: 365}, true
	default:
		return PeriodSpec{}, false
	}
}

// ApplyDiscountRate applies a membership percent rate (90 = 9折) to a base price.
func ApplyDiscountRate(basePrice, rate decimal.Decimal) (memberPrice decimal.Decimal, memberDiscount decimal.Decimal) {
	if rate.LessThanOrEqual(decimal.Zero) || rate.GreaterThanOrEqual(decimal.NewFromInt(100)) {
		return basePrice, decimal.Zero
	}
	mp := basePrice.Mul(rate).Div(decimal.NewFromInt(100)).Round(2)
	if mp.LessThan(basePrice) {
		return mp, basePrice.Sub(mp).Round(2)
	}
	return basePrice, decimal.Zero
}

// MembershipActive reports whether the user currently has a paid membership term.
func MembershipActive(user *userdomain.User, now time.Time) bool {
	if user == nil || user.MembershipExpiresAt == nil {
		return false
	}
	if !user.MembershipExpiresAt.After(now) {
		return false
	}
	return user.MembershipDiscountRate.Decimal.GreaterThan(decimal.Zero) &&
		user.MembershipDiscountRate.Decimal.LessThan(decimal.NewFromInt(100))
}

// ApplyPurchase writes a paid term onto the user, replacing any previous term.
func ApplyPurchase(user *userdomain.User, period string, durationDays int, discountRate decimal.Decimal, now time.Time) bool {
	if user == nil || durationDays <= 0 {
		return false
	}
	if _, ok := SpecForPeriod(period); !ok {
		return false
	}
	if discountRate.LessThanOrEqual(decimal.Zero) || discountRate.GreaterThanOrEqual(decimal.NewFromInt(100)) {
		return false
	}
	// 续费叠加时长而不是覆盖：老会员还剩的天数必须保留，
	// 否则「还剩 300 天时续一个月」会把 300 天直接抹成 30 天。
	base := now
	if MembershipActive(user, now) {
		base = *user.MembershipExpiresAt
	}
	expires := base.Add(time.Duration(durationDays) * 24 * time.Hour)
	// 档位以最新一次购买为准：买了年卡就按年卡的折扣走，
	// 时长叠加与折扣档位是两件事。
	user.MembershipPeriod = period
	user.MembershipDiscountRate = money.FromDecimal(discountRate)
	user.MembershipExpiresAt = &expires
	return true
}

// DefaultPlans are the three seed rows. Price and copy are admin-editable after seed.
func DefaultPlans() []Plan {
	return []Plan{
		{
			Period: PeriodMonth,
			Price:  money.FromDecimal(decimal.RequireFromString("9.90")),
			NameJSON: jsonmap.JSON{
				"zh-CN": "月度会员",
				"zh-TW": "月度會員",
				"en-US": "Monthly membership",
				"id-ID": "Member Bulanan",
			},
			DescriptionJSON: jsonmap.JSON{
				"zh-CN": "全站商品 9 折，有效期 30 天。",
				"zh-TW": "全站商品 9 折，有效期 30 天。",
				"en-US": "10% off storewide for 30 days.",
				"id-ID": "Diskon 10% untuk semua produk, berlaku 30 hari.",
			},
			IsActive: true,
		},
		{
			Period: PeriodQuarter,
			Price:  money.FromDecimal(decimal.RequireFromString("24.90")),
			NameJSON: jsonmap.JSON{
				"zh-CN": "季度会员",
				"zh-TW": "季度會員",
				"en-US": "Quarterly membership",
				"id-ID": "Member Kuartalan",
			},
			DescriptionJSON: jsonmap.JSON{
				"zh-CN": "全站商品 8.5 折，有效期 90 天。",
				"zh-TW": "全站商品 8.5 折，有效期 90 天。",
				"en-US": "15% off storewide for 90 days.",
				"id-ID": "Diskon 15% untuk semua produk, berlaku 90 hari.",
			},
			IsActive: true,
		},
		{
			Period: PeriodYear,
			Price:  money.FromDecimal(decimal.RequireFromString("79.90")),
			NameJSON: jsonmap.JSON{
				"zh-CN": "年度会员",
				"zh-TW": "年度會員",
				"en-US": "Yearly membership",
				"id-ID": "Member Tahunan",
			},
			DescriptionJSON: jsonmap.JSON{
				"zh-CN": "全站商品 8 折，有效期 365 天。",
				"zh-TW": "全站商品 8 折，有效期 365 天。",
				"en-US": "20% off storewide for 365 days.",
				"id-ID": "Diskon 20% untuk semua produk, berlaku 365 hari.",
			},
			IsActive: true,
		},
	}
}
