package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ShawnLiuSZ/Helix/internal/consts"
	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

const kind = "deepseek"

// Adapter DeepSeek 适配器
type Adapter struct{}

func (a *Adapter) Kind() string { return kind }

func (a *Adapter) Create(cfg provider.Config) (provider.Provider, error) {
	if err := a.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	models := make([]provider.ModelInfo, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		models = append(models, provider.ModelInfo{
			ID:            m.ID,
			Name:          m.Name,
			ContextWindow: m.ContextWindow,
			Cost: provider.ModelCost{
				Input:       m.CostInput,
				CachedInput: m.CostCachedInput,
				Output:      m.CostOutput,
			},
		})
	}

	// DeepSeek 特有：前缀缓存 + 可能需要工具修复
	caps := provider.Capabilities{
		SupportsReasoning:    true,
		SupportsToolCall:     true,
		SupportsPrefixCache:  true,
		SupportsStreaming:    true,
		NeedsToolRepair:      true,
		CacheTTL:             5 * time.Minute,
		MaxToolCallsPerRound: 16,
	}

	return &DeepSeekProvider{
		name:   cfg.Name,
		models: models,
		caps:   caps,
		client: provider.NewRetryableClient(consts.DefaultHTTPTimeout),
		cfg:    cfg,
	}, nil
}

func (a *Adapter) ValidateConfig(cfg provider.Config) error {
	if cfg.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if len(cfg.Models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	return nil
}

// DeepSeekProvider DeepSeek API Provider
type DeepSeekProvider struct {
	name   string
	models []provider.ModelInfo
	caps   provider.Capabilities
	client *provider.RetryableHTTPClient
	cfg    provider.Config
}

func (p *DeepSeekProvider) Name() string                          { return p.name }
func (p *DeepSeekProvider) Models() []provider.ModelInfo          { return p.models }
func (p *DeepSeekProvider) Capabilities() provider.Capabilities   { return p.caps }

func (p *DeepSeekProvider) Cost(modelID string, usage provider.Usage) provider.Cost {
	var modelCost provider.ModelCost
	for _, m := range p.models {
		if m.ID == modelID {
			modelCost = m.Cost
			break
		}
	}

	uncachedTokens := usage.PromptTokens - usage.CachedInputTokens
	if uncachedTokens < 0 {
		uncachedTokens = usage.PromptTokens
	}
	inputCost := float64(uncachedTokens)/1_000_000*modelCost.Input +
		float64(usage.CachedInputTokens)/1_000_000*modelCost.CachedInput
	outputCost := float64(usage.CompletionTokens) / 1_000_000 * modelCost.Output

	return provider.Cost{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Currency:   "USD",
	}
}

// Chat 非流式对话
func (p *DeepSeekProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	url := p.cfg.BaseURL + "/chat/completions"
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return parseChatResponse(respBody)
}

// Stream 流式对话（支持 reasoning_content）
func (p *DeepSeekProvider) Stream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	url := p.cfg.BaseURL + "/chat/completions"
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	ch := make(chan provider.StreamEvent, consts.StreamChannelBufferSize)
	go p.readDeepSeekStream(resp, ch)

	return ch, nil
}

// parseChatResponse 解析非流式响应
func parseChatResponse(data []byte) (*provider.ChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			TotalTokens         int `json:"total_tokens"`
			PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	resp := &provider.ChatResponse{
		Usage: provider.Usage{
			PromptTokens:      raw.Usage.PromptTokens,
			CompletionTokens:  raw.Usage.CompletionTokens,
			TotalTokens:       raw.Usage.TotalTokens,
			CachedInputTokens: raw.Usage.PromptCacheHitTokens,
		},
	}

	if len(raw.Choices) > 0 {
		msg := raw.Choices[0].Message
		resp.Content = msg.Content
		resp.ReasoningContent = msg.ReasoningContent

		for _, tc := range msg.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = nil
			}
			resp.ToolCalls = append(resp.ToolCalls, provider.ToolCall{
				ID:   tc.ID,
				Function: provider.ToolCallFunc{Name: tc.Function.Name},
				Args: args,
			})
		}
	}

	return resp, nil
}

// readDeepSeekStream 读取 DeepSeek SSE 流（含 reasoning_content）
func (p *DeepSeekProvider) readDeepSeekStream(resp *http.Response, ch chan<- provider.StreamEvent) {
	ctx := context.Background()
	provider.ReadSSEStream(ctx, resp, ch, func(data []byte) (string, []provider.ToolCallDelta, *provider.Usage, map[string]any, bool, error) {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens         int `json:"prompt_tokens"`
				CompletionTokens     int `json:"completion_tokens"`
				TotalTokens          int `json:"total_tokens"`
				PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(data, &chunk); err != nil {
			return "", nil, nil, nil, false, fmt.Errorf("parse deepseek SSE chunk: %w", err)
		}

		var content string
		var toolCalls []provider.ToolCallDelta
		var extra map[string]any

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta

			// DeepSeek thinking 模式：reasoning_content 和 content 分开处理
			if delta.ReasoningContent != "" {
				extra = map[string]any{"reasoning_content": delta.ReasoningContent}
			}
			if delta.Content != "" {
				content = delta.Content
			}

			for _, tc := range delta.ToolCalls {
				toolCalls = append(toolCalls, provider.ToolCallDelta{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}

		var usage *provider.Usage
		if chunk.Usage != nil {
			usage = &provider.Usage{
				PromptTokens:      chunk.Usage.PromptTokens,
				CompletionTokens:  chunk.Usage.CompletionTokens,
				TotalTokens:       chunk.Usage.TotalTokens,
				CachedInputTokens: chunk.Usage.PromptCacheHitTokens,
			}
		}

		return content, toolCalls, usage, extra, false, nil
	})
}

// newLineScanner 创建基于 bufio.Scanner 的逐行扫描器
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024) // 1MB max line
	return scanner
}
