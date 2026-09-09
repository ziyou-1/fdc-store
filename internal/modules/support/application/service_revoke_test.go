package application

import (
	"errors"
	"testing"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
)

// seedConversation 造一个买家会话，返回会话 ID。
func seedConversation(t *testing.T, repo *fakeRepo, userID uint) uint {
	t.Helper()
	conversation := &supportdomain.Conversation{UserID: userID}
	if err := repo.CreateConversation(conversation); err != nil {
		t.Fatalf("造会话失败: %v", err)
	}
	return conversation.ID
}

func TestRevokeAgentMessageMarksInsteadOfDeleting(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversationID := seedConversation(t, repo, 7)

	sent, err := service.SendAgentMessage(3, conversationID, supportcontract.SendMessageInput{Content: "打错了"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	revoked, err := service.RevokeAgentMessage(sent.ID, 3)
	if err != nil {
		t.Fatalf("撤回失败: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("撤回后 revoked_at 仍为空")
	}

	// 只标记不删除：行必须还在，否则买家侧会凭空少一条消息。
	messages, _, err := repo.ListMessages(supportcontract.MessageListFilter{ConversationID: conversationID})
	if err != nil {
		t.Fatalf("列消息失败: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("撤回不应删行，期望 1 条，实际 %d 条", len(messages))
	}
	if messages[0].Content != "打错了" {
		t.Fatalf("撤回不应清空内容，实际 %q", messages[0].Content)
	}
}

func TestRevokeAgentMessageRejectsOtherAgent(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversationID := seedConversation(t, repo, 7)

	sent, err := service.SendAgentMessage(3, conversationID, supportcontract.SendMessageInput{Content: "你好"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	if _, err := service.RevokeAgentMessage(sent.ID, 99); !errors.Is(err, supportcontract.ErrRevokeNotAllowed) {
		t.Fatalf("别的客服撤回应被拒，实际 %v", err)
	}
}

func TestRevokeAgentMessageRejectsBuyerMessage(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	seedConversation(t, repo, 7)

	sent, err := service.SendUserMessage(7, supportcontract.SendMessageInput{Content: "在吗"})
	if err != nil {
		t.Fatalf("买家发送失败: %v", err)
	}

	// 客服撤掉买家说过的话等于篡改证据，任何情况下都不允许。
	if _, err := service.RevokeAgentMessage(sent.ID, 3); !errors.Is(err, supportcontract.ErrRevokeNotAllowed) {
		t.Fatalf("撤回买家消息应被拒，实际 %v", err)
	}
}

func TestRevokeAgentMessageTwiceRejected(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversationID := seedConversation(t, repo, 7)

	sent, err := service.SendAgentMessage(3, conversationID, supportcontract.SendMessageInput{Content: "重复撤回"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if _, err := service.RevokeAgentMessage(sent.ID, 3); err != nil {
		t.Fatalf("首次撤回失败: %v", err)
	}
	if _, err := service.RevokeAgentMessage(sent.ID, 3); !errors.Is(err, supportcontract.ErrRevokeNotAllowed) {
		t.Fatalf("二次撤回应被拒，实际 %v", err)
	}
}

func TestRevokeAgentMessageNotFound(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	if _, err := service.RevokeAgentMessage(4242, 3); !errors.Is(err, supportcontract.ErrMessageNotFound) {
		t.Fatalf("不存在的消息应返回 not found，实际 %v", err)
	}
}

func TestSendAgentMessageKeepsValidReply(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversationID := seedConversation(t, repo, 7)

	buyer, err := service.SendUserMessage(7, supportcontract.SendMessageInput{Content: "订单没到"})
	if err != nil {
		t.Fatalf("买家发送失败: %v", err)
	}

	reply, err := service.SendAgentMessage(3, conversationID, supportcontract.SendMessageInput{
		Content:   "帮您查一下",
		ReplyToID: buyer.ID,
	})
	if err != nil {
		t.Fatalf("引用回复失败: %v", err)
	}
	if reply.ReplyToID != buyer.ID {
		t.Fatalf("引用 ID 应为 %d，实际 %d", buyer.ID, reply.ReplyToID)
	}
}

func TestSendAgentMessageDropsCrossConversationReply(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	seedConversation(t, repo, 7)
	otherConversationID := seedConversation(t, repo, 8)

	// 别人会话里的一条消息
	foreign, err := service.SendAgentMessage(3, otherConversationID, supportcontract.SendMessageInput{Content: "另一个买家的对话"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("取会话失败: %v", err)
	}
	reply, err := service.SendAgentMessage(3, conversation.ID, supportcontract.SendMessageInput{
		Content:   "你好",
		ReplyToID: foreign.ID,
	})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	// 跨会话引用会把别人的对话内容搬进这个会话，必须丢弃而不是报错。
	if reply.ReplyToID != 0 {
		t.Fatalf("跨会话引用应被丢弃，实际 %d", reply.ReplyToID)
	}
}

func TestSendAgentMessageDropsMissingReply(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversationID := seedConversation(t, repo, 7)

	reply, err := service.SendAgentMessage(3, conversationID, supportcontract.SendMessageInput{
		Content:   "你好",
		ReplyToID: 9999,
	})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if reply.ReplyToID != 0 {
		t.Fatalf("引用不存在的消息应被丢弃，实际 %d", reply.ReplyToID)
	}
}

func TestRevokeDoesNotRollBackUnread(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversationID := seedConversation(t, repo, 7)

	sent, err := service.SendAgentMessage(3, conversationID, supportcontract.SendMessageInput{Content: "先发后撤"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	before, err := repo.GetConversationByID(conversationID)
	if err != nil {
		t.Fatalf("取会话失败: %v", err)
	}
	unreadBefore := before.UnreadForUser

	if _, err := service.RevokeAgentMessage(sent.ID, 3); err != nil {
		t.Fatalf("撤回失败: %v", err)
	}

	after, err := repo.GetConversationByID(conversationID)
	if err != nil {
		t.Fatalf("取会话失败: %v", err)
	}
	// 买家很可能已经读到了，硬减未读只会让工作台计数和买家看到的对不上。
	if after.UnreadForUser != unreadBefore {
		t.Fatalf("撤回不应回退未读，撤回前 %d 撤回后 %d", unreadBefore, after.UnreadForUser)
	}
}
