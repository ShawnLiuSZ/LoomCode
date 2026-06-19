package agent

import (
	"context"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

func TestTaskStatus(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskPending, "pending"},
		{TaskRunning, "running"},
		{TaskCompleted, "completed"},
		{TaskFailed, "failed"},
		{TaskCancelled, "cancelled"},
		{TaskStatus(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("TaskStatus.String() = %q, want %q", got, tt.expected)
		}
	}
}

func TestWorker(t *testing.T) {
	t.Run("NewWorker", func(t *testing.T) {
		agent := &Agent{}
		worker := NewWorker("w1", agent)
		if worker == nil {
			t.Fatal("expected non-nil worker")
		}
		if worker.ID != "w1" {
			t.Errorf("expected ID w1, got %s", worker.ID)
		}
	})

	t.Run("IsBusy", func(t *testing.T) {
		agent := &Agent{}
		worker := NewWorker("w1", agent)
		if worker.IsBusy() {
			t.Error("expected worker to not be busy")
		}
	})
}

func TestScheduler(t *testing.T) {
	t.Run("NewScheduler", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		if scheduler == nil {
			t.Fatal("expected non-nil scheduler")
		}
	})

	t.Run("Submit", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		task := scheduler.Submit("test", "input")
		if task == nil {
			t.Fatal("expected non-nil task")
		}
		if task.Status != TaskPending {
			t.Errorf("expected pending status, got %s", task.Status)
		}
	})

	t.Run("GetTask", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		task := scheduler.Submit("test", "input")

		got := scheduler.GetTask(task.ID)
		if got == nil {
			t.Fatal("expected to find task")
		}
		if got.ID != task.ID {
			t.Errorf("expected task ID %s, got %s", task.ID, got.ID)
		}
	})

	t.Run("GetTask_NotFound", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		got := scheduler.GetTask("nonexistent")
		if got != nil {
			t.Error("expected nil task")
		}
	})

	t.Run("GetTasks", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		scheduler.Submit("test1", "input1")
		scheduler.Submit("test2", "input2")

		tasks := scheduler.GetTasks()
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(tasks))
		}
	})

	t.Run("GetPendingTasks", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		scheduler.Submit("test1", "input1")
		scheduler.Submit("test2", "input2")

		tasks := scheduler.GetPendingTasks()
		if len(tasks) != 2 {
			t.Errorf("expected 2 pending tasks, got %d", len(tasks))
		}
	})

	t.Run("Cancel", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		task := scheduler.Submit("test", "input")

		ok := scheduler.Cancel(task.ID)
		if !ok {
			t.Error("expected cancel to succeed")
		}

		got := scheduler.GetTask(task.ID)
		if got.Status != TaskCancelled {
			t.Errorf("expected cancelled status, got %s", got.Status)
		}
	})

	t.Run("Cancel_NotFound", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		ok := scheduler.Cancel("nonexistent")
		if ok {
			t.Error("expected cancel to fail")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		scheduler := NewScheduler(nil)
		scheduler.Submit("test1", "input1")
		scheduler.Submit("test2", "input2")

		pending, running, completed, failed := scheduler.GetStats()
		if pending != 2 {
			t.Errorf("expected 2 pending, got %d", pending)
		}
		if running != 0 {
			t.Errorf("expected 0 running, got %d", running)
		}
		if completed != 0 {
			t.Errorf("expected 0 completed, got %d", completed)
		}
		if failed != 0 {
			t.Errorf("expected 0 failed, got %d", failed)
		}
	})
}

// mockProviderForTest 模拟 Provider
type mockProviderForTest struct{}

func (m *mockProviderForTest) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	return &provider.ChatResponse{Content: "mock response"}, nil
}

func (m *mockProviderForTest) Stream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	ch := make(chan provider.StreamEvent, 1)
	ch <- provider.StreamEvent{Type: provider.EventText, Content: "mock stream"}
	close(ch)
	return ch, nil
}

func (m *mockProviderForTest) Name() string                              { return "mock" }
func (m *mockProviderForTest) Models() []provider.ModelInfo              { return nil }
func (m *mockProviderForTest) Capabilities() provider.Capabilities       { return provider.Capabilities{} }
func (m *mockProviderForTest) Cost(modelID string, usage provider.Usage) provider.Cost {
	return provider.Cost{}
}

func TestDistributedAgent(t *testing.T) {
	t.Run("NewDistributedAgent", func(t *testing.T) {
		providers := []provider.Provider{&mockProviderForTest{}}
		da := NewDistributedAgent(providers, nil, 2)
		if da == nil {
			t.Fatal("expected non-nil distributed agent")
		}
	})

	t.Run("Submit", func(t *testing.T) {
		providers := []provider.Provider{&mockProviderForTest{}}
		da := NewDistributedAgent(providers, nil, 2)
		task := da.Submit("test", "input")
		if task == nil {
			t.Fatal("expected non-nil task")
		}
	})

	t.Run("GetTask", func(t *testing.T) {
		providers := []provider.Provider{&mockProviderForTest{}}
		da := NewDistributedAgent(providers, nil, 2)
		task := da.Submit("test", "input")

		got := da.GetTask(task.ID)
		if got == nil {
			t.Fatal("expected to find task")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		providers := []provider.Provider{&mockProviderForTest{}}
		da := NewDistributedAgent(providers, nil, 2)
		da.Submit("test1", "input1")

		pending, running, completed, failed := da.GetStats()
		if pending != 1 {
			t.Errorf("expected 1 pending, got %d", pending)
		}
		if running != 0 {
			t.Errorf("expected 0 running, got %d", running)
		}
		if completed != 0 {
			t.Errorf("expected 0 completed, got %d", completed)
		}
		if failed != 0 {
			t.Errorf("expected 0 failed, got %d", failed)
		}
	})
}

func TestWorkerStats(t *testing.T) {
	agent := &Agent{}
	worker := NewWorker("w1", agent)

	completed, failed := worker.Stats()
	if completed != 0 {
		t.Errorf("expected 0 completed, got %d", completed)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
}

func TestSchedulerClose(t *testing.T) {
	scheduler := NewScheduler(nil)
	scheduler.Close()
	// 应该不会 panic
}
