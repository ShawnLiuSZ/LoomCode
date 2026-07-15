package tool

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// 确定性剪枝阈值：超过此行数的结果保留头尾各 N 行，中间用占位符替代。
// 剪枝规则确定性（无随机数、无时间戳），保证相同输入产生相同输出 → prefix hash 稳定，
// 提升 prefix cache 命中率（P2 架构级优化）。
const (
	maxToolResultLines = 200 // 超过此行数触发剪枝
	headKeepLines      = 50  // 保留头部行数
	tailKeepLines      = 50  // 保留尾部行数
)

// Executor 工具执行引擎
type Executor struct {
	registry       *Registry
	guards         []Guard
	hooks          *HookManager
	maxParallel    int
	maxResultLines int // 触发剪枝的行数阈值，<=0 时使用 maxToolResultLines
	mu             sync.Mutex
}

// Guard 执行守卫函数
type Guard func(tc Call) error

// Call 工具调用请求
type Call struct {
	Name string
	Args map[string]any
}

// NewExecutor 创建执行引擎
func NewExecutor(registry *Registry) *Executor {
	return &Executor{
		registry:       registry,
		maxParallel:    3,
		maxResultLines: maxToolResultLines,
	}
}

// SetMaxParallel 设置最大并行数
func (e *Executor) SetMaxParallel(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n < 1 {
		n = 1
	}
	if n > 16 {
		n = 16
	}
	e.maxParallel = n
}

// MaxParallel 返回最大并行数
func (e *Executor) MaxParallel() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.maxParallel
}

// SetMaxResultLines 设置工具结果剪枝行数阈值。
// n <= 0 时恢复默认值 maxToolResultLines。
func (e *Executor) SetMaxResultLines(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.maxResultLines = n
}

// MaxResultLines 返回当前剪枝行数阈值（<=0 视为默认值）。
func (e *Executor) MaxResultLines() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.maxResultLines <= 0 {
		return maxToolResultLines
	}
	return e.maxResultLines
}

// SetHooks 设置钩子管理器
func (e *Executor) SetHooks(hm *HookManager) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hooks = hm
}

// AddGuard 添加执行守卫
func (e *Executor) AddGuard(g Guard) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.guards = append(e.guards, g)
}

// Execute 执行工具调用（并行调度）
func (e *Executor) Execute(ctx context.Context, calls []Call) []*Result {
	if len(calls) == 0 {
		return nil
	}

	// 分区：只读 + 写入，记录原始索引
	type indexedCall struct {
		call Call
		idx  int
	}
	var readCalls, writeCalls []indexedCall
	for i, c := range calls {
		t, ok := e.registry.Get(c.Name)
		if ok && t.IsReadOnly() {
			readCalls = append(readCalls, indexedCall{call: c, idx: i})
		} else {
			writeCalls = append(writeCalls, indexedCall{call: c, idx: i})
		}
	}

	// 结果数组，按原始顺序
	results := make([]*Result, len(calls))
	maxParallel := e.MaxParallel()

	// 并行执行只读工具
	if len(readCalls) > 0 {
		sem := make(chan struct{}, maxParallel)
		var wg sync.WaitGroup
		for _, ic := range readCalls {
			wg.Add(1)
			go func(idx int, c Call) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						results[idx] = &Result{Error: fmt.Sprintf("tool %q panicked: %v", c.Name, r)}
					}
				}()
				sem <- struct{}{}
				defer func() { <-sem }()
				results[idx] = e.executeOne(ctx, c)
			}(ic.idx, ic.call)
		}
		wg.Wait()
	}

	// 串行执行写入工具
	for _, ic := range writeCalls {
		results[ic.idx] = e.executeOne(ctx, ic.call)
	}

	return results
}

// partition 精确分区：查询 registry 获取 IsReadOnly
func (e *Executor) partition(calls []Call) (read, write []Call) {
	for _, c := range calls {
		t, ok := e.registry.Get(c.Name)
		if ok && t.IsReadOnly() {
			read = append(read, c)
		} else {
			write = append(write, c)
		}
	}
	return
}

// executeOne 执行单个工具调用
func (e *Executor) executeOne(ctx context.Context, call Call) *Result {
	// 守卫链
	e.mu.Lock()
	guards := make([]Guard, len(e.guards))
	copy(guards, e.guards)
	hooks := e.hooks
	e.mu.Unlock()

	for _, g := range guards {
		if err := g(call); err != nil {
			return &Result{Error: fmt.Sprintf("guard: %s", err.Error())}
		}
	}

	// Pre-hooks
	if hooks != nil {
		if err := hooks.RunPreHooks(ctx, call); err != nil {
			return &Result{Error: fmt.Sprintf("pre-hook: %s", err.Error())}
		}
	}

	// 查找工具
	tool, ok := e.registry.Get(call.Name)
	if !ok {
		return &Result{Error: fmt.Sprintf("unknown tool: %s", call.Name)}
	}

	// 执行
	result, err := tool.Execute(ctx, call.Args)
	if err != nil {
		return &Result{Error: err.Error()}
	}
	if result == nil {
		return &Result{Error: fmt.Sprintf("tool %q returned nil result", call.Name)}
	}

	// Post-hooks
	if hooks != nil {
		if hookErr := hooks.RunPostHooks(ctx, call, result); hookErr != nil {
			if result.Error != "" {
				result.Error = result.Error + "; post-hook: " + hookErr.Error()
			} else {
				result.Error = fmt.Sprintf("post-hook: %s", hookErr.Error())
			}
		}
	}

	// 确定性剪枝：仅对成功结果剪枝，错误结果保留完整信息用于调试。
	// 大体积工具结果（read_file 大文件、bash 长日志）在多次 Run 间内容易变，
	// 剪枝后保证相同输入产生相同输出，稳定 prefix hash，提升 cache 命中率。
	if result.Error == "" {
		result.Content = pruneResult(result.Content, e.MaxResultLines())
	}

	return result
}

// pruneResult 确定性剪枝工具结果。
// 超过 maxLines 行的内容，保留头部 headKeepLines 行与尾部 tailKeepLines 行，
// 中间用占位符替代。占位符仅含被剪枝行数与总行数（确定性），不含时间戳/随机数，
// 因此相同输入恒产生相同输出 → prefix hash 稳定。
// maxLines <= 0 时使用默认值 maxToolResultLines。
func pruneResult(content string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = maxToolResultLines
	}
	lines := strings.Split(content, "\n")
	keep := headKeepLines + tailKeepLines
	// 防御性 clamp：maxLines 过小时直接返回全文，避免头尾重叠、占位符负数导致输出膨胀。
	if len(lines) <= maxLines || maxLines < keep {
		return content
	}
	head := lines[:headKeepLines]
	tail := lines[len(lines)-tailKeepLines:]
	omitted := len(lines) - keep
	return strings.Join(head, "\n") +
		fmt.Sprintf("\n... (pruned %d lines, %d total) ...\n", omitted, len(lines)) +
		strings.Join(tail, "\n")
}
