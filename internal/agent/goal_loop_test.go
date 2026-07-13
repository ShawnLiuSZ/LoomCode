package agent

import (
	"context"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// The goal judge is an LLM call. It must NOT run on a fixed cadence mid-loop
// (that multiplies cost/latency); only a single final check after the loop ends.
func TestRun_GoalJudgeNotCalledPeriodically(t *testing.T) {
	judgeCalls := 0
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		if len(req.Messages) > 0 && req.Messages[0].Content == judgeSystemPrompt {
			judgeCalls++
			return &provider.ChatResponse{Content: "NOT_ACHIEVED: still working"}, nil
		}
		// Main loop never finishes: always emit a tool call so we hit maxSteps.
		return &provider.ChatResponse{
			ToolCalls: []provider.ToolCall{
				{ID: "c", Function: provider.ToolCallFunc{Name: "read_file"}, Args: map[string]any{"path": "/tmp/x"}},
			},
		}, nil
	})
	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})

	a := New(p, r)
	a.SetMaxSteps(6)
	a.SetGoal("do the thing")

	_, _ = a.Run(context.Background(), "task")

	if judgeCalls != 1 {
		t.Errorf("judge evaluated %d times; want exactly 1 (final check only, no periodic evaluation)", judgeCalls)
	}
}
