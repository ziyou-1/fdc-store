package application

import (
	"errors"
	"strings"
	"time"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

// Service 实现积分用例。
type Service struct {
	points          pointscontract.Repository
	transactions    pointscontract.UnitOfWork
	rechargeCreator pointscontract.RechargePaymentCreator
	inviteIssuer    pointscontract.InviteCodeIssuer
}

// SetRechargeCreator 注入积分充值建单端口。
// 用 setter 而不是构造参数：支付服务本身依赖 points，构造期存在先后顺序问题，
// 这与仓库既有的 SetProcurementService / SetContentFulfillService 同一模式。
func (s *Service) SetRechargeCreator(creator pointscontract.RechargePaymentCreator) {
	if s == nil {
		return
	}
	s.rechargeCreator = creator
}

// SetInviteIssuer 注入邀请码签发端口。推荐奖励复用 affiliate_code，
// 但积分模块不能 import affiliate/application，只能由装配层注入。
func (s *Service) SetInviteIssuer(issuer pointscontract.InviteCodeIssuer) {
	if s == nil {
		return
	}
	s.inviteIssuer = issuer
}

var _ pointscontract.UseCase = (*Service)(nil)

// NewService 构造积分服务。
func NewService(points pointscontract.Repository, transactions pointscontract.UnitOfWork) *Service {
	return &Service{points: points, transactions: transactions}
}

// GetAccount 读取积分账户，未开户时返回零值账户而不是 nil。
func (s *Service) GetAccount(userID uint) (*pointsdomain.Account, error) {
	if userID == 0 {
		return nil, pointscontract.ErrAccountNotFound
	}
	account, err := s.points.GetAccountByUserID(userID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return pointscontract.EmptyAccount(userID), nil
	}
	return account, nil
}

// GetBalancesByUserIDs 批量读取积分余额，缺账户的用户不出现在结果中。
func (s *Service) GetBalancesByUserIDs(userIDs []uint) (map[uint]int64, error) {
	balances := make(map[uint]int64, len(userIDs))
	if len(userIDs) == 0 {
		return balances, nil
	}
	accounts, err := s.points.GetAccountsByUserIDs(userIDs)
	if err != nil {
		return nil, err
	}
	for index := range accounts {
		balances[accounts[index].UserID] = accounts[index].Balance
	}
	return balances, nil
}

// ListTransactions 分页查询积分流水。
func (s *Service) ListTransactions(filter pointscontract.TransactionListFilter) ([]pointsdomain.Transaction, int64, error) {
	return s.points.ListTransactions(filter)
}

// Credit 积分入账，自开事务。
func (s *Service) Credit(input pointscontract.CreditInput) (*pointsdomain.Account, *pointsdomain.Transaction, error) {
	if s.transactions == nil {
		return nil, nil, pointscontract.ErrTransactionRequired
	}
	var accountResult *pointsdomain.Account
	var transactionResult *pointsdomain.Transaction
	if err := s.transactions.WithinTransaction(func(tx pointscontract.Transaction) error {
		account, transaction, err := s.CreditInTransaction(tx, input)
		if err != nil {
			return err
		}
		accountResult, transactionResult = account, transaction
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return accountResult, transactionResult, nil
}

// CreditInTransaction 在调用方已开启的事务内入账。Reference 相同的重复调用只入账一次。
func (s *Service) CreditInTransaction(
	tx pointscontract.Transaction,
	input pointscontract.CreditInput,
) (*pointsdomain.Account, *pointsdomain.Transaction, error) {
	if tx == nil {
		return nil, nil, pointscontract.ErrTransactionRequired
	}
	repository := tx.Points()
	if repository == nil {
		return nil, nil, pointscontract.ErrTransactionRequired
	}
	if input.UserID == 0 {
		return nil, nil, pointscontract.ErrAccountNotFound
	}
	if input.Amount <= 0 {
		return nil, nil, pointscontract.ErrInvalidAmount
	}
	reference := strings.TrimSpace(input.Reference)
	if reference == "" {
		return nil, nil, pointscontract.ErrReferenceRequired
	}

	existing, err := repository.GetTransactionByReference(reference)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		account, accountErr := ensureAccountForUpdate(repository, input.UserID, time.Now())
		if accountErr != nil {
			return nil, nil, accountErr
		}
		return account, existing, nil
	}

	now := time.Now()
	account, err := ensureAccountForUpdate(repository, input.UserID, now)
	if err != nil {
		return nil, nil, err
	}
	before := account.Balance
	after := before + input.Amount

	account.Balance = after
	account.TotalEarned += input.Amount
	account.UpdatedAt = now
	if err := repository.UpdateAccount(account); err != nil {
		return nil, nil, pointscontract.ErrAccountUpdateFailed
	}

	transaction := &pointsdomain.Transaction{
		UserID:        input.UserID,
		OrderID:       input.OrderID,
		Type:          normalizeType(input.Type, pointscontract.TxnTypeAdminAdjust),
		Direction:     pointscontract.TxnDirectionIn,
		Amount:        input.Amount,
		BalanceBefore: before,
		BalanceAfter:  after,
		Reference:     reference,
		Remark:        cleanRemark(input.Remark, "积分入账"),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repository.CreateTransaction(transaction); err != nil {
		// 并发下另一路已经用同一 reference 入账：唯一索引挡下后回读既有流水，
		// 保持"同 reference 只入账一次"的幂等语义，而不是把并发报成失败。
		if errors.Is(err, pointscontract.ErrDuplicateReference) {
			existing, queryErr := repository.GetTransactionByReference(reference)
			if queryErr == nil && existing != nil {
				return account, existing, nil
			}
		}
		return nil, nil, pointscontract.ErrTransactionCreateFailed
	}
	return account, transaction, nil
}

// Debit 积分扣减，余额不足时整笔失败。
func (s *Service) Debit(input pointscontract.DebitInput) (*pointsdomain.Account, *pointsdomain.Transaction, error) {
	if input.Amount <= 0 {
		return nil, nil, pointscontract.ErrInvalidAmount
	}
	reference := strings.TrimSpace(input.Reference)
	if reference == "" {
		return nil, nil, pointscontract.ErrReferenceRequired
	}
	return s.changeBalance(
		input.UserID,
		-input.Amount,
		normalizeType(input.Type, pointscontract.TxnTypeExchange),
		input.OrderID,
		reference,
		cleanRemark(input.Remark, "积分扣减"),
		nil,
	)
}

// AdminAdjust 后台人工增减积分，落库时记录操作人。
func (s *Service) AdminAdjust(input pointscontract.AdjustInput) (*pointsdomain.Account, *pointsdomain.Transaction, error) {
	if input.Delta == 0 {
		return nil, nil, pointscontract.ErrInvalidAmount
	}
	reference := buildReference("adjust", input.UserID, time.Now())
	return s.changeBalance(
		input.UserID,
		input.Delta,
		pointscontract.TxnTypeAdminAdjust,
		nil,
		reference,
		cleanRemark(input.Remark, "后台调整"),
		input.OperatorAdminID,
	)
}

func (s *Service) changeBalance(
	userID uint,
	delta int64,
	transactionType string,
	orderID *uint,
	reference, remark string,
	operatorAdminID *uint,
) (*pointsdomain.Account, *pointsdomain.Transaction, error) {
	if s.transactions == nil {
		return nil, nil, pointscontract.ErrTransactionRequired
	}
	if userID == 0 {
		return nil, nil, pointscontract.ErrAccountNotFound
	}
	var accountResult *pointsdomain.Account
	var transactionResult *pointsdomain.Transaction
	if err := s.transactions.WithinTransaction(func(tx pointscontract.Transaction) error {
		repository := tx.Points()
		if repository == nil {
			return pointscontract.ErrTransactionRequired
		}
		now := time.Now()
		account, err := ensureAccountForUpdate(repository, userID, now)
		if err != nil {
			return err
		}

		before := account.Balance
		after := before + delta
		if after < 0 {
			return pointscontract.ErrInsufficientPoints
		}

		direction := pointscontract.TxnDirectionIn
		amount := delta
		if delta < 0 {
			direction = pointscontract.TxnDirectionOut
			amount = -delta
			account.TotalSpent += amount
		} else {
			account.TotalEarned += amount
		}

		account.Balance = after
		account.UpdatedAt = now
		if err := repository.UpdateAccount(account); err != nil {
			return pointscontract.ErrAccountUpdateFailed
		}

		transaction := &pointsdomain.Transaction{
			UserID:          userID,
			OperatorAdminID: operatorAdminID,
			OrderID:         orderID,
			Type:            transactionType,
			Direction:       direction,
			Amount:          amount,
			BalanceBefore:   before,
			BalanceAfter:    after,
			Reference:       strings.TrimSpace(reference),
			Remark:          remark,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := repository.CreateTransaction(transaction); err != nil {
			return pointscontract.ErrTransactionCreateFailed
		}
		accountResult, transactionResult = account, transaction
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return accountResult, transactionResult, nil
}

func ensureAccountForUpdate(
	repository pointscontract.Repository,
	userID uint,
	now time.Time,
) (*pointsdomain.Account, error) {
	if repository == nil {
		return nil, pointscontract.ErrTransactionRequired
	}
	account, err := repository.GetAccountByUserIDForUpdate(userID)
	if err != nil {
		return nil, err
	}
	if account != nil {
		return account, nil
	}
	account = &pointsdomain.Account{UserID: userID, CreatedAt: now, UpdatedAt: now}
	if err := repository.CreateAccount(account); err != nil {
		created, queryErr := repository.GetAccountByUserIDForUpdate(userID)
		if queryErr == nil && created != nil {
			return created, nil
		}
		return nil, pointscontract.ErrAccountCreateFailed
	}
	return account, nil
}

func normalizeType(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	return value
}

func cleanRemark(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	if len([]rune(value)) > 80 {
		return string([]rune(value)[:80])
	}
	return value
}
