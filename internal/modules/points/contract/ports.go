package contract

import (
	"time"

	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

// Repository 承担积分聚合的全部持久化。事务内调用方通过 Transaction 拿到
// 绑定在同一事务上的同一套端口。
type Repository interface {
	GetAccountByUserID(userID uint) (*pointsdomain.Account, error)
	GetAccountByUserIDForUpdate(userID uint) (*pointsdomain.Account, error)
	GetAccountsByUserIDs(userIDs []uint) ([]pointsdomain.Account, error)
	CreateAccount(account *pointsdomain.Account) error
	UpdateAccount(account *pointsdomain.Account) error

	CreateTransaction(transaction *pointsdomain.Transaction) error
	GetTransactionByReference(reference string) (*pointsdomain.Transaction, error)
	ListTransactions(filter TransactionListFilter) ([]pointsdomain.Transaction, int64, error)
	// SumPointsByTypeSince 统计某用户自 since 起某类流水的积分合计，用于每日兑换上限。
	SumPointsByTypeSince(userID uint, transactionType string, since time.Time) (int64, error)

	CreateCheckin(record *pointsdomain.Checkin) error
	GetCheckinByDate(userID uint, date string) (*pointsdomain.Checkin, error)

	GetConfig() (*pointsdomain.Config, error)
	SaveConfig(config *pointsdomain.Config) error
}

// Transaction 是积分模块视角下的已开启数据库事务，不暴露任何 ORM 原语。
type Transaction interface {
	Points() Repository
}

// UnitOfWork 开启一个积分事务。
type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// InviteCodeIssuer 为积分推荐签发邀请码。
// 邀请码复用 affiliate_profiles.affiliate_code；现金分销开关不影响签发。
type InviteCodeIssuer interface {
	EnsureInviteCode(userID uint) (string, error)
}

// RechargePaymentCreator 是 points 向支付模块请求建单的端口。
// points/application 不能 import payment/application（后者已 import pointsapp，
// 反向引用即编译环），因此跨模块协作必须走这个端口 + 装配层适配器。
type RechargePaymentCreator interface {
	CreatePointsRecharge(input RechargePaymentInput) (*RechargeResult, error)
	ListPointsRecharges(userID uint, page, pageSize int, status string) ([]RechargeRecord, int64, error)
}

// UseCase 是积分模块对外暴露的用例集合。
type UseCase interface {
	GetAccount(userID uint) (*pointsdomain.Account, error)
	GetBalancesByUserIDs(userIDs []uint) (map[uint]int64, error)
	ListTransactions(filter TransactionListFilter) ([]pointsdomain.Transaction, int64, error)

	Credit(input CreditInput) (*pointsdomain.Account, *pointsdomain.Transaction, error)
	CreditInTransaction(tx Transaction, input CreditInput) (*pointsdomain.Account, *pointsdomain.Transaction, error)
	Debit(input DebitInput) (*pointsdomain.Account, *pointsdomain.Transaction, error)
	AdminAdjust(input AdjustInput) (*pointsdomain.Account, *pointsdomain.Transaction, error)
	// DebitOrderPoints 在调用方已开启的事务内扣减订单所需积分。
	// 绝不复用 Debit —— Debit 走 changeBalance，内部自开事务，
	// 在订单事务里调用会导致积分脱离订单事务、且在 SQLite 单连接池下自锁。
	DebitOrderPoints(tx Transaction, input OrderPointsInput) (int64, error)

	Checkin(userID uint) (*CheckinResult, error)
	CheckinStatus(userID uint) (*CheckinStatus, error)

	GetConfig() (*pointsdomain.Config, error)
	UpdateConfig(input ConfigInput) (*pointsdomain.Config, error)
}
