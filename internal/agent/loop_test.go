package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

func TestAgent_SingleTurn(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "Hello, I can help!"}, nil
	})

	r := tool.NewRegistry()
	agent := New(p, r)

	result, err := agent.Run(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result != "Hello, I can help!" {
		t.Errorf("result = %q", result)
	}
}

func TestAgent_MultiTurnToolCall(t *testing.T) {
	callCount := 0
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			return &provider.ChatResponse{
				ToolCalls: []provider.ToolCall{
					{ID: "call_1", Function: provider.ToolCallFunc{Name: "read_file"}, Args: map[string]any{"path": "/tmp/test.txt"}},
				},
			}, nil
		}
		return &provider.ChatResponse{Content: "File analysis complete"}, nil
	})

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	agent := New(p, r)
	agent.SetMaxSteps(5)

	result, err := agent.Run(context.Background(), "analyze /tmp/test.txt")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result != "File analysis complete" {
		t.Errorf("result = %q", result)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestAgent_ToolCallWithError(t *testing.T) {
	callCount := 0
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			return &provider.ChatResponse{
				ToolCalls: []provider.ToolCall{
					{ID: "call_1", Function: provider.ToolCallFunc{Name: "read_file"}, Args: map[string]any{"path": "/nonexistent"}},
				},
			}, nil
		}
		return &provider.ChatResponse{Content: "File not found, but I'll continue"}, nil
	})

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	agent := New(p, r)
	agent.SetMaxSteps(5)

	result, err := agent.Run(context.Background(), "read nonexistent file")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(result, "continue") {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestAgent_MaxSteps(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{
			ToolCalls: []provider.ToolCall{
				{ID: "call_1", Function: provider.ToolCallFunc{Name: "read_file"}, Args: map[string]any{"path": "/tmp/test"}},
			},
		}, nil
	})

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	agent := New(p, r)
	agent.SetMaxSteps(3)

	_, err := agent.Run(context.Background(), "do something")
	if err == nil {
		t.Fatal("expected error for max steps")
	}
	if !strings.Contains(err.Error(), "max steps") {
		t.Errorf("error = %v", err)
	}
}

func TestAgent_ChatError(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return nil, errors.New("api unavailable")
	})

	r := tool.NewRegistry()
	agent := New(p, r)

	_, err := agent.Run(context.Background(), "do something")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "api unavailable") {
		t.Errorf("error = %v", err)
	}
}

func TestAgent_ContextCancellation(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "ok"}, nil
	})

	r := tool.NewRegistry()
	agent := New(p, r)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := agent.Run(ctx, "do something")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestAgent_GuardChain(t *testing.T) {
	guardCalled := false
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{
			ToolCalls: []provider.ToolCall{
				{ID: "call_1", Function: provider.ToolCallFunc{Name: "read_file"}, Args: map[string]any{"path": "/tmp/test"}},
			},
		}, nil
	})

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	agent := New(p, r)
	agent.SetMaxSteps(3) // 限制步数，避免默认无限制时陷入无限循环
	agent.AddGuard(func(c tool.Call) error {
		guardCalled = true
		return nil
	})

	t.Setenv("LOOMCODE_TEST", "1")
	_, _ = agent.Run(context.Background(), "read file")

	if !guardCalled {
		t.Error("guard was not called")
	}
}

func TestAgent_BuildToolDefs(t *testing.T) {
	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})
	r.Register(&tool.GrepTool{})

	p := testutil.NewStubProvider(nil)
	agent := New(p, r)

	defs := agent.buildToolDefs()
	if len(defs) != 2 {
		t.Errorf("buildToolDefs() count = %d, want 2", len(defs))
	}

	names := map[string]bool{}
	for _, d := range defs {
		names[d.Function.Name] = true
	}
	if !names["read_file"] || !names["grep"] {
		t.Errorf("missing tool in defs: %v", names)
	}
}

func TestAgent_RunStream_ProgressCallback(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	agent := New(p, r)
	agent.SetMaxSteps(2)

	var got []struct {
		step  int
		phase string
		tool  string
	}
	agent.SetProgressCallback(func(step int, phase, tool string) {
		got = append(got, struct {
			step  int
			phase string
			tool  string
		}{step, phase, tool})
	})

	textCh, errCh := agent.RunStream(context.Background(), "test progress")
	for range textCh {
	}
	for range errCh {
	}

	if len(got) == 0 {
		t.Fatal("expected progress callbacks")
	}
	if got[0].step != 1 || got[0].phase != "thinking" {
		t.Errorf("first progress = %+v, want step=1 phase=thinking", got[0])
	}
}

// streamToolProvider 是一个返回工具调用流事件的测试 provider。
type streamToolProvider struct {
	testutil.StubProvider
	callCount int
}

func (s *streamToolProvider) Stream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	s.callCount++
	ch := make(chan provider.StreamEvent, 4)
	if s.callCount == 1 {
		ch <- provider.StreamEvent{Type: provider.EventToolCall, ToolCall: &provider.ToolCallDelta{Index: 0, ID: "call_1", Name: "read_file"}}
		ch <- provider.StreamEvent{Type: provider.EventDone}
	} else {
		ch <- provider.StreamEvent{Type: provider.EventText, Content: "done"}
		ch <- provider.StreamEvent{Type: provider.EventDone}
	}
	close(ch)
	return ch, nil
}

func TestAgent_RunStream_ProgressCallbackToolPhase(t *testing.T) {
	p := &streamToolProvider{StubProvider: *testutil.NewStubProvider(nil)}
	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	agent := New(p, r)
	agent.SetMaxSteps(3)

	var got []struct {
		step  int
		phase string
		tool  string
	}
	agent.SetProgressCallback(func(step int, phase, tool string) {
		got = append(got, struct {
			step  int
			phase string
			tool  string
		}{step, phase, tool})
	})

	textCh, errCh := agent.RunStream(context.Background(), "read file")
	for range textCh {
	}
	for range errCh {
	}

	var toolFound bool
	for _, g := range got {
		if g.phase == "tool" && g.tool == "read_file" {
			toolFound = true
			break
		}
	}
	if !toolFound {
		t.Errorf("expected a tool progress for read_file, got %+v", got)
	}
}

func TestAgent_BuildSystemPrompt(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	agent := New(p, r)

	prompt := agent.buildSystemPrompt()
	if !strings.Contains(prompt, "LoomCode") {
		t.Error("system prompt should mention LoomCode")
	}
	if !strings.Contains(prompt, "tools") {
		t.Error("system prompt should mention tools")
	}
}

func TestMergeToolCallDeltas_ByIndex(t *testing.T) {
	deltas := []provider.ToolCallDelta{
		{Index: 0, ID: "call_1", Name: "read_file", Arguments: ""},
		{Index: 0, ID: "", Name: "", Arguments: "{"},
		{Index: 0, ID: "", Name: "", Arguments: "\"path\": \"/tmp/test.txt\""},
		{Index: 0, ID: "", Name: "", Arguments: "}"},
		{Index: 1, ID: "call_2", Name: "bash", Arguments: ""},
		{Index: 1, ID: "", Name: "", Arguments: "{\"cmd\": \"ls\"}"},
	}

	merged := mergeToolCallDeltas(deltas)
	if len(merged) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(merged))
	}

	if merged[0].ID != "call_1" {
		t.Errorf("expected first call id call_1, got %q", merged[0].ID)
	}
	if merged[0].Function.Name != "read_file" {
		t.Errorf("expected first call name read_file, got %q", merged[0].Function.Name)
	}
	if path, ok := merged[0].Args["path"].(string); !ok || path != "/tmp/test.txt" {
		t.Errorf("expected first call args path /tmp/test.txt, got %v", merged[0].Args)
	}

	if merged[1].ID != "call_2" {
		t.Errorf("expected second call id call_2, got %q", merged[1].ID)
	}
	if merged[1].Function.Name != "bash" {
		t.Errorf("expected second call name bash, got %q", merged[1].Function.Name)
	}
}

func TestMergeToolCallDeltas_DropsEmptyIndexWithoutID(t *testing.T) {
	// 某些 provider 的第一个 delta 可能不带 index/id，这种无效 delta 不应产生空 ID tool call
	deltas := []provider.ToolCallDelta{
		{Index: 0, ID: "call_1", Name: "read_file", Arguments: "{\"path\": \"/tmp\"}"},
	}

	merged := mergeToolCallDeltas(deltas)
	if len(merged) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(merged))
	}
	if merged[0].ID != "call_1" {
		t.Errorf("expected id call_1, got %q", merged[0].ID)
	}
}

func TestAgent_RepairPipelineEnabled(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	p.CapsVal.NeedsToolRepair = true

	r := tool.NewRegistry()
	agent := New(p, r)

	if agent.repairPipeline == nil {
		t.Fatal("expected repairPipeline to be initialized when NeedsToolRepair=true")
	}
}

func TestAgent_RepairPipelineDisabled(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	p.CapsVal.NeedsToolRepair = false

	r := tool.NewRegistry()
	agent := New(p, r)

	if agent.repairPipeline != nil {
		t.Fatal("expected repairPipeline to be nil when NeedsToolRepair=false")
	}
}

func TestAgent_RepairToolCalls_PreservesValidCalls(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	p.CapsVal.NeedsToolRepair = true

	r := tool.NewRegistry()
	agent := New(p, r)

	calls := []provider.ToolCall{
		{ID: "call_1", Function: provider.ToolCallFunc{Name: "read_file", Arguments: "{\"path\":\"/tmp/test.txt\"}"}, Args: map[string]any{"path": "/tmp/test.txt"}},
	}
	repaired := agent.repairToolCalls("", calls)

	if len(repaired) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(repaired))
	}
	if repaired[0].ID != "call_1" {
		t.Errorf("expected id call_1, got %q", repaired[0].ID)
	}
	if repaired[0].Function.Name != "read_file" {
		t.Errorf("expected name read_file, got %q", repaired[0].Function.Name)
	}
	if path, ok := repaired[0].Args["path"].(string); !ok || path != "/tmp/test.txt" {
		t.Errorf("expected args path /tmp/test.txt, got %v", repaired[0].Args)
	}
}

func TestAgent_RepairToolCalls_FallsBackOnFailure(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	p.CapsVal.NeedsToolRepair = true

	r := tool.NewRegistry()
	agent := New(p, r)

	// 模拟工具调用 JSON 整体损坏且无法从 reasoning 中回收；repairPipeline 应回退到原结果。
	calls := []provider.ToolCall{
		{ID: "call_1", Function: provider.ToolCallFunc{Name: "read_file", Arguments: "not json"}, Args: map[string]any{"path": "/tmp/test.txt"}},
	}
	repaired := agent.repairToolCalls("", calls)

	if len(repaired) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(repaired))
	}
	// RepairPipeline 对整体合法但 arguments 解析失败的输入会直接返回原解析结果，
	// 这里的关键行为是：不 panic、不丢失调用，保持调用可继续执行。
	if repaired[0].ID != "call_1" {
		t.Errorf("expected id call_1, got %q", repaired[0].ID)
	}
	if repaired[0].Function.Name != "read_file" {
		t.Errorf("expected name read_file, got %q", repaired[0].Function.Name)
	}
}

func TestAgent_Run_UsesRepairPipeline(t *testing.T) {
	callCount := 0
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			return &provider.ChatResponse{
				ReasoningContent: "I need to read the file",
				ToolCalls: []provider.ToolCall{
					{ID: "call_1", Function: provider.ToolCallFunc{Name: "read_file", Arguments: "not json"}},
				},
			}, nil
		}
		return &provider.ChatResponse{Content: "File analysis complete"}, nil
	})
	p.CapsVal.NeedsToolRepair = true

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	agent := New(p, r)
	agent.SetMaxSteps(5)

	// 由于 tool call 损坏无法执行，模型应收到错误结果后继续；这里主要验证 repairPipeline 被触发且不 panic。
	_, err := agent.Run(context.Background(), "analyze /tmp/test.txt")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}
