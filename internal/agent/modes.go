package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/skills"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// Mode Agent 模式
type Mode int

const (
	ModeBuild   Mode = iota // 默认，完整工具权限
	ModePlan                // 只读分析
	ModeCompose             // 编排模式
	ModeMax                 // 并行选优（实验性）
)

func (m Mode) String() string {
	switch m {
	case ModeBuild:
		return "build"
	case ModePlan:
		return "plan"
	case ModeCompose:
		return "compose"
	case ModeMax:
		return "max"
	default:
		return "unknown"
	}
}

// ModeFromString 从字符串解析模式
func ModeFromString(s string) Mode {
	switch strings.ToLower(s) {
	case "build":
		return ModeBuild
	case "plan":
		return ModePlan
	case "compose":
		return ModeCompose
	case "max":
		return ModeMax
	default:
		return ModeBuild
	}
}

// MultiAgent 多模式 Agent
type MultiAgent struct {
	provider  provider.Provider
	tools     *tool.Registry
	executor  *tool.Executor
	mode      Mode
	model     string
	maxSteps  int
	messages  []provider.Message
	goal      *GoalStopCondition
	skillsMgr *skills.Manager
	memory    MemoryProvider
	onCost    func(float64)
	effort    *EffortManager
	workDir   string
	hooks     *tool.HookManager

	plan string
	spec string

	conversation []provider.Message // 跨轮次对话历史（不含 system），维持多轮连续

	guards []tool.Guard

	costBudget       float64
	onBudgetExceeded func()
}

// NewMultiAgent 创建多模式 Agent
func NewMultiAgent(p provider.Provider, registry *tool.Registry) *MultiAgent {
	effort := NewEffortManager()
	return &MultiAgent{
		provider: p,
		tools:    registry,
		executor: tool.NewExecutor(registry),
		mode:     ModeBuild,
		maxSteps: effort.GetMaxSteps(),
		goal:     NewGoalStopCondition(p),
		effort:   effort,
	}
}

// SetMode 切换 Agent 模式
func (a *MultiAgent) SetMode(mode Mode) {
	a.mode = mode
}

// Mode 返回当前模式
func (a *MultiAgent) Mode() Mode {
	return a.mode
}

// SetMaxSteps 设置最大步数
func (a *MultiAgent) SetMaxSteps(n int) {
	a.maxSteps = n
}

// GetMaxSteps 获取最大步数
func (a *MultiAgent) GetMaxSteps() int {
	return a.maxSteps
}

// SetEffort 设置思考强度管理器
func (a *MultiAgent) SetEffort(e *EffortManager) {
	a.effort = e
	a.maxSteps = e.GetMaxSteps()
}

// GetEffort 获取思考强度管理器
func (a *MultiAgent) GetEffort() *EffortManager {
	return a.effort
}

// SetModel 设置模型名
func (a *MultiAgent) SetModel(m string) {
	a.model = m
}

// SetWorkDir 设置工作目录
func (a *MultiAgent) SetWorkDir(d string) {
	a.workDir = d
}

// SetSkillsManager 设置 Skills 管理器
func (a *MultiAgent) SetSkillsManager(mgr *skills.Manager) {
	a.skillsMgr = mgr
}

// SetMemory 设置记忆提供者（注入系统提示）
func (a *MultiAgent) SetMemory(m MemoryProvider) {
	a.memory = m
}

// SetHistory 用历史消息（不含 system）初始化对话，用于恢复历史会话后保持上下文连续
func (a *MultiAgent) SetHistory(msgs []provider.Message) {
	a.conversation = msgs
}

// ResetConversation 清空跨轮次对话历史（如 /clear）
func (a *MultiAgent) ResetConversation() {
	a.conversation = nil
}

// SetCostCallback 设置成本回调
func (a *MultiAgent) SetCostCallback(fn func(float64)) {
	a.onCost = fn
}

// SetCostBudget 设置成本预算上限
func (a *MultiAgent) SetCostBudget(b float64) {
	a.costBudget = b
}

// SetOnBudgetExceeded 设置预算超限回调
func (a *MultiAgent) SetOnBudgetExceeded(fn func()) {
	a.onBudgetExceeded = fn
}

// SetGoal 设置停止条件
func (a *MultiAgent) SetGoal(goal string) {
	a.goal.SetGoal(goal)
}

// GetGoal 获取当前停止条件
func (a *MultiAgent) GetGoal() string {
	return a.goal.GetGoal()
}

// ClearGoal 清除停止条件
func (a *MultiAgent) ClearGoal() {
	a.goal.Clear()
}

func (a *MultiAgent) AddGuard(g tool.Guard) {
	a.guards = append(a.guards, g)
}

func (a *MultiAgent) SetHooks(hm *tool.HookManager) {
	a.hooks = hm
}

// SetPlan 设置计划内容（Plan 模式）
func (a *MultiAgent) SetPlan(plan string) {
	a.plan = plan
}

// SetSpec 设置规格内容（Compose 模式）
func (a *MultiAgent) SetSpec(spec string) {
	a.spec = spec
}

// Run 根据当前模式执行任务
func (a *MultiAgent) Run(ctx context.Context, task string) (string, error) {
	switch a.mode {
	case ModeBuild:
		return a.runBuild(ctx, task)
	case ModePlan:
		return a.runPlan(ctx, task)
	case ModeCompose:
		return a.runCompose(ctx, task)
	case ModeMax:
		return a.runMax(ctx, task)
	default:
		return a.runBuild(ctx, task)
	}
}

// RunStream 流式执行任务（仅 Build 模式支持）
func (a *MultiAgent) RunStream(ctx context.Context, task string) (<-chan string, <-chan error) {
	ag := New(a.provider, a.tools)
	ag.SetMaxSteps(a.maxSteps)
	ag.SetModel(a.model)
	ag.SetWorkDir(a.workDir)
	if a.effort != nil {
		ag.SetEffort(a.effort)
	}
	if a.skillsMgr != nil {
		ag.SetSkillsManager(a.skillsMgr)
	}
	if a.memory != nil {
		ag.SetMemory(a.memory)
	}
	if a.onCost != nil {
		ag.SetCostCallback(a.onCost)
	}
	if a.costBudget > 0 {
		ag.SetCostBudget(a.costBudget)
	}
	if a.onBudgetExceeded != nil {
		ag.SetOnBudgetExceeded(a.onBudgetExceeded)
	}
	for _, g := range a.guards {
		ag.AddGuard(g)
	}
	ag.SetHistory(a.conversation)

	textCh, errCh := ag.RunStream(ctx, task)

	// 包装通道：流结束后捕获本轮对话，供下一轮延续。
	// TUI 串行执行（用户等回复后才发下一条），故对 a.conversation 的写入无竞态。
	outText := make(chan string, 100)
	outErr := make(chan error, 1)
	go func() {
		defer close(outText)
		defer close(outErr)
		for t := range textCh {
			outText <- t
		}
		var rerr error
		if e, ok := <-errCh; ok {
			rerr = e
		}
		a.conversation = ag.ConversationMessages()
		if rerr != nil {
			outErr <- rerr
		}
	}()
	return outText, outErr
}

// runBuild Build 模式：完整工具权限
func (a *MultiAgent) runBuild(ctx context.Context, task string) (string, error) {
	ag := New(a.provider, a.tools)
	ag.SetMaxSteps(a.maxSteps)
	ag.SetModel(a.model)
	ag.SetWorkDir(a.workDir)
	if a.effort != nil {
		ag.SetEffort(a.effort)
	}
	if a.skillsMgr != nil {
		ag.SetSkillsManager(a.skillsMgr)
	}
	if a.memory != nil {
		ag.SetMemory(a.memory)
	}
	if a.onCost != nil {
		ag.SetCostCallback(a.onCost)
	}
	if a.costBudget > 0 {
		ag.SetCostBudget(a.costBudget)
	}
	if a.onBudgetExceeded != nil {
		ag.SetOnBudgetExceeded(a.onBudgetExceeded)
	}
	if a.goal.IsEnabled() {
		ag.SetGoal(a.goal.GetGoal())
	}
	for _, g := range a.guards {
		ag.AddGuard(g)
	}
	ag.SetHistory(a.conversation)
	result, err := ag.Run(ctx, task)
	a.conversation = ag.ConversationMessages() // 捕获本轮对话，供下一轮延续
	return result, err
}

// runPlan Plan 模式：只读分析，不执行写操作
func (a *MultiAgent) runPlan(ctx context.Context, task string) (string, error) {
	a.messages = []provider.Message{
		{Role: "system", Content: a.buildPlanPrompt()},
		{Role: "user", Content: task},
	}

	// 如果有预设计划，加入上下文
	if a.plan != "" {
		a.messages = append(a.messages, provider.Message{
			Role: "system", Content: "Existing plan:\n" + a.plan,
		})
	}

	resp, err := a.provider.Chat(ctx, &provider.ChatRequest{
		Model:    "",
		Messages: a.messages,
		Tools:    a.buildReadOnlyToolDefs(),
	})
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// runCompose Compose 模式：规格驱动开发
func (a *MultiAgent) runCompose(ctx context.Context, task string) (string, error) {
	a.messages = []provider.Message{
		{Role: "system", Content: a.buildComposePrompt()},
		{Role: "user", Content: task},
	}

	if a.spec != "" {
		a.messages = append(a.messages, provider.Message{
			Role: "system", Content: "Specification:\n" + a.spec,
		})
	}

	// Compose 使用完整工具集
	agent := New(a.provider, a.tools)
	agent.SetMaxSteps(a.maxSteps)
	agent.SetWorkDir(a.workDir)
	if a.effort != nil {
		agent.SetEffort(a.effort)
	}
	if a.memory != nil {
		agent.SetMemory(a.memory)
	}
	return agent.Run(ctx, task)
}

// runMax Max 模式：并行 N 候选 + judge 选最优
func (a *MultiAgent) runMax(ctx context.Context, task string) (string, error) {
	const candidates = 3
	type result struct {
		content string
		index   int
	}

	ch := make(chan result, candidates)
	for i := 0; i < candidates; i++ {
		go func(idx int) {
			agent := New(a.provider, a.tools)
			agent.SetMaxSteps(a.maxSteps)
			agent.SetWorkDir(a.workDir)
			if a.effort != nil {
				agent.SetEffort(a.effort)
			}
			r, err := agent.Run(ctx, task)
			if err != nil {
				ch <- result{content: fmt.Sprintf("[error: %v]", err), index: idx}
				return
			}
			ch <- result{content: r, index: idx}
		}(i)
	}

	// 收集候选
	candidateResults := make([]string, candidates)
	for i := 0; i < candidates; i++ {
		r := <-ch
		candidateResults[r.index] = r.content
	}

	best, judgeErr := a.judgeMaxCandidates(ctx, task, candidateResults)
	if judgeErr == nil && best != "" {
		return best, nil
	}

	var bestFallback string
	var bestScore int
	for _, r := range candidateResults {
		if r == "" {
			continue
		}
		score := len(r)
		if !strings.HasPrefix(r, "[error:") {
			score += 10000
		}
		trimmed := strings.TrimSpace(r)
		if len(trimmed) > 0 {
			score += 1000
		}
		if score > bestScore {
			bestScore = score
			bestFallback = r
		}
	}

	if bestFallback == "" {
		return "", fmt.Errorf("all %d candidates failed or returned empty", candidates)
	}
	return bestFallback, nil
}

func (a *MultiAgent) judgeMaxCandidates(ctx context.Context, task string, candidates []string) (string, error) {
	if len(candidates) <= 1 {
		if len(candidates) == 1 && candidates[0] != "" && !strings.HasPrefix(candidates[0], "[error:") {
			return candidates[0], nil
		}
		return "", fmt.Errorf("not enough valid candidates to judge")
	}

	var sb strings.Builder
	sb.WriteString("You are a judge evaluating candidate answers to a coding task.\n\n")
	sb.WriteString("## Task\n")
	sb.WriteString(task)
	sb.WriteString("\n\n## Candidates\n\n")
	for i, c := range candidates {
		sb.WriteString(fmt.Sprintf("### Candidate %d\n%s\n\n", i+1, c))
	}
	sb.WriteString(fmt.Sprintf("Select the best candidate based on correctness, completeness, and code quality.\n"))
	sb.WriteString(fmt.Sprintf("Return ONLY the number (1-%d) of the best candidate. No explanation.", len(candidates)))

	resp, err := a.provider.Chat(ctx, &provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: sb.String()},
		},
	})
	if err != nil {
		return "", fmt.Errorf("judge LLM call failed: %w", err)
	}

	choice := strings.TrimSpace(resp.Content)
	for _, ch := range choice {
		if ch >= '1' && ch <= '9' {
			idx := int(ch - '1')
			if idx < len(candidates) && candidates[idx] != "" && !strings.HasPrefix(candidates[idx], "[error:") {
				return candidates[idx], nil
			}
			return "", fmt.Errorf("judge selected candidate %d which is empty or errored", ch-'0')
		}
	}
	return "", fmt.Errorf("judge returned invalid response: %q", choice)
}

// buildPlanPrompt Plan 模式系统提示
func (a *MultiAgent) buildPlanPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are Helix in Plan mode.\n")
	sb.WriteString("Your role is to analyze code and create development plans.\n")
	sb.WriteString("You have READ-ONLY access to tools. Do NOT modify any files.\n")
	sb.WriteString("Provide a clear, step-by-step plan for implementation.\n")
	return sb.String()
}

// buildComposePrompt Compose 模式系统提示
func (a *MultiAgent) buildComposePrompt() string {
	var sb strings.Builder
	sb.WriteString("You are Helix in Compose mode.\n")
	sb.WriteString("Your role is specification-driven development.\n")
	sb.WriteString("Follow the specification to implement features step by step.\n")
	sb.WriteString("Test your changes after each implementation step.\n")
	return sb.String()
}

// buildReadOnlyToolDefs 构建只读工具定义
func (a *MultiAgent) buildReadOnlyToolDefs() []provider.ToolDef {
	allTools := a.tools.List()
	defs := make([]provider.ToolDef, 0)
	for _, t := range allTools {
		if t.IsReadOnly() {
			schema := t.Schema()
			params := map[string]any{
				"type":       schema.Type,
				"properties": schema.Properties,
			}
			// 仅在非空时设置 required，避免 nil slice 序列化成 "required": null
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
	}
	return defs
}
