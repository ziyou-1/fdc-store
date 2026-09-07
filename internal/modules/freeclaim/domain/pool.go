package domain

import "time"

// PoolItem 免费领取商品池条目。后台维护，每天由系统从中选出一个对全站展示。
type PoolItem struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	ProductID uint      `gorm:"index;not null" json:"product_id"`
	SKUID     uint      `gorm:"column:sku_id;index;not null" json:"sku_id"`
	IsActive  bool      `gorm:"not null;default:true;index" json:"is_active"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `gorm:"index" json:"updated_at"`
}

func (PoolItem) TableName() string {
	return "free_claim_pool_items"
}
