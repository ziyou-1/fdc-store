package contract

import (
	"context"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type ListFilter struct {
	IsActive *bool
	Page     int
	PageSize int
}

type LevelRepository interface {
	GetByID(id uint) (*memberleveldomain.MemberLevel, error)
	GetBySlug(slug string) (*memberleveldomain.MemberLevel, error)
	GetDefault() (*memberleveldomain.MemberLevel, error)
	GetActiveBySortOrder(sortOrder int, excludeID uint) (*memberleveldomain.MemberLevel, error)
	ListAllActive() ([]memberleveldomain.MemberLevel, error)
	Create(level *memberleveldomain.MemberLevel) error
	Update(level *memberleveldomain.MemberLevel) error
	Delete(id uint) error
	List(filter ListFilter) ([]memberleveldomain.MemberLevel, int64, error)
	ClearDefault(excludeID uint) error
}

type PriceRepository interface {
	GetByID(id uint) (*memberleveldomain.MemberLevelPrice, error)
	GetByLevelAndProductAndSKU(levelID, productID, skuID uint) (*memberleveldomain.MemberLevelPrice, error)
	ListByProduct(productID uint) ([]memberleveldomain.MemberLevelPrice, error)
	ListByLevelAndProducts(levelID uint, productIDs []uint) ([]memberleveldomain.MemberLevelPrice, error)
	BatchUpsert(prices []memberleveldomain.MemberLevelPrice) error
	Delete(id uint) error
	DeleteByProduct(productID uint) error
}

// UserRepository is the member-level consumer's minimal user persistence port.
type UserRepository interface {
	GetByID(id uint) (*userdomain.User, error)
	Update(user *userdomain.User) error
	IncrementTotalRecharged(userID uint, amount decimal.Decimal) error
	IncrementTotalSpent(userID uint, amount decimal.Decimal) error
	UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error)
	AssignDefaultMemberLevel(defaultLevelID uint) (int64, error)
	// ExpireMemberships 清掉已到期的付费会员档位，返回受影响行数。
	ExpireMemberships(now time.Time) (int64, error)
	// ListMembershipsExpiringBetween 返回到期时间落在 [from, to) 的付费会员。
	ListMembershipsExpiringBetween(from, to time.Time, limit int) ([]userdomain.User, error)
	// ClaimMembershipExpiryReminder 用一条条件 UPDATE 抢占「给该到期时刻发提醒」的资格。
	// 返回 true 表示本次抢到，调用方才可以发信；并发下只会有一个调用者拿到 true。
	// 窗口参数用于防止「扫描之后、抢占之前」用户刚好续费导致误发。
	ClaimMembershipExpiryReminder(userID uint, from, to time.Time) (bool, error)
	// ReleaseMembershipExpiryReminder 撤销抢占，供发信失败后让下一轮重试。
	ReleaseMembershipExpiryReminder(userID uint) error
}

type PlanRepository interface {
	GetByPeriod(period string) (*memberleveldomain.Plan, error)
	List() ([]memberleveldomain.Plan, error)
	Create(plan *memberleveldomain.Plan) error
	Update(plan *memberleveldomain.Plan) error
}

type PurchasePaymentInput struct {
	UserID        uint
	ChannelID     uint
	Amount        money.Amount
	Currency      string
	Remark        string
	ClientIP      string
	Context       context.Context
	RequestScheme string
	Period        string
	DurationDays  int
	DiscountRate  money.Amount
}

type PurchaseResult struct {
	RechargeNo      string       `json:"recharge_no"`
	PaymentID       uint         `json:"payment_id"`
	PayableAmount   money.Amount `json:"payable_amount"`
	Currency        string       `json:"currency"`
	Period          string       `json:"period"`
	RedirectURL     string       `json:"redirect_url"`
	QRCode          string       `json:"qr_code"`
	InteractionMode string       `json:"interaction_mode"`
}

type PurchasePaymentCreator interface {
	CreateMembershipPurchase(input PurchasePaymentInput) (*PurchaseResult, error)
}
