package agent

import (
	"context"
	"testing"
	"time"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

func TestSubAgentManager_Spawn(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	m := NewSubAgentManager()

	sa := m.Spawn("explorer", "explore", p, r)
	if sa.ID == "" {
		t.Error("sub-agent ID should not be empty")
	}
	if sa.Name != "explorer" {
		t.Errorf("Name = %q", sa.Name)
	}
	if sa.Status() != StatusPending {
		t.Errorf("initial Status = %v, want pending", sa.Status())
	}
}

func TestSubAgentManager_Get(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	m := NewSubAgentManager()

	sa := m.Spawn("test", "test", p, r)
	got, ok := m.Get(sa.ID)
	if !ok {
		t.Fatal("sub-agent not found")
	}
	if got.ID != sa.ID {
		t.Errorf("ID mismatch")
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent sub-agent")
	}
}

func TestSubAgentManager_List(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	m := NewSubAgentManager()

	m.Spawn("a", "role_a", p, r)
	m.Spawn("b", "role_b", p, r)

	list := m.List()
	if len(list) != 2 {
		t.Errorf("List() count = %d, want 2", len(list))
	}
}

func TestSubAgent_Run(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "sub-agent result"}, nil
	})

	r := tool.NewRegistry()
	m := NewSubAgentManager()
	sa := m.Spawn("worker", "work", p, r)

	sa.Run("do something")
	sa.Wait()

	if sa.Status() != StatusCompleted {
		t.Errorf("Status() = %v, want completed", sa.Status())
	}
	if sa.Result() != "sub-agent result" {
		t.Errorf("Result() = %q", sa.Result())
	}
}

func TestSubAgent_RunError(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return nil, context.DeadlineExceeded
	})

	r := tool.NewRegistry()
	m := NewSubAgentManager()
	sa := m.Spawn("failing", "fail", p, r)

	sa.Run("do something")
	sa.Wait()

	if sa.Status() != StatusFailed {
		t.Errorf("Status() = %v, want failed", sa.Status())
	}
	if sa.Error() == nil {
		t.Error("expected error")
	}
}

func TestSubAgent_Cancel(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		// 检查取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		return &provider.ChatResponse{Content: "slow response"}, nil
	})

	r := tool.NewRegistry()
	m := NewSubAgentManager()
	sa := m.Spawn("slow", "slow", p, r)

	sa.Run("slow task")
	time.Sleep(10 * time.Millisecond)
	m.Cancel(sa.ID)

	sa.Wait()
	if sa.Status() != StatusCancelled {
		t.Errorf("Status() = %v, want cancelled", sa.Status())
	}
}

func TestSubAgent_WaitTimeout(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		time.Sleep(500 * time.Millisecond)
		return &provider.ChatResponse{Content: "too slow"}, nil
	})

	r := tool.NewRegistry()
	m := NewSubAgentManager()
	sa := m.Spawn("timeout", "timeout", p, r)

	sa.Run("slow task")
	err := sa.WaitTimeout(50 * time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestSubAgentManager_RunParallel(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "done"}, nil
	})

	r := tool.NewRegistry()
	m := NewSubAgentManager()

	sa1 := m.Spawn("worker1", "role", p, r)
	sa2 := m.Spawn("worker2", "role", p, r)

	tasks := map[string]string{
		sa1.ID: "task 1",
		sa2.ID: "task 2",
	}

	results := m.RunParallel(tasks)
	if len(results) != 2 {
		t.Errorf("parallel results count = %d, want 2", len(results))
	}
}

func TestSubAgent_SetMaxSteps(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	m := NewSubAgentManager()
	sa := m.Spawn("limited", "limited", p, r)

	sa.SetMaxSteps(5)
	// 验证通过内部 agent 的 maxSteps
	if sa.agent.maxSteps != 5 {
		t.Errorf("maxSteps = %d, want 5", sa.agent.maxSteps)
	}
}

func TestSubAgentStatus_String(t *testing.T) {
	tests := []struct {
		status SubAgentStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if tt.status.String() != tt.want {
			t.Errorf("SubAgentStatus(%d).String() = %q, want %q", tt.status, tt.status.String(), tt.want)
		}
	}
}
