package domain

import (
	"testing"
	"time"
)

func agentMsg(agentID uint, age time.Duration, revoked bool) Message {
	now := time.Now()
	m := Message{
		SenderType: SenderTypeAgent,
		SenderID:   agentID,
		CreatedAt:  now.Add(-age),
	}
	if revoked {
		t := now.Add(-age).Add(time.Second)
		m.RevokedAt = &t
	}
	return m
}

func TestCanRevokeByAllowsOwnRecentAgentMessage(t *testing.T) {
	now := time.Now()
	if !agentMsg(7, 30*time.Second, false).CanRevokeBy(7, now) {
		t.Fatal("自己刚发的消息应当可撤回")
	}
}

// 超过时限就不能撤了，否则「消息已撤回」会在买家早已读过之后才冒出来。
func TestCanRevokeByRejectsAfterWindow(t *testing.T) {
	now := time.Now()
	if agentMsg(7, RevokeWindow+time.Second, false).CanRevokeBy(7, now) {
		t.Fatal("超出时限的消息不该可撤回")
	}
	// 边界：恰好等于时限仍然可撤
	if !agentMsg(7, RevokeWindow, false).CanRevokeBy(7, now) {
		t.Fatal("恰好在时限上应当仍可撤回")
	}
}

// 不能撤别的客服发的消息。
func TestCanRevokeByRejectsOtherAgent(t *testing.T) {
	if agentMsg(7, time.Second, false).CanRevokeBy(8, time.Now()) {
		t.Fatal("不该允许撤回其他客服的消息")
	}
}

// 已撤回的不能再撤，避免重复写入与重复审计记录。
func TestCanRevokeByRejectsAlreadyRevoked(t *testing.T) {
	if agentMsg(7, time.Second, true).CanRevokeBy(7, time.Now()) {
		t.Fatal("已撤回的消息不该再次可撤")
	}
}

// 买家消息一律不可撤——客服撤掉买家说过的话等于篡改证据。
func TestCanRevokeByRejectsUserMessage(t *testing.T) {
	m := Message{SenderType: SenderTypeUser, SenderID: 7, CreatedAt: time.Now()}
	if m.CanRevokeBy(7, time.Now()) {
		t.Fatal("买家消息不该被客服撤回")
	}
}
