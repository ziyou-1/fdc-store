package gormstore

import (
	"fmt"
	"testing"
	"time"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupStore(t *testing.T) *Store {
	t.Helper()
	dsn := fmt.Sprintf("file:support_revoke_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&supportdomain.Conversation{}, &supportdomain.Message{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return New(db)
}

func seedMessage(t *testing.T, s *Store, conversationID uint, senderType, content string) *supportdomain.Message {
	t.Helper()
	message := &supportdomain.Message{
		ConversationID: conversationID,
		SenderType:     senderType,
		SenderID:       1,
		ContentType:    supportdomain.ContentTypeText,
		Content:        content,
		CreatedAt:      time.Now(),
	}
	if err := s.CreateMessage(message); err != nil {
		t.Fatalf("create message: %v", err)
	}
	return message
}

// 这是撤回能不能不刷新就同步的关键一环，而且是唯一走真实 SQL 的一环：
// revoked_at 在 SQLite 里是文本，比较是按字面量做的。GORM 绑定 time.Time
// 时用的格式必须和写进去的格式一致，否则 `revoked_at >= ?` 一行都查不出来，
// 前端表现就是「对方撤回了，我这边还显示着」。
func TestListRevokedSinceFindsRevocationAfterCursor(t *testing.T) {
	store := setupStore(t)
	message := seedMessage(t, store, 1, supportdomain.SenderTypeUser, "手滑了")

	cursor := time.Now()
	revokedAt := cursor.Add(time.Second)
	if err := store.RevokeMessage(message.ID, revokedAt); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	got, err := store.ListRevokedSince(1, cursor, supportcontract.RevokedFeedLimit)
	if err != nil {
		t.Fatalf("list revoked: %v", err)
	}
	if len(got) != 1 || got[0].ID != message.ID {
		t.Fatalf("游标之后的撤回没查出来，实际 %#v", got)
	}
	if got[0].RevokedAt == nil {
		t.Fatal("查回来的行必须带 revoked_at")
	}
}

// 服务端把最新的 revoked_at 当游标发给前端，前端原样带回。
// 这一轮回必须闭环：拿自己刚发出去的游标再查一次，那条应当还在（>= 语义）。
func TestListRevokedSinceRoundTripsItsOwnCursor(t *testing.T) {
	store := setupStore(t)
	message := seedMessage(t, store, 1, supportdomain.SenderTypeUser, "手滑了")
	if err := store.RevokeMessage(message.ID, time.Now()); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	first, err := store.ListRevokedSince(1, time.Time{}, supportcontract.RevokedFeedLimit)
	if err != nil {
		t.Fatalf("first list: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("不带游标应当返回全部撤回，实际 %d 条", len(first))
	}

	// 服务端会把 first[0].RevokedAt 作为游标回传，前端下一轮原样带回来。
	// 用 >= 而不是 >，所以这条应当还在——重复无害，但漏掉就永远补不回来了。
	second, err := store.ListRevokedSince(1, *first[0].RevokedAt, supportcontract.RevokedFeedLimit)
	if err != nil {
		t.Fatalf("second list: %v", err)
	}
	if len(second) != 1 || second[0].ID != message.ID {
		t.Fatalf("拿自己发出的游标回查应当仍能命中，实际 %#v", second)
	}
}

// 游标之前的撤回不该重复回传。
func TestListRevokedSinceExcludesOlderRevocations(t *testing.T) {
	store := setupStore(t)
	old := seedMessage(t, store, 1, supportdomain.SenderTypeAgent, "老的")
	fresh := seedMessage(t, store, 1, supportdomain.SenderTypeUser, "新的")

	base := time.Now()
	if err := store.RevokeMessage(old.ID, base); err != nil {
		t.Fatalf("revoke old: %v", err)
	}
	if err := store.RevokeMessage(fresh.ID, base.Add(2*time.Second)); err != nil {
		t.Fatalf("revoke fresh: %v", err)
	}

	got, err := store.ListRevokedSince(1, base.Add(time.Second), supportcontract.RevokedFeedLimit)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != fresh.ID {
		t.Fatalf("只应回传游标之后的那条，实际 %#v", got)
	}
}

// 按 revoked_at 倒序，首条即最新——服务层直接拿它当下一个游标。
func TestListRevokedSinceOrdersNewestFirst(t *testing.T) {
	store := setupStore(t)
	first := seedMessage(t, store, 1, supportdomain.SenderTypeAgent, "一")
	second := seedMessage(t, store, 1, supportdomain.SenderTypeUser, "二")

	base := time.Now()
	if err := store.RevokeMessage(first.ID, base); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := store.RevokeMessage(second.ID, base.Add(time.Second)); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	got, err := store.ListRevokedSince(1, time.Time{}, supportcontract.RevokedFeedLimit)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].ID != second.ID {
		t.Fatalf("应当按撤回时间倒序，实际 %#v", got)
	}
}

// 别的会话的撤回不能串进来。
func TestListRevokedSinceIsScopedToConversation(t *testing.T) {
	store := setupStore(t)
	mine := seedMessage(t, store, 1, supportdomain.SenderTypeUser, "我的")
	other := seedMessage(t, store, 2, supportdomain.SenderTypeUser, "别人的")

	now := time.Now()
	if err := store.RevokeMessage(mine.ID, now); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := store.RevokeMessage(other.ID, now); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	got, err := store.ListRevokedSince(1, time.Time{}, supportcontract.RevokedFeedLimit)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != mine.ID {
		t.Fatalf("串会话了，实际 %#v", got)
	}
}

// 未撤回的消息不该出现在撤回增量里。
func TestListRevokedSinceIgnoresLiveMessages(t *testing.T) {
	store := setupStore(t)
	seedMessage(t, store, 1, supportdomain.SenderTypeUser, "还在")

	got, err := store.ListRevokedSince(1, time.Time{}, supportcontract.RevokedFeedLimit)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("没有撤回时不该有返回，实际 %#v", got)
	}
}

// ListMessages 的整页与增量分支都会带出已撤回的行（前端靠 revoked_at 渲染），
// 这条锁住它不被误加过滤条件。
func TestListMessagesStillReturnsRevokedRows(t *testing.T) {
	store := setupStore(t)
	message := seedMessage(t, store, 1, supportdomain.SenderTypeUser, "撤了的")
	if err := store.RevokeMessage(message.ID, time.Now()); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	page, _, err := store.ListMessages(supportcontract.MessageListFilter{ConversationID: 1, Page: 1, PageSize: 30})
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if len(page) != 1 || page[0].RevokedAt == nil {
		t.Fatalf("整页应当带出撤回行且保留 revoked_at，实际 %#v", page)
	}
}
