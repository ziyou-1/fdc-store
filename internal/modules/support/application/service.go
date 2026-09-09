package application

import (
	"errors"
	"fmt"
	"strings"
	"time"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
)

// previewMaxRunes 是会话列表里最后一条消息的摘要长度。
const previewMaxRunes = 60

// Service 实现客服会话用例。
type Service struct {
	repo              supportcontract.Repository
	users             supportcontract.UserReader
	translateUserText func(string) (string, error)
}

// NewService 构造客服服务。
func NewService(repo supportcontract.Repository) *Service {
	return &Service{repo: repo}
}

// SetUserReader 注入买家标识读取端口，供工作台展示邮箱。
func (s *Service) SetUserReader(users supportcontract.UserReader) {
	if s == nil {
		return
	}
	s.users = users
}

// SetUserTextTranslator 注入买家文本翻译。nil 表示未配置，不翻译。
func (s *Service) SetUserTextTranslator(fn func(string) (string, error)) {
	if s == nil {
		return
	}
	s.translateUserText = fn
}

// EnsureConversation 返回买家的会话，不存在则创建。
// 每个用户恒定一条会话——需求要求会话不自动关闭、随时可继续留言，
// 因此不做"一次咨询一个工单"的模型，避免买家回来找不到上次的上下文。
func (s *Service) EnsureConversation(userID uint) (*supportdomain.Conversation, error) {
	if userID == 0 {
		return nil, supportcontract.ErrGuestForbidden
	}
	existing, err := s.repo.GetConversationByUserID(userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	now := time.Now()
	conversation := &supportdomain.Conversation{
		UserID:    userID,
		Status:    supportdomain.ConversationStatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateConversation(conversation); err != nil {
		return nil, supportcontract.ErrCreateFailed
	}
	return conversation, nil
}

// SendUserMessage 买家发送消息。文本与启用中的常见问题标题一致时，追加一条客服解决方案。
func (s *Service) SendUserMessage(userID uint, input supportcontract.SendMessageInput) (*supportdomain.Message, error) {
	if userID == 0 {
		return nil, supportcontract.ErrGuestForbidden
	}
	conversation, err := s.EnsureConversation(userID)
	if err != nil {
		return nil, err
	}
	input.ConversationID = conversation.ID
	input.SenderType = supportdomain.SenderTypeUser
	input.SenderID = userID

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = supportdomain.ContentTypeText
	}
	var replies []supportdomain.QuickReply
	if contentType == supportdomain.ContentTypeText {
		if strings.TrimSpace(input.Content) == "" {
			return nil, supportcontract.ErrMessageEmpty
		}
		replies, err = s.repo.ListQuickReplies(true)
		if err != nil {
			return nil, err
		}
		if s.translateUserText != nil {
			translated, translateErr := s.translateUserText(strings.TrimSpace(input.Content))
			if translateErr != nil {
				return nil, fmt.Errorf("%w: %v", supportcontract.ErrTranslateFailed, translateErr)
			}
			translated = strings.TrimSpace(translated)
			if translated == "" {
				return nil, fmt.Errorf("%w: empty translation", supportcontract.ErrTranslateFailed)
			}
			input.Translation = translated
		}
	}

	userMessage, err := s.appendMessage(conversation, input)
	if err != nil {
		return nil, err
	}
	if solution := matchFAQSolution(input.Content, replies); solution != "" {
		if _, err := s.appendMessage(conversation, supportcontract.SendMessageInput{
			ConversationID: conversation.ID,
			SenderType:     supportdomain.SenderTypeAgent,
			ContentType:    supportdomain.ContentTypeText,
			Content:        solution,
		}); err != nil {
			return nil, err
		}
	}
	return userMessage, nil
}

// SendAgentMessage 客服回复。
func (s *Service) SendAgentMessage(adminID, conversationID uint, input supportcontract.SendMessageInput) (*supportdomain.Message, error) {
	conversation, err := s.repo.GetConversationByID(conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, supportcontract.ErrConversationNotFound
	}
	input.ConversationID = conversation.ID
	input.SenderType = supportdomain.SenderTypeAgent
	input.SenderID = adminID
	return s.appendMessage(conversation, input)
}

// RevokeAgentMessage 撤回客服自己发的消息。
func (s *Service) RevokeAgentMessage(messageID, agentID uint) (*supportdomain.Message, error) {
	return s.revokeMessage(messageID, supportdomain.SenderTypeAgent, agentID, 0)
}

// RevokeUserMessage 撤回买家自己发的消息。
//
// 除了「是本人发的」，还要确认这条消息确实落在该买家自己的会话里。
// 光比 sender_id 不够：买家 ID 只在自己的会话里有意义，跨会话引用同一个数字
// 就能撤到别人对话里的消息。
func (s *Service) RevokeUserMessage(messageID, userID uint) (*supportdomain.Message, error) {
	if userID == 0 {
		return nil, supportcontract.ErrGuestForbidden
	}
	conversation, err := s.EnsureConversation(userID)
	if err != nil {
		return nil, err
	}
	return s.revokeMessage(messageID, supportdomain.SenderTypeUser, userID, conversation.ID)
}

// revokeMessage 是撤回的唯一实现。
//
// 只打标记不删行：这张表的约定是只追加，行留着才有审计痕迹，
// 对端也才知道「这里原本有条消息」而不是凭空少一条。
//
// 撤回不回退会话的未读计数。对方很可能已经读到了，硬把未读数减回去
// 只会让计数与对方实际看到的对不上——宁可计数偏保守。
//
// requireConversationID 非 0 时额外校验消息归属；客服本就能看全部会话，传 0。
func (s *Service) revokeMessage(
	messageID uint,
	senderType string,
	senderID uint,
	requireConversationID uint,
) (*supportdomain.Message, error) {
	if messageID == 0 || senderID == 0 {
		return nil, supportcontract.ErrMessageNotFound
	}
	message, err := s.repo.GetMessageByID(messageID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, supportcontract.ErrMessageNotFound
	}
	if requireConversationID != 0 && message.ConversationID != requireConversationID {
		// 不区分「不存在」与「不是你的」：区分开就能拿它探别人会话里有没有某条消息。
		return nil, supportcontract.ErrMessageNotFound
	}
	now := time.Now()
	if !message.CanRevokeBy(senderType, senderID, now) {
		return nil, supportcontract.ErrRevokeNotAllowed
	}
	if err := s.repo.RevokeMessage(messageID, now); err != nil {
		// 仓储的条件更新影响 0 行说明期间已被撤回，按不允许处理。
		if errors.Is(err, supportcontract.ErrMessageNotFound) {
			return nil, supportcontract.ErrRevokeNotAllowed
		}
		return nil, err
	}
	message.RevokedAt = &now
	redacted := message.Redacted()
	return &redacted, nil
}

// appendMessage 落一条消息并同步会话上的冗余字段。
func (s *Service) appendMessage(
	conversation *supportdomain.Conversation,
	input supportcontract.SendMessageInput,
) (*supportdomain.Message, error) {
	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = supportdomain.ContentTypeText
	}
	switch contentType {
	case supportdomain.ContentTypeText:
		if strings.TrimSpace(input.Content) == "" {
			return nil, supportcontract.ErrMessageEmpty
		}
	case supportdomain.ContentTypeImage, supportdomain.ContentTypeVideo:
		if strings.TrimSpace(input.MediaURL) == "" {
			return nil, supportcontract.ErrMediaURLRequired
		}
	default:
		return nil, supportcontract.ErrContentTypeInvalid
	}

	// 引用必须指向同一会话里的真实消息，否则丢弃。
	// 允许跨会话引用等于把别人的对话内容搬进这个会话。
	replyToID := input.ReplyToID
	if replyToID != 0 {
		referenced, err := s.repo.GetMessageByID(replyToID)
		if err != nil {
			return nil, err
		}
		if referenced == nil || referenced.ConversationID != conversation.ID {
			replyToID = 0
		}
	}

	now := time.Now()
	message := &supportdomain.Message{
		ConversationID: conversation.ID,
		SenderType:     input.SenderType,
		SenderID:       input.SenderID,
		ContentType:    contentType,
		Content:        strings.TrimSpace(input.Content),
		Translation:    strings.TrimSpace(input.Translation),
		MediaURL:       strings.TrimSpace(input.MediaURL),
		ReplyToID:      replyToID,
		CreatedAt:      now,
	}
	if err := s.repo.CreateMessage(message); err != nil {
		return nil, supportcontract.ErrCreateFailed
	}

	conversation.LastMessageAt = &now
	conversation.LastMessagePreview = buildPreview(contentType, message.Content)
	conversation.UpdatedAt = now
	if input.SenderType == supportdomain.SenderTypeUser {
		conversation.UnreadForAgent++
		// 买家再次留言会把已关闭的会话重新打开：需求要求会话不终结，
		// 客服关闭只是"我这边处理完了"，不是"你不能再说话了"。
		conversation.Status = supportdomain.ConversationStatusOpen
	} else {
		conversation.UnreadForUser++
	}
	if err := s.repo.UpdateConversation(conversation); err != nil {
		return nil, supportcontract.ErrUpdateFailed
	}
	return message, nil
}

// ListUserMessages 买家拉取自己会话的消息。
func (s *Service) ListUserMessages(
	userID uint, afterID uint, page, pageSize int, revokedSince time.Time,
) (*supportcontract.MessageListResult, error) {
	conversation, err := s.EnsureConversation(userID)
	if err != nil {
		return nil, err
	}
	return s.listMessages(conversation, afterID, page, pageSize, revokedSince)
}

// ListAgentMessages 客服拉取指定会话的消息。
func (s *Service) ListAgentMessages(
	conversationID, afterID uint, page, pageSize int, revokedSince time.Time,
) (*supportcontract.MessageListResult, error) {
	conversation, err := s.repo.GetConversationByID(conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, supportcontract.ErrConversationNotFound
	}
	return s.listMessages(conversation, afterID, page, pageSize, revokedSince)
}

// listMessages 是两侧共用的读取实现。
//
// 撤回单独查一次而不是混进 Messages：混进去的话，前端那句
// 「本轮有新消息就标记已读 / 滚到底部」会被一条几分钟前的撤回反复触发。
// 撤回是状态变更，不是新消息，两者必须分开回传。
func (s *Service) listMessages(
	conversation *supportdomain.Conversation,
	afterID uint,
	page, pageSize int,
	revokedSince time.Time,
) (*supportcontract.MessageListResult, error) {
	messages, total, err := s.repo.ListMessages(supportcontract.MessageListFilter{
		ConversationID: conversation.ID,
		AfterID:        afterID,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		return nil, err
	}
	revoked, err := s.repo.ListRevokedSince(conversation.ID, revokedSince, supportcontract.RevokedFeedLimit)
	if err != nil {
		return nil, err
	}
	result := &supportcontract.MessageListResult{
		Conversation: conversation,
		Messages:     supportdomain.RedactRevoked(messages),
		Total:        total,
		Revoked:      supportdomain.RedactRevoked(revoked),
	}
	// 游标取本次最新的撤回时间（查询按 revoked_at 倒序，首条即最新）；
	// 本轮没有撤回就把请求带来的下界原样还回去，免得前端把游标退回零。
	switch {
	case len(revoked) > 0 && revoked[0].RevokedAt != nil:
		result.RevokedCursor = revoked[0].RevokedAt
	case !revokedSince.IsZero():
		since := revokedSince
		result.RevokedCursor = &since
	}
	return result, nil
}

// MarkReadByUser 买家已读，清零买家侧未读。
func (s *Service) MarkReadByUser(userID uint) error {
	conversation, err := s.EnsureConversation(userID)
	if err != nil {
		return err
	}
	if conversation.UnreadForUser == 0 {
		return nil
	}
	conversation.UnreadForUser = 0
	conversation.UpdatedAt = time.Now()
	return s.repo.UpdateConversation(conversation)
}

// MarkReadByAgent 客服已读，清零客服侧未读。
func (s *Service) MarkReadByAgent(conversationID uint) error {
	conversation, err := s.repo.GetConversationByID(conversationID)
	if err != nil {
		return err
	}
	if conversation == nil {
		return supportcontract.ErrConversationNotFound
	}
	if conversation.UnreadForAgent == 0 {
		return nil
	}
	conversation.UnreadForAgent = 0
	conversation.UpdatedAt = time.Now()
	return s.repo.UpdateConversation(conversation)
}

// SetConversationStatus 客服显式开关会话。买家再次留言会自动恢复为 open。
func (s *Service) SetConversationStatus(conversationID uint, status string) (*supportdomain.Conversation, error) {
	conversation, err := s.repo.GetConversationByID(conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, supportcontract.ErrConversationNotFound
	}
	normalized := strings.TrimSpace(status)
	if normalized != supportdomain.ConversationStatusOpen &&
		normalized != supportdomain.ConversationStatusClosed {
		return nil, supportcontract.ErrUpdateFailed
	}
	conversation.Status = normalized
	conversation.UpdatedAt = time.Now()
	if err := s.repo.UpdateConversation(conversation); err != nil {
		return nil, supportcontract.ErrUpdateFailed
	}
	return conversation, nil
}

// ListConversations 工作台会话列表，附带买家邮箱。
func (s *Service) ListConversations(filter supportcontract.ConversationListFilter) (
	[]supportcontract.ConversationView, int64, error,
) {
	conversations, total, err := s.repo.ListConversations(filter)
	if err != nil {
		return nil, 0, err
	}
	views := make([]supportcontract.ConversationView, 0, len(conversations))
	emails := map[uint]string{}
	if s.users != nil && len(conversations) > 0 {
		ids := make([]uint, 0, len(conversations))
		for index := range conversations {
			ids = append(ids, conversations[index].UserID)
		}
		if resolved, emailErr := s.users.GetEmailsByIDs(ids); emailErr == nil {
			emails = resolved
		}
	}
	for index := range conversations {
		views = append(views, supportcontract.ConversationView{
			Conversation: conversations[index],
			UserEmail:    emails[conversations[index].UserID],
		})
	}
	return views, total, nil
}

// CountUnreadConversations 工作台角标：有未读的会话数。
func (s *Service) CountUnreadConversations() (int64, error) {
	return s.repo.CountUnreadConversations()
}

func buildPreview(contentType, content string) string {
	switch contentType {
	case supportdomain.ContentTypeImage:
		return "[图片]"
	case supportdomain.ContentTypeVideo:
		return "[视频]"
	}
	trimmed := strings.TrimSpace(content)
	runes := []rune(trimmed)
	if len(runes) > previewMaxRunes {
		return string(runes[:previewMaxRunes])
	}
	return trimmed
}
