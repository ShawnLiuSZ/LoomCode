package agent

import (
	"context"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// H11 第一半：完成后再次 Run 不得 panic（close of closed channel），且 Run 是一次性的。
func TestSubAgent_Run_OnceOnly(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "done"}, nil
	})
	r := tool.NewRegistry()
	m := NewSubAgentManager()
	sa := m.Spawn("worker", "work", p, r)

	sa.Run("task one")
	sa.Wait()
	if sa.Status() != StatusCompleted {
		t.Fatalf("after first run Status = %v, want completed", sa.Status())
	}

	// 二次 Run：必须是 no-op，绝不能再起 goroutine 二次 close(done) 而 panic。
	sa.Run("task two")
	sa.Wait()

	if sa.Status() != StatusCompleted {
		t.Errorf("after second run Status = %v, want completed (one-shot)", sa.Status())
	}
	if sa.Result() != "done" {
		t.Errorf("Result() = %q, want %q", sa.Result(), "done")
	}
}

// H11 加固：所有创建都走单一私有入口 spawn，深度由它原子设置并强制兜底——
// 任何路径都无法创建深度超过 maxSubAgentDepth 的子 agent。
func TestSubAgentManager_Spawn_DepthGuard(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	m := NewSubAgentManager()

	// 超限深度：兜底返回 nil，且不注册。
	if sa := m.spawn("x", "y", "parent", maxSubAgentDepth+1, p, r); sa != nil {
		t.Error("spawn beyond maxSubAgentDepth must return nil")
	}
	if len(m.List()) != 0 {
		t.Errorf("over-depth spawn must not register a sub-agent, got %d", len(m.List()))
	}

	// 合法深度：创建成功并原子设置 ParentID/Depth。
	sa := m.spawn("c", "role", "sub_99", maxSubAgentDepth, p, r)
	if sa == nil {
		t.Fatal("valid-depth spawn returned nil")
	}
	if sa.Depth != maxSubAgentDepth || sa.ParentID != "sub_99" {
		t.Errorf("spawn set Depth=%d ParentID=%q, want %d/%q", sa.Depth, sa.ParentID, maxSubAgentDepth, "sub_99")
	}

	// 顶层 Spawn 始终是 depth 0、无父。
	root := m.Spawn("root", "role", p, r)
	if root.Depth != 0 || root.ParentID != "" {
		t.Errorf("Spawn root Depth=%d ParentID=%q, want 0/empty", root.Depth, root.ParentID)
	}
}

// H11 第二半：SpawnChild 必须设置 ParentID/Depth 并限制递归深度。
func TestSubAgentManager_SpawnChild_DepthLimit(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	m := NewSubAgentManager()

	root := m.Spawn("root", "root", p, r)
	if root.Depth != 0 {
		t.Errorf("root Depth = %d, want 0", root.Depth)
	}

	parent := root
	for d := 1; d <= maxSubAgentDepth; d++ {
		child, err := m.SpawnChild(parent, "child", "role", p, r)
		if err != nil {
			t.Fatalf("depth %d should be allowed: %v", d, err)
		}
		if child.Depth != d {
			t.Errorf("child Depth = %d, want %d", child.Depth, d)
		}
		if child.ParentID != parent.ID {
			t.Errorf("child ParentID = %q, want %q", child.ParentID, parent.ID)
		}
		parent = child
	}

	// 超出最大深度必须报错且不创建子 agent。
	before := len(m.List())
	if _, err := m.SpawnChild(parent, "toodeep", "role", p, r); err == nil {
		t.Error("exceeding maxSubAgentDepth must return an error")
	}
	if len(m.List()) != before {
		t.Error("over-depth SpawnChild must not register a sub-agent")
	}
}
