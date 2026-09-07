package domain

import "time"

// Account 用户积分账户。积分为整数，不参与货币运算。
type Account struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	Balance     int64     `gorm:"not null;default:0" json:"balance"`
	TotalEarned int64     `gorm:"not null;default:0" json:"total_earned"`
	TotalSpent  int64     `gorm:"not null;default:0" json:"total_spent"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `gorm:"index" json:"updated_at"`
}

func (Account) TableName() string {
	return "points_accounts"
}
