package contextpkg

import (
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

func TestIncrementalContext_Append(t *testing.T) {
	c := NewIncrementalContext()

	c.Append(provider.Message{Role: "user", Content: "hello"})
	c.Append(provider.Message{Role: "assistant", Content: "hi"})

	if c.Len() != 2 {
		t.Errorf("Len() = %d, want 2", c.Len())
	}
}

func TestIncrementalContext_IsDirty(t *testing.T) {
	c := NewIncrementalContext()

	if c.IsDirty() {
		t.Error("new context should be clean")
	}

	c.Append(provider.Message{Role: "user", Content: "test"})
	if !c.IsDirty() {
		t.Error("context should be dirty after append")
	}

	c.MarkClean()
	if c.IsDirty() {
		t.Error("context should be clean after MarkClean")
	}
}

func TestIncrementalContext_Version(t *testing.T) {
	c := NewIncrementalContext()

	if c.Version() != 0 {
		t.Errorf("initial version = %d, want 0", c.Version())
	}

	c.Append(provider.Message{Role: "user", Content: "msg1"})
	if c.Version() != 1 {
		t.Errorf("version after 1st append = %d, want 1", c.Version())
	}

	c.Append(provider.Message{Role: "user", Content: "msg2"})
	if c.Version() != 2 {
		t.Errorf("version after 2nd append = %d, want 2", c.Version())
	}
}

func TestIncrementalContext_Messages(t *testing.T) {
	c := NewIncrementalContext()
	c.Append(provider.Message{Role: "user", Content: "a"})
	c.Append(provider.Message{Role: "assistant", Content: "b"})

	msgs := c.Messages()
	if len(msgs) != 2 {
		t.Errorf("Messages() count = %d, want 2", len(msgs))
	}

	// 返回的是副本
	msgs[0].Content = "modified"
	if c.Messages()[0].Content != "a" {
		t.Error("Messages() should return a copy")
	}
}

func TestIncrementalContext_Last(t *testing.T) {
	c := NewIncrementalContext()
	c.Append(provider.Message{Role: "user", Content: "1"})
	c.Append(provider.Message{Role: "assistant", Content: "2"})
	c.Append(provider.Message{Role: "user", Content: "3"})

	last := c.Last(2)
	if len(last) != 2 {
		t.Errorf("Last(2) count = %d, want 2", len(last))
	}
	if last[0].Content != "2" {
		t.Errorf("last[0] = %q", last[0].Content)
	}
	if last[1].Content != "3" {
		t.Errorf("last[1] = %q", last[1].Content)
	}

	// n 超过长度
	all := c.Last(10)
	if len(all) != 3 {
		t.Errorf("Last(10) count = %d, want 3", len(all))
	}
}
