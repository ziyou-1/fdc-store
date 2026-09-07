package application

import (
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

	now := time.Now()
	message := &supportdomain.Message{
		ConversationID: conversation.ID,
		SenderType:     input.SenderType,
		SenderID:       input.SenderID,
		ContentType:    contentType,
		Content:        strings.TrimSpace(input.Content),
		Translation:    strings.TrimSpace(input.Translation),
		MediaURL:       strings.TrimSpace(input.MediaURL),
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
func (s *Service) ListUserMessages(userID uint, afterID uint, page, pageSize int) (
	*supportdomain.Conversation, []supportdomain.Message, int64, error,
) {
	conversation, err := s.EnsureConversation(userID)
	if err != nil {
		return nil, nil, 0, err
	}
	messages, total, err := s.repo.ListMessages(supportcontract.MessageListFilter{
		ConversationID: conversation.ID,
		AfterID:        afterID,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		return nil, nil, 0, err
	}
	return conversation, messages, total, nil
}

// ListAgentMessages 客服拉取指定会话的消息。
func (s *Service) ListAgentMessages(conversationID, afterID uint, page, pageSize int) (
	*supportdomain.Conversation, []supportdomain.Message, int64, error,
) {
	conversation, err := s.repo.GetConversationByID(conversationID)
	if err != nil {
		return nil, nil, 0, err
	}
	if conversation == nil {
		return nil, nil, 0, supportcontract.ErrConversationNotFound
	}
	messages, total, err := s.repo.ListMessages(supportcontract.MessageListFilter{
		ConversationID: conversationID,
		AfterID:        afterID,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		return nil, nil, 0, err
	}
	return conversation, messages, total, nil
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
