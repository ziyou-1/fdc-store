package domain

import "time"

// 消息发送方。
const (
	SenderTypeUser  = "user"
	SenderTypeAgent = "agent"
)

// 消息内容类型。
const (
	ContentTypeText  = "text"
	ContentTypeImage = "image"
	ContentTypeVideo = "video"
)

// Message 是一条客服会话消息。只追加，不修改。
type Message struct {
	ID             uint   `gorm:"primarykey" json:"id"`
	ConversationID uint   `gorm:"index:idx_support_msg_conv_id,priority:1;not null" json:"conversation_id"`
	SenderType     string `gorm:"type:varchar(16);not null" json:"sender_type"`
	// SenderID 是发送方的用户 ID 或管理员 ID，按 SenderType 解释。
	SenderID    uint   `gorm:"index" json:"sender_id"`
	ContentType string `gorm:"type:varchar(16);not null;default:'text'" json:"content_type"`
	// Content 文本内容；媒体消息可为空。买家原文，不做改写。
	Content string `gorm:"type:text" json:"content"`
	// Translation 客服可见的自动译文（DeepL，目标简体中文）。媒体或未开启翻译时为空。
	Translation string `gorm:"type:text" json:"translation"`
	// MediaURL 图片或视频地址，文本消息为空。
	MediaURL  string    `gorm:"type:varchar(512)" json:"media_url"`
	CreatedAt time.Time `gorm:"index:idx_support_msg_conv_id,priority:2;index" json:"created_at"`
}

func (Message) TableName() string {
	return "support_messages"
}

// IsMediaContentType 判断内容类型是否为媒体。
func IsMediaContentType(contentType string) bool {
	return contentType == ContentTypeImage || contentType == ContentTypeVideo
}
