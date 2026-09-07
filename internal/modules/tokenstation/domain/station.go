package domain

import "time"

// Station 是别人网页 token 中转站的一条跳转链接。
type Station struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	BaseURL   string    `gorm:"type:varchar(500);not null" json:"base_url"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Station) TableName() string {
	return "token_stations"
}
