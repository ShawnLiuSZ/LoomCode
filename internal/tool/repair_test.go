package tool

import (
	"encoding/json"
	"testing"
)

func TestParseToolCalls_Valid(t *testing.T) {
	raw := `[{"function": {"name": "read_file", "arguments": "{\"path\":\"/tmp/test\"}"}}]`

	calls, err := parseToolCalls(raw)
	if err != nil {
		t.Fatalf("parseToolCalls() error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("name = %q", calls[0].Name)
	}
	if calls[0].Args["path"] != "/tmp/test" {
		t.Errorf("args[path] = %v", calls[0].Args["path"])
	}
}

func TestParseToolCalls_Invalid(t *testing.T) {
	_, err := parseToolCalls("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFlattenStep(t *testing.T) {
	step := &FlattenStep{}

	// 带点号的扁平参数
	raw := `{"file.path": "/tmp/test", "file.content": "hello"}`
	calls, err := step.Repair("", raw)
	if err != nil {
		t.Fatalf("FlattenStep error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	args := calls[0].Args
	file, ok := args["file"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested file object, got %T", args["file"])
	}
	if file["path"] != "/tmp/test" {
		t.Errorf("file.path = %v", file["path"])
	}
	if file["content"] != "hello" {
		t.Errorf("file.content = %v", file["content"])
	}
}

func TestFlattenStep_NoNested(t *testing.T) {
	step := &FlattenStep{}
	_, err := step.Repair("", `{"path": "/tmp/test"}`)
	if err == nil {
		t.Error("expected error when no nested keys")
	}
}

func TestScavengeStep(t *testing.T) {
	step := &ScavengeStep{}

	reasoning := `I need to read the file. Let me use the tool: {"name": "read_file", "arguments": "{\"path\":\"/tmp/test\"}"}`
	calls, err := step.Repair(reasoning, "")
	if err != nil {
		t.Fatalf("ScavengeStep error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("name = %q", calls[0].Name)
	}
}

func TestScavengeStep_NoReasoning(t *testing.T) {
	step := &ScavengeStep{}
	_, err := step.Repair("", "")
	if err == nil {
		t.Error("expected error when no reasoning content")
	}
}

func TestTruncationStep_UnbalancedBraces(t *testing.T) {
	step := &TruncationStep{}

	// 缺少一个 }
	raw := `[{"function": {"name": "read_file", "arguments": "{\"path\":\"/tmp/test\"}"}`
	calls, err := step.Repair("", raw)
	if err != nil {
		t.Fatalf("TruncationStep error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("name = %q", calls[0].Name)
	}
}

func TestTruncationStep_Valid(t *testing.T) {
	step := &TruncationStep{}
	raw := `[{"function": {"name": "read_file", "arguments": "{\"path\":\"/tmp/test\"}"}}]`
	calls, err := step.Repair("", raw)
	if err != nil {
		t.Fatalf("TruncationStep error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
}

func TestRepairPipeline_DirectParse(t *testing.T) {
	rp := NewRepairPipeline()

	raw := `[{"function": {"name": "read_file", "arguments": "{\"path\":\"/tmp/test\"}"}}]`
	calls, err := rp.Repair("", raw)
	if err != nil {
		t.Fatalf("Repair error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
}

func TestRepairPipeline_ScavengeFallback(t *testing.T) {
	rp := NewRepairPipeline()

	reasoning := `I'll use the tool: {"name": "write_file", "arguments": "{\"path\":\"/tmp/out\",\"content\":\"hello\"}"}`
	calls, err := rp.Repair(reasoning, "invalid json")
	if err != nil {
		t.Fatalf("Repair error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "write_file" {
		t.Errorf("name = %q", calls[0].Name)
	}
}

func TestRepairPipeline_TruncationFallback(t *testing.T) {
	rp := NewRepairPipeline()

	// 截断的 JSON
	raw := `[{"function": {"name": "read_file", "arguments": "{\"path\":\"/tmp/test\"}"}`
	calls, err := rp.Repair("", raw)
	if err != nil {
		t.Fatalf("Repair error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
}

func TestRepairPipeline_AllFail(t *testing.T) {
	rp := NewRepairPipeline()
	_, err := rp.Repair("", "not json at all")
	if err == nil {
		t.Error("expected error when all repairs fail")
	}
}

func TestToolCallJSON_MarshalUnmarshal(t *testing.T) {
	// 验证工具调用 JSON 往返
	calls := []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{
		{
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "read_file",
				Arguments: `{"path":"/tmp/test"}`,
			},
		},
	}

	data, err := json.Marshal(calls)
	if err != nil {
		t.Fatal(err)
	}

	var parsed []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed[0].Function.Name != "read_file" {
		t.Errorf("name = %q", parsed[0].Function.Name)
	}
}
