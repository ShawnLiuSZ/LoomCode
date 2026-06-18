package provider

import (
	"fmt"
	"sync"
)

// Registry Provider 适配器注册中心
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

// NewRegistry 创建注册中心
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]Adapter),
	}
}

// Register 注册适配器
func (r *Registry) Register(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[a.Kind()] = a
}

// Create 根据配置创建 Provider 实例
func (r *Registry) Create(kind string, cfg Config) (Provider, error) {
	r.mu.RLock()
	adapter, ok := r.adapters[kind]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider kind: %q", kind)
	}

	return adapter.Create(cfg)
}

// ValidateConfig 验证配置
func (r *Registry) ValidateConfig(kind string, cfg Config) error {
	r.mu.RLock()
	adapter, ok := r.adapters[kind]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown provider kind: %q", kind)
	}

	return adapter.ValidateConfig(cfg)
}
