package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/skills"
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
	goal      *GoalStopCondition // 停止条件
	skillsMgr *skills.Manager    // Skills 管理器
	onCost    func(float64)      // 成本回调
}

// New 创建 Agent
func New(p provider.Provider, registry *tool.Registry) *Agent {
	return &Agent{
		provider:  p,
		tools:     registry,
		executor:  tool.NewExecutor(registry),
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

// SetSkillsManager 设置 Skills 管理器
func (a *Agent) SetSkillsManager(mgr *skills.Manager) { a.skillsMgr = mgr }

// SetCostCallback 设置成本回调（每次 API 调用后触发）
func (a *Agent) SetCostCallback(fn func(float64)) { a.onCost = fn }

// AddGuard 添加工具执行守卫
func (a *Agent) AddGuard(g tool.Guard) { a.executor.AddGuard(g) }

// getContextWindow 获取当前模型的上下文窗口大小
func (a *Agent) getContextWindow() int {
	for _, m := range a.provider.Models() {
		if m.ID == a.model && m.ContextWindow > 0 {
			return m.ContextWindow
		}
	}
	return 0 // 未知模型，不截断
}

// estimateTokens 粗略估算消息列表的 token 数（1 token ≈ 4 bytes for English, ~2 for CJK）
func estimateTokens(messages []provider.Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 3 // 粗略估算
		if len(msg.ToolCalls) > 0 {
			total += 50 * len(msg.ToolCalls) // 工具调用开销
		}
	}
	return total
}

// truncateMessages 截断消息列表以适应上下文窗口
// 按"轮次"删除：找到最旧的完整 tool-call 轮次（assistant+tool_results），整轮删除
// 避免删除 tool 消息但保留 assistant tool_calls 导致 API 400 错误
func (a *Agent) truncateMessages(ctxWindow int) {
	if ctxWindow <= 0 {
		return
	}

	maxInput := ctxWindow * 80 / 100

	tokens := estimateTokens(a.messages)
	if tokens <= maxInput {
		return
	}

	// 保留 system prompt (index 0) 和最近 4 条消息
	const keepRecent = 4

	for len(a.messages) > keepRecent+1 && tokens > maxInput {
		// 找到最旧的完整轮次：从后往前找第一个 assistant(tool_calls) + 其 tool results
		roundStart := -1
		roundEnd := -1

		for i := len(a.messages) - keepRecent - 1; i >= 1; i-- {
			msg := a.messages[i]
			if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				// 找到 assistant tool_calls，向后找所有对应的 tool 结果
				roundStart = i
				roundEnd = i
				for j := i + 1; j < len(a.messages)-keepRecent; j++ {
					if a.messages[j].Role == "tool" {
						roundEnd = j
					} else {
						break
					}
				}
				break
			}
		}

		// 如果没找到完整轮次，删除最旧的非 system 消息
		if roundStart == -1 {
			roundStart = 1
			roundEnd = 1
		}

		// 删除整个轮次 [roundStart, roundEnd]
		deleted := a.messages[roundStart : roundEnd+1]
		tokens -= estimateTokens(deleted)
		a.messages = append(a.messages[:roundStart], a.messages[roundEnd+1:]...)
	}
}

// RunStream 流式运行 Agent，通过 channel 返回文本增量
func (a *Agent) RunStream(ctx context.Context, task string) (<-chan string, <-chan error) {
	textCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
		defer close(errCh)

		sysPrompt := a.buildSystemPrompt()

		a.messages = []provider.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: task},
		}

		for step := 0; step < a.maxSteps; step++ {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			// 截断以适应上下文窗口
			a.truncateMessages(a.getContextWindow())

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
			var lastUsage *provider.Usage

			for event := range streamCh {
				switch event.Type {
				case provider.EventText:
					fullContent += event.Content
					textCh <- event.Content
				case provider.EventToolCall:
					if event.ToolCall != nil {
						toolCallDeltas = append(toolCallDeltas, *event.ToolCall)
					}
				case provider.EventDone:
					if event.Usage != nil {
						lastUsage = event.Usage
					}
				case provider.EventError:
					errCh <- fmt.Errorf("stream error: %s", event.Content)
					return
				}
			}

			if lastUsage != nil && a.onCost != nil {
				cost := a.provider.Cost(a.model, *lastUsage)
				a.onCost(cost.TotalCost)
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
				// 安全检查：确保结果存在
				if i >= len(toolResults) || toolResults[i] == nil {
					a.messages = append(a.messages, provider.Message{
						Role:       "tool",
						Content:    "Error: tool execution returned no result",
						ToolCallID: tc.ID,
					})
					continue
				}
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
	sysPrompt := a.buildSystemPrompt()

	a.messages = []provider.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: task},
	}

	for step := 0; step < a.maxSteps; step++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// 截断以适应上下文窗口
		a.truncateMessages(a.getContextWindow())

		resp, err := a.provider.Chat(ctx, &provider.ChatRequest{
			Model:    a.model,
			Messages: a.messages,
			Tools:    a.buildToolDefs(),
		})
		if err != nil {
			return "", fmt.Errorf("chat error (step %d): %w", step, err)
		}

		if a.onCost != nil {
			cost := a.provider.Cost(a.model, resp.Usage)
			a.onCost(cost.TotalCost)
		}

		// 追加 assistant 消息（含 tool_calls）
		assistantMsg := provider.Message{Role: "assistant"}
		if resp.Content != "" {
			assistantMsg.Content = resp.Content
		}
		if len(resp.ToolCalls) > 0 {
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

		// 追加工具结果（带安全检查）
		for i, tc := range resp.ToolCalls {
			if i >= len(toolResults) || toolResults[i] == nil {
				a.messages = append(a.messages, provider.Message{
					Role:       "tool",
					Content:    "Error: tool execution returned no result",
					ToolCallID: tc.ID,
				})
				continue
			}
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

		// 每 3 步评估一次 Goal
		if a.goal.IsEnabled() && (step+1)%3 == 0 {
			achieved, reason, evalErr := a.goal.Evaluate(ctx, a.messages)
			if evalErr == nil && achieved {
				return fmt.Sprintf("Goal achieved: %s", reason), nil
			}
		}
	}

	// 最终检查 Goal
	if a.goal.IsEnabled() {
		achieved, reason, evalErr := a.goal.Evaluate(ctx, a.messages)
		if evalErr == nil && achieved {
			return fmt.Sprintf("Goal achieved: %s", reason), nil
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

	if a.skillsMgr != nil {
		skillList := a.skillsMgr.List()
		if len(skillList) > 0 {
			sb.WriteString("\n## Available Skills (use /skills <name> to load full content)\n")
			for _, s := range skillList {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
			}
		}
	}

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
