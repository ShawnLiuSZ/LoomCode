package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	contextpkg "github.com/ShawnLiuSZ/Helix/internal/context"
	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// Agent 核心 Agent
type Agent struct {
	provider  provider.Provider
	tools     *tool.Registry
	executor  *tool.Executor
	messages  []provider.Message
	maxSteps  int
	model     string
	partition *contextpkg.Partition // 上下文分区（缓存感知）
	goal      *GoalStopCondition   // 停止条件
}

// New 创建 Agent
func New(p provider.Provider, registry *tool.Registry) *Agent {
	caps := p.Capabilities()
	ttl := caps.CacheTTL
	if ttl == 0 {
		ttl = 5 * time.Minute // 默认 5 分钟
	}

	return &Agent{
		provider:  p,
		tools:     registry,
		executor:  tool.NewExecutor(registry),
		partition: contextpkg.NewPartition(ttl),
		goal:      NewGoalStopCondition(p),
		maxSteps:  10,
	}
}

// SetMaxSteps 设置最大推理步数
func (a *Agent) SetMaxSteps(n int) { a.maxSteps = n }

// SetModel 设置模型名
func (a *Agent) SetModel(m string) { a.model = m }

// SetGoal 设置停止条件
func (a *Agent) SetGoal(goal string) { a.goal.SetGoal(goal) }

// GetGoal 获取当前停止条件
func (a *Agent) GetGoal() string { return a.goal.GetGoal() }

// ClearGoal 清除停止条件
func (a *Agent) ClearGoal() { a.goal.Clear() }

// AddGuard 添加工具执行守卫
func (a *Agent) AddGuard(g tool.Guard) { a.executor.AddGuard(g) }

// RunStream 流式运行 Agent，通过 channel 返回文本增量
func (a *Agent) RunStream(ctx context.Context, task string) (<-chan string, <-chan error) {
	textCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
		defer close(errCh)

		a.messages = []provider.Message{
			{Role: "system", Content: a.buildSystemPrompt()},
			{Role: "user", Content: task},
		}

		for step := 0; step < a.maxSteps; step++ {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			streamCh, err := a.provider.Stream(ctx, &provider.ChatRequest{
				Model:    a.model,
				Messages: a.messages,
				Tools:    a.buildToolDefs(),
			})
			if err != nil {
				errCh <- fmt.Errorf("stream error (step %d): %w", step, err)
				return
			}

			var fullContent string
			var toolCalls []provider.ToolCall
			var toolCallDeltas []provider.ToolCallDelta

			for event := range streamCh {
				switch event.Type {
				case provider.EventText:
					fullContent += event.Content
					textCh <- event.Content
				case provider.EventToolCall:
					if event.ToolCall != nil {
						toolCallDeltas = append(toolCallDeltas, *event.ToolCall)
					}
				case provider.EventError:
					errCh <- fmt.Errorf("stream error: %s", event.Content)
					return
				}
			}

			// 合并工具调用增量
			toolCalls = mergeToolCallDeltas(toolCallDeltas)

			// 追加 assistant 消息
			assistantMsg := provider.Message{Role: "assistant", Content: fullContent}
			if len(toolCalls) > 0 {
				for i := range toolCalls {
					argsJSON, _ := json.Marshal(toolCalls[i].Args)
					toolCalls[i].Function.Arguments = string(argsJSON)
					toolCalls[i].Type = "function"
				}
				assistantMsg.ToolCalls = toolCalls
			}
			a.messages = append(a.messages, assistantMsg)

			// 无工具调用 → 完成
			if len(toolCalls) == 0 {
				return
			}

			// 执行工具
			toolResults := a.executeTools(ctx, toolCalls)
			for i, tc := range toolCalls {
				content := toolResults[i].Content
				if !toolResults[i].OK() {
					content = fmt.Sprintf("Error: %s", toolResults[i].Error)
				}
				a.messages = append(a.messages, provider.Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: tc.ID,
				})
			}
		}

		errCh <- fmt.Errorf("max steps (%d) reached", a.maxSteps)
	}()

	return textCh, errCh
}

// mergeToolCallDeltas 合并流式工具调用增量
func mergeToolCallDeltas(deltas []provider.ToolCallDelta) []provider.ToolCall {
	byID := make(map[string]*provider.ToolCall)
	for _, d := range deltas {
		tc, ok := byID[d.ID]
		if !ok {
			tc = &provider.ToolCall{ID: d.ID, Function: provider.ToolCallFunc{Name: d.Name}}
			byID[d.ID] = tc
		}
		if d.Name != "" {
			tc.Function.Name = d.Name
		}
		// 累积 arguments
		if d.Arguments != "" {
			tc.Function.Arguments += d.Arguments
		}
	}

	var result []provider.ToolCall
	for _, tc := range byID {
		if tc.Function.Arguments != "" {
			json.Unmarshal([]byte(tc.Function.Arguments), &tc.Args)
		}
		result = append(result, *tc)
	}
	return result
}
func (a *Agent) Run(ctx context.Context, task string) (string, error) {
	a.messages = []provider.Message{
		{Role: "system", Content: a.buildSystemPrompt()},
		{Role: "user", Content: task},
	}

	for step := 0; step < a.maxSteps; step++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		resp, err := a.provider.Chat(ctx, &provider.ChatRequest{
			Model:    a.model,
			Messages: a.messages,
			Tools:    a.buildToolDefs(),
		})
		if err != nil {
			return "", fmt.Errorf("chat error (step %d): %w", step, err)
		}

		// 追加 assistant 消息（含 tool_calls）
		assistantMsg := provider.Message{Role: "assistant"}
		if resp.Content != "" {
			assistantMsg.Content = resp.Content
		}
		if len(resp.ToolCalls) > 0 {
			// 序列化 Args 到 Function.Arguments
			for i := range resp.ToolCalls {
				argsJSON, _ := json.Marshal(resp.ToolCalls[i].Args)
				resp.ToolCalls[i].Function.Arguments = string(argsJSON)
				resp.ToolCalls[i].Type = "function"
			}
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		a.messages = append(a.messages, assistantMsg)

		// 无工具调用 → 返回最终答案
		if len(resp.ToolCalls) == 0 {
			// 检查 Goal 是否达成
			if a.goal.IsEnabled() {
				achieved, _, evalErr := a.goal.Evaluate(ctx, a.messages)
				if evalErr == nil && achieved {
					return resp.Content, nil
				}
			}
			return resp.Content, nil
		}

		// 执行工具调用
		toolResults := a.executeTools(ctx, resp.ToolCalls)

		// 追加工具结果
		for i, tc := range resp.ToolCalls {
			result := toolResults[i]
			content := result.Content
			if !result.OK() {
				content = fmt.Sprintf("Error: %s", result.Error)
			}
			a.messages = append(a.messages, provider.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: tc.ID,
			})
		}

		// 每 3 步评估一次 Goal（避免过于频繁）
		if a.goal.IsEnabled() && (step+1)%3 == 0 {
			achieved, reason, evalErr := a.goal.Evaluate(ctx, a.messages)
			if evalErr == nil && achieved {
				return fmt.Sprintf("Goal achieved: %s", reason), nil
			}
		}
	}

	return "", fmt.Errorf("max steps (%d) reached", a.maxSteps)
}

// executeTools 执行工具调用
func (a *Agent) executeTools(ctx context.Context, toolCalls []provider.ToolCall) []*tool.Result {
	calls := make([]tool.Call, len(toolCalls))
	for i, tc := range toolCalls {
		calls[i] = tool.Call{
			Name: tc.Function.Name,
			Args: tc.Args,
		}
	}
	return a.executor.Execute(ctx, calls)
}

// buildSystemPrompt 构建系统提示词
func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are Helix, an AI coding assistant.\n")
	sb.WriteString("You have access to tools for reading/writing files, executing commands, and searching code.\n")
	sb.WriteString("When asked to complete a task, use the appropriate tools and provide a final answer.\n")
	return sb.String()
}

// buildToolDefs 构建工具定义列表
func (a *Agent) buildToolDefs() []provider.ToolDef {
	if a.tools == nil {
		return nil
	}
	tools := a.tools.List()
	defs := make([]provider.ToolDef, 0, len(tools))
	for _, t := range tools {
		schema := t.Schema()
		defs = append(defs, provider.ToolDef{
			Type: "function",
			Function: provider.FunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters: map[string]any{
					"type":       schema.Type,
					"properties": schema.Properties,
					"required":   schema.Required,
				},
			},
		})
	}
	return defs
}
