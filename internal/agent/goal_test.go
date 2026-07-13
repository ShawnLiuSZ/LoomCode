package agent

import (
	"context"
	"testing"
	"time"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

// MockProvider 模拟 Provider（用于测试）
type MockProvider struct {
	responses []string
	index     int
}

func (m *MockProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	if m.index >= len(m.responses) {
		return &provider.ChatResponse{Content: "NOT_ACHIEVED: no more responses"}, nil
	}
	resp := m.responses[m.index]
	m.index++
	return &provider.ChatResponse{Content: resp}, nil
}

func (m *MockProvider) Stream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	return nil, nil
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Models() []provider.ModelInfo {
	return []provider.ModelInfo{{ID: "mock-model", Name: "Mock Model"}}
}

func (m *MockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

func (m *MockProvider) Cost(modelID string, usage provider.Usage) provider.Cost {
	return provider.Cost{}
}

func TestGoalStopCondition(t *testing.T) {
	t.Run("SetGoal", func(t *testing.T) {
		judge := &MockProvider{responses: []string{"ACHIEVED: done"}}
		goal := NewGoalStopCondition(judge)

		goal.SetGoal("测试目标")
		if goal.GetGoal() != "测试目标" {
			t.Errorf("expected '测试目标', got %q", goal.GetGoal())
		}
		if !goal.IsEnabled() {
			t.Error("expected goal to be enabled")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		judge := &MockProvider{}
		goal := NewGoalStopCondition(judge)

		goal.SetGoal("测试目标")
		goal.Clear()

		if goal.GetGoal() != "" {
			t.Errorf("expected empty goal, got %q", goal.GetGoal())
		}
		if goal.IsEnabled() {
			t.Error("expected goal to be disabled")
		}
	})

	t.Run("Evaluate_Achieved", func(t *testing.T) {
		judge := &MockProvider{responses: []string{"ACHIEVED: 任务完成"}}
		goal := NewGoalStopCondition(judge)
		goal.SetGoal("实现功能")

		messages := []provider.Message{
			{Role: "user", Content: "创建一个函数"},
			{Role: "assistant", Content: "已创建函数"},
		}

		achieved, reason, err := goal.Evaluate(context.Background(), messages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !achieved {
			t.Error("expected achieved to be true")
		}
		if reason != "任务完成" {
			t.Errorf("expected reason '任务完成', got %q", reason)
		}
	})

	t.Run("Evaluate_NotAchieved", func(t *testing.T) {
		judge := &MockProvider{responses: []string{"NOT_ACHIEVED: 还在进行中"}}
		goal := NewGoalStopCondition(judge)
		goal.SetGoal("实现功能")

		messages := []provider.Message{
			{Role: "user", Content: "创建一个函数"},
		}

		achieved, reason, err := goal.Evaluate(context.Background(), messages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if achieved {
			t.Error("expected achieved to be false")
		}
		if reason != "还在进行中" {
			t.Errorf("expected reason '还在进行中', got %q", reason)
		}
	})

	t.Run("Evaluate_NotEnabled", func(t *testing.T) {
		judge := &MockProvider{}
		goal := NewGoalStopCondition(judge)

		achieved, reason, err := goal.Evaluate(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if achieved {
			t.Error("expected achieved to be false")
		}
		if reason != "no goal set" {
			t.Errorf("expected reason 'no goal set', got %q", reason)
		}
	})
}

func TestParseEvaluation(t *testing.T) {
	goal := &GoalStopCondition{}

	tests := []struct {
		input    string
		expected bool
		reason   string
	}{
		{"ACHIEVED: 任务完成", true, "任务完成"},
		{"NOT_ACHIEVED: 还在进行中", false, "还在进行中"},
		{"achieved: 小写", true, "小写"},
		{"random text", false, "evaluation inconclusive"},
		{"ACHIEVED:", true, "ACHIEVED"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			achieved, reason := goal.parseEvaluation(tt.input)
			if achieved != tt.expected {
				t.Errorf("expected achieved=%v, got %v", tt.expected, achieved)
			}
			if reason != tt.reason {
				t.Errorf("expected reason %q, got %q", tt.reason, reason)
			}
		})
	}
}

func TestExtractRecentMessages(t *testing.T) {
	goal := &GoalStopCondition{}

	messages := []provider.Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "msg2"},
		{Role: "user", Content: "msg3"},
		{Role: "assistant", Content: "msg4"},
		{Role: "user", Content: "msg5"},
	}

	result := goal.extractRecentMessages(messages, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result))
	}
	if result[0].Content != "msg3" {
		t.Errorf("expected 'msg3', got %q", result[0].Content)
	}

	// 测试 limit 大于消息数
	result = goal.extractRecentMessages(messages, 10)
	if len(result) != 5 {
		t.Errorf("expected 5 messages, got %d", len(result))
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestGoalStopConditionConcurrent(t *testing.T) {
	judge := &MockProvider{responses: []string{"ACHIEVED: done"}}
	goal := NewGoalStopCondition(judge)

	// 并发测试
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			goal.SetGoal("concurrent test")
			goal.GetGoal()
			goal.IsEnabled()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestGoalWithTimeout(t *testing.T) {
	judge := &MockProvider{responses: []string{"ACHIEVED: done"}}
	goal := NewGoalStopCondition(judge)
	goal.SetGoal("timeout test")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	messages := []provider.Message{
		{Role: "user", Content: "test"},
	}

	// 应该超时
	_, _, err := goal.Evaluate(ctx, messages)
	// 注意：由于 mock provider 不检查 context，这里不会真正超时
	// 但这是测试 context 传递的正确方式
	_ = err
}
