package contextpkg

import (
	"sync"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

// IncrementalContext 增量上下文管理器
// 只在消息变化时重新构建，避免全量序列化
type IncrementalContext struct {
	mu       sync.RWMutex
	messages []provider.Message
	dirty    bool
	cached   string
	version  int
}

// NewIncrementalContext 创建增量上下文
func NewIncrementalContext() *IncrementalContext {
	return &IncrementalContext{}
}

// Append 追加消息（标记为脏）
func (c *IncrementalContext) Append(msg provider.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
	c.dirty = true
	c.version++
}

// Messages 返回消息列表
func (c *IncrementalContext) Messages() []provider.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]provider.Message, len(c.messages))
	copy(result, c.messages)
	return result
}

// Len 返回消息数量
func (c *IncrementalContext) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages)
}

// IsDirty 检查是否需要重建
func (c *IncrementalContext) IsDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty
}

// MarkClean 标记为干净
func (c *IncrementalContext) MarkClean() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = false
}

// Version 返回当前版本
func (c *IncrementalContext) Version() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.version
}

// Last 返回最后 n 条消息
func (c *IncrementalContext) Last(n int) []provider.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if n >= len(c.messages) {
		result := make([]provider.Message, len(c.messages))
		copy(result, c.messages)
		return result
	}
	start := len(c.messages) - n
	result := make([]provider.Message, n)
	copy(result, c.messages[start:])
	return result
}
