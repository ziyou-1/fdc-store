package contract

import (
	"context"
	"time"

	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// 积分流水类型。
const (
	TxnTypeCheckin          = "checkin"
	TxnTypeReferralRegister = "referral_register"
	TxnTypeReferralOrder    = "referral_order"
	TxnTypeRecharge         = "recharge"
	TxnTypeExchange         = "exchange"
	TxnTypeExchangeRefund   = "exchange_refund"
	TxnTypeAdminAdjust      = "admin_adjust"
)

// 积分流水方向。
const (
	TxnDirectionIn  = "in"
	TxnDirectionOut = "out"
)

// TransactionListFilter 积分流水查询条件。
type TransactionListFilter struct {
	Page      int
	PageSize  int
	UserID    uint
	Type      string
	Direction string
}

// CreditInput 积分入账入参。Reference 用于幂等，重复调用只入账一次。
type CreditInput struct {
	UserID    uint
	Amount    int64
	Type      string
	Reference string
	Remark    string
	OrderID   *uint
}

// DebitInput 积分扣减入参。
type DebitInput struct {
	UserID    uint
	Amount    int64
	Type      string
	Reference string
	Remark    string
	OrderID   *uint
}

// AdjustInput 后台人工增减积分入参，Delta 可正可负。
type AdjustInput struct {
	UserID          uint
	OperatorAdminID *uint
	Delta           int64
	Remark          string
}

// ConfigInput 后台修改积分规则入参。全量覆盖语义：前端漏传即写零值。
type ConfigInput struct {
	CheckinEnabled              bool
	CheckinPoints               int64
	ReferralRegisterPoints      int64
	ReferralFirstOrderPoints    int64
	RechargePointsPerUnit       int64
	ExchangeEnabled             bool
	ExchangeDailyLimit          int64
	ExchangeSKUDailyLimit       int64
	RechargeEnabled             bool
	ReferralEnabled             bool
	ReferralDailyLimit          int64
	ReferralFirstOrderMinAmount money.Amount
}

// RechargeInput 用户发起积分充值的入参。
type RechargeInput struct {
	UserID        uint
	ChannelID     uint
	Amount        money.Amount
	Currency      string
	Remark        string
	ClientIP      string
	Context       context.Context
	RequestScheme string
	// PayWithBalance 为 true 时用钱包余额同步支付，忽略 ChannelID，无外部网关。
	PayWithBalance bool
}

// RechargePaymentInput 是 points 请求支付模块建单时传出的入参。
type RechargePaymentInput struct {
	UserID        uint
	ChannelID     uint
	Amount        money.Amount
	Currency      string
	Remark        string
	ClientIP      string
	Context       context.Context
	RequestScheme string
	PointsAmount  int64
	PointsPerUnit int64
	// PayWithBalance 为 true 时改走钱包余额同步支付路径。
	PayWithBalance bool
}

// RechargeResult 建单结果，字段刻意保持原始类型，避免 points 依赖支付模块的类型。
type RechargeResult struct {
	RechargeNo      string       `json:"recharge_no"`
	PaymentID       uint         `json:"payment_id"`
	PayableAmount   money.Amount `json:"payable_amount"`
	Currency        string       `json:"currency"`
	PointsAmount    int64        `json:"points_amount"`
	RedirectURL     string       `json:"redirect_url"`
	QRCode          string       `json:"qr_code"`
	InteractionMode string       `json:"interaction_mode"`
	// Status 便于前台区分"待支付(pending)"与钱包余额支付的"即时成功(success)"。
	Status string `json:"status"`
}

// RechargeRecord 积分充值记录的只读视图。字段保持原始类型，
// 避免 points 模块对钱包/支付领域类型产生依赖。
type RechargeRecord struct {
	RechargeNo    string       `json:"recharge_no"`
	Status        string       `json:"status"`
	Amount        money.Amount `json:"amount"`
	PayableAmount money.Amount `json:"payable_amount"`
	Currency      string       `json:"currency"`
	PointsAmount  int64        `json:"points_amount"`
	PointsPerUnit int64        `json:"points_per_unit"`
	PaidAt        *time.Time   `json:"paid_at"`
	CreatedAt     time.Time    `json:"created_at"`
}

// OrderPointsInput 订单积分抵扣入参。
type OrderPointsInput struct {
	OrderID    uint
	UserID     uint
	Points     int64
	DailyLimit int64
	Remark     string
}

// CheckinResult 签到结果。
type CheckinResult struct {
	Points     int64
	StreakDays int
	Balance    int64
	Date       string
}

// CheckinStatus 今日签到状态。
type CheckinStatus struct {
	CheckedIn     bool   `json:"checked_in"`
	Date          string `json:"date"`
	StreakDays    int    `json:"streak_days"`
	CheckinPoints int64  `json:"checkin_points"`
	Enabled       bool   `json:"enabled"`
	Balance       int64  `json:"balance"`
}

// Invite 买家可见的推荐入口。关闭时 Code 为空，打开发放邀请码与奖励额度。
type Invite struct {
	Enabled          bool
	Code             string
	RegisterPoints   int64
	FirstOrderPoints int64
}

// EmptyAccount 返回未开户用户的零值账户，避免上层判空。
func EmptyAccount(userID uint) *pointsdomain.Account {
	return &pointsdomain.Account{UserID: userID}
}
