package application

import (
	"time"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	freeclaimcontract "github.com/dujiao-next/internal/modules/freeclaim/contract"
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"
	pointsapp "github.com/dujiao-next/internal/modules/points/application"
	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// ApplyWalletBalance coordinates the order allocation update with the
// wallet debit in the caller's transaction. Wallet owns account movements;
// Order owns wallet_paid_amount and online_paid_amount.
func ApplyWalletBalance(
	wallets *walletapp.Service,
	tx ordercontract.Transaction,
	order *orderdomain.Order,
	useBalance bool,
) (decimal.Decimal, error) {
	if tx == nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	if order == nil {
		return decimal.Zero, ErrOrderNotFound
	}
	if wallets == nil {
		return decimal.Zero, walletcontract.ErrAccountNotFound
	}

	amount, err := wallets.ApplyOrderBalance(tx.Wallets(), walletcontract.OrderBalanceInput{
		OrderID:          order.ID,
		UserID:           order.UserID,
		TotalAmount:      order.TotalAmount,
		WalletPaidAmount: order.WalletPaidAmount,
		Currency:         order.Currency,
		UseBalance:       useBalance,
	})
	if err != nil {
		return decimal.Zero, err
	}
	deducted := amount.Decimal.Round(2)
	if !useBalance || order.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) || deducted.LessThanOrEqual(decimal.Zero) {
		return deducted, nil
	}

	now := time.Now()
	onlineAmount := normalizeOrderAmount(order.TotalAmount.Decimal.Sub(deducted))
	if err := tx.Orders().UpdateFields(order.ID, map[string]interface{}{
		"wallet_paid_amount": money.FromDecimal(deducted),
		"online_paid_amount": money.FromDecimal(onlineAmount),
		"updated_at":         now,
	}); err != nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	order.WalletPaidAmount = money.FromDecimal(deducted)
	order.OnlinePaidAmount = money.FromDecimal(onlineAmount)
	order.UpdatedAt = now
	return deducted, nil
}

// ReleaseWalletBalance atomically claims the order allocation before
// asking Wallet to credit it back. This prevents duplicate credits while
// keeping the aggregate write in the owning context.
func ReleaseWalletBalance(
	wallets *walletapp.Service,
	tx ordercontract.Transaction,
	order *orderdomain.Order,
	transactionType string,
	remark string,
) (decimal.Decimal, error) {
	if tx == nil {
		return decimal.Zero, ErrOrderUpdateFailed
	}
	if order == nil || order.UserID == 0 || order.WalletPaidAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, nil
	}
	if wallets == nil {
		return decimal.Zero, walletcontract.ErrAccountNotFound
	}

	claimed := false
	amount, err := wallets.ReleaseOrderBalance(
		tx.Wallets(),
		walletcontract.OrderReleaseInput{
			OrderID:          order.ID,
			UserID:           order.UserID,
			WalletPaidAmount: order.WalletPaidAmount,
			TotalAmount:      order.TotalAmount,
			Currency:         order.Currency,
			TransactionType:  transactionType,
			Remark:           remark,
		},
		func(now time.Time) (bool, error) {
			affected, updateErr := tx.Orders().UpdateFieldsWhereWalletPaid(order.ID, map[string]interface{}{
				"wallet_paid_amount": money.FromDecimal(decimal.Zero),
				"online_paid_amount": money.FromDecimal(order.TotalAmount.Decimal.Round(2)),
				"updated_at":         now,
			})
			if updateErr != nil {
				return false, ErrOrderUpdateFailed
			}
			claimed = affected > 0
			return claimed, nil
		},
	)
	if err != nil {
		return decimal.Zero, err
	}
	if claimed {
		order.WalletPaidAmount = money.FromDecimal(decimal.Zero)
		order.OnlinePaidAmount = money.FromDecimal(order.TotalAmount.Decimal.Round(2))
		order.UpdatedAt = time.Now()
	}
	return amount.Decimal.Round(2), nil
}

// PointsExchangePolicy 是积分兑换的运行期策略，由调用方在事务外读取配置后传入，
// 避免在事务内再读一次配置（SQLite 单连接池下会自锁）。
type PointsExchangePolicy struct {
	DailyLimit    int64
	SKUDailyLimit int64
}

// ApplyOrderPoints 在调用方事务内用积分整单全额抵扣订单，并把抵扣结果写回订单。
//
// 这里是积分兑换所有业务门控的唯一权威点——门控散落各处必然漏。
// 设计上只支持"整单全额抵扣"：抵扣后应付在线金额必为 0，CreatePayment 会在同一事务内
// 造合成支付并 markOrderPaid，因此不存在"积分已扣但订单仍待支付"的中间态，
// 也就不需要任何积分回退补偿逻辑（取消/超时/支付失败四条路径一行不改）。
func ApplyOrderPoints(
	points *pointsapp.Service,
	tx ordercontract.Transaction,
	order *orderdomain.Order,
	policy PointsExchangePolicy,
) (int64, error) {
	if tx == nil {
		return 0, ErrOrderUpdateFailed
	}
	if order == nil {
		return 0, ErrOrderNotFound
	}
	if points == nil {
		return 0, pointscontract.ErrExchangeDisabled
	}

	// 幂等早退：已抵扣过的订单直接返回，对齐钱包侧 existingPaid 的处理。
	if order.PointsPaidAmount.Decimal.GreaterThan(decimal.Zero) {
		return order.PointsAmount, nil
	}
	if order.UserID == 0 {
		return 0, pointscontract.ErrExchangeGuestForbidden
	}
	// 分销站不开放积分兑换：分销订单有独立的利润与结算链路，
	// 而积分订单不进在线回调、不产生利润流水，混在一起会让分销对账凭空缺行。
	if order.ResellerID != nil {
		return 0, pointscontract.ErrExchangeNotSupported
	}
	// 禁止叠加优惠券：下单事务里券已被真实核销并计入使用次数，
	// 若再整单积分抵扣，买家一分钱没省却白白消耗一张券。
	if order.CouponID != nil {
		return 0, pointscontract.ErrExchangeCouponNotAllowed
	}
	if order.PointsAmount <= 0 {
		return 0, pointscontract.ErrExchangeNotSupported
	}

	// 单 SKU 每日兑换上限。content 交付商品完全不占库存，
	// 只有用户维度上限挡不住一个热门教程被全站薅光。
	if policy.SKUDailyLimit > 0 {
		if err := ensureSKUDailyExchangeLimit(tx, order, policy.SKUDailyLimit); err != nil {
			return 0, err
		}
	}

	used, err := points.DebitOrderPoints(tx.Points(), pointscontract.OrderPointsInput{
		OrderID:    order.ID,
		UserID:     order.UserID,
		Points:     order.PointsAmount,
		DailyLimit: policy.DailyLimit,
		Remark:     "订单积分兑换",
	})
	if err != nil {
		return 0, err
	}

	now := time.Now()
	zero := money.FromDecimal(decimal.Zero)
	paid := money.FromDecimal(normalizeOrderAmount(order.TotalAmount.Decimal))
	if err := tx.Orders().UpdateFields(order.ID, map[string]interface{}{
		"points_paid_amount": paid,
		"online_paid_amount": zero,
		"updated_at":         now,
	}); err != nil {
		return 0, ErrOrderUpdateFailed
	}
	order.PointsPaidAmount = paid
	order.OnlinePaidAmount = zero
	order.UpdatedAt = now

	// 子单同样写 points_paid_amount：两条后台现金退款路径都接受任意订单 id，
	// 只写父单就留下一个"对子单发起退款即可全额套现"的口子。
	for index := range order.Children {
		child := &order.Children[index]
		childPaid := money.FromDecimal(normalizeOrderAmount(child.TotalAmount.Decimal))
		if err := tx.Orders().UpdateFields(child.ID, map[string]interface{}{
			"points_paid_amount": childPaid,
			"online_paid_amount": zero,
			"updated_at":         now,
		}); err != nil {
			return 0, ErrOrderUpdateFailed
		}
		child.PointsPaidAmount = childPaid
		child.OnlinePaidAmount = zero
		child.UpdatedAt = now
	}
	return used, nil
}

// ensureSKUDailyExchangeLimit 校验订单中每个 SKU 当日被兑换的件数是否超限。
func ensureSKUDailyExchangeLimit(tx ordercontract.Transaction, order *orderdomain.Order, limit int64) error {
	wanted := make(map[uint]int64)
	collect := func(items []orderdomain.OrderItem) {
		for index := range items {
			item := &items[index]
			if item.SKUID == 0 || item.Quantity <= 0 {
				continue
			}
			wanted[item.SKUID] += int64(item.Quantity)
		}
	}
	collect(order.Items)
	for index := range order.Children {
		collect(order.Children[index].Items)
	}
	if len(wanted) == 0 {
		return nil
	}

	now := time.Now()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for skuID, quantity := range wanted {
		used, err := tx.Orders().CountPointsExchangedQuantityBySKUSince(skuID, since)
		if err != nil {
			return err
		}
		if used+quantity > limit {
			return pointscontract.ErrSKUDailyLimitExceeded
		}
	}
	return nil
}

// FreeClaimIntent 描述一次免费领取的意图，由调用方在事务外准备好。
type FreeClaimIntent struct {
	ClaimDate string
	ProductID uint
	SKUID     uint
}

// ApplyFreeClaim 在调用方事务内以"免费领取"结算整单，并写入领取记录。
//
// 领取记录与订单结算必须在同一事务：free_claim_records 的 (user_id, claim_date)
// 唯一索引是"每天只能领一次"在并发下唯一真正生效的防线，
// 分成两步写就会出现"订单已白拿、记录没落上"的窗口。
func ApplyFreeClaim(
	tx ordercontract.Transaction,
	order *orderdomain.Order,
	intent FreeClaimIntent,
) error {
	if tx == nil {
		return ErrOrderUpdateFailed
	}
	if order == nil {
		return ErrOrderNotFound
	}
	if order.UserID == 0 {
		return freeclaimcontract.ErrGuestForbidden
	}
	// 幂等早退：已结算过的订单直接返回，对齐钱包与积分侧的处理。
	if order.FreeClaimPaidAmount.Decimal.GreaterThan(decimal.Zero) {
		return nil
	}
	// 分销站不参与免费领取，与积分兑换保持一致。
	if order.ResellerID != nil {
		return freeclaimcontract.ErrDisabled
	}
	// 所领必须是当日展示的那一件：订单内容与领取意图不符时拒绝，
	// 防止前端拼一个任意商品的订单来"领取"。
	//
	// 条目要从父单与子单一起收集：本仓库对每个 plan 无条件拆子单，
	// 单商品订单的条目也挂在子单上，只看 order.Items 会永远判定不符。
	claimItems := append([]orderdomain.OrderItem{}, order.Items...)
	for index := range order.Children {
		claimItems = append(claimItems, order.Children[index].Items...)
	}
	if len(claimItems) != 1 ||
		claimItems[0].ProductID != intent.ProductID ||
		claimItems[0].SKUID != intent.SKUID ||
		claimItems[0].Quantity != 1 {
		return freeclaimcontract.ErrItemMismatch
	}

	claims := tx.FreeClaims()
	if claims == nil {
		return freeclaimcontract.ErrTransactionRequired
	}
	now := time.Now()
	if err := claims.CreateClaim(&freeclaimdomain.Claim{
		UserID:    order.UserID,
		ClaimDate: intent.ClaimDate,
		ProductID: intent.ProductID,
		SKUID:     intent.SKUID,
		OrderID:   order.ID,
		CreatedAt: now,
	}); err != nil {
		return err
	}

	zero := money.FromDecimal(decimal.Zero)
	paid := money.FromDecimal(normalizeOrderAmount(order.TotalAmount.Decimal))
	if err := tx.Orders().UpdateFields(order.ID, map[string]interface{}{
		"free_claim_paid_amount": paid,
		"online_paid_amount":     zero,
		"updated_at":             now,
	}); err != nil {
		return ErrOrderUpdateFailed
	}
	order.FreeClaimPaidAmount = paid
	order.OnlinePaidAmount = zero
	order.UpdatedAt = now

	// 子单同样写：两条后台现金退款路径都接受任意订单 id，
	// 只写父单会留下"对子单退款即可把免费商品换成现金"的口子。
	for index := range order.Children {
		child := &order.Children[index]
		childPaid := money.FromDecimal(normalizeOrderAmount(child.TotalAmount.Decimal))
		if err := tx.Orders().UpdateFields(child.ID, map[string]interface{}{
			"free_claim_paid_amount": childPaid,
			"online_paid_amount":     zero,
			"updated_at":             now,
		}); err != nil {
			return ErrOrderUpdateFailed
		}
		child.FreeClaimPaidAmount = childPaid
		child.OnlinePaidAmount = zero
		child.UpdatedAt = now
	}
	return nil
}
