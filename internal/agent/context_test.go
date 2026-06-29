package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/testutil"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// --- System prompt environment grounding ---

func TestBuildSystemPrompt_IncludesEnvironment(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	a := New(p, tool.NewRegistry())

	dir := t.TempDir()
	a.SetWorkDir(dir)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/feature-x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt := a.buildSystemPrompt()

	if !strings.Contains(prompt, runtime.GOOS) {
		t.Errorf("prompt missing OS %q", runtime.GOOS)
	}
	if !strings.Contains(prompt, dir) {
		t.Error("prompt missing working directory")
	}
	if !strings.Contains(prompt, "feature-x") {
		t.Error("prompt missing git branch")
	}
	if !strings.Contains(prompt, "main.go") {
		t.Error("prompt missing directory listing")
	}
}

func TestBuildSystemPrompt_NoWorkDirStillValid(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	a := New(p, tool.NewRegistry())

	prompt := a.buildSystemPrompt()
	if !strings.Contains(prompt, "Helix") {
		t.Error("system prompt should mention Helix")
	}
	if !strings.Contains(prompt, "tools") {
		t.Error("system prompt should mention tools")
	}
}

// --- Cache-aware compaction ---

func bigMsgs() []provider.Message {
	return []provider.Message{
		{Role: "system", Content: strings.Repeat("s", 30)},
		{Role: "user", Content: strings.Repeat("u", 300)},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: strings.Repeat("t", 300)},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "c2"}}},
		{Role: "tool", ToolCallID: "c2", Content: strings.Repeat("t", 300)},
		{Role: "user", Content: "more"},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "again"},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "c3"}}},
		{Role: "tool", ToolCallID: "c3", Content: strings.Repeat("t", 300)},
		{Role: "user", Content: "more2"},
		{Role: "assistant", Content: "answer2"},
	}
}

func TestCompactMessages_SummarizesOldRounds(t *testing.T) {
	var summarizeReq *provider.ChatRequest
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		summarizeReq = req
		return &provider.ChatResponse{Content: "summary of earlier work"}, nil
	})
	a := New(p, tool.NewRegistry())
	a.SetModel("m")
	a.messages = bigMsgs()

	a.compactMessages(context.Background(), 100) // maxInput=80, tokens far exceed it

	if summarizeReq == nil {
		t.Fatal("expected a summarization LLM call")
	}
	// Prefix must stay: real system prompt first, then a single summary block.
	if a.messages[0].Role != "system" {
		t.Fatalf("messages[0] role = %q, want system", a.messages[0].Role)
	}
	if !strings.Contains(a.messages[1].Content, "summary of earlier work") {
		t.Errorf("messages[1] should hold the summary, got %q", a.messages[1].Content)
	}
	// Kept region must not start with an orphan tool result.
	if a.messages[2].Role == "tool" {
		t.Error("kept region starts with an orphan tool result")
	}
	// Recent tail preserved.
	last := a.messages[len(a.messages)-1]
	if last.Role != "assistant" || last.Content != "answer2" {
		t.Errorf("recent tail not preserved, last = %+v", last)
	}
	if len(a.messages) >= len(bigMsgs()) {
		t.Errorf("compaction did not shrink message list: %d", len(a.messages))
	}
}

func TestCompactMessages_CutAdjustsPastToolMessage(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "S"}, nil
	})
	a := New(p, tool.NewRegistry())
	a.SetModel("m")
	// len=11, keepRecent=8 -> naive cut=3 lands on a tool message; must advance to 4 (user).
	a.messages = []provider.Message{
		{Role: "system", Content: strings.Repeat("s", 30)},
		{Role: "user", Content: strings.Repeat("u", 300)},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: strings.Repeat("t", 300)},
		{Role: "user", Content: "U4"},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "c2"}}},
		{Role: "tool", ToolCallID: "c2", Content: "r2"},
		{Role: "user", Content: "U7"},
		{Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "c3"}}},
		{Role: "tool", ToolCallID: "c3", Content: "r3"},
		{Role: "user", Content: "U10"},
	}

	a.compactMessages(context.Background(), 100)

	// [system, summary, user(U4), assistant(c2), tool(c2), user(U7), assistant(c3), tool(c3), user(U10)]
	if a.messages[2].Role != "user" || a.messages[2].Content != "U4" {
		t.Errorf("kept region should begin at the user message U4, got %+v", a.messages[2])
	}
	if a.messages[3].Role != "assistant" || a.messages[4].Role != "tool" {
		t.Error("assistant/tool pairing in kept region broken")
	}
}

func TestCompactMessages_FallbackOnSummarizeError(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return nil, errors.New("summarize boom")
	})
	a := New(p, tool.NewRegistry())
	a.SetModel("m")
	a.messages = bigMsgs()
	before := len(a.messages)

	a.compactMessages(context.Background(), 100)

	if len(a.messages) >= before {
		t.Errorf("fallback truncation should shrink list: before=%d after=%d", before, len(a.messages))
	}
	if a.messages[0].Role != "system" {
		t.Error("system prompt must be preserved by fallback")
	}
	// No summary block was inserted.
	if len(a.messages) > 1 && strings.Contains(a.messages[1].Content, "summary") {
		t.Error("fallback must not insert a summary block")
	}
}

func TestCompactMessages_NoOpUnderThreshold(t *testing.T) {
	called := false
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		called = true
		return &provider.ChatResponse{Content: "x"}, nil
	})
	a := New(p, tool.NewRegistry())
	a.SetModel("m")
	a.messages = []provider.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "ok"},
	}

	a.compactMessages(context.Background(), 100000) // huge window -> never triggers

	if called {
		t.Error("should not summarize when under threshold")
	}
	if len(a.messages) != 3 {
		t.Errorf("messages should be untouched, got %d", len(a.messages))
	}
}

func TestCompactMessages_UnknownWindowNoOp(t *testing.T) {
	called := false
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		called = true
		return &provider.ChatResponse{Content: "x"}, nil
	})
	a := New(p, tool.NewRegistry())
	a.SetModel("m")
	a.messages = bigMsgs()

	a.compactMessages(context.Background(), 0) // unknown context window

	if called {
		t.Error("should not summarize when context window unknown")
	}
	if len(a.messages) != len(bigMsgs()) {
		t.Error("messages should be untouched when window unknown")
	}
}
