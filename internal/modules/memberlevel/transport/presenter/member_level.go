package presenter

import (
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// MemberLevel 是公开会员等级响应。
type MemberLevel struct {
	ID                uint         `json:"id"`
	Name              jsonmap.JSON `json:"name"`
	Slug              string       `json:"slug"`
	Icon              string       `json:"icon"`
	DiscountRate      float64      `json:"discount_rate"`
	RechargeThreshold float64      `json:"recharge_threshold"`
	SpendThreshold    float64      `json:"spend_threshold"`
	IsDefault         bool         `json:"is_default"`
	SortOrder         int          `json:"sort_order"`
}

// Plan is a purchasable membership term.
type Plan struct {
	Period       string       `json:"period"`
	Price        money.Amount `json:"price"`
	DiscountRate float64      `json:"discount_rate"`
	DurationDays int          `json:"duration_days"`
	Name         jsonmap.JSON `json:"name"`
	Description  jsonmap.JSON `json:"description"`
	IsActive     bool         `json:"is_active"`
}

func NewPlan(plan memberleveldomain.Plan) Plan {
	rate := 0.0
	days := 0
	if spec, ok := memberleveldomain.SpecForPeriod(plan.Period); ok {
		rate = spec.DiscountRate.InexactFloat64()
		days = spec.DurationDays
	}
	return Plan{
		Period:       plan.Period,
		Price:        plan.Price,
		DiscountRate: rate,
		DurationDays: days,
		Name:         plan.NameJSON,
		Description:  plan.DescriptionJSON,
		IsActive:     plan.IsActive,
	}
}

func NewPlans(plans []memberleveldomain.Plan) []Plan {
	views := make([]Plan, 0, len(plans))
	for _, plan := range plans {
		views = append(views, NewPlan(plan))
	}
	return views
}
