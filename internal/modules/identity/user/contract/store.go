package usercontract

import (
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/shopspring/decimal"
)

// ListFilter describes the administration user-directory query.
type ListFilter struct {
	Page          int
	PageSize      int
	UserID        uint
	Keyword       string
	Status        string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	LastLoginFrom *time.Time
	LastLoginTo   *time.Time
	SortBy        string
	SortOrder     string
}

// Store is the persistence port for user accounts and their authentication state.
type Store interface {
	GetByEmail(string) (*userdomain.User, error)
	GetByID(uint) (*userdomain.User, error)
	ListByIDs([]uint) ([]userdomain.User, error)
	Create(*userdomain.User) error
	Update(*userdomain.User) error
	IncrementTotalRecharged(uint, decimal.Decimal) error
	IncrementTotalSpent(uint, decimal.Decimal) error
	UpdateMemberLevelIfCurrent(uint, uint, uint) (int64, error)
	List(ListFilter) ([]userdomain.User, int64, error)
	BatchUpdateStatus([]uint, string) error
	AssignDefaultMemberLevel(uint) (int64, error)
	UpdateTOTPPending(uint, string, time.Time) error
	UpdateTOTPEnabled(uint, string, time.Time, string) error
	UpdateRecoveryCodes(uint, string) error
	ClearTOTP(uint) error
	// ExpireMemberships 把已到期的付费会员清档，返回受影响行数。
	// 用一条 UPDATE 完成，避免把全部过期用户读进内存。
	ExpireMemberships(now time.Time) (int64, error)
	// ListMembershipsExpiringBetween 返回到期时间落在 [from, to) 的付费会员，
	// 供到期前提醒使用。
	ListMembershipsExpiringBetween(from, to time.Time, limit int) ([]userdomain.User, error)
}
