package domain

import (
	"testing"
	"time"
)

func msg(senderType string, senderID uint, age time.Duration, revoked bool) Message {
	now := time.Now()
	m := Message{
		SenderType: senderType,
		SenderID:   senderID,
		CreatedAt:  now.Add(-age),
	}
	if revoked {
		t := now.Add(-age).Add(time.Second)
		m.RevokedAt = &t
	}
	return m
}

func agentMsg(agentID uint, age time.Duration, revoked bool) Message {
	return msg(SenderTypeAgent, agentID, age, revoked)
}

func userMsg(userID uint, age time.Duration, revoked bool) Message {
	return msg(SenderTypeUser, userID, age, revoked)
}

func TestCanRevokeByAllowsOwnRecentAgentMessage(t *testing.T) {
	now := time.Now()
	if !agentMsg(7, 30*time.Second, false).CanRevokeBy(SenderTypeAgent, 7, now) {
		t.Fatal("客服自己刚发的消息应当可撤回")
	}
}

// 买家也能撤自己刚发的，跟微信/Shopee 一致。
func TestCanRevokeByAllowsOwnRecentUserMessage(t *testing.T) {
	now := time.Now()
	if !userMsg(7, 30*time.Second, false).CanRevokeBy(SenderTypeUser, 7, now) {
		t.Fatal("买家自己刚发的消息应当可撤回")
	}
}

// 超过时限就不能撤了，否则「消息已撤回」会在对方早已读过之后才冒出来。
func TestCanRevokeByRejectsAfterWindow(t *testing.T) {
	now := time.Now()
	if agentMsg(7, RevokeWindow+time.Second, false).CanRevokeBy(SenderTypeAgent, 7, now) {
		t.Fatal("超出时限的消息不该可撤回")
	}
	// 边界：恰好等于时限仍然可撤
	if !agentMsg(7, RevokeWindow, false).CanRevokeBy(SenderTypeAgent, 7, now) {
		t.Fatal("恰好在时限上应当仍可撤回")
	}
}

// 不能撤别的客服发的消息。
func TestCanRevokeByRejectsOtherAgent(t *testing.T) {
	if agentMsg(7, time.Second, false).CanRevokeBy(SenderTypeAgent, 8, time.Now()) {
		t.Fatal("不该允许撤回其他客服的消息")
	}
}

// 也不能撤别的买家发的消息。
func TestCanRevokeByRejectsOtherUser(t *testing.T) {
	if userMsg(7, time.Second, false).CanRevokeBy(SenderTypeUser, 8, time.Now()) {
		t.Fatal("不该允许撤回其他买家的消息")
	}
}

// 已撤回的不能再撤，避免重复写入与重复审计记录。
func TestCanRevokeByRejectsAlreadyRevoked(t *testing.T) {
	if agentMsg(7, time.Second, true).CanRevokeBy(SenderTypeAgent, 7, time.Now()) {
		t.Fatal("已撤回的消息不该再次可撤")
	}
}

// 客服撤掉买家说过的话等于篡改证据，任何情况下都不允许。
func TestCanRevokeByRejectsCrossRole(t *testing.T) {
	if userMsg(7, time.Second, false).CanRevokeBy(SenderTypeAgent, 7, time.Now()) {
		t.Fatal("客服不该能撤买家的消息")
	}
	if agentMsg(7, time.Second, false).CanRevokeBy(SenderTypeUser, 7, time.Now()) {
		t.Fatal("买家不该能撤客服的消息")
	}
}

// 这条是本次改动的核心动机：买家 ID 与管理员 ID 是两套独立自增序列，
// 都存在 sender_id 一列里。只比数字的话 5 号客服能撤 5 号买家的消息。
func TestCanRevokeByDoesNotConfuseAgentAndUserIDs(t *testing.T) {
	const collidingID = 5
	buyerMessage := userMsg(collidingID, time.Second, false)

	if buyerMessage.CanRevokeBy(SenderTypeAgent, collidingID, time.Now()) {
		t.Fatal("ID 撞号时客服仍不该能撤买家的消息")
	}
	if !buyerMessage.CanRevokeBy(SenderTypeUser, collidingID, time.Now()) {
		t.Fatal("买家撤自己的消息应当放行")
	}
}

// senderType 传了非法值一律拒绝，别让空串这种默认值意外匹配上。
func TestCanRevokeByRejectsUnknownSenderType(t *testing.T) {
	if agentMsg(7, time.Second, false).CanRevokeBy("", 7, time.Now()) {
		t.Fatal("空的 senderType 不该放行")
	}
	if agentMsg(7, time.Second, false).CanRevokeBy("robot", 7, time.Now()) {
		t.Fatal("未知的 senderType 不该放行")
	}
}

// 撤回后内容不能再出网：不抹的话对端翻一下网络面板就能读到原文，等于没撤。
func TestRedactedBlanksRevokedContent(t *testing.T) {
	m := agentMsg(7, time.Second, true)
	m.Content = "打错了"
	m.Translation = "typo"
	m.MediaURL = "/uploads/a.png"

	got := m.Redacted()

	if got.Content != "" || got.Translation != "" || got.MediaURL != "" {
		t.Fatalf("撤回后内容应被抹掉，实际 content=%q translation=%q media=%q",
			got.Content, got.Translation, got.MediaURL)
	}
	if got.RevokedAt == nil {
		t.Fatal("撤回时间必须保留，前端要靠它渲染「已撤回」")
	}
	// 原对象不能被就地改掉——它可能还要写回库里做审计。
	if m.Content != "打错了" {
		t.Fatal("Redacted 不该修改原对象")
	}
}

func TestRedactedKeepsLiveContent(t *testing.T) {
	m := agentMsg(7, time.Second, false)
	m.Content = "你好"

	if got := m.Redacted(); got.Content != "你好" {
		t.Fatalf("未撤回的消息内容不该被动，实际 %q", got.Content)
	}
}

func TestRedactRevokedOnlyTouchesRevokedOnes(t *testing.T) {
	live := agentMsg(7, time.Second, false)
	live.Content = "还在"
	gone := agentMsg(7, time.Second, true)
	gone.Content = "撤了"

	got := RedactRevoked([]Message{live, gone})

	if got[0].Content != "还在" {
		t.Fatalf("未撤回的被误伤，实际 %q", got[0].Content)
	}
	if got[1].Content != "" {
		t.Fatalf("已撤回的没抹干净，实际 %q", got[1].Content)
	}
}
