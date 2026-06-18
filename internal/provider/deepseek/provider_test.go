package deepseek

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

func TestAdapter_Kind(t *testing.T) {
	a := &Adapter{}
	if a.Kind() != "deepseek" {
		t.Errorf("Kind() = %q, want %q", a.Kind(), "deepseek")
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
			BaseURL: "https://api.deepseek.com",
			Models:  []provider.ModelConfigItem{{ID: "deepseek-v4-flash"}},
		}, false},
		{"no base_url", provider.Config{
			Models: []provider.ModelConfigItem{{ID: "m1"}},
		}, true},
		{"no models", provider.Config{
			BaseURL: "https://api.deepseek.com",
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
		Name:    "test-deepseek",
		BaseURL: "https://api.deepseek.com",
		APIKey:  "sk-test",
		Models: []provider.ModelConfigItem{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextWindow: 131072},
		},
	}

	p, err := a.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if p.Name() != "test-deepseek" {
		t.Errorf("Name() = %q", p.Name())
	}
	if len(p.Models()) != 1 {
		t.Errorf("Models() count = %d", len(p.Models()))
	}

	caps := p.Capabilities()
	if !caps.SupportsReasoning {
		t.Error("SupportsReasoning should be true")
	}
	if !caps.SupportsPrefixCache {
		t.Error("SupportsPrefixCache should be true")
	}
	if !caps.NeedsToolRepair {
		t.Error("NeedsToolRepair should be true")
	}
}

func TestParseChatResponse_Text(t *testing.T) {
	data := []byte(`{
		"choices": [{
			"message": {
				"content": "Hello from DeepSeek!"
			}
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15,
			"prompt_cache_hit_tokens": 8
		}
	}`)

	resp, err := parseChatResponse(data)
	if err != nil {
		t.Fatalf("parseChatResponse() error: %v", err)
	}
	if resp.Content != "Hello from DeepSeek!" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestParseChatResponse_Reasoning(t *testing.T) {
	data := []byte(`{
		"choices": [{
			"message": {
				"reasoning_content": "Let me think about this...",
				"content": "The answer is 42"
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

	if !strings.Contains(resp.Content, "Let me think") {
		t.Errorf("Content should contain reasoning: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "The answer is 42") {
		t.Errorf("Content should contain final answer: %q", resp.Content)
	}
}

func TestParseChatResponse_ToolCall(t *testing.T) {
	data := []byte(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_abc",
					"function": {
						"name": "read_file",
						"arguments": "{\"path\":\"/tmp/test\"}"
					}
				}]
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
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("tool name = %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].ID != "call_abc" {
		t.Errorf("tool id = %q", resp.ToolCalls[0].ID)
	}
}

func TestParseChatResponse_CacheHit(t *testing.T) {
	data := []byte(`{
		"choices": [{"message": {"content": "ok"}}],
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 10,
			"total_tokens": 110,
			"prompt_cache_hit_tokens": 90
		}
	}`)

	resp, err := parseChatResponse(data)
	if err != nil {
		t.Fatalf("parseChatResponse() error: %v", err)
	}
	if resp.Usage.TotalTokens != 110 {
		t.Errorf("TotalTokens = %d", resp.Usage.TotalTokens)
	}
}

func TestDeepSeekProvider_Chat_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"DeepSeek response"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	cfg := provider.Config{
		Name:    "test",
		BaseURL: server.URL,
		Models: []provider.ModelConfigItem{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash"},
		},
	}

	a := &Adapter{}
	p, err := a.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	resp, err := p.Chat(t.Context(), &provider.ChatRequest{
		Model:    "deepseek-v4-flash",
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp.Content != "DeepSeek response" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestDeepSeekProvider_Chat_AuthHeader(t *testing.T) {
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
		APIKey:  "sk-deepseek-key",
		Models: []provider.ModelConfigItem{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash"},
		},
	}

	a := &Adapter{}
	p, _ := a.Create(cfg)
	p.Chat(t.Context(), &provider.ChatRequest{
		Model:    "deepseek-v4-flash",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})

	if receivedAuth != "Bearer sk-deepseek-key" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer sk-deepseek-key")
	}
}

func TestDeepSeekProvider_Chat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	cfg := provider.Config{
		Name:    "test",
		BaseURL: server.URL,
		Models: []provider.ModelConfigItem{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash"},
		},
	}

	a := &Adapter{}
	p, _ := a.Create(cfg)

	_, err := p.Chat(t.Context(), &provider.ChatRequest{
		Model:    "deepseek-v4-flash",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestCost(t *testing.T) {
	p := &DeepSeekProvider{
		cfg: provider.Config{
			Models: []provider.ModelConfigItem{
				{ID: "deepseek-v4-flash", CostInput: 0.14, CostCachedInput: 0.014, CostOutput: 0.28},
			},
		},
		models: []provider.ModelInfo{
			{ID: "deepseek-v4-flash", Cost: provider.ModelCost{Input: 0.14, CachedInput: 0.014, Output: 0.28}},
		},
	}

	cost := p.Cost("deepseek-v4-flash", provider.Usage{
		PromptTokens:     1000000,
		CompletionTokens: 1000000,
	})

	if cost.InputCost != 0.14 {
		t.Errorf("InputCost = %f, want 0.14", cost.InputCost)
	}
	if cost.OutputCost != 0.28 {
		t.Errorf("OutputCost = %f, want 0.28", cost.OutputCost)
	}
}
