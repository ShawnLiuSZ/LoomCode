package agent

import (
	"context"
	"fmt"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// plannerSystemPrompt 规划器专用系统提示。
// planner 只分析、用只读工具探查代码库，产出结构化计划文本，不执行任何写操作。
const plannerSystemPrompt = `You are LoomCode in Planner mode.

Your role is to analyze the task and the codebase, then produce a clear, step-by-step plan.
You have READ-ONLY access to tools (read_file, grep, glob, git_status, git_diff, git_log, recall_memory, skill).
Use them to gather facts before planning — do not guess file contents or paths.

Rules:
- Do NOT execute, modify, or write any files. Only plan.
- Produce a concrete, ordered, step-by-step plan that the executor can follow.
- Keep the plan concise but complete: each step should be independently actionable.
- End with a short summary of risks or edge cases to watch for.`

// Coordinator 管理 planner/executor 分离 session 的协调器。
//
// 设计目标（参考 DeepSeek-Reasonix SPEC.md §3.5）：
// 当使用不同模型（planner 用强模型规划，executor 用快模型执行）时，
// 两个模型的对话历史若混在一个 session 里会互相污染 prefix cache。
//
// Coordinator 让 planner 和 executor 在两个独立 session 中运行：
//   - planner 在自己的 session 中用只读工具集产出结构化计划
//   - executor 在自己的 session 中用完整工具集执行计划
//
// 两个 session 的 messages 互不混合，各自 prepend-only 增长，保持 cache-friendly。
type Coordinator struct {
	planner      *Agent // 规划器（只读工具，独立 session）
	executor     *Agent // 执行器（完整工具，独立 session）
	plannerModel string // 规划器模型（可选，空则用 executor 模型）
}

// NewCoordinator 创建协调器。
// plannerProvider / executorProvider 可以相同（单模型）或不同（双模型）。
// registry 是完整工具集；planner 会自动获得一个只含只读工具的子集。
func NewCoordinator(plannerProvider, executorProvider provider.Provider, registry *tool.Registry) *Coordinator {
	readonlyReg := newReadOnlyRegistry(registry)

	planner := New(plannerProvider, readonlyReg)
	planner.SetSystemPrompt(plannerSystemPrompt)

	executor := New(executorProvider, registry)

	return &Coordinator{
		planner:  planner,
		executor: executor,
	}
}

// SetPlannerModel 设置规划器模型（独立于 executor 模型）。
// 空字符串表示与 executor 共用模型。
func (c *Coordinator) SetPlannerModel(model string) {
	c.plannerModel = model
	c.planner.SetModel(model)
}

// SetExecutorModel 设置执行器模型。
func (c *Coordinator) SetExecutorModel(model string) {
	c.executor.SetModel(model)
}

// SetWorkDir 设置两个 agent 的工作目录。
func (c *Coordinator) SetWorkDir(dir string) {
	c.planner.SetWorkDir(dir)
	c.executor.SetWorkDir(dir)
}

// SetMaxSteps 设置两个 agent 的最大步数。
func (c *Coordinator) SetMaxSteps(n int) {
	c.planner.SetMaxSteps(n)
	c.executor.SetMaxSteps(n)
}

// Planner 暴露 planner Agent（供测试与高级用法）。
func (c *Coordinator) Planner() *Agent { return c.planner }

// Executor 暴露 executor Agent（供测试与高级用法）。
func (c *Coordinator) Executor() *Agent { return c.executor }

// Run 执行任务：planner 先在独立 session 中规划，executor 再在独立 session 中执行。
//
// 两阶段：
//  1. planner 阶段：planner 在自己的 session 中产出纯文本计划
//  2. executor 阶段：executor 在自己的 session 中拿到计划后执行
//
// 两个 session 的 messages 数组互不交叉：planner 只看到原始 task，
// executor 只看到 "Plan:\n{plan}\n\nExecute the plan above."。
func (c *Coordinator) Run(ctx context.Context, task string) (string, error) {
	// 阶段 1：planner 规划
	plan, err := c.planner.Run(ctx, task)
	if err != nil {
		return "", fmt.Errorf("planner phase failed: %w", err)
	}
	if plan == "" {
		return "", fmt.Errorf("planner produced empty plan")
	}

	// 阶段 2：executor 执行计划
	execTask := fmt.Sprintf("Plan:\n%s\n\nExecute the plan above.", plan)
	return c.executor.Run(ctx, execTask)
}

// RunStream 流式执行任务（executor 阶段流式输出）。
//
// planner 阶段为非流式（产出计划文本）；executor 阶段流式输出执行过程。
// 流式增量写入 textCh，错误写入 errCh。
//
// 调用方负责创建和关闭通道；应在 RunStream 返回后再关闭通道，
// 并应在单独 goroutine 中读取 textCh 以避免死锁。
func (c *Coordinator) RunStream(ctx context.Context, task string, textCh chan<- string, errCh chan<- error) {
	// 阶段 1：planner 规划（非流式）
	plan, err := c.planner.Run(ctx, task)
	if err != nil {
		errCh <- fmt.Errorf("planner phase failed: %w", err)
		return
	}
	if plan == "" {
		errCh <- fmt.Errorf("planner produced empty plan")
		return
	}

	// 阶段 2：executor 流式执行
	execTask := fmt.Sprintf("Plan:\n%s\n\nExecute the plan above.", plan)
	execTextCh, execErrCh := c.executor.RunStream(ctx, execTask)

	for t := range execTextCh {
		textCh <- t
	}
	if e, ok := <-execErrCh; ok {
		errCh <- e
	}
}

// newReadOnlyRegistry 从完整 registry 过滤出只读工具，构建新的 Registry。
// planner 只能用只读工具（read_file, grep, glob, git_status, git_diff, git_log,
// recall_memory, skill 等），不能调用写工具（write_file, edit_file, bash, git_commit 等）。
//
// 通过 Tool.IsReadOnly() 接口方法判定，而非硬编码工具名列表，
// 这样未来新增的只读工具会自动被 planner 纳入，无需维护白名单。
func newReadOnlyRegistry(registry *tool.Registry) *tool.Registry {
	readonly := tool.NewRegistry()
	if registry == nil {
		return readonly
	}
	for _, t := range registry.List() {
		if !t.IsReadOnly() {
			continue
		}
		// 忽略重复注册错误（源 registry 本身无重名，理论上不会触发）
		_ = readonly.Register(t)
	}
	return readonly
}
