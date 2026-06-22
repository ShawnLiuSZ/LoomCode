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
	onCost    func(float64)

	// Plan 模式：计划内容
	plan string

	// Compose 模式：规格内容
	spec string
}

// NewMultiAgent 创建多模式 Agent
func NewMultiAgent(p provider.Provider, registry *tool.Registry) *MultiAgent {
	return &MultiAgent{
		provider: p,
		tools:    registry,
		executor: tool.NewExecutor(registry),
		mode:     ModeBuild,
		maxSteps: 20,
		goal:     NewGoalStopCondition(p),
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

// SetModel 设置模型名
func (a *MultiAgent) SetModel(m string) {
	a.model = m
}

// SetSkillsManager 设置 Skills 管理器
func (a *MultiAgent) SetSkillsManager(mgr *skills.Manager) {
	a.skillsMgr = mgr
}

// SetCostCallback 设置成本回调
func (a *MultiAgent) SetCostCallback(fn func(float64)) {
	a.onCost = fn
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
	if a.skillsMgr != nil {
		ag.SetSkillsManager(a.skillsMgr)
	}
	if a.onCost != nil {
		ag.SetCostCallback(a.onCost)
	}
	return ag.RunStream(ctx, task)
}

// runBuild Build 模式：完整工具权限
func (a *MultiAgent) runBuild(ctx context.Context, task string) (string, error) {
	ag := New(a.provider, a.tools)
	ag.SetMaxSteps(a.maxSteps)
	ag.SetModel(a.model)
	if a.skillsMgr != nil {
		ag.SetSkillsManager(a.skillsMgr)
	}
	if a.onCost != nil {
		ag.SetCostCallback(a.onCost)
	}
	if a.goal.IsEnabled() {
		ag.SetGoal(a.goal.GetGoal())
	}
	return ag.Run(ctx, task)
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

	// 启发式选优：优先选非错误、非空的最长结果
	var best string
	var bestScore int
	for _, r := range candidateResults {
		if r == "" {
			continue
		}
		score := len(r)
		// 非错误结果加分
		if !strings.HasPrefix(r, "[error:") {
			score += 10000
		}
		// 包含实际内容（非纯空格/换行）加分
		trimmed := strings.TrimSpace(r)
		if len(trimmed) > 0 {
			score += 1000
		}
		if score > bestScore {
			bestScore = score
			best = r
		}
	}

	if best == "" {
		return "", fmt.Errorf("all %d candidates failed or returned empty", candidates)
	}
	return best, nil
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
	}
	return defs
}
