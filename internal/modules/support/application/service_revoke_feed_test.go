package application

import (
	"errors"
	"testing"
	"time"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
)

// 买家能撤自己刚发的消息。
func TestRevokeUserMessageAllowsOwnRecent(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	sent, err := service.SendUserMessage(7, supportcontract.SendMessageInput{Content: "发错了"})
	if err != nil {
		t.Fatalf("买家发送失败: %v", err)
	}

	revoked, err := service.RevokeUserMessage(sent.ID, 7)
	if err != nil {
		t.Fatalf("买家撤回失败: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("撤回后 revoked_at 仍为空")
	}
	// 返回给前端的那份必须已经抹掉内容。
	if revoked.Content != "" {
		t.Fatalf("撤回响应不该再带原文，实际 %q", revoked.Content)
	}
}

// 买家撤不了客服的消息。
func TestRevokeUserMessageRejectsAgentMessage(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	agentMessage, err := service.SendAgentMessage(3, conversation.ID, supportcontract.SendMessageInput{Content: "您好"})
	if err != nil {
		t.Fatalf("客服发送失败: %v", err)
	}

	if _, err := service.RevokeUserMessage(agentMessage.ID, 7); !errors.Is(err, supportcontract.ErrRevokeNotAllowed) {
		t.Fatalf("买家撤客服消息应被拒，实际 %v", err)
	}
}

// 这是本次改动最要紧的一条：买家 ID 与管理员 ID 是两套独立序列，
// 都存在 sender_id 一列里。撞号时不能让买家撤到客服的消息，反之亦然。
func TestRevokeDoesNotConfuseAgentAndUserIDs(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	const collidingID = 5

	conversation, err := service.EnsureConversation(collidingID)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	// 5 号客服发的消息
	agentMessage, err := service.SendAgentMessage(collidingID, conversation.ID,
		supportcontract.SendMessageInput{Content: "客服说的话"})
	if err != nil {
		t.Fatalf("客服发送失败: %v", err)
	}
	// 5 号买家发的消息
	userMessage, err := service.SendUserMessage(collidingID, supportcontract.SendMessageInput{Content: "买家说的话"})
	if err != nil {
		t.Fatalf("买家发送失败: %v", err)
	}

	if _, err := service.RevokeUserMessage(agentMessage.ID, collidingID); !errors.Is(err, supportcontract.ErrRevokeNotAllowed) {
		t.Fatalf("5 号买家不该撤到 5 号客服的消息，实际 %v", err)
	}
	if _, err := service.RevokeAgentMessage(userMessage.ID, collidingID); !errors.Is(err, supportcontract.ErrRevokeNotAllowed) {
		t.Fatalf("5 号客服不该撤到 5 号买家的消息，实际 %v", err)
	}
}

// 买家撤不到别人会话里的消息，哪怕消息 ID 猜对了。
func TestRevokeUserMessageRejectsForeignConversation(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	foreign, err := service.SendUserMessage(8, supportcontract.SendMessageInput{Content: "别人的话"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	// 不该泄露「这条消息存在但不是你的」，统一按找不到处理。
	if _, err := service.RevokeUserMessage(foreign.ID, 7); !errors.Is(err, supportcontract.ErrMessageNotFound) {
		t.Fatalf("跨会话撤回应按找不到处理，实际 %v", err)
	}
}

// 买家也能引用回复。
func TestSendUserMessageKeepsValidReply(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	agentMessage, err := service.SendAgentMessage(3, conversation.ID,
		supportcontract.SendMessageInput{Content: "请提供订单号"})
	if err != nil {
		t.Fatalf("客服发送失败: %v", err)
	}

	reply, err := service.SendUserMessage(7, supportcontract.SendMessageInput{
		Content:   "20260909001",
		ReplyToID: agentMessage.ID,
	})
	if err != nil {
		t.Fatalf("买家引用回复失败: %v", err)
	}
	if reply.ReplyToID != agentMessage.ID {
		t.Fatalf("引用 ID 应为 %d，实际 %d", agentMessage.ID, reply.ReplyToID)
	}
}

// ---- 撤回的增量回传：用户报的「撤回后要刷新一下才看得到」就是这里 ----

// 撤回不产生新消息，按 after_id 的增量结果里永远不会有它，
// 必须走 Revoked 这条独立通道回传。
func TestListUserMessagesFeedsRevocationsThatIncrementalCursorMisses(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	sent, err := service.SendAgentMessage(3, conversation.ID, supportcontract.SendMessageInput{Content: "打错了"})
	if err != nil {
		t.Fatalf("客服发送失败: %v", err)
	}

	// 买家此刻已经收到了这条，游标停在它身上
	cursor := sent.ID

	if _, err := service.RevokeAgentMessage(sent.ID, 3); err != nil {
		t.Fatalf("撤回失败: %v", err)
	}

	result, err := service.ListUserMessages(7, cursor, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("增量拉取失败: %v", err)
	}

	// 增量结果里没有它——这正是不刷新就看不到撤回的根因
	if len(result.Messages) != 0 {
		t.Fatalf("撤回不该出现在增量消息里，实际 %#v", result.Messages)
	}
	// 但必须从 Revoked 通道回来
	if len(result.Revoked) != 1 || result.Revoked[0].ID != sent.ID {
		t.Fatalf("撤回应通过 Revoked 回传，实际 %#v", result.Revoked)
	}
	if result.RevokedCursor == nil {
		t.Fatal("必须回传撤回游标，否则前端下一轮会把全部撤回再拉一遍")
	}
}

// 撤回回传里不能带原文，否则对端翻网络面板就能读到，等于没撤。
func TestListUserMessagesRedactsRevokedContent(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	sent, err := service.SendAgentMessage(3, conversation.ID,
		supportcontract.SendMessageInput{Content: "不该被看到的原文"})
	if err != nil {
		t.Fatalf("客服发送失败: %v", err)
	}
	if _, err := service.RevokeAgentMessage(sent.ID, 3); err != nil {
		t.Fatalf("撤回失败: %v", err)
	}

	result, err := service.ListUserMessages(7, 0, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("整页拉取失败: %v", err)
	}

	for _, m := range result.Messages {
		if m.ID == sent.ID && m.Content != "" {
			t.Fatalf("整页结果里的撤回消息仍带原文: %q", m.Content)
		}
	}
	for _, m := range result.Revoked {
		if m.Content != "" {
			t.Fatalf("撤回通道里仍带原文: %q", m.Content)
		}
	}

	// 库里那行必须原样保留，撤回是软标记，审计痕迹不能丢
	stored, err := repo.GetMessageByID(sent.ID)
	if err != nil {
		t.Fatalf("读库失败: %v", err)
	}
	if stored.Content != "不该被看到的原文" {
		t.Fatalf("库里的原文不该被抹掉，实际 %q", stored.Content)
	}
}

// 游标推进后不该再重复回传更早的撤回。
func TestRevokedCursorAdvances(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	first, err := service.SendAgentMessage(3, conversation.ID, supportcontract.SendMessageInput{Content: "一"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if _, err := service.RevokeAgentMessage(first.ID, 3); err != nil {
		t.Fatalf("撤回失败: %v", err)
	}

	first_result, err := service.ListUserMessages(7, 0, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	if first_result.RevokedCursor == nil {
		t.Fatal("首轮应回传游标")
	}

	// 带着游标再拉一次：撤回时间等于游标，仍会回来（>= 语义），但不会更多
	second, err := service.ListUserMessages(7, first.ID, 1, 30, *first_result.RevokedCursor)
	if err != nil {
		t.Fatalf("二次拉取失败: %v", err)
	}
	if len(second.Revoked) > 1 {
		t.Fatalf("带游标后不该回传更早的撤回，实际 %d 条", len(second.Revoked))
	}

	// 游标之后没有新撤回时，游标必须原样还回去，不能退回零值
	if second.RevokedCursor == nil {
		t.Fatal("没有新撤回时游标不该丢失，否则前端会退回零值把全部撤回再拉一遍")
	}
}

// 没有任何撤回时，Revoked 为空且不回传游标，前端保持自己的游标不变。
func TestListMessagesWithoutRevocations(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	if _, err := service.SendUserMessage(7, supportcontract.SendMessageInput{Content: "你好"}); err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	result, err := service.ListUserMessages(7, 0, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	if len(result.Revoked) != 0 {
		t.Fatalf("没有撤回时不该回传，实际 %#v", result.Revoked)
	}
	if result.RevokedCursor != nil {
		t.Fatalf("没有撤回也没有入参游标时不该回传游标，实际 %v", *result.RevokedCursor)
	}
}

// 客服侧同样要能拿到撤回增量，否则买家撤回后客服也要刷新才看得到。
func TestListAgentMessagesFeedsRevocations(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	sent, err := service.SendUserMessage(7, supportcontract.SendMessageInput{Content: "手滑了"})
	if err != nil {
		t.Fatalf("买家发送失败: %v", err)
	}
	if _, err := service.RevokeUserMessage(sent.ID, 7); err != nil {
		t.Fatalf("买家撤回失败: %v", err)
	}

	result, err := service.ListAgentMessages(sent.ConversationID, sent.ID, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("客服拉取失败: %v", err)
	}
	if len(result.Revoked) != 1 || result.Revoked[0].ID != sent.ID {
		t.Fatalf("客服侧应收到撤回增量，实际 %#v", result.Revoked)
	}
	if result.Revoked[0].Content != "" {
		t.Fatalf("客服侧的撤回回传也不该带原文，实际 %q", result.Revoked[0].Content)
	}
}

// 撤回消息不能混进 Messages。混进去的话前端那句
// 「本轮有新消息就标记已读 / 滚到底部」会被一条旧撤回反复触发。
func TestRevocationsNeverLeakIntoIncrementalMessages(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	old, err := service.SendAgentMessage(3, conversation.ID, supportcontract.SendMessageInput{Content: "旧消息"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if _, err := service.RevokeAgentMessage(old.ID, 3); err != nil {
		t.Fatalf("撤回失败: %v", err)
	}

	// 游标已经越过这条撤回的消息，之后连拉三轮都不该有"新消息"
	for round := 1; round <= 3; round++ {
		result, err := service.ListUserMessages(7, old.ID, 1, 30, time.Time{})
		if err != nil {
			t.Fatalf("第 %d 轮拉取失败: %v", round, err)
		}
		if len(result.Messages) != 0 {
			t.Fatalf("第 %d 轮把撤回当成了新消息: %#v", round, result.Messages)
		}
	}
}

// 消息发出后又立刻撤回、买家还没来得及轮询到：
// 这条会走正常的增量通道回来，且已经是撤回态。
func TestMessageSentAndRevokedBetweenPollsArrivesAlreadyRevoked(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)

	conversation, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	before, err := service.SendAgentMessage(3, conversation.ID, supportcontract.SendMessageInput{Content: "上一条"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	flash, err := service.SendAgentMessage(3, conversation.ID, supportcontract.SendMessageInput{Content: "秒撤"})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if _, err := service.RevokeAgentMessage(flash.ID, 3); err != nil {
		t.Fatalf("撤回失败: %v", err)
	}

	result, err := service.ListUserMessages(7, before.ID, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	if len(result.Messages) != 1 || result.Messages[0].ID != flash.ID {
		t.Fatalf("秒撤的消息仍应通过增量通道回来，实际 %#v", result.Messages)
	}
	if !result.Messages[0].IsRevoked() {
		t.Fatal("它到达时就该是撤回态")
	}
	if result.Messages[0].Content != "" {
		t.Fatalf("从没见过它的一方更不该拿到原文，实际 %q", result.Messages[0].Content)
	}
}
