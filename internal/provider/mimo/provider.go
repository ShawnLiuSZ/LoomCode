package mimo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/ShawnLiuSZ/loomcode/internal/consts"
	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

const kind = "mimo"

// Adapter MiMo 适配器
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

	// MiMo 特性：OAuth 认证、语音支持、推理
	// 对齐 DeepSeek 的缓存/修复能力声明：
	//   - SupportsPrefixCache + CacheTTL 启用 Agent 的 CacheScheduler（loop.go:57），
	//     使 MiMo 会话也享受稳定前缀带来的 prefix cache 命中收益；
	//   - NeedsToolRepair 声明 MiMo 流式存在重复 tool-call 发射风险（MiMo-Code 文档记录），
	//     为后续接入工具调用修复流水线提供门控依据。
	caps := provider.Capabilities{
		SupportsReasoning:    true,
		SupportsToolCall:     true,
		SupportsPrefixCache:  true,
		SupportsStreaming:    true,
		SupportsVoice:        true,
		SupportsOAuth:        cfg.AuthMethod == "oauth",
		NeedsToolRepair:      true,
		CacheTTL:             5 * time.Minute,
		MaxToolCallsPerRound: 16,
	}

	return &MiMoProvider{
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

// MiMoProvider MiMo API Provider
type MiMoProvider struct {
	name   string
	models []provider.ModelInfo
	caps   provider.Capabilities
	client *provider.RetryableHTTPClient
	cfg    provider.Config
}

func (p *MiMoProvider) Name() string                        { return p.name }
func (p *MiMoProvider) Models() []provider.ModelInfo        { return p.models }
func (p *MiMoProvider) Capabilities() provider.Capabilities { return p.caps }

func (p *MiMoProvider) Cost(modelID string, usage provider.Usage) provider.Cost {
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
		Currency:   "CNY",
	}
}

// Chat 非流式对话（OpenAI 兼容）
func (p *MiMoProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
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
	p.setHeaders(httpReq)

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
		// H23 修复：错误响应体可能含敏感内容，仅保留截断预览。
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, truncateSensitive(string(respBody)))
	}

	return parseChatResponse(respBody)
}

// Stream 流式对话
func (p *MiMoProvider) Stream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
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
	p.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

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

// setHeaders 设置请求头（支持 OAuth 和 API Key）
func (p *MiMoProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	if p.cfg.AuthMethod == "oauth" {
		// OAuth: 使用 Bearer token
		if p.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
		}
	} else {
		// API Key
		if p.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
		}
	}
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
				Type: "function",
				Function: provider.ToolCallFunc{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
				Args: args,
			})
		}
	}

	return resp, nil
}

// readSSEStream 读取 SSE 流
func (p *MiMoProvider) readSSEStream(ctx context.Context, resp *http.Response, ch chan<- provider.StreamEvent) {
	provider.ReadSSEStream(ctx, resp, ch, func(data []byte) (string, []provider.ToolCallDelta, *provider.Usage, map[string]any, bool, error) {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(data, &chunk); err != nil {
			return "", nil, nil, nil, false, fmt.Errorf("parse mimo SSE chunk: %w", err)
		}

		var content string
		var toolCalls []provider.ToolCallDelta
		var extra map[string]any

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta

			if delta.ReasoningContent != "" {
				extra = map[string]any{"reasoning_content": delta.ReasoningContent}
			}
			if delta.Content != "" {
				content = delta.Content
			}

			for _, tc := range delta.ToolCalls {
				toolCalls = append(toolCalls, provider.ToolCallDelta{
					Index:     tc.Index,
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}

		var usage *provider.Usage
		if chunk.Usage != nil {
			usage = &provider.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		return content, toolCalls, usage, extra, false, nil
	})
}

// truncateSensitive 截断可能含敏感内容的字符串，仅保留前 N 字节用于错误诊断。
// H23 修复：避免把完整响应体（含用户输入/工具结果）写入错误字符串被日志持久化。
func truncateSensitive(s string) string {
	const maxLen = 200
	if len(s) <= maxLen {
		return s
	}
	end := maxLen
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "...(truncated)"
}
