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
	// 建索引是因为撤回要走独立的增量游标：两端每 5 秒按 conversation_id + revoked_at
	// 查一次，没索引就是每轮对整个会话做前缀扫。
	RevokedAt *time.Time `gorm:"index" json:"revoked_at"`
	CreatedAt time.Time  `gorm:"index:idx_support_msg_conv_id,priority:2;index" json:"created_at"`
}

// RevokeWindow 是消息可撤回的时限。
// 取 2 分钟与微信/Shopee 一致：够长到能救回手滑，短到买家基本已经看过。
const RevokeWindow = 2 * time.Minute

// IsRevoked 判断消息是否已撤回。
func (m Message) IsRevoked() bool {
	return m.RevokedAt != nil
}

// CanRevokeBy 判断某个身份此刻能否撤回这条消息。
// 三个条件缺一不可：必须是本人发的、必须还没撤过、必须在时限内。
//
// senderType 必须一起传，不能只比 SenderID。买家 ID 与管理员 ID 是两套
// 独立自增序列，都存在 sender_id 这一列里，光比数字会让 5 号客服撤掉
// 5 号买家的消息。调用方只能传自己那一侧的常量，越权在类型层就没法表达。
func (m Message) CanRevokeBy(senderType string, senderID uint, now time.Time) bool {
	if senderType != SenderTypeUser && senderType != SenderTypeAgent {
		return false
	}
	if m.SenderType != senderType || m.SenderID != senderID {
		return false
	}
	if m.IsRevoked() {
		return false
	}
	return now.Sub(m.CreatedAt) <= RevokeWindow
}

// Redacted 返回一份把已撤回内容抹掉的副本，用于出网。
//
// 行留在库里是为了审计，不是为了还能再看见。不抹的话撤回只是「前端不渲染」，
// 对端翻一下网络面板就能读到原文——那等于没撤。
func (m Message) Redacted() Message {
	if !m.IsRevoked() {
		return m
	}
	m.Content = ""
	m.Translation = ""
	m.MediaURL = ""
	return m
}

// RedactRevoked 批量抹掉切片里已撤回消息的内容。
func RedactRevoked(messages []Message) []Message {
	for i := range messages {
		messages[i] = messages[i].Redacted()
	}
	return messages
}

func (Message) TableName() string {
	return "support_messages"
}

// IsMediaContentType 判断内容类型是否为媒体。
func IsMediaContentType(contentType string) bool {
	return contentType == ContentTypeImage || contentType == ContentTypeVideo
}
