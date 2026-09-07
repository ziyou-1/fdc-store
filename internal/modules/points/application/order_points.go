package application

import (
	"errors"
	"fmt"
	"time"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

// orderPointsReference 订单积分抵扣的稳定业务键。
// 必须是固定值而不是像 AdminAdjust 那样带时间戳：同一订单的重复调用要落在同一条流水上，
// 这样唯一索引才能在并发下真正兜住"只扣一次"。
func orderPointsReference(orderID uint) string {
	return fmt.Sprintf("order:%d:points_pay", orderID)
}

// DebitOrderPoints 在调用方已开启的事务内扣减订单所需积分。
//
// 为什么不复用 Service.Debit：Debit 走 changeBalance，而 changeBalance 内部自己调用
// s.transactions.WithinTransaction —— s.transactions 绑在根 *gorm.DB 上。在订单事务里调用它
// 会另开一条连接上的独立事务，订单回滚而积分照扣；SQLite 单连接池下还会直接自锁。
func (s *Service) DebitOrderPoints(
	tx pointscontract.Transaction,
	input pointscontract.OrderPointsInput,
) (int64, error) {
	if tx == nil {
		return 0, pointscontract.ErrTransactionRequired
	}
	repository := tx.Points()
	if repository == nil {
		return 0, pointscontract.ErrTransactionRequired
	}
	if input.UserID == 0 {
		return 0, pointscontract.ErrExchangeGuestForbidden
	}
	if input.OrderID == 0 {
		return 0, pointscontract.ErrExchangeNotSupported
	}
	if input.Points <= 0 {
		return 0, pointscontract.ErrInvalidAmount
	}

	reference := orderPointsReference(input.OrderID)

	// 幂等：同一订单重复调用只扣一次。changeBalance 没有这段短路，
	// 所以这里必须自己写，否则重复调用会直接撞 reference 唯一索引。
	existing, err := repository.GetTransactionByReference(reference)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return existing.Amount, nil
	}

	now := time.Now()

	// 每日兑换上限：按当日零点起的 exchange 类流水合计判断。
	if input.DailyLimit > 0 {
		used, sumErr := repository.SumPointsByTypeSince(
			input.UserID, pointscontract.TxnTypeExchange, startOfDay(now),
		)
		if sumErr != nil {
			return 0, sumErr
		}
		if used+input.Points > input.DailyLimit {
			return 0, pointscontract.ErrDailyLimitExceeded
		}
	}

	account, err := ensureAccountForUpdate(repository, input.UserID, now)
	if err != nil {
		return 0, err
	}
	before := account.Balance
	after := before - input.Points
	if after < 0 {
		return 0, pointscontract.ErrInsufficientPoints
	}

	account.Balance = after
	account.TotalSpent += input.Points
	account.UpdatedAt = now
	if err := repository.UpdateAccount(account); err != nil {
		return 0, pointscontract.ErrAccountUpdateFailed
	}

	orderID := input.OrderID
	transaction := &pointsdomain.Transaction{
		UserID:        input.UserID,
		OrderID:       &orderID,
		Type:          pointscontract.TxnTypeExchange,
		Direction:     pointscontract.TxnDirectionOut,
		Amount:        input.Points,
		BalanceBefore: before,
		BalanceAfter:  after,
		Reference:     reference,
		Remark:        cleanRemark(input.Remark, "订单积分兑换"),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repository.CreateTransaction(transaction); err != nil {
		// 并发下另一路已经扣过：唯一索引挡下后回读既有流水。
		// SQLite 上行锁是 no-op，这一层才是真正生效的并发防线。
		if errors.Is(err, pointscontract.ErrDuplicateReference) {
			if duplicated, queryErr := repository.GetTransactionByReference(reference); queryErr == nil && duplicated != nil {
				return duplicated.Amount, nil
			}
		}
		return 0, pointscontract.ErrTransactionCreateFailed
	}
	return input.Points, nil
}

// startOfDay 返回本地时区当日零点，与签到的自然日口径保持一致。
func startOfDay(now time.Time) time.Time {
	year, month, day := now.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, now.Location())
}
