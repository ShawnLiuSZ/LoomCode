package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

type fakeMemory struct{ prompt string }

func (f fakeMemory) BuildContextPrompt() string { return f.prompt }

func TestSetMemory_InjectsIntoRecallMemoryTool(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	tools := tool.NewRegistry()
	tools.RegisterDefaults() // 注册 RecallMemoryTool 占位
	a := New(p, tools)
	a.SetMemory(fakeMemory{prompt: "## Project Knowledge\n- build: use make build\n"})

	tl, ok := tools.Get("recall_memory")
	if !ok {
		t.Fatal("recall_memory tool should be registered by RegisterDefaults")
	}
	res, err := tl.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(res.Content, "use make build") {
		t.Errorf("recall_memory should return memory content, got: %s", res.Content)
	}
}

func TestBuildSystemPrompt_NoMemoryNoError(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	a := New(p, tool.NewRegistry())

	prompt := a.buildSystemPrompt() // no memory set
	if !strings.Contains(prompt, "LoomCode") {
		t.Error("prompt should still be built without memory")
	}
}

func TestBuildSystemPrompt_EmptyMemoryOmitted(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	a := New(p, tool.NewRegistry())
	a.SetMemory(fakeMemory{prompt: ""}) // nothing remembered yet

	prompt := a.buildSystemPrompt()
	if strings.Contains(prompt, "Project Knowledge") {
		t.Error("empty memory should not add a heading")
	}
}
