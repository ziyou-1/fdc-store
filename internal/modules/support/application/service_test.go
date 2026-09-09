package application

import (
	"errors"
	"sort"
	"testing"
	"time"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
)

// fakeRepo 是内存仓储，覆盖会话与消息用例，无需数据库。
type fakeRepo struct {
	conversations map[uint]*supportdomain.Conversation
	byUser        map[uint]uint
	messages      []supportdomain.Message
	replies       map[uint]*supportdomain.QuickReply
	nextID        uint
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		conversations: map[uint]*supportdomain.Conversation{},
		byUser:        map[uint]uint{},
		replies:       map[uint]*supportdomain.QuickReply{},
		nextID:        1,
	}
}

func (r *fakeRepo) take() uint {
	id := r.nextID
	r.nextID++
	return id
}

func (r *fakeRepo) GetConversationByUserID(userID uint) (*supportdomain.Conversation, error) {
	id, ok := r.byUser[userID]
	if !ok {
		return nil, nil
	}
	return r.conversations[id], nil
}

func (r *fakeRepo) GetConversationByID(id uint) (*supportdomain.Conversation, error) {
	return r.conversations[id], nil
}

func (r *fakeRepo) CreateConversation(conversation *supportdomain.Conversation) error {
	conversation.ID = r.take()
	stored := *conversation
	r.conversations[conversation.ID] = &stored
	r.byUser[conversation.UserID] = conversation.ID
	return nil
}

func (r *fakeRepo) UpdateConversation(conversation *supportdomain.Conversation) error {
	if _, ok := r.conversations[conversation.ID]; !ok {
		return supportcontract.ErrUpdateFailed
	}
	stored := *conversation
	r.conversations[conversation.ID] = &stored
	return nil
}

func (r *fakeRepo) ListConversations(filter supportcontract.ConversationListFilter) (
	[]supportdomain.Conversation, int64, error,
) {
	result := make([]supportdomain.Conversation, 0, len(r.conversations))
	for _, conversation := range r.conversations {
		if filter.UnreadOnly && conversation.UnreadForAgent == 0 {
			continue
		}
		if filter.Status != "" && conversation.Status != filter.Status {
			continue
		}
		result = append(result, *conversation)
	}
	return result, int64(len(result)), nil
}

func (r *fakeRepo) CountUnreadConversations() (int64, error) {
	var total int64
	for _, conversation := range r.conversations {
		if conversation.UnreadForAgent > 0 {
			total++
		}
	}
	return total, nil
}

func (r *fakeRepo) CreateMessage(message *supportdomain.Message) error {
	message.ID = r.take()
	r.messages = append(r.messages, *message)
	return nil
}

func (r *fakeRepo) GetMessageByID(id uint) (*supportdomain.Message, error) {
	for i := range r.messages {
		if r.messages[i].ID == id {
			message := r.messages[i]
			return &message, nil
		}
	}
	return nil, nil
}

func (r *fakeRepo) RevokeMessage(id uint, revokedAt time.Time) error {
	for i := range r.messages {
		if r.messages[i].ID != id {
			continue
		}
		// 与 gormstore 的 WHERE revoked_at IS NULL 对齐：重复撤回视为找不到。
		if r.messages[i].RevokedAt != nil {
			return supportcontract.ErrMessageNotFound
		}
		r.messages[i].RevokedAt = &revokedAt
		return nil
	}
	return supportcontract.ErrMessageNotFound
}

func (r *fakeRepo) ListRevokedSince(conversationID uint, since time.Time, limit int) (
	[]supportdomain.Message, error,
) {
	result := make([]supportdomain.Message, 0)
	for _, message := range r.messages {
		if message.ConversationID != conversationID || message.RevokedAt == nil {
			continue
		}
		// 与 gormstore 一致用 >=：撤回时间撞到同一时刻时 > 会永久漏掉一条。
		if !since.IsZero() && message.RevokedAt.Before(since) {
			continue
		}
		result = append(result, message)
	}
	// 按 revoked_at 倒序，首条即最新，服务层拿它当游标。
	sort.Slice(result, func(i, j int) bool {
		return result[i].RevokedAt.After(*result[j].RevokedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *fakeRepo) ListMessages(filter supportcontract.MessageListFilter) (
	[]supportdomain.Message, int64, error,
) {
	result := make([]supportdomain.Message, 0, len(r.messages))
	for _, message := range r.messages {
		if message.ConversationID != filter.ConversationID {
			continue
		}
		if filter.AfterID > 0 && message.ID <= filter.AfterID {
			continue
		}
		result = append(result, message)
	}
	return result, int64(len(result)), nil
}

func (r *fakeRepo) ListQuickReplies(activeOnly bool) ([]supportdomain.QuickReply, error) {
	result := make([]supportdomain.QuickReply, 0, len(r.replies))
	for _, reply := range r.replies {
		if activeOnly && !reply.IsActive {
			continue
		}
		result = append(result, *reply)
	}
	return result, nil
}

func (r *fakeRepo) GetQuickReplyByID(id uint) (*supportdomain.QuickReply, error) {
	return r.replies[id], nil
}

func (r *fakeRepo) CreateQuickReply(reply *supportdomain.QuickReply) error {
	reply.ID = r.take()
	stored := *reply
	r.replies[reply.ID] = &stored
	return nil
}

func (r *fakeRepo) UpdateQuickReply(reply *supportdomain.QuickReply) error {
	stored := *reply
	r.replies[reply.ID] = &stored
	return nil
}

func (r *fakeRepo) DeleteQuickReply(id uint) error {
	delete(r.replies, id)
	return nil
}

func TestEnsureConversationIsIdempotentPerUser(t *testing.T) {
	service := NewService(newFakeRepo())
	first, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	second, err := service.EnsureConversation(7)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected one conversation per user, got %d and %d", first.ID, second.ID)
	}
}

func TestEnsureConversationRejectsGuest(t *testing.T) {
	service := NewService(newFakeRepo())
	if _, err := service.EnsureConversation(0); !errors.Is(err, supportcontract.ErrGuestForbidden) {
		t.Fatalf("expected guest forbidden, got %v", err)
	}
}

func TestSendUserMessageBumpsAgentUnreadAndPreview(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	if _, err := service.SendUserMessage(3, supportcontract.SendMessageInput{Content: "订单没收到卡密"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	conversation, err := service.EnsureConversation(3)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if conversation.UnreadForAgent != 1 {
		t.Fatalf("expected agent unread 1, got %d", conversation.UnreadForAgent)
	}
	if conversation.UnreadForUser != 0 {
		t.Fatalf("expected user unread 0, got %d", conversation.UnreadForUser)
	}
	if conversation.LastMessagePreview != "订单没收到卡密" {
		t.Fatalf("unexpected preview %q", conversation.LastMessagePreview)
	}
	if conversation.LastMessageAt == nil {
		t.Fatal("expected last message timestamp")
	}
}

func TestSendAgentMessageBumpsUserUnreadOnly(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversation, err := service.EnsureConversation(4)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := service.SendAgentMessage(11, conversation.ID,
		supportcontract.SendMessageInput{Content: "已补发，请查收"}); err != nil {
		t.Fatalf("agent send: %v", err)
	}
	refreshed, err := repo.GetConversationByID(conversation.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if refreshed.UnreadForUser != 1 || refreshed.UnreadForAgent != 0 {
		t.Fatalf("unexpected unread counters: user=%d agent=%d",
			refreshed.UnreadForUser, refreshed.UnreadForAgent)
	}
}

func TestUserMessageReopensClosedConversation(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversation, err := service.EnsureConversation(5)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := service.SetConversationStatus(conversation.ID, supportdomain.ConversationStatusClosed); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := service.SendUserMessage(5, supportcontract.SendMessageInput{Content: "还有个问题"}); err != nil {
		t.Fatalf("send after close: %v", err)
	}
	refreshed, err := repo.GetConversationByID(conversation.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if refreshed.Status != supportdomain.ConversationStatusOpen {
		t.Fatalf("expected reopened conversation, got %q", refreshed.Status)
	}
}

func TestSendMessageValidatesContent(t *testing.T) {
	service := NewService(newFakeRepo())
	if _, err := service.SendUserMessage(6, supportcontract.SendMessageInput{Content: "   "}); !errors.Is(err, supportcontract.ErrMessageEmpty) {
		t.Fatalf("expected empty message error, got %v", err)
	}
	if _, err := service.SendUserMessage(6, supportcontract.SendMessageInput{
		ContentType: supportdomain.ContentTypeImage,
	}); !errors.Is(err, supportcontract.ErrMediaURLRequired) {
		t.Fatalf("expected media url required, got %v", err)
	}
	if _, err := service.SendUserMessage(6, supportcontract.SendMessageInput{
		ContentType: "audio",
		Content:     "x",
	}); !errors.Is(err, supportcontract.ErrContentTypeInvalid) {
		t.Fatalf("expected content type invalid, got %v", err)
	}
}

func TestMediaMessagePreviewIsPlaceholder(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	if _, err := service.SendUserMessage(8, supportcontract.SendMessageInput{
		ContentType: supportdomain.ContentTypeVideo,
		MediaURL:    "/uploads/support/2026/09/a.mp4",
	}); err != nil {
		t.Fatalf("send video: %v", err)
	}
	conversation, err := service.EnsureConversation(8)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if conversation.LastMessagePreview != "[视频]" {
		t.Fatalf("unexpected preview %q", conversation.LastMessagePreview)
	}
}

func TestMarkReadClearsOnlyOwnSide(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	conversation, err := service.EnsureConversation(9)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := service.SendUserMessage(9, supportcontract.SendMessageInput{Content: "在吗"}); err != nil {
		t.Fatalf("user send: %v", err)
	}
	if _, err := service.SendAgentMessage(1, conversation.ID,
		supportcontract.SendMessageInput{Content: "在的"}); err != nil {
		t.Fatalf("agent send: %v", err)
	}
	if err := service.MarkReadByAgent(conversation.ID); err != nil {
		t.Fatalf("agent read: %v", err)
	}
	refreshed, err := repo.GetConversationByID(conversation.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if refreshed.UnreadForAgent != 0 {
		t.Fatalf("expected agent unread cleared, got %d", refreshed.UnreadForAgent)
	}
	if refreshed.UnreadForUser != 1 {
		t.Fatalf("agent read must not clear buyer unread, got %d", refreshed.UnreadForUser)
	}
	if err := service.MarkReadByUser(9); err != nil {
		t.Fatalf("user read: %v", err)
	}
	refreshed, err = repo.GetConversationByID(conversation.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if refreshed.UnreadForUser != 0 {
		t.Fatalf("expected buyer unread cleared, got %d", refreshed.UnreadForUser)
	}
}

func TestListUserMessagesSupportsIncrementalCursor(t *testing.T) {
	service := NewService(newFakeRepo())
	first, err := service.SendUserMessage(12, supportcontract.SendMessageInput{Content: "一"})
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	if _, err := service.SendUserMessage(12, supportcontract.SendMessageInput{Content: "二"}); err != nil {
		t.Fatalf("second send: %v", err)
	}
	result, err := service.ListUserMessages(12, first.ID, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result.Messages) != 1 || result.Messages[0].Content != "二" {
		t.Fatalf("expected only result.Messages after cursor, got %#v", result.Messages)
	}
}

func TestSetConversationStatusRejectsUnknownValue(t *testing.T) {
	service := NewService(newFakeRepo())
	conversation, err := service.EnsureConversation(13)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := service.SetConversationStatus(conversation.ID, "archived"); err == nil {
		t.Fatal("expected unknown status to be rejected")
	}
}

func TestSendUserMessageAutoRepliesMatchingFAQ(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	if _, err := service.CreateQuickReply(supportcontract.QuickReplyInput{
		Title:    "我是你的智能助手",
		Content:  "请把订单号发我，我帮你查激活进度。",
		IsActive: true,
	}); err != nil {
		t.Fatalf("create faq: %v", err)
	}

	userMessage, err := service.SendUserMessage(31, supportcontract.SendMessageInput{
		Content: "  我是你的智能助手  ",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if userMessage.SenderType != supportdomain.SenderTypeUser {
		t.Fatalf("send must still return the buyer message, got %s", userMessage.SenderType)
	}

	conversation, err := service.EnsureConversation(31)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	result, err := service.ListUserMessages(31, 0, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected buyer question plus FAQ solution, got %d result.Messages: %#v", len(result.Messages), result.Messages)
	}
	if result.Messages[1].SenderType != supportdomain.SenderTypeAgent {
		t.Fatalf("solution must be an agent message on the left, got %s", result.Messages[1].SenderType)
	}
	if result.Messages[1].Content != "请把订单号发我，我帮你查激活进度。" {
		t.Fatalf("unexpected solution %q", result.Messages[1].Content)
	}
	if conversation.LastMessagePreview != "请把订单号发我，我帮你查激活进度。" {
		t.Fatalf("unexpected preview %q", conversation.LastMessagePreview)
	}
	if conversation.UnreadForUser != 1 {
		t.Fatalf("expected buyer unread 1 after auto-reply, got %d", conversation.UnreadForUser)
	}
}

func TestSendUserMessageTranslatesBuyerTextAndStillAutoRepliesFAQ(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	service.SetUserTextTranslator(func(text string) (string, error) {
		return "译：" + text, nil
	})
	if _, err := service.CreateQuickReply(supportcontract.QuickReplyInput{
		Title:    "怎么激活",
		Content:  "打开官网粘贴卡密",
		IsActive: true,
	}); err != nil {
		t.Fatalf("create faq: %v", err)
	}
	if _, err := service.SendUserMessage(32, supportcontract.SendMessageInput{Content: "怎么激活"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	result, err := service.ListUserMessages(32, 0, 1, 30, time.Time{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected buyer plus FAQ, got %#v", result.Messages)
	}
	if result.Messages[0].Translation != "译：怎么激活" {
		t.Fatalf("buyer translation %q", result.Messages[0].Translation)
	}
	if result.Messages[1].Translation != "" || result.Messages[1].Content != "打开官网粘贴卡密" {
		t.Fatalf("FAQ reply must stay untranslated, got %#v", result.Messages[1])
	}
}

func TestSendUserMessageDoesNotAutoReplyWhenFAQDoesNotMatch(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	if _, err := service.CreateQuickReply(supportcontract.QuickReplyInput{
		Title:    "怎么激活",
		Content:  "打开官网粘贴卡密",
		IsActive: true,
	}); err != nil {
		t.Fatalf("create faq: %v", err)
	}
	if _, err := service.CreateQuickReply(supportcontract.QuickReplyInput{
		Title:    "已停用的问题",
		Content:  "不该出现",
		IsActive: false,
	}); err != nil {
		t.Fatalf("create inactive faq: %v", err)
	}
	if _, err := service.CreateQuickReply(supportcontract.QuickReplyInput{
		Title:    "只有标题",
		IsActive: true,
	}); err != nil {
		t.Fatalf("create empty faq: %v", err)
	}

	cases := []string{"订单没收到卡密", "已停用的问题", "只有标题"}
	for _, content := range cases {
		userID := uint(40 + len(content))
		if _, err := service.SendUserMessage(userID, supportcontract.SendMessageInput{Content: content}); err != nil {
			t.Fatalf("send %q: %v", content, err)
		}
		result, err := service.ListUserMessages(userID, 0, 1, 30, time.Time{})
		if err != nil {
			t.Fatalf("list %q: %v", content, err)
		}
		if len(result.Messages) != 1 {
			t.Fatalf("expected no auto-reply for %q, got %#v", content, result.Messages)
		}
	}
}

func TestQuickReplyLifecycle(t *testing.T) {
	service := NewService(newFakeRepo())
	if _, err := service.CreateQuickReply(supportcontract.QuickReplyInput{Title: "  "}); !errors.Is(
		err, supportcontract.ErrQuickReplyInvalid,
	) {
		t.Fatalf("expected invalid title rejection, got %v", err)
	}
	created, err := service.CreateQuickReply(supportcontract.QuickReplyInput{
		Title:    "怎么激活",
		Content:  "复制卡密后在官网登录激活",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	active, err := service.ListQuickReplies(true)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active reply, got %d", len(active))
	}
	if _, err := service.UpdateQuickReply(created.ID, supportcontract.QuickReplyInput{
		Title:    "怎么激活",
		Content:  "见教程",
		IsActive: false,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	active, err = service.ListQuickReplies(true)
	if err != nil {
		t.Fatalf("list active after disable: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected disabled reply hidden from buyers, got %d", len(active))
	}
	if _, err := service.UpdateQuickReply(created.ID+99, supportcontract.QuickReplyInput{
		Title: "不存在",
	}); !errors.Is(err, supportcontract.ErrQuickReplyNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := service.DeleteQuickReply(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestListConversationsAttachesUserEmail(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	service.SetUserReader(stubUserReader{emails: map[uint]string{20: "buyer@example.com"}})
	if _, err := service.SendUserMessage(20, supportcontract.SendMessageInput{Content: "你好"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	views, total, err := service.ListConversations(supportcontract.ConversationListFilter{UnreadOnly: true})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(views) != 1 {
		t.Fatalf("expected 1 unread conversation, got %d", total)
	}
	if views[0].UserEmail != "buyer@example.com" {
		t.Fatalf("unexpected email %q", views[0].UserEmail)
	}
}

func TestListConversationsToleratesUserReaderFailure(t *testing.T) {
	service := NewService(newFakeRepo())
	service.SetUserReader(stubUserReader{err: errors.New("user store down")})
	if _, err := service.SendUserMessage(21, supportcontract.SendMessageInput{Content: "你好"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	views, _, err := service.ListConversations(supportcontract.ConversationListFilter{})
	if err != nil {
		t.Fatalf("list must not fail when email lookup fails: %v", err)
	}
	if len(views) != 1 || views[0].UserEmail != "" {
		t.Fatalf("expected conversation without email, got %#v", views)
	}
}

type stubUserReader struct {
	emails map[uint]string
	err    error
}

func (s stubUserReader) GetEmailsByIDs([]uint) (map[uint]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.emails, nil
}

func TestSendUserMessageStoresDeepLTranslation(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	var seen string
	service.SetUserTextTranslator(func(text string) (string, error) {
		seen = text
		return "您好，抱歉，我先取消一下哦", nil
	})

	message, err := service.SendUserMessage(50, supportcontract.SendMessageInput{
		Content: "Kak maaf saya batalkan dulu ya",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if seen != "Kak maaf saya batalkan dulu ya" {
		t.Fatalf("translator saw %q", seen)
	}
	if message.Content != "Kak maaf saya batalkan dulu ya" {
		t.Fatalf("original must stay, got %q", message.Content)
	}
	if message.Translation != "您好，抱歉，我先取消一下哦" {
		t.Fatalf("unexpected translation %q", message.Translation)
	}
}

func TestSendUserMessageDoesNotTranslateMedia(t *testing.T) {
	service := NewService(newFakeRepo())
	service.SetUserTextTranslator(func(string) (string, error) {
		t.Fatal("translator must not run for media")
		return "", nil
	})
	message, err := service.SendUserMessage(51, supportcontract.SendMessageInput{
		ContentType: supportdomain.ContentTypeImage,
		MediaURL:    "/uploads/support/a.png",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if message.Translation != "" {
		t.Fatalf("media must not store translation, got %q", message.Translation)
	}
}

func TestSendAgentMessageDoesNotTranslate(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	service.SetUserTextTranslator(func(string) (string, error) {
		t.Fatal("translator must not run for agent replies")
		return "", nil
	})
	conversation, err := service.EnsureConversation(52)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	message, err := service.SendAgentMessage(1, conversation.ID, supportcontract.SendMessageInput{
		Content: "oke makasih",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if message.Translation != "" {
		t.Fatalf("agent reply must not store translation, got %q", message.Translation)
	}
}

func TestSendUserMessageFailsClosedWhenTranslateFails(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	service.SetUserTextTranslator(func(string) (string, error) {
		return "", errors.New("deepl status 456: Quota exceeded")
	})
	_, err := service.SendUserMessage(53, supportcontract.SendMessageInput{Content: "halo"})
	if !errors.Is(err, supportcontract.ErrTranslateFailed) {
		t.Fatalf("expected translate failed, got %v", err)
	}
	if len(repo.messages) != 0 {
		t.Fatalf("failed translate must not persist the message, got %#v", repo.messages)
	}
}

func TestSendUserMessageFailsClosedWhenTranslationEmpty(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo)
	service.SetUserTextTranslator(func(string) (string, error) {
		return "   ", nil
	})
	_, err := service.SendUserMessage(54, supportcontract.SendMessageInput{Content: "halo"})
	if !errors.Is(err, supportcontract.ErrTranslateFailed) {
		t.Fatalf("expected translate failed, got %v", err)
	}
	if len(repo.messages) != 0 {
		t.Fatalf("empty translation must not persist the message, got %#v", repo.messages)
	}
}
