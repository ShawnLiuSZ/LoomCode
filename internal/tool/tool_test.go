package tool

import (
	"context"
	"os"
	"testing"
)

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	tl := &ReadFileTool{}

	err := r.Register(tl)
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	// 重复注册应报错
	err = r.Register(tl)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})

	tl, ok := r.Get("read_file")
	if !ok {
		t.Fatal("expected tool to be found")
	}
	if tl.Name() != "read_file" {
		t.Errorf("Name() = %q", tl.Name())
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent tool")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})

	tools := r.List()
	if len(tools) != 2 {
		t.Errorf("List() count = %d, want 2", len(tools))
	}
}

func TestRegistry_RegisterDefaults(t *testing.T) {
	r := NewRegistry()
	r.RegisterDefaults()

	expected := []string{"read_file", "write_file", "edit_file", "bash", "grep", "glob"}
	for _, name := range expected {
		if _, ok := r.Get(name); !ok {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

func TestReadFileTool(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.txt"
	content := "hello world"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tl := &ReadFileTool{}
	result, err := tl.Execute(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Content != content {
		t.Errorf("Content = %q, want %q", result.Content, content)
	}
}

func TestReadFileTool_MissingPath(t *testing.T) {
	tl := &ReadFileTool{}
	_, err := tl.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestReadFileTool_Nonexistent(t *testing.T) {
	tl := &ReadFileTool{}
	_, err := tl.Execute(context.Background(), map[string]any{"path": "/nonexistent/file"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestWriteFileTool(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/output.txt"

	tl := &WriteFileTool{}
	result, err := tl.Execute(context.Background(), map[string]any{
		"path":    path,
		"content": "new content",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected result content")
	}

	// 验证文件已写入
	data, _ := os.ReadFile(path)
	if string(data) != "new content" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestWriteFileTool_MissingArgs(t *testing.T) {
	tl := &WriteFileTool{}
	_, err := tl.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestEditFileTool(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/edit.txt"
	os.WriteFile(path, []byte("Hello, World!\nGoodbye, Earth!"), 0644)

	tl := &EditFileTool{}
	_, err := tl.Execute(context.Background(), map[string]any{
		"path":     path,
		"old_text": "World", // unique match
		"new_text": "LoomCode",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "Hello, LoomCode!\nGoodbye, Earth!" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestEditFileTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/edit.txt"
	os.WriteFile(path, []byte("hello"), 0644)

	tl := &EditFileTool{}
	_, err := tl.Execute(context.Background(), map[string]any{
		"path":     path,
		"old_text": "nonexistent",
		"new_text": "replacement",
	})
	if err == nil {
		t.Error("expected error for old_text not found")
	}
}

func TestBashTool(t *testing.T) {
	tl := &BashTool{}
	result, err := tl.Execute(context.Background(), map[string]any{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Content != "hello\n" {
		t.Errorf("Content = %q", result.Content)
	}
}

func TestBashTool_Error(t *testing.T) {
	tl := &BashTool{}
	result, err := tl.Execute(context.Background(), map[string]any{
		"command": "exit 1",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error in result")
	}
}

func TestGrepTool(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/search.txt"
	os.WriteFile(path, []byte("line one\nline two\nhello world"), 0644)

	tl := &GrepTool{}
	result, err := tl.Execute(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected search results")
	}
}

func TestGlobTool(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/test.go", []byte("package main"), 0644)
	os.WriteFile(dir+"/test.txt", []byte("text"), 0644)

	tl := &GlobTool{}
	result, err := tl.Execute(context.Background(), map[string]any{
		"pattern": "*.go",
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected glob results")
	}
}

func TestExecutor_Execute(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})

	dir := t.TempDir()
	path := dir + "/data.txt"
	os.WriteFile(path, []byte("test data"), 0644)

	executor := NewExecutor(r)
	results := executor.Execute(context.Background(), []Call{
		{Name: "read_file", Args: map[string]any{"path": path}},
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "test data" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestExecutor_UnknownTool(t *testing.T) {
	r := NewRegistry()
	executor := NewExecutor(r)

	results := executor.Execute(context.Background(), []Call{
		{Name: "nonexistent", Args: map[string]any{}},
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error for unknown tool")
	}
}

func TestExecutor_GuardChain(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})

	executor := NewExecutor(r)
	guardCalled := false
	executor.AddGuard(func(c Call) error {
		guardCalled = true
		return nil
	})

	dir := t.TempDir()
	path := dir + "/guard.txt"
	os.WriteFile(path, []byte("data"), 0644)

	executor.Execute(context.Background(), []Call{
		{Name: "read_file", Args: map[string]any{"path": path}},
	})

	if !guardCalled {
		t.Error("guard was not called")
	}
}

func TestExecutor_GuardBlocks(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})

	executor := NewExecutor(r)
	executor.AddGuard(func(c Call) error {
		return context.DeadlineExceeded // any error blocks execution
	})

	dir := t.TempDir()
	path := dir + "/blocked.txt"
	os.WriteFile(path, []byte("data"), 0644)

	results := executor.Execute(context.Background(), []Call{
		{Name: "read_file", Args: map[string]any{"path": path}},
	})

	if results[0].Error == "" {
		t.Error("expected guard to block execution")
	}
}

func TestResult_OK(t *testing.T) {
	r := &Result{Content: "ok"}
	if !r.OK() {
		t.Error("OK() should be true")
	}

	r = &Result{Error: "failed"}
	if r.OK() {
		t.Error("OK() should be false when Error is set")
	}
}
