package openai

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

func TestAdapter_Kind(t *testing.T) {
	a := &Adapter{}
	if a.Kind() != "openai" {
		t.Errorf("Kind() = %q, want %q", a.Kind(), "openai")
	}
}

func TestAdapter_ValidateConfig(t *testing.T) {
	a := &Adapter{}

	tests := []struct {
		name    string
		cfg     provider.Config
		wantErr bool
	}{
		{"valid", provider.Config{
			BaseURL: "https://api.openai.com/v1",
			Models: []provider.ModelConfigItem{
				{ID: "gpt-4", Name: "GPT-4"},
			},
		}, false},
		{"no base_url", provider.Config{
			Models: []provider.ModelConfigItem{{ID: "m1"}},
		}, true},
		{"no models", provider.Config{
			BaseURL: "https://api.example.com",
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := a.ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdapter_Create(t *testing.T) {
	a := &Adapter{}
	cfg := provider.Config{
		Name:    "test-openai",
		BaseURL: "https://api.example.com/v1",
		APIKey:  "sk-test",
		Models: []provider.ModelConfigItem{
			{ID: "gpt-4", Name: "GPT-4", ContextWindow: 8192},
		},
	}

	p, err := a.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if p.Name() != "test-openai" {
		t.Errorf("Name() = %q", p.Name())
	}
	if len(p.Models()) != 1 {
		t.Errorf("Models() count = %d", len(p.Models()))
	}
	if p.Models()[0].ID != "gpt-4" {
		t.Errorf("Model ID = %q", p.Models()[0].ID)
	}
}

func TestParseChatResponse_Text(t *testing.T) {
	data := []byte(`{
		"choices": [{
			"message": {
				"content": "Hello, world!"
			}
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15
		}
	}`)

	resp, err := parseChatResponse(data)
	if err != nil {
		t.Fatalf("parseChatResponse() error: %v", err)
	}
	if resp.Content != "Hello, world!" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d", resp.Usage.TotalTokens)
	}
}

func TestParseChatResponse_ToolCall(t *testing.T) {
	data := []byte(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_1",
					"function": {
						"name": "read_file",
						"arguments": "{\"path\":\"/tmp/test\"}"
					}
				}]
			}
		}],
		"usage": {
			"prompt_tokens": 20,
			"completion_tokens": 10,
			"total_tokens": 30
		}
	}`)

	resp, err := parseChatResponse(data)
	if err != nil {
		t.Fatalf("parseChatResponse() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("tool name = %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].ID != "call_1" {
		t.Errorf("tool id = %q", resp.ToolCalls[0].ID)
	}
}

func TestParseChatResponse_InvalidJSON(t *testing.T) {
	_, err := parseChatResponse([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCost(t *testing.T) {
	p := &OpenAIProvider{
		cfg: provider.Config{
			Models: []provider.ModelConfigItem{
				{ID: "gpt-4", CostInput: 10.0, CostOutput: 30.0},
			},
		},
		models: []provider.ModelInfo{
			{ID: "gpt-4", Cost: provider.ModelCost{Input: 10.0, Output: 30.0}},
		},
	}

	cost := p.Cost("gpt-4", provider.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
	})

	// 1000/1M * 10 = 0.01, 500/1M * 30 = 0.015
	expectedInput := 0.01
	expectedOutput := 0.015

	if cost.InputCost != expectedInput {
		t.Errorf("InputCost = %f, want %f", cost.InputCost, expectedInput)
	}
	if cost.OutputCost != expectedOutput {
		t.Errorf("OutputCost = %f, want %f", cost.OutputCost, expectedOutput)
	}
	if cost.TotalCost != expectedInput+expectedOutput {
		t.Errorf("TotalCost = %f", cost.TotalCost)
	}
	if cost.Currency != "USD" {
		t.Errorf("Currency = %q", cost.Currency)
	}
}

func TestOpenAIProvider_Chat_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"hello from mock"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	cfg := provider.Config{
		Name:    "test",
		BaseURL: server.URL,
		Models: []provider.ModelConfigItem{
			{ID: "test-model", Name: "Test Model"},
		},
	}

	a := &Adapter{}
	p, err := a.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	resp, err := p.Chat(t.Context(), &provider.ChatRequest{
		Model: "test-model",
		Messages: []provider.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp.Content != "hello from mock" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestOpenAIProvider_Chat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	cfg := provider.Config{
		Name:    "test",
		BaseURL: server.URL,
		Models: []provider.ModelConfigItem{
			{ID: "test-model", Name: "Test Model"},
		},
	}

	a := &Adapter{}
	p, _ := a.Create(cfg)

	_, err := p.Chat(t.Context(), &provider.ChatRequest{
		Model: "test-model",
		Messages: []provider.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestOpenAIProvider_Chat_AuthHeader(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()

	cfg := provider.Config{
		Name:    "test",
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Models: []provider.ModelConfigItem{
			{ID: "test-model", Name: "Test Model"},
		},
	}

	a := &Adapter{}
	p, _ := a.Create(cfg)
	p.Chat(t.Context(), &provider.ChatRequest{
		Model:    "test-model",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})

	if receivedAuth != "Bearer sk-test-key" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer sk-test-key")
	}
}

