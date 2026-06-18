package session

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestReplayer(t *testing.T) {
	t.Run("NewReplayer", func(t *testing.T) {
		session := &Session{
			Meta: Meta{Name: "test"},
		}
		var buf bytes.Buffer
		replayer := NewReplayer(session, &buf)
		if replayer == nil {
			t.Fatal("expected non-nil replayer")
		}
	})

	t.Run("SetDelay", func(t *testing.T) {
		session := &Session{}
		var buf bytes.Buffer
		replayer := NewReplayer(session, &buf)
		replayer.SetDelay(50 * time.Millisecond)
		if replayer.delay != 50*time.Millisecond {
			t.Errorf("expected 50ms, got %v", replayer.delay)
		}
	})

	t.Run("SetVerbose", func(t *testing.T) {
		session := &Session{}
		var buf bytes.Buffer
		replayer := NewReplayer(session, &buf)
		replayer.SetVerbose(true)
		if !replayer.verbose {
			t.Error("expected verbose to be true")
		}
	})
}

func TestReplayerReplay(t *testing.T) {
	t.Run("Replay_EmptySession", func(t *testing.T) {
		session := &Session{
			Meta: Meta{Name: "empty"},
		}
		var buf bytes.Buffer
		replayer := NewReplayer(session, &buf)
		replayer.SetDelay(0)

		err := replayer.Replay(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !containsStr(output, "empty") {
			t.Error("expected output to contain session name")
		}
	})

	t.Run("Replay_WithMessages", func(t *testing.T) {
		session := &Session{
			Meta: Meta{Name: "test"},
			Messages: []Message{
				{Role: "user", Content: "hello", Timestamp: time.Now()},
				{Role: "assistant", Content: "hi there", Timestamp: time.Now()},
			},
		}
		var buf bytes.Buffer
		replayer := NewReplayer(session, &buf)
		replayer.SetDelay(0)

		err := replayer.Replay(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !containsStr(output, "hello") {
			t.Error("expected output to contain user message")
		}
		if !containsStr(output, "hi there") {
			t.Error("expected output to contain assistant message")
		}
	})

	t.Run("Replay_WithToolMessage", func(t *testing.T) {
		session := &Session{
			Meta: Meta{Name: "test"},
			Messages: []Message{
				{Role: "tool", Content: "tool result", ToolName: "bash", Timestamp: time.Now()},
			},
		}
		var buf bytes.Buffer
		replayer := NewReplayer(session, &buf)
		replayer.SetDelay(0)
		replayer.SetVerbose(true)

		err := replayer.Replay(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !containsStr(output, "tool result") {
			t.Error("expected output to contain tool result")
		}
	})

	t.Run("Replay_NilSession", func(t *testing.T) {
		var buf bytes.Buffer
		replayer := NewReplayer(nil, &buf)

		err := replayer.Replay(context.Background())
		if err == nil {
			t.Error("expected error for nil session")
		}
	})

	t.Run("Replay_ContextCancelled", func(t *testing.T) {
		session := &Session{
			Meta: Meta{Name: "test"},
			Messages: []Message{
				{Role: "user", Content: "hello", Timestamp: time.Now()},
				{Role: "assistant", Content: "hi", Timestamp: time.Now()},
			},
		}
		var buf bytes.Buffer
		replayer := NewReplayer(session, &buf)
		replayer.SetDelay(100 * time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := replayer.Replay(ctx)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestGetSessionInfo(t *testing.T) {
	session := &Session{
		Meta: Meta{
			Name:      "test session",
			Model:     "gpt-4",
			Provider:  "openai",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}

	info := GetSessionInfo(session)
	if !containsStr(info, "test session") {
		t.Error("expected info to contain session name")
	}
	if !containsStr(info, "gpt-4") {
		t.Error("expected info to contain model")
	}
	if !containsStr(info, "openai") {
		t.Error("expected info to contain provider")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
