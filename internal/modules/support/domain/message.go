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
	MediaURL string `gorm:"type:varchar(512)" json:"media_url"`
	// ReplyToID 引用的消息 ID，0 表示不是引用回复。
	// 只存 ID 不冗余内容：被引用消息若被撤回，引用块要跟着变成「已撤回」，
	// 冗余一份快照就会留下一段撤不掉的原文。
	ReplyToID uint `gorm:"index" json:"reply_to_id"`
	// RevokedAt 撤回时间，nil 表示未撤回。
	// 撤回是软标记而不是删行：这张表的约定是只追加，行留着才有审计痕迹，
	// 买家侧也才知道「这里原本有条消息」而不是凭空少一条。
	RevokedAt *time.Time `json:"revoked_at"`
	CreatedAt time.Time  `gorm:"index:idx_support_msg_conv_id,priority:2;index" json:"created_at"`
}

// RevokeWindow 是消息可撤回的时限。
// 取 2 分钟与微信/Shopee 一致：够长到能救回手滑，短到买家基本已经看过。
const RevokeWindow = 2 * time.Minute

// IsRevoked 判断消息是否已撤回。
func (m Message) IsRevoked() bool {
	return m.RevokedAt != nil
}

// CanRevokeBy 判断某个客服此刻能否撤回这条消息。
// 三个条件缺一不可：必须是客服自己发的、必须还没撤过、必须在时限内。
// 买家消息一律不可撤——客服撤掉买家说过的话等于篡改证据。
func (m Message) CanRevokeBy(agentID uint, now time.Time) bool {
	if m.SenderType != SenderTypeAgent || m.SenderID != agentID {
		return false
	}
	if m.IsRevoked() {
		return false
	}
	return now.Sub(m.CreatedAt) <= RevokeWindow
}

func (Message) TableName() string {
	return "support_messages"
}

// IsMediaContentType 判断内容类型是否为媒体。
func IsMediaContentType(contentType string) bool {
	return contentType == ContentTypeImage || contentType == ContentTypeVideo
}
