package contract

import "errors"

var (
	ErrNotFound         = errors.New("member_level_not_found")
	ErrSlugExists       = errors.New("member_level_slug_exists")
	ErrSortOrderUsed    = errors.New("member_level_sort_order_used")
	ErrDeleteDefault    = errors.New("member_level_cannot_delete_default")
	ErrUserNotFound     = errors.New("user_not_found")
	ErrInvalidPeriod    = errors.New("membership_period_invalid")
	ErrPlanNotFound     = errors.New("membership_plan_not_found")
	ErrPlanInactive     = errors.New("membership_plan_inactive")
	ErrPlanNotPriced    = errors.New("membership_plan_not_priced")
	ErrPurchaseDisabled = errors.New("membership_purchase_disabled")
)
