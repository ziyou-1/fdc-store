package application

import (
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/shopspring/decimal"
)

func normalizeOrderAmount(amount decimal.Decimal) decimal.Decimal {
	normalized := amount.Round(2)
	if normalized.LessThan(decimal.Zero) {
		return decimal.Zero
	}
	return normalized
}

// ResolveOnlinePayableAmount 返回扣掉全部内部 tender（钱包余额 + 积分抵扣）之后
// 仍需在线支付的法币额。
//
// 把这个减法收口成一处，是因为它原本散落在四个地方：CreatePayment 决定是否走合成支付、
// 回调校验实付是否足额、markOrderPaid 回写 online_paid_amount、以及余额释放后的回写。
// 任何一处漏加新 tender 都会静默算错钱——少付会被当成欠款转进钱包且订单不履约。
func ResolveOnlinePayableAmount(order *orderdomain.Order) decimal.Decimal {
	if order == nil {
		return decimal.Zero
	}
	return normalizeOrderAmount(
		order.TotalAmount.Decimal.
			Sub(order.WalletPaidAmount.Decimal).
			Sub(order.PointsPaidAmount.Decimal).
			Sub(order.FreeClaimPaidAmount.Decimal),
	)
}
