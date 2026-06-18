package contextpkg

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

// Partition 三层上下文分区管理器
// 不可变前缀 + 追加日志 + 易变草稿
type Partition struct {
	mu sync.RWMutex

	// 不可变前缀（会话级固定，缓存命中候选）
	prefix      string
	prefixHash  string
	prefixReady bool

	// 追加日志（单调增长，保留前序轮次前缀）
	log []LogEntry

	// 易变草稿（每轮重置，不发送给模型）
	scratch string

	// 缓存管理
	cacheTTL      time.Duration
	lastCacheHit  time.Time
	cacheHitCount int
	cacheMissCount int
}

// LogEntry 追加日志条目
type LogEntry struct {
	Role    string // "assistant" | "tool" | "tool_result"
	Content string
	ToolCall *LogToolCall
}

// LogToolCall 日志中的工具调用记录
type LogToolCall struct {
	Name string
	Args map[string]any
}

// NewPartition 创建上下文分区
func NewPartition(cacheTTL time.Duration) *Partition {
	return &Partition{
		cacheTTL: cacheTTL,
	}
}

// SetPrefix 设置不可变前缀（只能设置一次）
func (p *Partition) SetPrefix(content string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.prefixReady {
		return // 不可变
	}

	p.prefix = content
	p.prefixHash = hashContent(content)
	p.prefixReady = true
}

// Prefix 返回不可变前缀
func (p *Partition) Prefix() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.prefix
}

// PrefixHash 返回前缀哈希
func (p *Partition) PrefixHash() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.prefixHash
}

// AppendLog 追加日志条目
func (p *Partition) AppendLog(entry LogEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.log = append(p.log, entry)
}

// Log 返回完整日志（副本）
func (p *Partition) Log() []LogEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]LogEntry, len(p.log))
	copy(result, p.log)
	return result
}

// LogSize 返回日志条目数
func (p *Partition) LogSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.log)
}

// SetScratch 设置易变草稿
func (p *Partition) SetScratch(content string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scratch = content
}

// Scratch 返回易变草稿
func (p *Partition) Scratch() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.scratch
}

// ResetScratch 重置草稿区
func (p *Partition) ResetScratch() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scratch = ""
}

// BuildMessages 构建发送给模型的消息列表
// 前缀 + 日志（不含草稿）
func (p *Partition) BuildMessages() []provider.Message {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var messages []provider.Message

	// 不可变前缀作为 system 消息
	if p.prefix != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: p.prefix,
		})
	}

	// 追加日志
	for _, entry := range p.log {
		messages = append(messages, provider.Message{
			Role:    entry.Role,
			Content: entry.Content,
		})
	}

	return messages
}

// IsCacheValid 检查缓存是否仍有效
func (p *Partition) IsCacheValid() bool {
	if p.cacheTTL == 0 {
		return false
	}
	return time.Since(p.lastCacheHit) < p.cacheTTL
}

// RecordCacheHit 记录缓存命中
func (p *Partition) RecordCacheHit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheHitCount++
	p.lastCacheHit = time.Now()
}

// RecordCacheMiss 记录缓存未命中
func (p *Partition) RecordCacheMiss() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheMissCount++
}

// CacheStats 返回缓存统计
func (p *Partition) CacheStats() (hits, misses int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cacheHitCount, p.cacheMissCount
}

// CacheHitRate 返回缓存命中率
func (p *Partition) CacheHitRate() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := p.cacheHitCount + p.cacheMissCount
	if total == 0 {
		return 0
	}
	return float64(p.cacheHitCount) / float64(total)
}

// hashContent 计算内容的 SHA256 哈希
func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8]) // 取前 8 字节
}
