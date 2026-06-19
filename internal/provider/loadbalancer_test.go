package provider

import (
	"context"
	"testing"
	"time"
)

// mockProvider 模拟 Provider
type mockProvider struct {
	name    string
	latency time.Duration
}

func (m *mockProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	time.Sleep(m.latency)
	return &ChatResponse{Content: "response from " + m.name}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Type: EventText, Content: "stream from " + m.name}
	close(ch)
	return ch, nil
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Models() []ModelInfo {
	return []ModelInfo{{ID: "model-" + m.name, Name: "Model " + m.name}}
}

func (m *mockProvider) Capabilities() Capabilities {
	return Capabilities{}
}

func (m *mockProvider) Cost(modelID string, usage Usage) Cost {
	return Cost{TotalCost: 0.01, Currency: "USD"}
}

func TestLoadBalancer(t *testing.T) {
	t.Run("NewLoadBalancer", func(t *testing.T) {
		providers := []Provider{
			&mockProvider{name: "p1"},
			&mockProvider{name: "p2"},
		}
		lb := NewLoadBalancer(providers, RoundRobin)
		if lb == nil {
			t.Fatal("expected non-nil load balancer")
		}
		if len(lb.GetProviders()) != 2 {
			t.Errorf("expected 2 providers, got %d", len(lb.GetProviders()))
		}
	})

	t.Run("RoundRobin", func(t *testing.T) {
		providers := []Provider{
			&mockProvider{name: "p1"},
			&mockProvider{name: "p2"},
			&mockProvider{name: "p3"},
		}
		lb := NewLoadBalancer(providers, RoundRobin)

		// 验证轮询
		seen := make(map[string]int)
		for i := 0; i < 6; i++ {
			p := lb.Select()
			seen[p.Name()]++
		}

		// 每个 Provider 应该被选中 2 次
		for name, count := range seen {
			if count != 2 {
				t.Errorf("expected %s to be selected 2 times, got %d", name, count)
			}
		}
	})

	t.Run("WeightedRandom", func(t *testing.T) {
		providers := []Provider{
			&mockProvider{name: "p1"},
			&mockProvider{name: "p2"},
		}
		lb := NewLoadBalancer(providers, WeightedRandom)
		lb.SetWeight("p1", 3)
		lb.SetWeight("p2", 1)

		// 多次选择，验证加权
		seen := make(map[string]int)
		for i := 0; i < 1000; i++ {
			p := lb.Select()
			seen[p.Name()]++
		}

		// p1 应该被选中更多次
		if seen["p1"] <= seen["p2"] {
			t.Errorf("expected p1 to be selected more than p2")
		}
	})

	t.Run("LeastLatency", func(t *testing.T) {
		providers := []Provider{
			&mockProvider{name: "fast", latency: 10 * time.Millisecond},
			&mockProvider{name: "slow", latency: 100 * time.Millisecond},
		}
		lb := NewLoadBalancer(providers, LeastLatency)

		// 记录一些请求
		lb.RecordRequest("fast", 10*time.Millisecond, false)
		lb.RecordRequest("slow", 100*time.Millisecond, false)

		// 应该选择延迟更低的
		p := lb.Select()
		if p.Name() != "fast" {
			t.Errorf("expected fast provider, got %s", p.Name())
		}
	})

	t.Run("Select", func(t *testing.T) {
		providers := []Provider{
			&mockProvider{name: "p1"},
		}
		lb := NewLoadBalancer(providers, RoundRobin)

		p := lb.Select()
		if p == nil {
			t.Fatal("expected non-nil provider")
		}
		if p.Name() != "p1" {
			t.Errorf("expected p1, got %s", p.Name())
		}
	})

	t.Run("Select_Empty", func(t *testing.T) {
		lb := NewLoadBalancer(nil, RoundRobin)
		p := lb.Select()
		if p != nil {
			t.Error("expected nil provider")
		}
	})
}

func TestLoadBalancerAddRemove(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "p1"},
	}
	lb := NewLoadBalancer(providers, RoundRobin)

	// 添加 Provider
	lb.AddProvider(&mockProvider{name: "p2"})
	if len(lb.GetProviders()) != 2 {
		t.Errorf("expected 2 providers, got %d", len(lb.GetProviders()))
	}

	// 移除 Provider
	lb.RemoveProvider("p1")
	if len(lb.GetProviders()) != 1 {
		t.Errorf("expected 1 provider, got %d", len(lb.GetProviders()))
	}
}

func TestProviderMetrics(t *testing.T) {
	metrics := &ProviderMetrics{}

	// 记录请求
	metrics.RecordRequest(100*time.Millisecond, false)
	metrics.RecordRequest(200*time.Millisecond, true)

	if metrics.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", metrics.TotalRequests)
	}
	if metrics.FailedRequests != 1 {
		t.Errorf("expected 1 failed request, got %d", metrics.FailedRequests)
	}

	// 检查成功率
	successRate := metrics.GetSuccessRate()
	if successRate != 0.5 {
		t.Errorf("expected 0.5 success rate, got %f", successRate)
	}
}

func TestStrategy(t *testing.T) {
	tests := []struct {
		strategy Strategy
		expected string
	}{
		{RoundRobin, "round_robin"},
		{WeightedRandom, "weighted_random"},
		{LeastLatency, "least_latency"},
		{CostOptimized, "cost_optimized"},
	}

	for _, tt := range tests {
		if got := tt.strategy.String(); got != tt.expected {
			t.Errorf("Strategy.String() = %q, want %q", got, tt.expected)
		}
	}
}

func TestParseStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected Strategy
	}{
		{"round_robin", RoundRobin},
		{"weighted_random", WeightedRandom},
		{"least_latency", LeastLatency},
		{"cost_optimized", CostOptimized},
		{"unknown", RoundRobin},
	}

	for _, tt := range tests {
		if got := ParseStrategy(tt.input); got != tt.expected {
			t.Errorf("ParseStrategy(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestLoadBalancerChat(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "p1"},
	}
	lb := NewLoadBalancer(providers, RoundRobin)

	resp, err := lb.Chat(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "response from p1" {
		t.Errorf("expected response from p1, got %s", resp.Content)
	}
}

func TestLoadBalancerStream(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "p1"},
	}
	lb := NewLoadBalancer(providers, RoundRobin)

	ch, err := lb.Stream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := <-ch
	if event.Content != "stream from p1" {
		t.Errorf("expected stream from p1, got %s", event.Content)
	}
}

func TestLoadBalancerConcurrent(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "p1"},
		&mockProvider{name: "p2"},
	}
	lb := NewLoadBalancer(providers, RoundRobin)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			lb.Select()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
