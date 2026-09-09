package gormstore

import (
	"errors"
	"strings"
	"time"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"

	"gorm.io/gorm"
)

type Store struct{ db *gorm.DB }

var _ supportcontract.Repository = (*Store)(nil)

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) GetConversationByUserID(userID uint) (*supportdomain.Conversation, error) {
	if userID == 0 {
		return nil, nil
	}
	var conversation supportdomain.Conversation
	if err := s.db.Where("user_id = ?", userID).First(&conversation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

func (s *Store) GetConversationByID(id uint) (*supportdomain.Conversation, error) {
	if id == 0 {
		return nil, nil
	}
	var conversation supportdomain.Conversation
	if err := s.db.Where("id = ?", id).First(&conversation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

func (s *Store) CreateConversation(conversation *supportdomain.Conversation) error {
	if conversation == nil {
		return supportcontract.ErrCreateFailed
	}
	if err := s.db.Create(conversation).Error; err != nil {
		// user_id 唯一索引兜住并发建会话：两个请求同时进来只会成一条。
		if isDuplicateKey(err) {
			existing, queryErr := s.GetConversationByUserID(conversation.UserID)
			if queryErr == nil && existing != nil {
				*conversation = *existing
				return nil
			}
		}
		return err
	}
	return nil
}

func (s *Store) UpdateConversation(conversation *supportdomain.Conversation) error {
	if conversation == nil || conversation.ID == 0 {
		return supportcontract.ErrUpdateFailed
	}
	return s.db.Model(&supportdomain.Conversation{}).
		Where("id = ?", conversation.ID).
		Updates(map[string]interface{}{
			"status":               conversation.Status,
			"last_message_at":      conversation.LastMessageAt,
			"last_message_preview": conversation.LastMessagePreview,
			"unread_for_agent":     conversation.UnreadForAgent,
			"unread_for_user":      conversation.UnreadForUser,
			"updated_at":           conversation.UpdatedAt,
		}).Error
}

func (s *Store) ListConversations(filter supportcontract.ConversationListFilter) ([]supportdomain.Conversation, int64, error) {
	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	query := s.db.Model(&supportdomain.Conversation{})
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if filter.UnreadOnly {
		query = query.Where("unread_for_agent > 0")
	}
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var conversations []supportdomain.Conversation
	// 未读优先、再按最后活跃倒序：客服打开工作台第一眼就该是待处理的。
	if err := query.Order("unread_for_agent DESC, last_message_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&conversations).Error; err != nil {
		return nil, 0, err
	}
	return conversations, total, nil
}

func (s *Store) CountUnreadConversations() (int64, error) {
	var total int64
	if err := s.db.Model(&supportdomain.Conversation{}).
		Where("unread_for_agent > 0").Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) CreateMessage(message *supportdomain.Message) error {
	if message == nil {
		return supportcontract.ErrCreateFailed
	}
	return s.db.Create(message).Error
}

func (s *Store) GetMessageByID(id uint) (*supportdomain.Message, error) {
	if id == 0 {
		return nil, nil
	}
	var message supportdomain.Message
	if err := s.db.Where("id = ?", id).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}

// RevokeMessage 只写 revoked_at。
// 条件里带上 revoked_at IS NULL：并发重复撤回时只有第一次会命中，
// 后到的那次影响 0 行，由服务层判定为「已撤回」而不是覆盖时间戳。
func (s *Store) RevokeMessage(id uint, revokedAt time.Time) error {
	if id == 0 {
		return supportcontract.ErrMessageNotFound
	}
	result := s.db.Model(&supportdomain.Message{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", revokedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return supportcontract.ErrMessageNotFound
	}
	return nil
}

func (s *Store) ListMessages(filter supportcontract.MessageListFilter) ([]supportdomain.Message, int64, error) {
	if filter.ConversationID == 0 {
		return nil, 0, nil
	}
	query := s.db.Model(&supportdomain.Message{}).Where("conversation_id = ?", filter.ConversationID)

	// 增量拉取：只取比游标更新的消息，按正序返回，前端直接追加即可。
	if filter.AfterID > 0 {
		var messages []supportdomain.Message
		if err := query.Where("id > ?", filter.AfterID).
			Order("id ASC").Limit(200).Find(&messages).Error; err != nil {
			return nil, 0, err
		}
		return messages, int64(len(messages)), nil
	}

	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var messages []supportdomain.Message
	// 首屏取最近 N 条，倒序查出后翻正，避免历史很长时把整段拉下来。
	if err := query.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&messages).Error; err != nil {
		return nil, 0, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, total, nil
}

func (s *Store) ListQuickReplies(activeOnly bool) ([]supportdomain.QuickReply, error) {
	query := s.db.Model(&supportdomain.QuickReply{})
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	var replies []supportdomain.QuickReply
	if err := query.Order("sort_order DESC, id ASC").Find(&replies).Error; err != nil {
		return nil, err
	}
	return replies, nil
}

func (s *Store) GetQuickReplyByID(id uint) (*supportdomain.QuickReply, error) {
	if id == 0 {
		return nil, nil
	}
	var reply supportdomain.QuickReply
	if err := s.db.Where("id = ?", id).First(&reply).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &reply, nil
}

func (s *Store) CreateQuickReply(reply *supportdomain.QuickReply) error {
	if reply == nil {
		return supportcontract.ErrQuickReplyInvalid
	}
	return s.db.Create(reply).Error
}

func (s *Store) UpdateQuickReply(reply *supportdomain.QuickReply) error {
	if reply == nil || reply.ID == 0 {
		return supportcontract.ErrQuickReplyInvalid
	}
	return s.db.Model(&supportdomain.QuickReply{}).
		Where("id = ?", reply.ID).
		Updates(map[string]interface{}{
			"title":      reply.Title,
			"content":    reply.Content,
			"is_active":  reply.IsActive,
			"sort_order": reply.SortOrder,
			"updated_at": reply.UpdatedAt,
		}).Error
}

func (s *Store) DeleteQuickReply(id uint) error {
	if id == 0 {
		return nil
	}
	return s.db.Where("id = ?", id).Delete(&supportdomain.QuickReply{}).Error
}

// isDuplicateKey 与 points/freeclaim 同款：gorm 未开启 TranslateError，
// errors.Is(err, gorm.ErrDuplicatedKey) 恒为假，只能按驱动报文匹配。
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key")
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = 30
	case pageSize > 100:
		pageSize = 100
	}
	return page, pageSize
}
