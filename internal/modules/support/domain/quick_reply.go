package domain

import "time"

// QuickReply 是客服在回复页面维护的"常见问题"。
// Title 是买家侧展示并发送的问题；买家发出与 Title 一致的文本时，
// Content 会作为客服消息自动回复。Content 为空则只作快捷提问、不自动回复。
type QuickReply struct {
	ID    uint   `gorm:"primarykey" json:"id"`
	Title string `gorm:"type:varchar(120);not null" json:"title"`
	// Content 是对应解决方案：买家问到该标题时自动回复，客服点击条目时填入输入框。
	Content   string    `gorm:"type:text" json:"content"`
	IsActive  bool      `gorm:"not null;default:true;index" json:"is_active"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `gorm:"index" json:"updated_at"`
}

func (QuickReply) TableName() string {
	return "support_quick_replies"
}
