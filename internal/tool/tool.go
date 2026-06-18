package tool

import "context"

// Tool 工具接口
type Tool interface {
	Name() string
	Description() string
	Schema() Schema
	IsReadOnly() bool
	Execute(ctx context.Context, args map[string]any) (*Result, error)
}

// Schema 工具 Schema（OpenAI function 格式）
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property Schema 属性
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Result 工具执行结果
type Result struct {
	Content string
	Error   string
}

// OK 判断执行是否成功
func (r *Result) OK() bool { return r.Error == "" }

// EnvProvider 环境变量提供者接口
// 由上层（CLI）注入，工具执行子进程时使用
type EnvProvider interface {
	// EnvForSubprocess 返回子进程应使用的环境变量
	EnvForSubprocess() []string
}

// envProvider 全局环境变量提供者
var globalEnvProvider EnvProvider

// SetEnvProvider 设置全局环境变量提供者
func SetEnvProvider(p EnvProvider) {
	globalEnvProvider = p
}

// EnvForSubprocess 获取子进程环境变量
func EnvForSubprocess() []string {
	if globalEnvProvider != nil {
		return globalEnvProvider.EnvForSubprocess()
	}
	return nil
}
