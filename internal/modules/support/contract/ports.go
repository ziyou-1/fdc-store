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
	// ListRevokedSince 取会话里 revoked_at >= since 的消息，按 revoked_at 倒序，最多 limit 条。
	// since 为零值表示不设下界。撤回不产生新行，AfterID 那条增量游标永远拉不到它，
	// 必须单开一条只走撤回时间的游标，否则对端只有整页重载才知道消息被撤了。
	ListRevokedSince(conversationID uint, since time.Time, limit int) ([]supportdomain.Message, error)

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

// RevokedFeedLimit 是一次回传的撤回条数上限。
// 撤回本身罕见，取 50 足够覆盖任何合理的离线时长；真超了下一轮游标会接着往前走。
const RevokedFeedLimit = 50

// MessageListResult 是一次消息拉取的完整结果。
//
// 用结构体而不是四个返回值，是因为撤回必须单独回传：撤回只改 revoked_at 不新增行，
// 按 AfterID 增量拉取的 Messages 里永远不会出现它。
type MessageListResult struct {
	Conversation *supportdomain.Conversation `json:"conversation"`
	Messages     []supportdomain.Message     `json:"messages"`
	Total        int64                       `json:"total"`
	// Revoked 是自请求带来的游标以来被撤回的消息，内容已抹掉，只用于就地打标。
	// 前端拿它「只更新已有的、不新增」——否则会凭空插进一条自己从没收到过的消息。
	Revoked []supportdomain.Message `json:"revoked"`
	// RevokedCursor 是本次回传里最新的撤回时间，前端原样带回来做下一轮的下界。
	// 游标由服务端产出、前端回显，因此不受两边时钟不一致影响。
	RevokedCursor *time.Time `json:"revoked_cursor"`
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
