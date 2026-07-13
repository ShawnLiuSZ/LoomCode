package agent

import (
	"context"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

func TestAgent_SeedsHistoryIntoRequest(t *testing.T) {
	var seen *provider.ChatRequest
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		seen = req
		return &provider.ChatResponse{Content: "ok"}, nil
	})
	a := New(p, tool.NewRegistry())
	a.SetHistory([]provider.Message{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
	})

	_, _ = a.Run(context.Background(), "second question")

	// 拆分 buildSystemPrompt 后，前缀为 [static-system, dynamic-system] + history + [user]
	msgs := seen.Messages
	if len(msgs) != 5 {
		t.Fatalf("expected [static-system, dynamic-system, user, assistant, user], got %d messages", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("msgs[0] = %q, want system (static)", msgs[0].Role)
	}
	if msgs[1].Role != "system" {
		t.Errorf("msgs[1] = %q, want system (dynamic context)", msgs[1].Role)
	}
	if msgs[2].Content != "first question" || msgs[3].Content != "first answer" {
		t.Errorf("prior turn not seeded: %+v", msgs[2:4])
	}
	if msgs[4].Content != "second question" {
		t.Errorf("new turn = %q", msgs[4].Content)
	}
}

func TestAgent_ConversationMessagesExcludesSystem(t *testing.T) {
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "answer"}, nil
	})
	a := New(p, tool.NewRegistry())

	_, _ = a.Run(context.Background(), "q")

	conv := a.ConversationMessages()
	if len(conv) != 2 {
		t.Fatalf("expected [user, assistant], got %d", len(conv))
	}
	if conv[0].Role != "user" || conv[0].Content != "q" {
		t.Errorf("conv[0] = %+v", conv[0])
	}
	if conv[1].Role != "assistant" || conv[1].Content != "answer" {
		t.Errorf("conv[1] = %+v", conv[1])
	}
}

func TestAgent_NoHistoryBehavesAsBefore(t *testing.T) {
	var seen *provider.ChatRequest
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		seen = req
		return &provider.ChatResponse{Content: "ok"}, nil
	})
	a := New(p, tool.NewRegistry())

	_, _ = a.Run(context.Background(), "only question")

	// 拆分后前缀为 [static-system, dynamic-system] + [user]
	if len(seen.Messages) != 3 {
		t.Fatalf("fresh agent should send [static-system, dynamic-system, user], got %d", len(seen.Messages))
	}
}

func TestMultiAgent_ConversationContinuity(t *testing.T) {
	var lastReq *provider.ChatRequest
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		lastReq = req
		return &provider.ChatResponse{Content: "reply"}, nil
	})
	ma := NewMultiAgent(p, tool.NewRegistry())

	_, _ = ma.Run(context.Background(), "turn one")
	_, _ = ma.Run(context.Background(), "turn two")

	sawFirst := false
	for _, m := range lastReq.Messages {
		if m.Content == "turn one" {
			sawFirst = true
		}
	}
	if !sawFirst {
		t.Error("second turn's request did not include the first turn (no continuity)")
	}
}

func TestMultiAgent_ResetConversation(t *testing.T) {
	var lastReq *provider.ChatRequest
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		lastReq = req
		return &provider.ChatResponse{Content: "reply"}, nil
	})
	ma := NewMultiAgent(p, tool.NewRegistry())

	_, _ = ma.Run(context.Background(), "turn one")
	ma.ResetConversation()
	_, _ = ma.Run(context.Background(), "turn two")

	for _, m := range lastReq.Messages {
		if m.Content == "turn one" {
			t.Error("ResetConversation did not clear prior history")
		}
	}
}

func TestMultiAgent_SetHistorySeedsRestore(t *testing.T) {
	var lastReq *provider.ChatRequest
	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		lastReq = req
		return &provider.ChatResponse{Content: "reply"}, nil
	})
	ma := NewMultiAgent(p, tool.NewRegistry())
	ma.SetHistory([]provider.Message{
		{Role: "user", Content: "restored turn"},
		{Role: "assistant", Content: "restored reply"},
	})

	_, _ = ma.Run(context.Background(), "new turn")

	sawRestored := false
	for _, m := range lastReq.Messages {
		if m.Content == "restored turn" {
			sawRestored = true
		}
	}
	if !sawRestored {
		t.Error("restored history was not seeded into the conversation")
	}
}
