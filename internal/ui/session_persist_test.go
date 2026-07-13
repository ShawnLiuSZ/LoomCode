package ui

import (
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/session"
)

// H3: 即使启动时没有 --session，saveSession 也应懒创建并持久化会话，
// 使对话在重启后可恢复（而不是默认用户的对话全部丢失）。
func TestSaveSession_LazyCreatesAndPersists(t *testing.T) {
	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	app.SetModel("test-model")
	app.SetSessionManager(mgr)
	// 注意：未调用 RestoreSession，模拟首次启动的默认会话场景。

	// 从干净状态开始（NewApp 会预置欢迎消息）。
	app.messages = []chatMessage{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "在的"},
	}
	app.savedMsgCount = 0
	app.saveSession()

	// 从磁盘重新加载，验证消息确实落盘。
	mgr2, err := session.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	sessions := mgr2.List()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 persisted session, got %d", len(sessions))
	}
	got := sessions[0]
	if len(got.Messages) != 2 {
		t.Fatalf("expected 2 persisted messages, got %d", len(got.Messages))
	}
	if got.Messages[0].Content != "你好" || got.Messages[1].Content != "在的" {
		t.Errorf("persisted messages = %+v", got.Messages)
	}
}
