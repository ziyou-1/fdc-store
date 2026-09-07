package application

import (
	"errors"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
)

var (
	ErrInvalidEmail                   = errors.New("invalid email")
	ErrInvalidOrderItem               = productdomain.ErrPurchaseQuantityInvalid
	ErrInvalidOrderAmount             = errors.New("invalid order amount")
	ErrOrderCurrencyMismatch          = errors.New("order currency mismatch")
	ErrOrderNotFound                  = resellercontract.ErrOrderNotFound
	ErrOrderCreateFailed              = errors.New("order create failed")
	ErrOrderFetchFailed               = errors.New("order fetch failed")
	ErrProductNotAvailable            = errors.New("product not available")
	ErrProductPurchaseNotAllowed      = errors.New("product purchase not allowed")
	ErrProductMaxPurchaseExceeded     = productdomain.ErrMaxPurchaseExceeded
	ErrProductMinPurchaseNotMet       = productdomain.ErrMinPurchaseNotMet
	ErrProductPriceInvalid            = productcontract.ErrProductPriceInvalid
	ErrProductSKURequired             = errors.New("product sku required")
	ErrProductSKUInvalid              = productcontract.ErrProductSKUInvalid
	ErrFulfillmentInvalid             = productcontract.ErrFulfillmentInvalid
	ErrOrderStatusInvalid             = errors.New("order status invalid")
	ErrOrderRefundExpired             = errors.New("order refund expired")
	ErrOrderCancelNotAllowed          = errors.New("order cancel not allowed")
	ErrOrderUpdateFailed              = errors.New("order update failed")
	ErrGuestOrderNotFound             = errors.New("guest order not found")
	ErrGuestEmailRequired             = errors.New("guest email required")
	ErrGuestPasswordRequired          = errors.New("guest password required")
	ErrRefundRecordCreateFailed       = errors.New("refund record create failed")
	ErrCardSecretInsufficient         = errors.New("card secret insufficient")
	ErrManualStockInsufficient        = errors.New("manual stock insufficient")
	ErrManualFormNotPending           = errors.New("manual form not pending")
	ErrPointsTenderExclusive          = errors.New("points tender exclusive")
	ErrPointsOrderCashRefundForbidden = errors.New("points order cash refund forbidden")
	ErrFreeClaimOrderRefundForbidden  = errors.New("free claim order refund forbidden")
	ErrQueueUnavailable               = errors.New("queue unavailable")
	ErrResellerProductNotListed       = productcontract.ErrResellerProductNotListed
	ErrResellerCouponNotAllowed       = errors.New("reseller coupon not allowed")
	ErrResellerPriceBelowBase         = resellercontract.ErrPriceBelowBase
	ErrResellerMarkupExceeded         = resellercontract.ErrMarkupExceeded
	ErrResellerPricingModeInvalid     = resellercontract.ErrPricingModeInvalid
)
