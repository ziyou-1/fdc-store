package domain

import "time"

// Transaction 积分流水。只追加，不修改。
type Transaction struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	UserID          uint      `gorm:"index;not null" json:"user_id"`
	OperatorAdminID *uint     `gorm:"index" json:"operator_admin_id,omitempty"`
	OrderID         *uint     `gorm:"index" json:"order_id,omitempty"`
	Type            string    `gorm:"type:varchar(40);index;not null" json:"type"`
	Direction       string    `gorm:"type:varchar(16);index;not null" json:"direction"`
	Amount          int64     `gorm:"not null" json:"amount"`
	BalanceBefore   int64     `gorm:"not null;default:0" json:"balance_before"`
	BalanceAfter    int64     `gorm:"not null;default:0" json:"balance_after"`
	Reference       string    `gorm:"type:varchar(120);uniqueIndex" json:"reference"`
	Remark          string    `gorm:"type:varchar(255)" json:"remark"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time `gorm:"index" json:"updated_at"`
}

func (Transaction) TableName() string {
	return "points_transactions"
}
