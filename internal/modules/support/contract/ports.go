package contract

import (
	"time"

	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
)

// Repository 承担客服会话聚合的全部持久化。
type Repository interface {
	GetConversationByUserID(userID uint) (*supportdomain.Conversation, error)
	GetConversationByID(id uint) (*supportdomain.Conversation, error)
	CreateConversation(conversation *supportdomain.Conversation) error
	UpdateConversation(conversation *supportdomain.Conversation) error
	ListConversations(filter ConversationListFilter) ([]supportdomain.Conversation, int64, error)
	CountUnreadConversations() (int64, error)

	CreateMessage(message *supportdomain.Message) error
	ListMessages(filter MessageListFilter) ([]supportdomain.Message, int64, error)
	GetMessageByID(id uint) (*supportdomain.Message, error)
	// RevokeMessage 只写 revoked_at，不动内容——撤回是标记不是删除。
	RevokeMessage(id uint, revokedAt time.Time) error

	ListQuickReplies(activeOnly bool) ([]supportdomain.QuickReply, error)
	GetQuickReplyByID(id uint) (*supportdomain.QuickReply, error)
	CreateQuickReply(reply *supportdomain.QuickReply) error
	UpdateQuickReply(reply *supportdomain.QuickReply) error
	DeleteQuickReply(id uint) error
}

// ConversationListFilter 工作台会话列表查询条件。
type ConversationListFilter struct {
	Page     int
	PageSize int
	Status   string
	// UnreadOnly 只看有未读的会话，客服日常主要用这个视图。
	UnreadOnly bool
	UserID     uint
}

// MessageListFilter 消息查询条件。
// AfterID 是增量拉取游标：前端轮询时只取比已有最大 ID 更新的消息，
// 既避免整段重传，也天然免疫分页在新消息插入时的错位。
type MessageListFilter struct {
	ConversationID uint
	AfterID        uint
	Page           int
	PageSize       int
}

// SendMessageInput 发送消息入参。
type SendMessageInput struct {
	ConversationID uint
	SenderType     string
	SenderID       uint
	ContentType    string
	Content        string
	Translation    string
	MediaURL       string
	// ReplyToID 引用的消息 ID，0 表示普通消息。
	ReplyToID uint
}

// QuickReplyInput 常见问题写入入参。
type QuickReplyInput struct {
	Title     string
	Content   string
	IsActive  bool
	SortOrder int
}

// ConversationView 是工作台列表项，附带买家标识便于客服识别。
type ConversationView struct {
	supportdomain.Conversation
	UserEmail string `json:"user_email"`
}

// UserReader 读取买家标识，供工作台展示。
type UserReader interface {
	GetEmailsByIDs(ids []uint) (map[uint]string, error)
}

// Clock 便于测试注入时间。
type Clock interface {
	Now() time.Time
}
