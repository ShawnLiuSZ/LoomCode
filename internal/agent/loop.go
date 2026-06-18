package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// Agent 核心 Agent
type Agent struct {
	provider provider.Provider
	tools    *tool.Registry
	executor *tool.Executor
	messages []provider.Message
	maxSteps int
	model    string
}

// New 创建 Agent
func New(p provider.Provider, registry *tool.Registry) *Agent {
	return &Agent{
		provider: p,
		tools:    registry,
		executor: tool.NewExecutor(registry),
		maxSteps: 10,
	}
}

// SetMaxSteps 设置最大推理步数
func (a *Agent) SetMaxSteps(n int) { a.maxSteps = n }

// SetModel 设置模型名
func (a *Agent) SetModel(m string) { a.model = m }

// AddGuard 添加工具执行守卫
func (a *Agent) AddGuard(g tool.Guard) { a.executor.AddGuard(g) }

// Run 运行 Agent，执行用户任务
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

		// 追加 assistant 消息
		assistantMsg := provider.Message{Role: "assistant"}
		if resp.Content != "" {
			assistantMsg.Content = resp.Content
		}
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		a.messages = append(a.messages, assistantMsg)

		// 无工具调用 → 返回最终答案
		if len(resp.ToolCalls) == 0 {
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
	}

	return "", fmt.Errorf("max steps (%d) reached", a.maxSteps)
}

// executeTools 执行工具调用
func (a *Agent) executeTools(ctx context.Context, toolCalls []provider.ToolCall) []*tool.Result {
	calls := make([]tool.Call, len(toolCalls))
	for i, tc := range toolCalls {
		calls[i] = tool.Call{
			Name: tc.Name,
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
