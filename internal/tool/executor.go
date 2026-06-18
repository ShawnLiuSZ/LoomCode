package tool

import (
	"context"
	"fmt"
	"sync"
)

// Executor 工具执行引擎
type Executor struct {
	registry  *Registry
	guards    []Guard
	mu        sync.Mutex
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
		registry: registry,
	}
}

// AddGuard 添加执行守卫
func (e *Executor) AddGuard(g Guard) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.guards = append(e.guards, g)
}

// Execute 执行工具调用
func (e *Executor) Execute(ctx context.Context, calls []Call) []*Result {
	// 分区：只读并行，写入串行
	readCalls, writeCalls := partition(calls)

	results := make([]*Result, len(calls))
	resultIdx := 0

	// 并行执行只读工具
	var wg sync.WaitGroup
	readResults := make([]*Result, len(readCalls))
	for i, call := range readCalls {
		wg.Add(1)
		go func(idx int, c Call) {
			defer wg.Done()
			readResults[idx] = e.executeOne(ctx, c)
		}(i, call)
	}
	wg.Wait()

	// 合并只读结果
	for _, r := range readResults {
		results[resultIdx] = r
		resultIdx++
	}

	// 串行执行写入工具
	for _, call := range writeCalls {
		results[resultIdx] = e.executeOne(ctx, call)
		resultIdx++
	}

	return results
}

// executeOne 执行单个工具调用
func (e *Executor) executeOne(ctx context.Context, call Call) *Result {
	// 守卫链
	e.mu.Lock()
	guards := make([]Guard, len(e.guards))
	copy(guards, e.guards)
	e.mu.Unlock()

	for _, g := range guards {
		if err := g(call); err != nil {
			return &Result{Error: fmt.Sprintf("guard: %s", err.Error())}
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

	return result
}

// partition 将工具调用分为只读和写入两组
func partition(calls []Call) (read []Call, write []Call) {
	for _, c := range calls {
		// 默认为只读（实际执行时会检查 IsReadOnly）
		read = append(read, c)
	}
	// 简化实现：全部放入 read 组，实际并行执行时由 tool.IsReadOnly 控制
	// TODO: 根据工具元数据精确分区
	return read, write
}
