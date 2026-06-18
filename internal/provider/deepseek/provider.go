package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
		client: &http.Client{Timeout: 120 * time.Second},
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
	client *http.Client
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

	inputCost := float64(usage.PromptTokens) / 1_000_000 * modelCost.Input
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
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	ch := make(chan provider.StreamEvent, 100)
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
			PromptTokens:     raw.Usage.PromptTokens,
			CompletionTokens: raw.Usage.CompletionTokens,
			TotalTokens:      raw.Usage.TotalTokens,
		},
	}

	if len(raw.Choices) > 0 {
		msg := raw.Choices[0].Message
		// reasoning_content 合并到 content 前缀
		if msg.ReasoningContent != "" {
			resp.Content = "/* reasoning */ " + msg.ReasoningContent + "\n" + msg.Content
		} else {
			resp.Content = msg.Content
		}

		for _, tc := range msg.ToolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			resp.ToolCalls = append(resp.ToolCalls, provider.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			})
		}
	}

	return resp, nil
}

// readDeepSeekStream 读取 DeepSeek SSE 流（含 reasoning_content）
func (p *DeepSeekProvider) readDeepSeekStream(resp *http.Response, ch chan<- provider.StreamEvent) {
	defer close(ch)
	defer resp.Body.Close()

	scanner := newLineScanner(resp.Body)
	var reasoningBuf strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 提取 data:
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			data, ok = strings.CutPrefix(line, "data:")
			if !ok {
				continue
			}
		}

		// [DONE]
		if strings.TrimSpace(data) == "[DONE]" {
			ch <- provider.StreamEvent{Type: provider.EventDone}
			return
		}

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
				PromptTokens        int `json:"prompt_tokens"`
				CompletionTokens    int `json:"completion_tokens"`
				TotalTokens         int `json:"total_tokens"`
				PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta

			// 推理内容（DeepSeek 特有）
			if delta.ReasoningContent != "" {
				reasoningBuf.WriteString(delta.ReasoningContent)
				ch <- provider.StreamEvent{
					Type:    provider.EventText,
					Content: delta.ReasoningContent,
				}
			}

			// 普通文本
			if delta.Content != "" {
				ch <- provider.StreamEvent{
					Type:    provider.EventText,
					Content: delta.Content,
				}
			}

			// 工具调用
			if len(delta.ToolCalls) > 0 {
				tc := delta.ToolCalls[0]
				ch <- provider.StreamEvent{
					Type: provider.EventToolCall,
					ToolCall: &provider.ToolCallDelta{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		// 用量统计（含缓存命中）
		if chunk.Usage != nil {
			ch <- provider.StreamEvent{
				Type: provider.EventDone,
				Usage: &provider.Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				},
			}
		}
	}
}

// lineScanner 简单的逐行扫描器
type lineScanner struct {
	reader io.Reader
	buf    []byte
}

func newLineScanner(r io.Reader) *lineScanner {
	return &lineScanner{reader: r, buf: make([]byte, 0, 4096)}
}

func (s *lineScanner) Scan() bool {
	tmp := make([]byte, 4096)
	n, err := s.reader.Read(tmp)
	if n > 0 {
		s.buf = append(s.buf, tmp[:n]...)
	}
	return len(s.buf) > 0 || (err == nil)
}

func (s *lineScanner) Text() string {
	for i := 0; i < len(s.buf); i++ {
		if s.buf[i] == '\n' {
			line := string(s.buf[:i])
			if i+1 < len(s.buf) {
				s.buf = s.buf[i+1:]
			} else {
				s.buf = s.buf[:0]
			}
			return line
		}
	}
	if len(s.buf) > 0 {
		line := string(s.buf)
		s.buf = s.buf[:0]
		return line
	}
	return ""
}
