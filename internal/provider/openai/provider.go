package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/ShawnLiuSZ/loomcode/internal/consts"
	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

const kind = "openai"

// Adapter OpenAI 适配器
type Adapter struct{}

// Kind 返回适配器类型
func (a *Adapter) Kind() string { return kind }

// Create 创建 Provider 实例
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

	caps := provider.Capabilities{
		SupportsStreaming:    true,
		SupportsToolCall:     true,
		MaxToolCallsPerRound: 16,
	}

	return &OpenAIProvider{
		name:   cfg.Name,
		models: models,
		caps:   caps,
		client: provider.NewRetryableClient(consts.DefaultHTTPTimeout),
		cfg:    cfg,
	}, nil
}

// ValidateConfig 验证配置
func (a *Adapter) ValidateConfig(cfg provider.Config) error {
	if cfg.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if len(cfg.Models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	return nil
}

// OpenAIProvider 通用 OpenAI 兼容 Provider
type OpenAIProvider struct {
	name   string
	models []provider.ModelInfo
	caps   provider.Capabilities
	client *provider.RetryableHTTPClient
	cfg    provider.Config
}

func (p *OpenAIProvider) Name() string                        { return p.name }
func (p *OpenAIProvider) Models() []provider.ModelInfo        { return p.models }
func (p *OpenAIProvider) Capabilities() provider.Capabilities { return p.caps }

func (p *OpenAIProvider) Cost(modelID string, usage provider.Usage) provider.Cost {
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
	cachedRate := modelCost.CachedInput
	if cachedRate == 0 {
		cachedRate = modelCost.Input
	}
	inputCost := float64(uncachedTokens)/1_000_000*modelCost.Input +
		float64(usage.CachedInputTokens)/1_000_000*cachedRate
	outputCost := float64(usage.CompletionTokens) / 1_000_000 * modelCost.Output

	return provider.Cost{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Currency:   "USD",
	}
}

// Chat 非流式对话
func (p *OpenAIProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
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
		// H23 修复：错误响应体可能含敏感内容（用户输入/工具结果等），
		// 仅保留截断预览，避免把完整响应体写进错误字符串被日志持久化泄露。
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, truncateSensitive(string(respBody)))
	}

	return parseChatResponse(respBody)
}

// Stream 流式对话
func (p *OpenAIProvider) Stream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
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
		// 5.4 修复：Stream 错误路径同样使用 truncateSensitive，与 Chat 保持一致。
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, truncateSensitive(string(body)))
	}

	ch := make(chan provider.StreamEvent, consts.StreamChannelBufferSize)
	go p.readSSEStream(ctx, resp, ch)

	return ch, nil
}

// parseChatResponse 解析非流式响应
func parseChatResponse(data []byte) (*provider.ChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	resp := &provider.ChatResponse{
		Usage: provider.Usage{
			PromptTokens:     raw.Usage.PromptTokens,
			CompletionTokens: raw.Usage.CompletionTokens,
			TotalTokens:      raw.Usage.TotalTokens,
		},
	}

	if len(raw.Choices) > 0 {
		resp.Content = raw.Choices[0].Message.Content
		for _, tc := range raw.Choices[0].Message.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = nil
			}
			resp.ToolCalls = append(resp.ToolCalls, provider.ToolCall{
				ID:       tc.ID,
				Function: provider.ToolCallFunc{Name: tc.Function.Name},
				Args:     args,
			})
		}
	}

	return resp, nil
}

// readSSEStream 读取 SSE 流
func (p *OpenAIProvider) readSSEStream(ctx context.Context, resp *http.Response, ch chan<- provider.StreamEvent) {
	provider.ReadSSEStream(ctx, resp, ch, func(data []byte) (string, []provider.ToolCallDelta, *provider.Usage, map[string]any, bool, error) {
		content, toolCalls, usage, _ := provider.ParseStandardChunk(data)
		return content, toolCalls, usage, nil, false, nil
	})
}

// truncateSensitive 截断可能含敏感内容的字符串，仅保留前 N 字节用于错误诊断。
// H23 修复：避免把完整响应体（含用户输入/工具结果）写入错误字符串被日志持久化。
func truncateSensitive(s string) string {
	const maxLen = 200
	if len(s) <= maxLen {
		return s
	}
	// 回退到最后一个 UTF-8 rune 边界，避免切断多字节字符产生乱码
	end := maxLen
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "...(truncated)"
}
