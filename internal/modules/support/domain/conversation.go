package domain

import "time"

// 会话状态。需求明确"会话不会自动关闭"，因此没有超时关闭逻辑；
// closed 只能由客服显式标记，且买家再次留言会自动恢复为 open。
const (
	ConversationStatusOpen   = "open"
	ConversationStatusClosed = "closed"
)

// Conversation 是买家与客服之间的持久会话，每个用户一条。
type Conversation struct {
	ID     uint   `gorm:"primarykey" json:"id"`
	UserID uint   `gorm:"uniqueIndex;not null" json:"user_id"`
	Status string `gorm:"type:varchar(16);not null;default:'open';index" json:"status"`
	// LastMessageAt 用于工作台按最新活跃排序。
	LastMessageAt *time.Time `gorm:"index" json:"last_message_at"`
	// LastMessagePreview 是最后一条消息的纯文本摘要，避免工作台列表为每行再查一次消息表。
	LastMessagePreview string `gorm:"type:varchar(160)" json:"last_message_preview"`
	// UnreadForAgent / UnreadForUser 分别是两侧的未读数。
	// 存计数而不是每次 COUNT：工作台列表是高频轮询接口。
	UnreadForAgent int       `gorm:"not null;default:0" json:"unread_for_agent"`
	UnreadForUser  int       `gorm:"not null;default:0" json:"unread_for_user"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time `gorm:"index" json:"updated_at"`
}

func (Conversation) TableName() string {
	return "support_conversations"
}
