package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// TestNewReadOnlyRegistry 验证从完整 registry 过滤出的只读 registry 只含 IsReadOnly 的工具。
func TestNewReadOnlyRegistry(t *testing.T) {
	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})  // read-only
	r.Register(&tool.GrepTool{})      // read-only
	r.Register(&tool.GlobTool{})      // read-only
	r.Register(&tool.WriteFileTool{}) // write
	r.Register(&tool.EditFileTool{})  // write
	r.Register(&tool.BashTool{})      // write（能执行任意命令）

	ro := newReadOnlyRegistry(r)
	tools := ro.List()
	if len(tools) != 3 {
		t.Fatalf("expected 3 read-only tools (read_file, grep, glob), got %d: %+v", len(tools), toolNames(tools))
	}
	for _, tl := range tools {
		if !tl.IsReadOnly() {
			t.Errorf("tool %q should be read-only", tl.Name())
		}
	}
}

// TestCoordinator_PlannerReadOnlyTools 验证 planner 的 registry 只含只读工具，
// executor 的 registry 含完整工具集。
func TestCoordinator_PlannerReadOnlyTools(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})  // read-only
	r.Register(&tool.GrepTool{})      // read-only
	r.Register(&tool.WriteFileTool{}) // write
	r.Register(&tool.EditFileTool{})  // write
	r.Register(&tool.BashTool{})      // write

	c := NewCoordinator(p, p, r)

	plannerTools := c.planner.tools.List()
	if len(plannerTools) != 2 {
		t.Errorf("planner should have 2 read-only tools, got %d: %+v", len(plannerTools), toolNames(plannerTools))
	}
	for _, tl := range plannerTools {
		if !tl.IsReadOnly() {
			t.Errorf("planner tool %q is not read-only", tl.Name())
		}
	}

	executorTools := c.executor.tools.List()
	if len(executorTools) != 5 {
		t.Errorf("executor should have all 5 tools, got %d", len(executorTools))
	}
}

// TestCoordinator_SeparateSessions 验证 planner 和 executor 的 messages 互不交叉：
//   - planner session 含原始 task，不含 executor 的 "Execute the plan above" 包装
//   - executor session 含计划文本，不含原始 task
func TestCoordinator_SeparateSessions(t *testing.T) {
	var callCount int
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			// planner 阶段：产出计划
			return &provider.ChatResponse{Content: "PLAN_MARKER: step 1 - read file"}, nil
		}
		// executor 阶段：执行计划
		return &provider.ChatResponse{Content: "executed successfully"}, nil
	})

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})
	r.Register(&tool.WriteFileTool{})

	c := NewCoordinator(p, p, r)
	_, err := c.Run(context.Background(), "TASK_MARKER: do something")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// planner session 应含原始 task，不含 executor 的包装提示
	plannerFlat := flattenContent(c.planner.ConversationMessages())
	if !strings.Contains(plannerFlat, "TASK_MARKER") {
		t.Error("planner session should contain the original task")
	}
	if strings.Contains(plannerFlat, "Execute the plan above") {
		t.Error("planner session must not contain executor's prompt wrapper — sessions are crossing")
	}

	// executor session 应含计划文本，不含原始 task
	executorFlat := flattenContent(c.executor.ConversationMessages())
	if !strings.Contains(executorFlat, "PLAN_MARKER") {
		t.Error("executor session should contain the plan text")
	}
	if strings.Contains(executorFlat, "TASK_MARKER") {
		t.Error("executor session must not contain the original task — sessions are crossing")
	}
}

// TestCoordinator_Run 端到端验证：planner 产出计划，executor 执行，最终结果正确。
func TestCoordinator_Run(t *testing.T) {
	var callCount int
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			// planner 阶段
			return &provider.ChatResponse{Content: "Step 1: read file\nStep 2: implement feature"}, nil
		}
		// executor 阶段
		return &provider.ChatResponse{Content: "Done: feature implemented"}, nil
	})

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})
	r.Register(&tool.WriteFileTool{})

	c := NewCoordinator(p, p, r)
	result, err := c.Run(context.Background(), "implement feature X")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(result, "Done") {
		t.Errorf("unexpected result: %q", result)
	}
	if callCount != 2 {
		t.Errorf("expected 2 Chat calls (planner + executor), got %d", callCount)
	}
}

// TestCoordinator_Run_PlannerError 验证 planner 阶段失败时 Run 返回错误。
func TestCoordinator_Run_PlannerError(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return nil, context.DeadlineExceeded
	})

	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	c := NewCoordinator(p, p, r)
	_, err := c.Run(context.Background(), "some task")
	if err == nil {
		t.Fatal("expected error when planner fails")
	}
	if !strings.Contains(err.Error(), "planner phase failed") {
		t.Errorf("error should mention planner phase: %v", err)
	}
}

// TestCoordinator_SetPlannerModel 验证 SetPlannerModel 正确设置 planner 的模型。
func TestCoordinator_SetPlannerModel(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()

	c := NewCoordinator(p, p, r)
	c.SetPlannerModel("strong-model-v1")

	if c.plannerModel != "strong-model-v1" {
		t.Errorf("plannerModel = %q, want %q", c.plannerModel, "strong-model-v1")
	}
	if c.planner.model != "strong-model-v1" {
		t.Errorf("planner.model = %q, want %q", c.planner.model, "strong-model-v1")
	}
}

// flattenContent 将消息列表的 Content 字段拼接成一个字符串，便于断言。
func flattenContent(msgs []provider.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

// toolNames 提取工具名列表，用于错误信息。
func toolNames(tools []tool.Tool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name()
	}
	return names
}
