package agent

import (
	"sync"
	"time"
)

// CacheScheduler CacheTTL-aware 请求调度器。
// 记录上次请求时间，帮助 Agent 在 TTL 窗口内复用 prefix（命中缓存），
// 在 TTL 过期后安全地执行破坏性操作（压缩、切模型）。
//
// 设计动机：DeepSeek 等 provider 的 prefix cache 有固定 TTL（如 5 分钟），
// 在 TTL 窗口内连续请求可命中缓存、显著降低成本与延迟；一旦执行压缩或
// 切模型等破坏性操作，prefix 失效、缓存被击穿。本调度器把"距上次请求的
// 时间"作为是否仍可能命中缓存的启发式信号，供 Agent 决定何时压缩最划算。
type CacheScheduler struct {
	mu            sync.Mutex
	cacheTTL      time.Duration
	lastRequest   time.Time
	totalRequests int64
	cacheHits     int64
}

// NewCacheScheduler 创建一个 CacheTTL-aware 调度器。
// ttl <= 0 表示禁用调度：ShouldReusePrefix 永远返回 false，
// CanDoDestructiveOp 永远返回 true（无缓存可保护）。
func NewCacheScheduler(ttl time.Duration) *CacheScheduler {
	return &CacheScheduler{cacheTTL: ttl}
}

// ShouldReusePrefix 距上次请求是否在 TTL 窗口内（理论上 prefix cache 仍有效）。
// TTL <= 0（禁用）或从未发起过请求时返回 false。
func (s *CacheScheduler) ShouldReusePrefix() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheTTL <= 0 || s.lastRequest.IsZero() {
		return false
	}
	return time.Since(s.lastRequest) < s.cacheTTL
}

// MarkRequest 记录一次请求发生（在发送 LLM 请求前调用）。
func (s *CacheScheduler) MarkRequest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRequest = time.Now()
	s.totalRequests++
}

// MarkCacheHit 记录一次缓存命中（在 provider 返回 CachedInputTokens > 0 时调用）。
func (s *CacheScheduler) MarkCacheHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheHits++
}

// CanDoDestructiveOp 是否可以安全执行破坏性操作（压缩/切模型）。
// TTL 过期后，缓存反正已失效，此时做破坏性操作"损失最小"。
// TTL <= 0（禁用）或从未发起过请求时返回 true（无缓存可保护）。
func (s *CacheScheduler) CanDoDestructiveOp() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheTTL <= 0 || s.lastRequest.IsZero() {
		return true
	}
	return time.Since(s.lastRequest) >= s.cacheTTL
}

// TimeSinceLastRequest 距上次请求的时间。
// 从未发起过请求时返回 0。
func (s *CacheScheduler) TimeSinceLastRequest() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastRequest.IsZero() {
		return 0
	}
	return time.Since(s.lastRequest)
}

// Stats 返回统计：总请求数、命中数、命中率。
// 总请求数为 0 时命中率返回 0。
func (s *CacheScheduler) Stats() (totalRequests, cacheHits int64, hitRate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	totalRequests = s.totalRequests
	cacheHits = s.cacheHits
	if totalRequests > 0 {
		hitRate = float64(cacheHits) / float64(totalRequests)
	}
	return
}

// Reset 重置缓存计时（切模型/压缩后调用，因为 prefix 已变）。
// 仅清零 lastRequest 使后续 ShouldReusePrefix 返回 false，
// 统计计数保留以便跨压缩事件观察整段会话的缓存命中率。
func (s *CacheScheduler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRequest = time.Time{}
}
