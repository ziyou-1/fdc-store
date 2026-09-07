package domain

import "time"

// Claim 免费领取记录。(user_id, claim_date) 唯一索引保证每人每天只能领一次，
// 并发下由数据库唯一约束兜底，不依赖应用层的先查后写。
type Claim struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uk_free_claim_user_date;not null" json:"user_id"`
	ClaimDate string    `gorm:"type:varchar(10);uniqueIndex:uk_free_claim_user_date;not null" json:"claim_date"`
	ProductID uint      `gorm:"index;not null" json:"product_id"`
	SKUID     uint      `gorm:"column:sku_id;index;not null" json:"sku_id"`
	OrderID   uint      `gorm:"index;not null" json:"order_id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (Claim) TableName() string {
	return "free_claim_records"
}
