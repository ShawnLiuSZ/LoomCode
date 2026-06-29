package tool

import "context"

// RecallMemoryTool 让模型按需检索长期记忆（项目知识、用户偏好），
// 替代此前把记忆正文直接拼进系统提示的做法——把记忆移出 prefix，
// 既缩短系统提示又稳定 prefix 以提升 cache 命中率。
//
// memoryFn 由上层（agent.SetMemory）注入；未注入时返回占位文本，
// 保证工具始终可调用、不因配置缺失而报错。
type RecallMemoryTool struct {
	memoryFn func() string
}

// SetMemoryProvider 注入记忆检索函数。传入的 fn 应返回当前记忆正文
//（可为空字符串）；传 nil 表示清除记忆来源。
func (t *RecallMemoryTool) SetMemoryProvider(fn func() string) {
	t.memoryFn = fn
}

func (t *RecallMemoryTool) Name() string     { return "recall_memory" }
func (t *RecallMemoryTool) IsReadOnly() bool { return true }

func (t *RecallMemoryTool) Description() string {
	return "Retrieve project knowledge and long-term memory. Call this when you need context about the project or prior decisions."
}

// Schema 返回空 schema：当前实现整体返回记忆正文，无需参数。
func (t *RecallMemoryTool) Schema() Schema {
	return Schema{Type: "object", Properties: map[string]Property{}}
}

func (t *RecallMemoryTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	if t.memoryFn == nil {
		return &Result{Content: "No memory configured."}, nil
	}
	if content := t.memoryFn(); content != "" {
		return &Result{Content: content}, nil
	}
	return &Result{Content: "No memory available."}, nil
}
