package provider

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("BaseDelay = %v", cfg.BaseDelay)
	}
}

func TestBackoff(t *testing.T) {
	c := NewRetryableClient(10 * time.Second)

	tests := []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{1, 1 * time.Second, 1 * time.Second},
		{2, 2 * time.Second, 2 * time.Second},
		{3, 4 * time.Second, 4 * time.Second},
		{10, 30 * time.Second, 30 * time.Second},
	}

	for _, tt := range tests {
		delay := c.backoff(tt.attempt)
		if delay < tt.min || delay > tt.max {
			t.Errorf("backoff(%d) = %v, want between %v and %v", tt.attempt, delay, tt.min, tt.max)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	c := NewRetryableClient(10 * time.Second)

	if !c.shouldRetry(429) {
		t.Error("429 should be retryable")
	}
	if !c.shouldRetry(502) {
		t.Error("502 should be retryable")
	}
	if c.shouldRetry(200) {
		t.Error("200 should not be retryable")
	}
	if c.shouldRetry(400) {
		t.Error("400 should not be retryable")
	}
}

func TestRetryableHTTPClient_Do(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryOn:    []int{503},
	}

	c := NewRetryableClientWithConfig(5*time.Second, cfg)

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestRetryableHTTPClient_MaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryOn:    []int{500},
	}

	c := NewRetryableClientWithConfig(5*time.Second, cfg)

	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err := c.Do(req)
	if err == nil {
		t.Error("expected error after max retries")
	}
}

func TestRetryableHTTPClient_BodyReset(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryOn:    []int{503},
	}

	c := NewRetryableClientWithConfig(5*time.Second, cfg)

	body := []byte(`{"test":true}`)
	req, _ := http.NewRequest("POST", server.URL, bytes.NewReader(body))
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	tests := []struct {
		header string
		want   time.Duration
	}{
		{"0", 0},
		{"1", 1 * time.Second},
		{"30", 30 * time.Second},
		{"120", 2 * time.Minute},
		{"99999", 5 * time.Minute}, // 超过上限截断
		{"-1", 0},                   // 负值
		{"", 0},                     // 空
		{"abc", 0},                  // 非数字
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("header=%q", tt.header), func(t *testing.T) {
			got := parseRetryAfter(tt.header)
			if got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	// 未来时间：应返回正值
	future := time.Now().Add(10 * time.Second).UTC().Format(time.RFC1123)
	got := parseRetryAfter(future)
	if got < 5*time.Second || got > 15*time.Second {
		t.Errorf("parseRetryAfter(future date) = %v, want ~10s", got)
	}

	// 过去时间：应返回 0
	past := time.Now().Add(-10 * time.Second).UTC().Format(time.RFC1123)
	got = parseRetryAfter(past)
	if got != 0 {
		t.Errorf("parseRetryAfter(past date) = %v, want 0", got)
	}

	// 未来超过 5 分钟：应截断
	farFuture := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC1123)
	got = parseRetryAfter(farFuture)
	if got != 5*time.Minute {
		t.Errorf("parseRetryAfter(far future date) = %v, want 5m", got)
	}
}

func TestAddJitter(t *testing.T) {
	c := NewRetryableClient(10 * time.Second)
	base := 100 * time.Millisecond

	for i := 0; i < 100; i++ {
		delay := c.addJitter(base)
		// ±25% 双边抖动：[75ms, 125ms]
		if delay < base*3/4 || delay > base*5/4 {
			t.Errorf("addJitter(%v) = %v, want between %v and %v", base, delay, base*3/4, base*5/4)
		}
	}

	// 零值不应 panic
	if c.addJitter(0) != 0 {
		t.Error("addJitter(0) should return 0")
	}
}

func TestAddJitter_VariesDelay(t *testing.T) {
	c := NewRetryableClient(10 * time.Second)
	base := 100 * time.Millisecond

	results := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		results[c.addJitter(base)] = true
	}

	if len(results) <= 1 {
		t.Errorf("addJitter returned same value %d times, expected variation", len(results))
	}
}

func TestRetryDelay_WithRetryAfter(t *testing.T) {
	c := NewRetryableClient(10 * time.Second)

	// 没有 Retry-After 时使用指数退避 + jitter
	delay := c.retryDelay(1, nil)
	if delay < 0 {
		t.Errorf("retryDelay(1, nil) = %v, want >= 0", delay)
	}

	// 有 Retry-After 时使用指定值 + ±25% jitter（即 3.75s ~ 6.25s）
	ra := 5 * time.Second
	delay = c.retryDelay(1, &ra)
	lower := ra * 3 / 4
	upper := ra * 5 / 4
	if delay < lower || delay > upper {
		t.Errorf("retryDelay(1, 5s) = %v, want between %v and %v", delay, lower, upper)
	}
}

func TestRetryableHTTPClient_RetryAfter429(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryOn:    []int{429},
	}

	c := NewRetryableClientWithConfig(5*time.Second, cfg)

	req, _ := http.NewRequest("GET", server.URL, nil)
	start := time.Now()
	resp, err := c.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}

	// Retry-After=1 应导致至少 750ms 延迟（1s ± 25%）
	if elapsed < 750*time.Millisecond {
		t.Errorf("elapsed %v too short, expected >= 750ms due to Retry-After=1", elapsed)
	}
}

func TestRetryableHTTPClient_RetryAfter_Expires(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryOn:    []int{429},
	}

	c := NewRetryableClientWithConfig(5*time.Second, cfg)

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	// 应重试 2 次（每次 Retry-After=1s）
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}
