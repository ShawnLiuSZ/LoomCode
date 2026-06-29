package agent

import (
	"testing"
	"time"
)

func TestCacheScheduler_ShouldReusePrefix(t *testing.T) {
	s := NewCacheScheduler(5 * time.Minute)

	// 从未发起请求：无 prefix 可复用
	if s.ShouldReusePrefix() {
		t.Fatal("expected ShouldReusePrefix=false before any request")
	}

	s.MarkRequest()
	// 0s 后仍在 TTL 窗口内
	if !s.ShouldReusePrefix() {
		t.Fatal("expected ShouldReusePrefix=true immediately after MarkRequest")
	}

	// 模拟 TTL 过期：把 lastRequest 往前推 6 分钟
	s.mu.Lock()
	s.lastRequest = time.Now().Add(-6 * time.Minute)
	s.mu.Unlock()

	if s.ShouldReusePrefix() {
		t.Fatal("expected ShouldReusePrefix=false after TTL expired")
	}
}

func TestCacheScheduler_CanDoDestructiveOp(t *testing.T) {
	s := NewCacheScheduler(5 * time.Minute)

	s.MarkRequest()
	// TTL 内：不宜做破坏性操作
	if s.CanDoDestructiveOp() {
		t.Fatal("expected CanDoDestructiveOp=false within TTL")
	}

	// 模拟 TTL 过期
	s.mu.Lock()
	s.lastRequest = time.Now().Add(-6 * time.Minute)
	s.mu.Unlock()

	if !s.CanDoDestructiveOp() {
		t.Fatal("expected CanDoDestructiveOp=true after TTL expired")
	}
}

func TestCacheScheduler_Stats(t *testing.T) {
	s := NewCacheScheduler(5 * time.Minute)

	// 记录 3 次请求，2 次命中
	s.MarkRequest()
	s.MarkCacheHit()
	s.MarkRequest()
	s.MarkCacheHit()
	s.MarkRequest()

	total, hits, rate := s.Stats()
	if total != 3 {
		t.Errorf("totalRequests = %d, want 3", total)
	}
	if hits != 2 {
		t.Errorf("cacheHits = %d, want 2", hits)
	}
	// 2/3 = 0.6666... ≈ 66.67%
	want := 2.0 / 3.0
	if rate < want-1e-9 || rate > want+1e-9 {
		t.Errorf("hitRate = %f, want %f (66.67%%)", rate, want)
	}
}

func TestCacheScheduler_Reset(t *testing.T) {
	s := NewCacheScheduler(5 * time.Minute)

	s.MarkRequest()
	s.MarkCacheHit()
	if !s.ShouldReusePrefix() {
		t.Fatal("expected ShouldReusePrefix=true before Reset")
	}

	s.Reset()
	// Reset 后 lastRequest 清零，ShouldReusePrefix=false
	if s.ShouldReusePrefix() {
		t.Fatal("expected ShouldReusePrefix=false after Reset")
	}
}
