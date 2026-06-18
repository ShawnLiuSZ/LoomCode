package provider

import "context"

// Provider 模型提供者实例接口
type Provider interface {
	// Chat 非流式对话
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Stream 流式对话
	Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// 元信息
	Name() string
	Models() []ModelInfo
	Capabilities() Capabilities

	// Cost 计算成本
	Cost(modelID string, usage Usage) Cost
}

// Adapter 适配器工厂接口
type Adapter interface {
	// Kind 返回适配器类型标识（对应配置中的 kind 字段）
	Kind() string

	// Create 根据配置创建 Provider 实例
	Create(cfg Config) (Provider, error)

	// ValidateConfig 验证配置是否合法
	ValidateConfig(cfg Config) error
}
