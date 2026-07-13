package control

import (
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

func TestCostController_ModelSelection(t *testing.T) {
	c := NewCostController("flash", "pro")

	if m := c.ModelForMain(); m != "flash" {
		t.Errorf("ModelForMain() = %q, want flash", m)
	}

	c.RequestUpgrade()
	if m := c.ModelForMain(); m != "pro" {
		t.Errorf("ModelForMain() after upgrade = %q, want pro", m)
	}

	c.ResetUpgrade()
	if m := c.ModelForMain(); m != "flash" {
		t.Errorf("ModelForMain() after reset = %q, want flash", m)
	}
}

func TestCostController_AuxModel(t *testing.T) {
	c := NewCostController("flash", "pro")
	c.SetAuxModel("cheap-model")

	if m := c.ModelForAux(); m != "cheap-model" {
		t.Errorf("ModelForAux() = %q, want cheap-model", m)
	}

	// 辅助模型不影响主模型
	if m := c.ModelForMain(); m != "flash" {
		t.Errorf("ModelForMain() = %q, want flash", m)
	}
}

func TestCostController_RecordCost(t *testing.T) {
	c := NewCostController("flash", "pro")

	c.RecordCost(provider.Cost{TotalCost: 0.03})
	c.RecordCost(provider.Cost{TotalCost: 0.07})

	if c.TotalCost() != 0.10 {
		t.Errorf("TotalCost() = %f, want 0.10", c.TotalCost())
	}
	if c.SessionCost() != 0.10 {
		t.Errorf("SessionCost() = %f, want 0.10", c.SessionCost())
	}
	if c.LastTurnCost() != 0.07 {
		t.Errorf("LastTurnCost() = %f, want 0.07", c.LastTurnCost())
	}
}

func TestCostController_ResetSession(t *testing.T) {
	c := NewCostController("flash", "pro")
	c.RecordCost(provider.Cost{TotalCost: 0.10})
	c.ResetSession()

	if c.SessionCost() != 0 {
		t.Errorf("SessionCost() = %f after reset", c.SessionCost())
	}
	// 总成本不应被重置
	if c.TotalCost() != 0.10 {
		t.Errorf("TotalCost() should not reset: %f", c.TotalCost())
	}
}

func TestCostController_CostLevel(t *testing.T) {
	c := NewCostController("flash", "pro")

	tests := []struct {
		cost  float64
		level CostLevel
	}{
		{0.01, LevelGreen},
		{0.04, LevelGreen},
		{0.05, LevelYellow},
		{0.19, LevelYellow},
		{0.20, LevelRed},
		{1.00, LevelRed},
	}

	for _, tt := range tests {
		c.RecordCost(provider.Cost{TotalCost: tt.cost})
		if lvl := c.CostLevel(); lvl != tt.level {
			t.Errorf("CostLevel() for $%.2f = %v, want %v", tt.cost, lvl, tt.level)
		}
	}
}

func TestCostController_ShouldCompress(t *testing.T) {
	c := NewCostController("flash", "pro")

	if c.ShouldCompress(100) {
		t.Error("ShouldCompress(100) should be false")
	}
	if !c.ShouldCompress(5000) {
		t.Error("ShouldCompress(5000) should be true")
	}
}

func TestCostController_CompressResult(t *testing.T) {
	c := NewCostController("flash", "pro")

	// 短内容不压缩
	short := "hello"
	if result := c.CompressResult(short); result != short {
		t.Errorf("short content should not be compressed: %q", result)
	}

	// 长内容压缩
	long := make([]byte, 5000)
	for i := range long {
		long[i] = 'a'
	}
	result := c.CompressResult(string(long))
	if len(result) >= 5000 {
		t.Errorf("long content should be compressed, got %d chars", len(result))
	}
	if result == "" {
		t.Error("compressed result should not be empty")
	}
}

func TestCostController_StatusReport(t *testing.T) {
	c := NewCostController("flash", "pro")
	c.RecordCost(provider.Cost{TotalCost: 0.03})

	report := c.StatusReport()
	if report == "" {
		t.Error("StatusReport() should not be empty")
	}
}

func TestCostLevel_String(t *testing.T) {
	tests := []struct {
		level CostLevel
		want  string
	}{
		{LevelGreen, "green"},
		{LevelYellow, "yellow"},
		{LevelRed, "red"},
	}

	for _, tt := range tests {
		if tt.level.String() != tt.want {
			t.Errorf("CostLevel(%d).String() = %q, want %q", tt.level, tt.level.String(), tt.want)
		}
	}
}
