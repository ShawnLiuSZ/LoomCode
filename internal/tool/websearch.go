package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SearchEngine 搜索引擎接口
type SearchEngine interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
	Name() string
}

// SearchResult 搜索结果
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WebSearchTool 网络搜索工具
type WebSearchTool struct {
	engine SearchEngine
}

// NewWebSearchTool 创建网络搜索工具
func NewWebSearchTool(engine SearchEngine) *WebSearchTool {
	return &WebSearchTool{engine: engine}
}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) Description() string { return "Search the web for information" }
func (t *WebSearchTool) IsReadOnly() bool    { return true }

func (t *WebSearchTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"query": {Type: "string", Description: "The search query"},
			"limit": {Type: "integer", Description: "Maximum number of results (default: 5)"},
		},
		Required: []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := 5
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	results, err := t.engine.Search(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %s\n\n", query))

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		sb.WriteString(fmt.Sprintf("   %s\n\n", result.Snippet))
	}

	return &Result{Content: sb.String()}, nil
}

// BingSearch Bing 搜索引擎
type BingSearch struct {
	apiKey string
	client *http.Client
}

// NewBingSearch 创建 Bing 搜索引擎
func NewBingSearch(apiKey string) *BingSearch {
	return &BingSearch{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *BingSearch) Name() string { return "bing" }

func (s *BingSearch) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Bing API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.bing.microsoft.com/v7.0/search?q=%s&count=%d",
		url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, page := range data.WebPages.Value {
		results = append(results, SearchResult{
			Title:   page.Name,
			URL:     page.URL,
			Snippet: page.Snippet,
		})
	}

	return results, nil
}

// TavilySearch Tavily 搜索引擎
type TavilySearch struct {
	apiKey string
	client *http.Client
}

// NewTavilySearch 创建 Tavily 搜索引擎
func NewTavilySearch(apiKey string) *TavilySearch {
	return &TavilySearch{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *TavilySearch) Name() string { return "tavily" }

func (s *TavilySearch) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Tavily API key not configured")
	}

	reqBody := map[string]any{
		"api_key": s.apiKey,
		"query":   query,
		"max_results": limit,
	}

	jsonBody, _ := json.Marshal(reqBody)

	resp, err := s.client.Post("https://api.tavily.com/search", "application/json",
		strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range data.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}

// SearXNGSearch SearXNG 搜索引擎（自托管）
type SearXNGSearch struct {
	endpoint string
	client   *http.Client
}

// NewSearXNGSearch 创建 SearXNG 搜索引擎
func NewSearXNGSearch(endpoint string) *SearXNGSearch {
	return &SearXNGSearch{
		endpoint: strings.TrimSuffix(endpoint, "/"),
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SearXNGSearch) Name() string { return "searxng" }

func (s *SearXNGSearch) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	reqURL := fmt.Sprintf("%s/search?q=%s&format=json&pageno=1",
		s.endpoint, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var results []SearchResult
	for i, r := range data.Results {
		if i >= limit {
			break
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}

// SearchEngineType 搜索引擎类型
type SearchEngineType string

const (
	SearchEngineBing     SearchEngineType = "bing"
	SearchEngineTavily   SearchEngineType = "tavily"
	SearchEngineSearXNG  SearchEngineType = "searxng"
)

// CreateSearchEngine 创建搜索引擎
func CreateSearchEngine(engineType SearchEngineType) (SearchEngine, error) {
	switch engineType {
	case SearchEngineBing:
		apiKey := os.Getenv("BING_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("BING_API_KEY not configured")
		}
		return NewBingSearch(apiKey), nil

	case SearchEngineTavily:
		apiKey := os.Getenv("TAVILY_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("TAVILY_API_KEY not configured")
		}
		return NewTavilySearch(apiKey), nil

	case SearchEngineSearXNG:
		endpoint := os.Getenv("SEARXNG_ENDPOINT")
		if endpoint == "" {
			endpoint = "http://localhost:8888"
		}
		return NewSearXNGSearch(endpoint), nil

	default:
		return nil, fmt.Errorf("unknown search engine: %s", engineType)
	}
}
