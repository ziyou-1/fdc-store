package contract

import "errors"

var (
	ErrInvalidAmount           = errors.New("points invalid amount")
	ErrInsufficientPoints      = errors.New("points insufficient balance")
	ErrAccountNotFound         = errors.New("points account not found")
	ErrAccountCreateFailed     = errors.New("points account create failed")
	ErrAccountUpdateFailed     = errors.New("points account update failed")
	ErrTransactionCreateFailed = errors.New("points transaction create failed")
	// ErrDuplicateReference 表示同一 reference 的积分流水已存在。
	// gorm 未开启 TranslateError（见 platform/database/gormdb/db.go 的 gorm.Config），
	// 因此重复键只能由 store 层按驱动报文翻译成这个错误。
	ErrDuplicateReference  = errors.New("points duplicate reference")
	ErrTransactionRequired = errors.New("points transaction required")
	ErrReferenceRequired   = errors.New("points reference required")
	ErrCheckinDisabled     = errors.New("points checkin disabled")
	ErrAlreadyCheckedIn    = errors.New("points already checked in today")
	ErrConfigUpdateFailed  = errors.New("points config update failed")
	ErrInvalidConfig       = errors.New("points invalid config")

	// 积分兑换商品相关
	ErrExchangeDisabled         = errors.New("points exchange disabled")
	ErrExchangeNotSupported     = errors.New("points exchange not supported for order")
	ErrExchangeGuestForbidden   = errors.New("points exchange forbidden for guest")
	ErrExchangeCouponNotAllowed = errors.New("points exchange coupon not allowed")
	ErrDailyLimitExceeded       = errors.New("points daily exchange limit exceeded")
	ErrSKUDailyLimitExceeded    = errors.New("points sku daily exchange limit exceeded")

	// 积分充值相关
	ErrRechargeDisabled       = errors.New("points recharge disabled")
	ErrRechargeRateInvalid    = errors.New("points recharge rate invalid")
	ErrRechargeNotFound       = errors.New("points recharge not found")
	ErrRechargeChannelInvalid = errors.New("points recharge channel invalid")

	// 推荐奖励相关
	ErrReferralLimitRequired = errors.New("points referral daily limit required")
	ErrInviteIssuerRequired  = errors.New("points invite issuer required")
	ErrInviteCodeEmpty       = errors.New("points invite code empty")
)
