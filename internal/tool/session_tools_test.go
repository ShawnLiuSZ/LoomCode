package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/session"
)

func TestListSessionsTool_RegisteredByDefault(t *testing.T) {
	r := NewRegistry()
	r.RegisterDefaults()

	for _, name := range []string{"list_sessions", "read_session"} {
		tool, ok := r.Get(name)
		if !ok {
			t.Fatalf("expected %q to be registered by default", name)
		}
		if !tool.IsReadOnly() {
			t.Fatalf("%q must be read-only", name)
		}
	}
}

func TestListSessionsTool_NoManager(t *testing.T) {
	tool := &ListSessionsTool{}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Content, "No session manager") {
		t.Fatalf("expected placeholder content, got %q", res.Content)
	}
}

func TestListSessionsTool_ListAndLimit(t *testing.T) {
	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	mgr.Create("first", "m1", "p1")
	mgr.Create("second", "m2", "p2")

	tool := &ListSessionsTool{mgr: mgr}

	// 默认 limit=20，应返回 2 条
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var all []map[string]any
	if err := json.Unmarshal([]byte(res.Content), &all); err != nil {
		t.Fatalf("unmarshal: %v\ncontent: %s", err, res.Content)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(all))
	}

	// limit=1 应只返回最近 1 条
	res, err = tool.Execute(context.Background(), map[string]any{"limit": float64(1)})
	if err != nil {
		t.Fatalf("execute with limit: %v", err)
	}
	var limited []map[string]any
	if err := json.Unmarshal([]byte(res.Content), &limited); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("want 1 session, got %d", len(limited))
	}
}

func TestReadSessionTool_NoManager(t *testing.T) {
	tool := &ReadSessionTool{}
	res, err := tool.Execute(context.Background(), map[string]any{"session_id": "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Content, "No session manager") {
		t.Fatalf("expected placeholder content, got %q", res.Content)
	}
}

func TestReadSessionTool_MissingID(t *testing.T) {
	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	tool := &ReadSessionTool{mgr: mgr}

	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Content, "Missing required argument") {
		t.Fatalf("expected missing arg message, got %q", res.Content)
	}
}

func TestReadSessionTool_ReadRecentMessages(t *testing.T) {
	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	s := mgr.Create("test", "m", "p")
	if s == nil {
		t.Fatal("session create failed")
	}

	// 添加 3 条消息
	for i := 0; i < 3; i++ {
		mgr.AddMessage(session.Message{Role: "user", Content: "msg"})
	}

	tool := &ReadSessionTool{mgr: mgr}
	res, err := tool.Execute(context.Background(), map[string]any{"session_id": s.ID, "limit": float64(2)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	var out struct {
		ID       string `json:"id"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
		t.Fatalf("unmarshal: %v\ncontent: %s", err, res.Content)
	}
	if out.ID != s.ID {
		t.Fatalf("want session id %q, got %q", s.ID, out.ID)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("want 2 messages (limit), got %d", len(out.Messages))
	}
}

func TestSetSessionManagerForTools(t *testing.T) {
	r := NewRegistry()
	r.RegisterDefaults()

	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	SetSessionManagerForTools(r, mgr)

	listTool, _ := r.Get("list_sessions")
	if lt, ok := listTool.(*ListSessionsTool); !ok || lt.mgr == nil {
		t.Fatal("list_sessions manager not injected")
	}
	readTool, _ := r.Get("read_session")
	if rt, ok := readTool.(*ReadSessionTool); !ok || rt.mgr == nil {
		t.Fatal("read_session manager not injected")
	}
}

func TestReadSessionTool_ContentTruncation(t *testing.T) {
	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	s := mgr.Create("long", "m", "p")
	if s == nil {
		t.Fatal("session create failed")
	}

	longContent := strings.Repeat("a", 600)
	mgr.AddMessage(session.Message{Role: "assistant", Content: longContent})

	tool := &ReadSessionTool{mgr: mgr}
	res, err := tool.Execute(context.Background(), map[string]any{"session_id": s.ID})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(res.Content, "...") {
		t.Fatal("expected content truncation marker")
	}
}

// 确保默认注册不会污染已有工具名称；与注册顺序无关。
func TestSessionTools_RegistrationIdempotent(t *testing.T) {
	r := NewRegistry()
	r.RegisterDefaults()

	// 第二次注册同名工具应返回错误，不会 panic。
	if err := r.Register(&ListSessionsTool{}); err == nil {
		t.Fatal("expected duplicate registration error")
	}
	if err := r.Register(&ReadSessionTool{}); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestListSessionsTool_EmptyList(t *testing.T) {
	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	tool := &ListSessionsTool{mgr: mgr}

	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(res.Content, "No historical sessions") {
		t.Fatalf("expected empty list message, got %q", res.Content)
	}
}

func TestReadSessionTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	mgr, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	tool := &ReadSessionTool{mgr: mgr}

	res, err := tool.Execute(context.Background(), map[string]any{"session_id": "missing"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(res.Content, "Failed to read session") {
		t.Fatalf("expected not-found message, got %q", res.Content)
	}
}

// 验证真实 Manager 从磁盘加载后，工具仍能读取历史消息。
func TestSessionTools_ReloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	mgr1, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager 1: %v", err)
	}
	s := mgr1.Create("reload", "m", "p")
	if s == nil {
		t.Fatal("session create failed")
	}
	mgr1.AddMessage(session.Message{Role: "user", Content: "hello"})

	// 重新创建 manager，模拟进程重启后的状态。
	mgr2, err := session.NewManager(dir)
	if err != nil {
		t.Fatalf("create manager 2: %v", err)
	}

	tool := &ReadSessionTool{mgr: mgr2}
	res, err := tool.Execute(context.Background(), map[string]any{"session_id": s.ID})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(res.Content, "hello") {
		t.Fatalf("expected reloaded message content, got %q", res.Content)
	}

	// 清理
	_ = os.RemoveAll(filepath.Join(dir, "*.jsonl"))
}
