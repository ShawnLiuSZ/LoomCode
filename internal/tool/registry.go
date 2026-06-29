package tool

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 工具注册中心
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建工具注册中心
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register 注册工具
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[t.Name()]; exists {
		return fmt.Errorf("tool %q already registered", t.Name())
	}
	r.tools[t.Name()] = t
	return nil
}

// Get 获取工具
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List 列出所有工具
// 按工具名升序返回，保证输出顺序稳定（Go map 遍历顺序随机，
// 排序后可让 buildToolDefs 产出的 tools 数组保持一致，从而命中 LLM prefix cache）。
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.tools))
	for name := range r.tools {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	tools := make([]Tool, 0, len(keys))
	for _, name := range keys {
		tools = append(tools, r.tools[name])
	}
	return tools
}

// RegisterDefaults 注册所有内置基础工具
func (r *Registry) RegisterDefaults() {
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&EditFileTool{})
	r.Register(&BashTool{})
	r.Register(&GrepTool{})
	r.Register(&GlobTool{})
	r.Register(&GitStatusTool{})
	r.Register(&GitDiffTool{})
	r.Register(&GitLogTool{})
	r.Register(&GitCommitTool{})
	// recall_memory 默认注册为占位工具（返回 "No memory configured."）；
	// 真正的记忆来源由 agent.SetMemory 通过 SetMemoryProvider 注入。
	r.Register(&RecallMemoryTool{})
}
