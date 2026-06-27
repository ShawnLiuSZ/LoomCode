package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	workDir   string
	goal      *GoalStopCondition // 停止条件
	skillsMgr *skills.Manager    // Skills 管理器
	memory    MemoryProvider     // 记忆提供者（项目知识/用户偏好）
	history   []provider.Message // 跨轮次对话历史（不含 system，每轮重建）
	onCost    func(float64)      // 成本回调
	effort    *EffortManager     // 思考强度管理器

	costBudget       float64
	onBudgetExceeded func()
	costAccumulated  float64
}

// New 创建 Agent
func New(p provider.Provider, registry *tool.Registry) *Agent {
	effort := NewEffortManager()
	return &Agent{
		provider: p,
		tools:    registry,
		executor: tool.NewExecutor(registry),
		goal:     NewGoalStopCondition(p),
		maxSteps: effort.GetMaxSteps(),
		effort:   effort,
	}
}

// SetMaxSteps 设置最大推理步数
func (a *Agent) SetMaxSteps(n int) { a.maxSteps = n }

// SetEffort 设置思考强度管理器
func (a *Agent) SetEffort(e *EffortManager) {
	a.effort = e
	a.maxSteps = e.GetMaxSteps()
}

// GetEffort 获取思考强度管理器
func (a *Agent) GetEffort() *EffortManager { return a.effort }

// SetModel 设置模型名
func (a *Agent) SetModel(m string) { a.model = m }

// SetWorkDir 设置工作目录
func (a *Agent) SetWorkDir(d string) { a.workDir = d }

// SetGoal 设置停止条件
func (a *Agent) SetGoal(goal string) { a.goal.SetGoal(goal) }

// GetGoal 获取当前停止条件
func (a *Agent) GetGoal() string { return a.goal.GetGoal() }

// ClearGoal 清除停止条件
func (a *Agent) ClearGoal() { a.goal.Clear() }

// SetSkillsManager 设置 Skills 管理器，并注册 `skill` 工具，
// 让模型可以按需加载某个 skill 的完整说明（渐进式披露）。
func (a *Agent) SetSkillsManager(mgr *skills.Manager) {
	a.skillsMgr = mgr
	if mgr == nil || a.tools == nil {
		return
	}
	skillTool := tool.NewSkillTool(
		func() []tool.SkillInfo {
			out := make([]tool.SkillInfo, 0)
			for _, s := range mgr.List() {
				out = append(out, tool.SkillInfo{Name: s.Name, Description: s.Description})
			}
			return out
		},
		func(name string) (string, error) {
			s, ok := mgr.Get(name)
			if !ok {
				return "", fmt.Errorf("skill %q not found", name)
			}
			return s.Content()
		},
	)
	_ = a.tools.Register(skillTool) // 重复注册时忽略错误（已存在即可）
}

// SetMemory 设置记忆提供者，其内容会注入到系统提示，实现跨会话的项目知识/偏好记忆
func (a *Agent) SetMemory(m MemoryProvider) { a.memory = m }

// SetHistory 设置跨轮次对话历史（不含 system 消息）。下一次 Run/RunStream
// 会以 [system] + history + [新 user] 作为起点，实现多轮上下文连续。
func (a *Agent) SetHistory(msgs []provider.Message) { a.history = msgs }

// ConversationMessages 返回本轮结束后的完整对话（不含 system 提示），
// 供上层（MultiAgent）作为下一轮的历史延续。
func (a *Agent) ConversationMessages() []provider.Message {
	if len(a.messages) <= 1 {
		return nil
	}
	out := make([]provider.Message, len(a.messages)-1)
	copy(out, a.messages[1:])
	return out
}

// initMessages 以 [system] + history + [user:task] 初始化本轮消息列表。
// 系统提示每轮重建（含动态环境/记忆/skills），历史保留以维持多轮连续。
func (a *Agent) initMessages(task string) {
	a.messages = make([]provider.Message, 0, len(a.history)+2)
	a.messages = append(a.messages, provider.Message{Role: "system", Content: a.buildSystemPrompt()})
	a.messages = append(a.messages, a.history...)
	a.messages = append(a.messages, provider.Message{Role: "user", Content: task})
}

// SetCostCallback 设置成本回调（每次 API 调用后触发）
func (a *Agent) SetCostCallback(fn func(float64)) { a.onCost = fn }

// SetCostBudget 设置成本预算上限
func (a *Agent) SetCostBudget(b float64) { a.costBudget = b }

// SetOnBudgetExceeded 设置预算超限回调
func (a *Agent) SetOnBudgetExceeded(fn func()) { a.onBudgetExceeded = fn }

// AddGuard 添加工具执行守卫
func (a *Agent) AddGuard(g tool.Guard) { a.executor.AddGuard(g) }

// SetHooks 设置钩子管理器
func (a *Agent) SetHooks(hm *tool.HookManager) { a.executor.SetHooks(hm) }

// reasoningEffort 返回当前思考强度对应的 reasoning_effort 参数，
// 仅当 provider 声明支持推理时才返回非空值（capability-driven，避免向不支持的厂商发送未知字段）。
func (a *Agent) reasoningEffort() string {
	if a.effort == nil || !a.provider.Capabilities().SupportsReasoning {
		return ""
	}
	return a.effort.GetReasoningEffort()
}

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

		// 防止留下 orphan tool 消息：删除范围之后的连续 tool 结果也要一并带走，
		// 否则 assistant tool_call 被删但 tool 结果留下，触发 LLM API 400。
		for roundEnd+1 < len(a.messages)-keepRecent && a.messages[roundEnd+1].Role == "tool" {
			roundEnd++
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

		a.initMessages(task)

		for step := 0; step < a.maxSteps; step++ {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			// 压缩以适应上下文窗口（缓存友好，失败时回退机械截断）
			a.compactMessages(ctx, a.getContextWindow())

			streamCh, err := a.provider.Stream(ctx, &provider.ChatRequest{
				Model:           a.model,
				Messages:        a.messages,
				Tools:           a.buildToolDefs(),
				ReasoningEffort: a.reasoningEffort(),
			})
			if err != nil {
				errCh <- fmt.Errorf("stream error (step %d): %w", step, err)
				return
			}

			var fullContent string
			var reasoningContent string
			var toolCalls []provider.ToolCall
			var toolCallDeltas []provider.ToolCallDelta
			var lastUsage *provider.Usage

			for event := range streamCh {
				switch event.Type {
				case provider.EventText:
					if event.ReasoningContent != "" {
						reasoningContent += event.ReasoningContent
					} else {
						fullContent += event.Content
					}
					// 只显示实际内容给用户，不显示推理过程
					if event.Content != "" {
						textCh <- event.Content
					}
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
				a.costAccumulated += cost.TotalCost
				if a.costBudget > 0 && a.costAccumulated >= a.costBudget && a.onBudgetExceeded != nil {
					a.onBudgetExceeded()
				}
			}

			// 合并工具调用增量
			toolCalls = mergeToolCallDeltas(toolCallDeltas)

			// 追加 assistant 消息（包含 reasoning_content）
			assistantMsg := provider.Message{
				Role:             "assistant",
				Content:          fullContent,
				ReasoningContent: reasoningContent,
			}
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
	order := make([]string, 0) // 保持顺序
	lastID := ""

	for _, d := range deltas {
		// 流式传输中后续 chunk 可能没有 ID，使用上一个 ID
		id := d.ID
		if id == "" {
			id = lastID
		} else {
			lastID = id
		}

		tc, ok := byID[id]
		if !ok {
			tc = &provider.ToolCall{ID: id, Function: provider.ToolCallFunc{Name: d.Name}}
			byID[id] = tc
			order = append(order, id)
		}
		if d.Name != "" {
			tc.Function.Name = d.Name
		}
		if d.Arguments != "" {
			tc.Function.Arguments += d.Arguments
		}
	}

	result := make([]provider.ToolCall, 0, len(order))
	for _, id := range order {
		tc := byID[id]
		if tc.Function.Arguments != "" {
			json.Unmarshal([]byte(tc.Function.Arguments), &tc.Args)
		}
		result = append(result, *tc)
	}
	return result
}

func (a *Agent) Run(ctx context.Context, task string) (string, error) {
	a.initMessages(task)

	for step := 0; step < a.maxSteps; step++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// 压缩以适应上下文窗口（缓存友好，失败时回退机械截断）
		a.compactMessages(ctx, a.getContextWindow())

		resp, err := a.provider.Chat(ctx, &provider.ChatRequest{
			Model:           a.model,
			Messages:        a.messages,
			Tools:           a.buildToolDefs(),
			ReasoningEffort: a.reasoningEffort(),
		})
		if err != nil {
			return "", fmt.Errorf("chat error (step %d): %w", step, err)
		}

		if a.onCost != nil {
			cost := a.provider.Cost(a.model, resp.Usage)
			a.onCost(cost.TotalCost)
			a.costAccumulated += cost.TotalCost
			if a.costBudget > 0 && a.costAccumulated >= a.costBudget && a.onBudgetExceeded != nil {
				a.onBudgetExceeded()
			}
		}

		// 追加 assistant 消息（含 tool_calls 和 reasoning_content）
		assistantMsg := provider.Message{
			Role:             "assistant",
			Content:          resp.Content,
			ReasoningContent: resp.ReasoningContent,
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
				achieved, reason, evalErr := a.goal.Evaluate(ctx, a.messages)
				if evalErr == nil {
					if achieved {
						return resp.Content, nil
					}
					// 目标未达成，返回提示并继续尝试
					return fmt.Sprintf("%s\n\n⚠️ 目标未完全达成: %s", resp.Content, reason), nil
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
		// 不在循环中按固定步频调用 judge（LLM 调用，成本/延迟翻倍）。
		// Goal 仅在模型给出最终答复时、以及循环耗尽后做一次性评估。
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
	sb.WriteString("You are Helix, an AI coding assistant operating in a terminal.\n")
	sb.WriteString("You have access to tools for reading/writing files, editing code, executing commands, and searching code.\n")
	sb.WriteString("\n## Working principles\n")
	sb.WriteString("- Use tools to gather facts before acting; do not guess file contents or paths.\n")
	sb.WriteString("- Make the smallest change that satisfies the request; match the surrounding code style and conventions.\n")
	sb.WriteString("- Prefer editing existing files over creating new ones; do not add files unless necessary.\n")
	sb.WriteString("- After modifying code, verify it (read it back, run the relevant build/test) before claiming success.\n")
	sb.WriteString("- Be concise. Report what you did and what you found; omit information that does not change the outcome.\n")
	sb.WriteString("- When the task is complete, give a final answer without further tool calls.\n")

	sb.WriteString(a.buildEnvContext())

	if a.memory != nil {
		if mem := strings.TrimSpace(a.memory.BuildContextPrompt()); mem != "" {
			sb.WriteString("\n## Long-term Memory\n")
			sb.WriteString(mem)
			sb.WriteString("\n")
		}
	}

	if a.workDir != "" {
		if instructions := loadProjectInstructions(a.workDir); instructions != "" {
			sb.WriteString("\n## Project Instructions\n")
			sb.WriteString(instructions)
			sb.WriteString("\n")
		}
	}

	if a.skillsMgr != nil {
		skillList := a.skillsMgr.List()
		if len(skillList) > 0 {
			sb.WriteString("\n## Available Skills (call the `skill` tool with a name to load its full instructions)\n")
			for _, s := range skillList {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
			}
		}
	}

	return sb.String()
}

var projectInstructionFiles = []string{"HELIX.md", "HELIX.local.md", ".helix/instructions.md"}

func loadProjectInstructions(root string) string {
	var parts []string
	for _, name := range projectInstructionFiles {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
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
		params := map[string]any{
			"type":       schema.Type,
			"properties": schema.Properties,
		}
		// 仅在非空时设置 required，避免 nil slice 序列化成 "required": null
		// （DeepSeek/OpenAI API 要求该字段为 array，null 会触发 400）
		if len(schema.Required) > 0 {
			params["required"] = schema.Required
		}
		defs = append(defs, provider.ToolDef{
			Type: "function",
			Function: provider.FunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		})
	}
	return defs
}
