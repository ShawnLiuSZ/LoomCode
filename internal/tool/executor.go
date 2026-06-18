package tool

import (
	"context"
	"fmt"
	"sync"
)

// Executor 工具执行引擎
type Executor struct {
	registry      *Registry
	guards        []Guard
	maxParallel   int
	mu            sync.Mutex
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
		registry:    registry,
		maxParallel: 3,
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

	// 分区：只读 + 写入
	readCalls, writeCalls := e.partition(calls)

	maxParallel := e.MaxParallel()

	// 并行执行只读工具（受 maxParallel 限制）
	readResults := e.executeParallel(ctx, readCalls, maxParallel)

	// 串行执行写入工具
	writeResults := e.executeSerial(ctx, writeCalls)

	// 按原始顺序合并结果
	return e.mergeResults(calls, readResults, writeResults)
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

// executeParallel 并行执行（带并发限制）
func (e *Executor) executeParallel(ctx context.Context, calls []Call, maxParallel int) map[string]*Result {
	results := make(map[string]*Result)
	var mu sync.Mutex

	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup

	for _, call := range calls {
		wg.Add(1)
		go func(c Call) {
			defer wg.Done()
			sem <- struct{}{}        // 获取令牌
			defer func() { <-sem }() // 释放令牌

			result := e.executeOne(ctx, c)
			mu.Lock()
			results[c.Name] = result
			mu.Unlock()
		}(call)
	}

	wg.Wait()
	return results
}

// executeSerial 串行执行
func (e *Executor) executeSerial(ctx context.Context, calls []Call) []*Result {
	results := make([]*Result, len(calls))
	for i, call := range calls {
		results[i] = e.executeOne(ctx, call)
	}
	return results
}

// mergeResults 按原始顺序合并结果
func (e *Executor) mergeResults(original []Call, readResults map[string]*Result, writeResults []*Result) []*Result {
	results := make([]*Result, len(original))
	writeIdx := 0

	for i, call := range original {
		if r, ok := readResults[call.Name]; ok {
			results[i] = r
		} else {
			results[i] = writeResults[writeIdx]
			writeIdx++
		}
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
