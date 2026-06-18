package tool

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestExecutor_MaxParallel(t *testing.T) {
	r := NewRegistry()
	e := NewExecutor(r)

	if e.MaxParallel() != 3 {
		t.Errorf("default MaxParallel = %d, want 3", e.MaxParallel())
	}

	e.SetMaxParallel(8)
	if e.MaxParallel() != 8 {
		t.Errorf("MaxParallel after set = %d, want 8", e.MaxParallel())
	}

	e.SetMaxParallel(0) // 自动修正为 1
	if e.MaxParallel() != 1 {
		t.Errorf("MaxParallel with 0 = %d, want 1", e.MaxParallel())
	}

	e.SetMaxParallel(20) // 上限 16
	if e.MaxParallel() != 16 {
		t.Errorf("MaxParallel with 20 = %d, want 16", e.MaxParallel())
	}
}

func TestExecutor_Partition(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&GrepTool{})

	e := NewExecutor(r)

	read, write := e.partition([]Call{
		{Name: "read_file", Args: map[string]any{"path": "/tmp/a"}},
		{Name: "write_file", Args: map[string]any{"path": "/tmp/b"}},
		{Name: "grep", Args: map[string]any{"pattern": "x"}},
	})

	if len(read) != 2 {
		t.Errorf("read count = %d, want 2", len(read))
	}
	if len(write) != 1 {
		t.Errorf("write count = %d, want 1", len(write))
	}
}

func TestExecutor_ParallelExecution(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})

	e := NewExecutor(r)
	e.SetMaxParallel(5)

	dir := t.TempDir()

	var callCount int32
	// 使用自定义工具来验证并行
	r2 := NewRegistry()
	r2.Register(&stubReadTool{fn: func() { atomic.AddInt32(&callCount, 1) }})

	e2 := NewExecutor(r2)
	e2.SetMaxParallel(5)

	calls := make([]Call, 10)
	for i := range calls {
		calls[i] = Call{Name: "read_file", Args: map[string]any{"path": dir}}
	}

	e2.Execute(context.Background(), calls)

	if atomic.LoadInt32(&callCount) != 10 {
		t.Errorf("callCount = %d, want 10", callCount)
	}
}

type stubReadTool struct {
	fn func()
}

func (s *stubReadTool) Name() string          { return "read_file" }
func (s *stubReadTool) Description() string   { return "stub" }
func (s *stubReadTool) Schema() Schema        { return Schema{Type: "object"} }
func (s *stubReadTool) IsReadOnly() bool      { return true }
func (s *stubReadTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	if s.fn != nil {
		s.fn()
	}
	return &Result{Content: "ok"}, nil
}

func TestExecutor_SerialWriteExecution(t *testing.T) {
	r := NewRegistry()
	r.Register(&WriteFileTool{})

	e := NewExecutor(r)

	dir := t.TempDir()
	path := dir + "/test.txt"

	results := e.Execute(context.Background(), []Call{
		{Name: "write_file", Args: map[string]any{"path": path, "content": "first"}},
		{Name: "write_file", Args: map[string]any{"path": path, "content": "second"}},
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].OK() {
		t.Errorf("result[0] error: %s", results[0].Error)
	}
	if !results[1].OK() {
		t.Errorf("result[1] error: %s", results[1].Error)
	}
}

func TestExecutor_MixedExecution(t *testing.T) {
	r := NewRegistry()
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})

	e := NewExecutor(r)
	e.SetMaxParallel(2)

	dir := t.TempDir()
	path := dir + "/mixed.txt"

	// 先写入文件
	wt := &WriteFileTool{}
	wt.Execute(context.Background(), map[string]any{"path": path, "content": "hello"})

	results := e.Execute(context.Background(), []Call{
		{Name: "read_file", Args: map[string]any{"path": path}},   // 只读（并行）
		{Name: "write_file", Args: map[string]any{"path": path, "content": "updated"}}, // 写入（串行）
		{Name: "read_file", Args: map[string]any{"path": path}},   // 只读（并行）
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// 验证结果顺序与调用顺序一致
	if !results[0].OK() {
		t.Errorf("results[0] error: %s", results[0].Error)
	}
	if !results[1].OK() {
		t.Errorf("results[1] error: %s", results[1].Error)
	}
	if !results[2].OK() {
		t.Errorf("results[2] error: %s", results[2].Error)
	}
}

func TestExecutor_EmptyCalls(t *testing.T) {
	r := NewRegistry()
	e := NewExecutor(r)

	results := e.Execute(context.Background(), nil)
	if results != nil {
		t.Error("expected nil for empty calls")
	}
}
