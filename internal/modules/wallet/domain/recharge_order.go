package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// RechargeOrder 钱包充值支付单。
type RechargeOrder struct {
	ID              uint         `gorm:"primarykey" json:"id"`
	RechargeNo      string       `gorm:"type:varchar(40);uniqueIndex;not null" json:"recharge_no"`
	UserID          uint         `gorm:"index;not null" json:"user_id"`
	PaymentID       uint         `gorm:"uniqueIndex;not null" json:"payment_id"`
	ChannelID       uint         `gorm:"index;not null" json:"channel_id"`
	ProviderType    string       `gorm:"type:varchar(32);not null" json:"provider_type"`
	ChannelType     string       `gorm:"type:varchar(32);not null" json:"channel_type"`
	InteractionMode string       `gorm:"type:varchar(32);not null" json:"interaction_mode"`
	Amount          money.Amount `gorm:"type:decimal(20,2);not null" json:"amount"`
	PayableAmount   money.Amount `gorm:"type:decimal(20,2);not null" json:"payable_amount"`
	FeeRate         money.Amount `gorm:"type:decimal(6,2);not null;default:0" json:"fee_rate"`
	FeeAmount       money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"fee_amount"`
	Currency        string       `gorm:"type:varchar(16);not null;default:'CNY'" json:"currency"`
	Status          string       `gorm:"type:varchar(20);index;not null" json:"status"`
	Remark          string       `gorm:"type:varchar(255)" json:"remark"`
	// CreditTarget 到账目标：wallet（余额，默认）或 points（积分）。
	// 复用同一张充值单表换来整条超时/幂等/对账链路零改动。
	CreditTarget string `gorm:"type:varchar(16);not null;default:'wallet';index" json:"credit_target"`
	// PointsAmount 建单时快照的到账积分；回调时绝不重读配置，避免换算比例中途被改。
	PointsAmount int64 `gorm:"not null;default:0" json:"points_amount"`
	// PointsPerUnit 建单时快照的换算系数，仅用于展示与对账追溯。
	PointsPerUnit int64 `gorm:"not null;default:0" json:"points_per_unit"`
	// MembershipPeriod 会员购买快照：month / quarter / year。
	MembershipPeriod string `gorm:"type:varchar(16);not null;default:''" json:"membership_period"`
	// MembershipDurationDays 建单时快照的有效天数，回调按此入账，不重读套餐。
	MembershipDurationDays int `gorm:"not null;default:0" json:"membership_duration_days"`
	// MembershipDiscountRate 建单时快照的折扣率。
	MembershipDiscountRate money.Amount `gorm:"type:decimal(6,2);not null;default:0" json:"membership_discount_rate"`
	PaidAt                 *time.Time   `gorm:"index" json:"paid_at"`
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"index" json:"updated_at"`
	DeletedAt     *time.Time `gorm:"index" json:"-"`
}

func (RechargeOrder) TableName() string {
	return "wallet_recharge_orders"
}
