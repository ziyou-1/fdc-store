package supporthttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"

	"github.com/gin-gonic/gin"
)

// stubSupport 只记录服务层收到了什么，不做业务。
type stubSupport struct {
	gotRevokedSince time.Time
	result          *supportcontract.MessageListResult
}

func (s *stubSupport) EnsureConversation(uint) (*supportdomain.Conversation, error) {
	return &supportdomain.Conversation{ID: 1}, nil
}

func (s *stubSupport) SendUserMessage(uint, supportcontract.SendMessageInput) (*supportdomain.Message, error) {
	return &supportdomain.Message{ID: 1}, nil
}

func (s *stubSupport) RevokeUserMessage(uint, uint) (*supportdomain.Message, error) {
	return &supportdomain.Message{ID: 1}, nil
}

func (s *stubSupport) ListUserMessages(
	_ uint, _ uint, _, _ int, revokedSince time.Time,
) (*supportcontract.MessageListResult, error) {
	s.gotRevokedSince = revokedSince
	return s.result, nil
}

func (s *stubSupport) MarkReadByUser(uint) error { return nil }

func (s *stubSupport) ListQuickReplies(bool) ([]supportdomain.QuickReply, error) { return nil, nil }

func newTestRouter(support *stubSupport) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(7))
		c.Next()
	})
	RegisterUserRoutes(router, NewUserHandler(support, nil))
	return router
}

func getJSON(t *testing.T, router *gin.Engine, target string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码 %d，响应体 %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析响应失败: %v，原文 %s", err, recorder.Body.String())
	}
	return envelope.Data
}

// 响应体里必须有 revoked 与 revoked_cursor 两个字段。
// 前端整条撤回同步都挂在它们身上，字段名写错就是「对方撤回了我这边还显示着」。
func TestListMessagesResponseCarriesRevokedFeed(t *testing.T) {
	revokedAt := time.Date(2026, 9, 9, 16, 17, 31, 405804012, time.FixedZone("CST", 8*3600))
	support := &stubSupport{result: &supportcontract.MessageListResult{
		Conversation:  &supportdomain.Conversation{ID: 1},
		Messages:      []supportdomain.Message{{ID: 9}},
		Total:         1,
		Revoked:       []supportdomain.Message{{ID: 5, RevokedAt: &revokedAt}},
		RevokedCursor: &revokedAt,
	}}

	data := getJSON(t, newTestRouter(support), "/support/messages?after_id=9")

	revoked, ok := data["revoked"].([]any)
	if !ok || len(revoked) != 1 {
		t.Fatalf("响应缺少 revoked 数组，实际 %#v", data["revoked"])
	}
	if data["revoked_cursor"] == nil {
		t.Fatal("响应缺少 revoked_cursor，前端游标会永远停在零值")
	}
	if _, ok := data["messages"]; !ok {
		t.Fatal("messages 字段丢了")
	}
}

// 游标是服务端产出、前端原样带回的 RFC3339 串，里面的 +08:00 在 query string
// 里必须经过百分号编码。这条锁住服务端确实把它解析回了同一个时刻——
// 解析失败会静默退化成「不设下界」，每轮重复回传全部撤回，看着像能用其实没走游标。
func TestRevokedAfterCursorRoundTrips(t *testing.T) {
	original := time.Date(2026, 9, 9, 16, 17, 31, 405804012, time.FixedZone("CST", 8*3600))
	// 服务端就是这样把游标序列化给前端的
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cursor := string(encoded[1 : len(encoded)-1]) // 去掉 JSON 的引号

	support := &stubSupport{result: &supportcontract.MessageListResult{
		Conversation: &supportdomain.Conversation{ID: 1},
	}}
	target := "/support/messages?after_id=9&revoked_after=" + url.QueryEscape(cursor)

	getJSON(t, newTestRouter(support), target)

	if support.gotRevokedSince.IsZero() {
		t.Fatalf("游标 %q 没被解析出来，服务端会退化成不设下界", cursor)
	}
	if !support.gotRevokedSince.Equal(original) {
		t.Fatalf("解析出的时刻与原值不同：want %v got %v", original, support.gotRevokedSince)
	}
}

// 有些客户端不会把 + 编码成 %2B，裸 + 在 query string 里会被解成空格。
// 这时必须仍然认得出来，否则游标永远推不动——表现是每轮重复回传全部撤回。
func TestRevokedAfterToleratesUnencodedPlus(t *testing.T) {
	original := time.Date(2026, 9, 9, 16, 17, 31, 405804012, time.FixedZone("CST", 8*3600))
	support := &stubSupport{result: &supportcontract.MessageListResult{
		Conversation: &supportdomain.Conversation{ID: 1},
	}}
	// 裸 + 会被 net/url 解成空格，服务端拿到的是 "...405804012 08:00"
	target := "/support/messages?revoked_after=2026-09-09T16:17:31.405804012+08:00"

	getJSON(t, newTestRouter(support), target)

	if support.gotRevokedSince.IsZero() {
		t.Fatal("裸 + 的游标应当仍能解析，否则游标永远推不动")
	}
	if !support.gotRevokedSince.Equal(original) {
		t.Fatalf("want %v got %v", original, support.gotRevokedSince)
	}
}

// 没带游标时是零值，服务层据此走「不设下界」的分支。
func TestRevokedAfterAbsentMeansNoLowerBound(t *testing.T) {
	support := &stubSupport{result: &supportcontract.MessageListResult{
		Conversation: &supportdomain.Conversation{ID: 1},
	}}

	getJSON(t, newTestRouter(support), "/support/messages")

	if !support.gotRevokedSince.IsZero() {
		t.Fatalf("没带游标时应当是零值，实际 %v", support.gotRevokedSince)
	}
}

// 垃圾游标不能让整个接口报错——退化成不设下界即可，前端打标是幂等的。
func TestRevokedAfterGarbageDegradesInsteadOfFailing(t *testing.T) {
	support := &stubSupport{result: &supportcontract.MessageListResult{
		Conversation: &supportdomain.Conversation{ID: 1},
	}}

	getJSON(t, newTestRouter(support), "/support/messages?revoked_after=not-a-time")

	if !support.gotRevokedSince.IsZero() {
		t.Fatalf("非法游标应当退化成零值，实际 %v", support.gotRevokedSince)
	}
}
