package application

import (
	"strings"
	"time"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
)

// ListQuickReplies 返回常见问题。activeOnly 为 true 时只返回启用项，供买家端提问与自动回复。
func (s *Service) ListQuickReplies(activeOnly bool) ([]supportdomain.QuickReply, error) {
	return s.repo.ListQuickReplies(activeOnly)
}

// CreateQuickReply 客服在回复页面新增一条常见问题。
func (s *Service) CreateQuickReply(input supportcontract.QuickReplyInput) (*supportdomain.QuickReply, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, supportcontract.ErrQuickReplyInvalid
	}
	now := time.Now()
	reply := &supportdomain.QuickReply{
		Title:     title,
		Content:   strings.TrimSpace(input.Content),
		IsActive:  input.IsActive,
		SortOrder: input.SortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateQuickReply(reply); err != nil {
		return nil, supportcontract.ErrCreateFailed
	}
	return reply, nil
}

// UpdateQuickReply 客服修改一条常见问题。
func (s *Service) UpdateQuickReply(id uint, input supportcontract.QuickReplyInput) (*supportdomain.QuickReply, error) {
	title := strings.TrimSpace(input.Title)
	if id == 0 || title == "" {
		return nil, supportcontract.ErrQuickReplyInvalid
	}
	reply, err := s.repo.GetQuickReplyByID(id)
	if err != nil {
		return nil, err
	}
	if reply == nil {
		return nil, supportcontract.ErrQuickReplyNotFound
	}
	reply.Title = title
	reply.Content = strings.TrimSpace(input.Content)
	reply.IsActive = input.IsActive
	reply.SortOrder = input.SortOrder
	reply.UpdatedAt = time.Now()
	if err := s.repo.UpdateQuickReply(reply); err != nil {
		return nil, supportcontract.ErrUpdateFailed
	}
	return reply, nil
}

// DeleteQuickReply 客服删除一条常见问题。
func (s *Service) DeleteQuickReply(id uint) error {
	return s.repo.DeleteQuickReply(id)
}

// matchFAQSolution 买家文本与启用中的常见问题标题完全一致时，返回对应解决方案。
func matchFAQSolution(text string, replies []supportdomain.QuickReply) string {
	question := strings.TrimSpace(text)
	if question == "" {
		return ""
	}
	for i := range replies {
		reply := replies[i]
		if !reply.IsActive {
			continue
		}
		if strings.TrimSpace(reply.Title) != question {
			continue
		}
		solution := strings.TrimSpace(reply.Content)
		if solution == "" {
			continue
		}
		return solution
	}
	return ""
}
