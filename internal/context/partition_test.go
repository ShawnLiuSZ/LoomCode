package contextpkg

import (
	"testing"
	"time"
)

func TestPartition_SetPrefix(t *testing.T) {
	p := NewPartition(5 * time.Minute)

	p.SetPrefix("system prompt v1")
	if p.Prefix() != "system prompt v1" {
		t.Errorf("Prefix() = %q", p.Prefix())
	}

	// 前缀不可变
	p.SetPrefix("system prompt v2")
	if p.Prefix() != "system prompt v1" {
		t.Errorf("Prefix should be immutable, got %q", p.Prefix())
	}
}

func TestPartition_PrefixHash(t *testing.T) {
	p := NewPartition(5 * time.Minute)
	p.SetPrefix("hello")

	hash := p.PrefixHash()
	if hash == "" {
		t.Error("PrefixHash() should not be empty")
	}

	// 相同内容产生相同哈希
	p2 := NewPartition(5 * time.Minute)
	p2.SetPrefix("hello")
	if p2.PrefixHash() != hash {
		t.Error("same content should produce same hash")
	}

	// 不同内容产生不同哈希
	p3 := NewPartition(5 * time.Minute)
	p3.SetPrefix("world")
	if p3.PrefixHash() == hash {
		t.Error("different content should produce different hash")
	}
}

func TestPartition_Log(t *testing.T) {
	p := NewPartition(5 * time.Minute)

	p.AppendLog(LogEntry{Role: "assistant", Content: "Hello"})
	p.AppendLog(LogEntry{Role: "tool", Content: "result"})

	if p.LogSize() != 2 {
		t.Errorf("LogSize() = %d, want 2", p.LogSize())
	}

	log := p.Log()
	if log[0].Content != "Hello" {
		t.Errorf("log[0] = %q", log[0].Content)
	}
	if log[1].Content != "result" {
		t.Errorf("log[1] = %q", log[1].Content)
	}

	// 返回的是副本，修改不影响原数据
	log[0].Content = "modified"
	if p.Log()[0].Content != "Hello" {
		t.Error("Log() should return a copy")
	}
}

func TestPartition_Scratch(t *testing.T) {
	p := NewPartition(5 * time.Minute)

	p.SetScratch("draft content")
	if p.Scratch() != "draft content" {
		t.Errorf("Scratch() = %q", p.Scratch())
	}

	p.ResetScratch()
	if p.Scratch() != "" {
		t.Errorf("Scratch() should be empty after reset")
	}
}

func TestPartition_BuildMessages(t *testing.T) {
	p := NewPartition(5 * time.Minute)
	p.SetPrefix("You are a helpful assistant")
	p.AppendLog(LogEntry{Role: "user", Content: "hello"})
	p.AppendLog(LogEntry{Role: "assistant", Content: "hi there"})

	msgs := p.BuildMessages()
	if len(msgs) != 3 {
		t.Fatalf("BuildMessages() count = %d, want 3", len(msgs))
	}

	if msgs[0].Role != "system" || msgs[0].Content != "You are a helpful assistant" {
		t.Errorf("msgs[0] = %+v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "hello" {
		t.Errorf("msgs[1] = %+v", msgs[1])
	}

	// 草稿不在消息中
	p.SetScratch("secret draft")
	msgs2 := p.BuildMessages()
	if len(msgs2) != 3 {
		t.Errorf("scratch should not appear in messages")
	}
}

func TestPartition_CacheStats(t *testing.T) {
	p := NewPartition(1 * time.Second)

	p.RecordCacheHit()
	p.RecordCacheHit()
	p.RecordCacheMiss()

	hits, misses := p.CacheStats()
	if hits != 2 {
		t.Errorf("hits = %d, want 2", hits)
	}
	if misses != 1 {
		t.Errorf("misses = %d, want 1", misses)
	}

	rate := p.CacheHitRate()
	expected := 2.0 / 3.0
	if rate != expected {
		t.Errorf("CacheHitRate() = %f, want %f", rate, expected)
	}
}

func TestPartition_CacheTTL(t *testing.T) {
	p := NewPartition(50 * time.Millisecond)
	p.RecordCacheHit()

	if !p.IsCacheValid() {
		t.Error("cache should be valid immediately after hit")
	}

	time.Sleep(100 * time.Millisecond)

	if p.IsCacheValid() {
		t.Error("cache should be invalid after TTL")
	}
}

func TestPartition_NoCacheTTL(t *testing.T) {
	p := NewPartition(0) // 不启用缓存
	p.RecordCacheHit()

	if p.IsCacheValid() {
		t.Error("cache should not be valid when TTL is 0")
	}
}
