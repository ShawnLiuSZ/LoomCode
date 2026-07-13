package mimo

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

func TestAdapter_Kind(t *testing.T) {
	a := &Adapter{}
	if a.Kind() != "mimo" {
		t.Errorf("Kind() = %q, want %q", a.Kind(), "mimo")
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
			BaseURL: "https://api.mimo.xiaomi.com/v1",
			Models:  []provider.ModelConfigItem{{ID: "mimo-v2.5-pro"}},
		}, false},
		{"no base_url", provider.Config{
			Models: []provider.ModelConfigItem{{ID: "m1"}},
		}, true},
		{"no models", provider.Config{
			BaseURL: "https://api.mimo.xiaomi.com",
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
		Name:    "test-mimo",
		BaseURL: "https://api.mimo.xiaomi.com/v1",
		APIKey:  "sk-test",
		Models: []provider.ModelConfigItem{
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro", ContextWindow: 262144},
		},
	}

	p, err := a.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if p.Name() != "test-mimo" {
		t.Errorf("Name() = %q", p.Name())
	}
	if len(p.Models()) != 1 {
		t.Errorf("Models() count = %d", len(p.Models()))
	}

	caps := p.Capabilities()
	if !caps.SupportsReasoning {
		t.Error("SupportsReasoning should be true")
	}
	if !caps.SupportsVoice {
		t.Error("SupportsVoice should be true")
	}
	if !caps.SupportsPrefixCache {
		t.Error("SupportsPrefixCache should be true")
	}
	if !caps.NeedsToolRepair {
		t.Error("NeedsToolRepair should be true")
	}
	if caps.CacheTTL <= 0 {
		t.Error("CacheTTL should be > 0 to enable CacheScheduler")
	}
}

func TestAdapter_Create_OAuth(t *testing.T) {
	a := &Adapter{}
	cfg := provider.Config{
		Name:       "mimo-oauth",
		BaseURL:    "https://api.mimo.xiaomi.com/v1",
		AuthMethod: "oauth",
		APIKey:     "oauth-token",
		Models: []provider.ModelConfigItem{
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro"},
		},
	}

	p, err := a.Create(cfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	caps := p.Capabilities()
	if !caps.SupportsOAuth {
		t.Error("SupportsOAuth should be true for oauth auth method")
	}
}

func TestParseChatResponse_Text(t *testing.T) {
	data := []byte(`{
		"choices": [{"message": {"content": "Hello from MiMo!"}}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`)

	resp, err := parseChatResponse(data)
	if err != nil {
		t.Fatalf("parseChatResponse() error: %v", err)
	}
	if resp.Content != "Hello from MiMo!" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestParseChatResponse_ToolCall(t *testing.T) {
	data := []byte(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_mimo",
					"function": {"name": "read_file", "arguments": "{\"path\":\"/test\"}"}
				}]
			}
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`)

	resp, err := parseChatResponse(data)
	if err != nil {
		t.Fatalf("parseChatResponse() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("tool name = %q", resp.ToolCalls[0].Function.Name)
	}
}

func TestMiMoProvider_Chat_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"MiMo response"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	cfg := provider.Config{
		Name:    "test",
		BaseURL: server.URL,
		Models: []provider.ModelConfigItem{
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro"},
		},
	}

	a := &Adapter{}
	p, _ := a.Create(cfg)

	resp, err := p.Chat(t.Context(), &provider.ChatRequest{
		Model:    "mimo-v2.5-pro",
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp.Content != "MiMo response" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestMiMoProvider_Chat_AuthHeader(t *testing.T) {
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
		APIKey:  "sk-mimo-key",
		Models: []provider.ModelConfigItem{
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro"},
		},
	}

	a := &Adapter{}
	p, _ := a.Create(cfg)
	p.Chat(t.Context(), &provider.ChatRequest{
		Model:    "mimo-v2.5-pro",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})

	if receivedAuth != "Bearer sk-mimo-key" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer sk-mimo-key")
	}
}

func TestMiMoProvider_Chat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	cfg := provider.Config{
		Name:    "test",
		BaseURL: server.URL,
		Models: []provider.ModelConfigItem{
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro"},
		},
	}

	a := &Adapter{}
	p, _ := a.Create(cfg)

	_, err := p.Chat(t.Context(), &provider.ChatRequest{
		Model:    "mimo-v2.5-pro",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestCost(t *testing.T) {
	p := &MiMoProvider{
		cfg: provider.Config{
			Models: []provider.ModelConfigItem{
				{ID: "mimo-v2.5-pro", CostInput: 0.60, CostOutput: 2.40},
			},
		},
		models: []provider.ModelInfo{
			{ID: "mimo-v2.5-pro", Cost: provider.ModelCost{Input: 0.60, Output: 2.40}},
		},
	}

	cost := p.Cost("mimo-v2.5-pro", provider.Usage{
		PromptTokens:     1000000,
		CompletionTokens: 1000000,
	})

	if cost.InputCost != 0.60 {
		t.Errorf("InputCost = %f, want 0.60", cost.InputCost)
	}
	if cost.OutputCost != 2.40 {
		t.Errorf("OutputCost = %f, want 2.40", cost.OutputCost)
	}
	if cost.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", cost.Currency)
	}
}
