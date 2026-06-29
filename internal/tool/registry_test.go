package tool

import (
	"context"
	"testing"
)

// fakeTool 仅用于测试，实现 Tool 接口，Name() 由构造时指定。
type fakeTool struct {
	name string
}

func (f *fakeTool) Name() string        { return f.name }
func (f *fakeTool) Description() string { return "fake tool for testing" }
func (f *fakeTool) Schema() Schema      { return Schema{Type: "object"} }
func (f *fakeTool) IsReadOnly() bool    { return true }
func (f *fakeTool) Execute(_ context.Context, _ map[string]any) (*Result, error) {
	return &Result{Content: f.name}, nil
}

func TestRegistry_ListStableOrder(t *testing.T) {
	r := NewRegistry()

	// 故意打乱注册顺序
	names := []string{"zebra", "apple", "mango", "banana", "cherry"}
	for _, n := range names {
		if err := r.Register(&fakeTool{name: n}); err != nil {
			t.Fatalf("Register(%q) error: %v", n, err)
		}
	}

	first := r.List()
	second := r.List()

	// 1. 两次调用顺序必须完全一致
	if len(first) != len(second) {
		t.Fatalf("len mismatch: first=%d second=%d", len(first), len(second))
	}
	for i := range first {
		if first[i].Name() != second[i].Name() {
			t.Fatalf("order not stable at index %d: first=%q second=%q",
				i, first[i].Name(), second[i].Name())
		}
	}

	// 2. 顺序必须为字典序升序
	want := []string{"apple", "banana", "cherry", "mango", "zebra"}
	if len(first) != len(want) {
		t.Fatalf("len = %d, want %d", len(first), len(want))
	}
	for i, w := range want {
		if first[i].Name() != w {
			t.Errorf("index %d: Name() = %q, want %q", i, first[i].Name(), w)
		}
	}
}
