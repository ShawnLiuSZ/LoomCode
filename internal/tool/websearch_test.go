package tool

import (
	"context"
	"testing"
)

func TestWebSearchTool(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		engine := &mockSearchEngine{}
		tool := NewWebSearchTool(engine)
		if tool.Name() != "web_search" {
			t.Errorf("expected 'web_search', got %q", tool.Name())
		}
	})

	t.Run("IsReadOnly", func(t *testing.T) {
		engine := &mockSearchEngine{}
		tool := NewWebSearchTool(engine)
		if !tool.IsReadOnly() {
			t.Error("expected web_search to be read-only")
		}
	})

	t.Run("Execute_Success", func(t *testing.T) {
		engine := &mockSearchEngine{
			results: []SearchResult{
				{Title: "Result 1", URL: "http://example.com/1", Snippet: "Snippet 1"},
				{Title: "Result 2", URL: "http://example.com/2", Snippet: "Snippet 2"},
			},
		}
		tool := NewWebSearchTool(engine)

		result, err := tool.Execute(context.Background(), map[string]any{
			"query": "test query",
			"limit": 2,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Content == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("Execute_EmptyQuery", func(t *testing.T) {
		engine := &mockSearchEngine{}
		tool := NewWebSearchTool(engine)

		_, err := tool.Execute(context.Background(), map[string]any{
			"query": "",
		})

		if err == nil {
			t.Error("expected error for empty query")
		}
	})
}

func TestSearchEngineTypes(t *testing.T) {
	tests := []struct {
		engineType SearchEngineType
		expected   string
	}{
		{SearchEngineBing, "bing"},
		{SearchEngineTavily, "tavily"},
		{SearchEngineSearXNG, "searxng"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.engineType) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.engineType))
			}
		})
	}
}

func TestCreateSearchEngine(t *testing.T) {
	t.Run("UnknownEngine", func(t *testing.T) {
		_, err := CreateSearchEngine("unknown")
		if err == nil {
			t.Error("expected error for unknown engine")
		}
	})
}

// mockSearchEngine 模拟搜索引擎
type mockSearchEngine struct {
	results []SearchResult
	err     error
}

func (m *mockSearchEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockSearchEngine) Name() string {
	return "mock"
}
