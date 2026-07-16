package agent

import (
	"math"
	"testing"
)

func TestEffortLevelString(t *testing.T) {
	tests := []struct {
		level    EffortLevel
		expected string
	}{
		{EffortLow, "low"},
		{EffortMedium, "medium"},
		{EffortHigh, "high"},
		{EffortLevel(999), "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("EffortLevel.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseEffortLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected EffortLevel
	}{
		{"low", EffortLow},
		{"l", EffortLow},
		{"medium", EffortMedium},
		{"m", EffortMedium},
		{"", EffortMedium},
		{"high", EffortHigh},
		{"h", EffortHigh},
		{"unknown", EffortMedium},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseEffortLevel(tt.input); got != tt.expected {
				t.Errorf("ParseEffortLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEffortManager(t *testing.T) {
	t.Run("NewEffortManager", func(t *testing.T) {
		manager := NewEffortManager()
		if manager == nil {
			t.Fatal("expected non-nil manager")
		}
		if manager.GetLevel() != EffortMedium {
			t.Errorf("expected EffortMedium, got %v", manager.GetLevel())
		}
	})

	t.Run("SetLevel", func(t *testing.T) {
		manager := NewEffortManager()
		manager.SetLevel(EffortHigh)
		if manager.GetLevel() != EffortHigh {
			t.Errorf("expected EffortHigh, got %v", manager.GetLevel())
		}
	})

	t.Run("GetMaxSteps", func(t *testing.T) {
		manager := NewEffortManager()

		for _, level := range []EffortLevel{EffortLow, EffortMedium, EffortHigh} {
			manager.SetLevel(level)
			if manager.GetMaxSteps() != math.MaxInt {
				t.Errorf("expected no default step limit for %s, got %d", level, manager.GetMaxSteps())
			}
		}
	})

	t.Run("GetReasoningEffort", func(t *testing.T) {
		manager := NewEffortManager()

		manager.SetLevel(EffortLow)
		if manager.GetReasoningEffort() != "low" {
			t.Errorf("expected 'low', got %q", manager.GetReasoningEffort())
		}

		manager.SetLevel(EffortHigh)
		if manager.GetReasoningEffort() != "high" {
			t.Errorf("expected 'high', got %q", manager.GetReasoningEffort())
		}
	})

	t.Run("SetMaxSteps", func(t *testing.T) {
		manager := NewEffortManager()
		manager.SetLevel(EffortLow)
		manager.SetMaxSteps(EffortLow, 3)
		if manager.GetMaxSteps() != 3 {
			t.Errorf("expected 3, got %d", manager.GetMaxSteps())
		}
	})

	t.Run("ListLevels", func(t *testing.T) {
		manager := NewEffortManager()
		levels := manager.ListLevels()
		if len(levels) != 3 {
			t.Errorf("expected 3 levels, got %d", len(levels))
		}
	})

	t.Run("GetDescription", func(t *testing.T) {
		manager := NewEffortManager()

		desc := manager.GetDescription(EffortLow)
		if desc == "" {
			t.Error("expected non-empty description")
		}

		desc = manager.GetDescription(EffortMedium)
		if desc == "" {
			t.Error("expected non-empty description")
		}

		desc = manager.GetDescription(EffortHigh)
		if desc == "" {
			t.Error("expected non-empty description")
		}
	})
}

func TestEffortManagerConcurrent(t *testing.T) {
	manager := NewEffortManager()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			manager.SetLevel(EffortHigh)
			manager.GetLevel()
			manager.GetMaxSteps()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
