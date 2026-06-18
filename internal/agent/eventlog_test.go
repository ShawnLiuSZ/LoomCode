package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEventLog(t *testing.T) {
	t.Run("NewEventLog", func(t *testing.T) {
		log := NewEventLog(100)
		if log == nil {
			t.Fatal("expected non-nil event log")
		}
		if log.Size() != 0 {
			t.Errorf("expected size 0, got %d", log.Size())
		}
	})

	t.Run("Log", func(t *testing.T) {
		log := NewEventLog(100)
		log.Log(EventToolCall, "test message", nil)

		if log.Size() != 1 {
			t.Errorf("expected size 1, got %d", log.Size())
		}
	})

	t.Run("LogToolCall", func(t *testing.T) {
		log := NewEventLog(100)
		log.LogToolCall("bash", map[string]any{"command": "ls"})

		events := log.EventsByType(EventToolCall)
		if len(events) != 1 {
			t.Errorf("expected 1 tool call event, got %d", len(events))
		}
	})

	t.Run("LogToolResult", func(t *testing.T) {
		log := NewEventLog(100)
		log.LogToolResult("bash", "output", nil)

		events := log.EventsByType(EventToolResult)
		if len(events) != 1 {
			t.Errorf("expected 1 tool result event, got %d", len(events))
		}
	})

	t.Run("LogError", func(t *testing.T) {
		log := NewEventLog(100)
		log.LogError(fmt.Errorf("test error"))

		events := log.EventsByType(EventError)
		if len(events) != 1 {
			t.Errorf("expected 1 error event, got %d", len(events))
		}
	})

	t.Run("LogCost", func(t *testing.T) {
		log := NewEventLog(100)
		log.LogCost(100, 50, 0.05)

		events := log.EventsByType(EventCost)
		if len(events) != 1 {
			t.Errorf("expected 1 cost event, got %d", len(events))
		}
	})

	t.Run("LogCacheHit", func(t *testing.T) {
		log := NewEventLog(100)
		log.LogCacheHit(1000)

		events := log.EventsByType(EventCacheHit)
		if len(events) != 1 {
			t.Errorf("expected 1 cache hit event, got %d", len(events))
		}
	})

	t.Run("LogMessage", func(t *testing.T) {
		log := NewEventLog(100)
		log.LogMessage("user", "hello")

		events := log.EventsByType(EventMessage)
		if len(events) != 1 {
			t.Errorf("expected 1 message event, got %d", len(events))
		}
	})

	t.Run("LogGoalCheck", func(t *testing.T) {
		log := NewEventLog(100)
		log.LogGoalCheck(true, "completed")

		events := log.EventsByType(EventGoalCheck)
		if len(events) != 1 {
			t.Errorf("expected 1 goal check event, got %d", len(events))
		}
	})
}

func TestEventLogMaxSize(t *testing.T) {
	log := NewEventLog(3)

	log.Log(EventMessage, "msg1", nil)
	log.Log(EventMessage, "msg2", nil)
	log.Log(EventMessage, "msg3", nil)
	log.Log(EventMessage, "msg4", nil)

	if log.Size() != 3 {
		t.Errorf("expected size 3, got %d", log.Size())
	}

	events := log.Events()
	if events[0].Message != "msg2" {
		t.Errorf("expected first event to be 'msg2', got %q", events[0].Message)
	}
}

func TestEventLogRecent(t *testing.T) {
	log := NewEventLog(100)

	log.Log(EventMessage, "msg1", nil)
	log.Log(EventMessage, "msg2", nil)
	log.Log(EventMessage, "msg3", nil)

	recent := log.Recent(2)
	if len(recent) != 2 {
		t.Errorf("expected 2 recent events, got %d", len(recent))
	}
	if recent[0].Message != "msg2" {
		t.Errorf("expected first recent event to be 'msg2', got %q", recent[0].Message)
	}
}

func TestEventLogClear(t *testing.T) {
	log := NewEventLog(100)

	log.Log(EventMessage, "msg1", nil)
	log.Log(EventMessage, "msg2", nil)

	log.Clear()

	if log.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", log.Size())
	}
}

func TestEventLogSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "events.json")

	log := NewEventLog(100)
	log.Log(EventMessage, "msg1", nil)
	log.Log(EventMessage, "msg2", nil)

	if err := log.Save(logFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("event log file not created")
	}

	// 加载并验证
	log2 := NewEventLog(100)
	if err := log2.Load(logFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if log2.Size() != 2 {
		t.Errorf("expected 2 events after load, got %d", log2.Size())
	}
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventToolCall, "tool_call"},
		{EventToolResult, "tool_result"},
		{EventError, "error"},
		{EventCost, "cost"},
		{EventCacheHit, "cache_hit"},
		{EventMessage, "message"},
		{EventGoalCheck, "goal_check"},
		{EventType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.expected {
				t.Errorf("EventType.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEventLogConcurrent(t *testing.T) {
	log := NewEventLog(100)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			log.Log(EventMessage, "concurrent", nil)
			log.Size()
			log.Events()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if log.Size() != 10 {
		t.Errorf("expected size 10, got %d", log.Size())
	}
}
