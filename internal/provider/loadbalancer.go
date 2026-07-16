package provider

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Strategy 负载均衡策略
type Strategy int

const (
	// RoundRobin 轮询策略
	RoundRobin Strategy = iota
	// WeightedRandom 加权随机
	WeightedRandom
	// LeastLatency 最低延迟
	LeastLatency
	// CostOptimized 成本优化
	CostOptimized
)

// String 返回策略字符串
func (s Strategy) String() string {
	switch s {
	case RoundRobin:
		return "round_robin"
	case WeightedRandom:
		return "weighted_random"
	case LeastLatency:
		return "least_latency"
	case CostOptimized:
		return "cost_optimized"
	default:
		return "round_robin"
	}
}

// ParseStrategy 解析策略
func ParseStrategy(s string) Strategy {
	switch s {
	case "round_robin":
		return RoundRobin
	case "weighted_random":
		return WeightedRandom
	case "least_latency":
		return LeastLatency
	case "cost_optimized":
		return CostOptimized
	default:
		return RoundRobin
	}
}

// ProviderMetrics Provider 指标
type ProviderMetrics struct {
	TotalRequests   int64
	FailedRequests  int64
	TotalLatency    int64 // 毫秒
	AvgLatency      float64
	LastRequestTime time.Time
	mu              sync.RWMutex
}

// RecordRequest 记录请求
func (m *ProviderMetrics) RecordRequest(latency time.Duration, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++
	if failed {
		m.FailedRequests++
	}
	m.TotalLatency += latency.Milliseconds()
	if m.TotalRequests > 0 {
		m.AvgLatency = float64(m.TotalLatency) / float64(m.TotalRequests)
	}
	m.LastRequestTime = time.Now()
}

// GetAvgLatency 获取平均延迟
func (m *ProviderMetrics) GetAvgLatency() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.AvgLatency
}

// GetSuccessRate 获取成功率
func (m *ProviderMetrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.TotalRequests == 0 {
		return 1.0
	}
	return float64(m.TotalRequests-m.FailedRequests) / float64(m.TotalRequests)
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	providers []Provider
	strategy  Strategy
	metrics   map[string]*ProviderMetrics
	weights   map[string]int
	counter   atomic.Int64
	mu        sync.RWMutex
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(providers []Provider, strategy Strategy) *LoadBalancer {
	lb := &LoadBalancer{
		providers: providers,
		strategy:  strategy,
		metrics:   make(map[string]*ProviderMetrics),
		weights:   make(map[string]int),
	}

	// 初始化指标和权重
	for _, p := range providers {
		lb.metrics[p.Name()] = &ProviderMetrics{}
		lb.weights[p.Name()] = 1
	}

	return lb
}

// SetWeight 设置 Provider 权重（最小为 0）
func (lb *LoadBalancer) SetWeight(name string, weight int) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if weight < 0 {
		weight = 0
	}
	lb.weights[name] = weight
}

// Select 选择一个 Provider
func (lb *LoadBalancer) Select() Provider {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if len(lb.providers) == 0 {
		return nil
	}

	switch lb.strategy {
	case RoundRobin:
		return lb.roundRobin()
	case WeightedRandom:
		return lb.weightedRandom()
	case LeastLatency:
		return lb.leastLatency()
	case CostOptimized:
		return lb.costOptimized()
	default:
		return lb.roundRobin()
	}
}

// roundRobin 轮询策略
func (lb *LoadBalancer) roundRobin() Provider {
	idx := lb.counter.Add(1) - 1
	return lb.providers[idx%int64(len(lb.providers))]
}

// weightedRandom 加权随机策略
func (lb *LoadBalancer) weightedRandom() Provider {
	totalWeight := 0
	for _, p := range lb.providers {
		w := lb.weights[p.Name()]
		if w > 0 {
			totalWeight += w
		}
	}

	if totalWeight <= 0 {
		return lb.providers[0]
	}

	r := rand.Intn(totalWeight)
	for _, p := range lb.providers {
		w := lb.weights[p.Name()]
		if w <= 0 {
			continue
		}
		r -= w
		if r < 0 {
			return p
		}
	}

	return lb.providers[0]
}

// leastLatency 最低延迟策略
func (lb *LoadBalancer) leastLatency() Provider {
	var best Provider
	var bestLatency float64 = -1

	for _, p := range lb.providers {
		metrics := lb.metrics[p.Name()]
		latency := metrics.GetAvgLatency()

		if bestLatency < 0 || latency < bestLatency {
			best = p
			bestLatency = latency
		}
	}

	return best
}

func (lb *LoadBalancer) costOptimized() Provider {
	var eligible []Provider
	for _, p := range lb.providers {
		metrics := lb.metrics[p.Name()]
		if metrics.GetSuccessRate() >= 0.5 {
			eligible = append(eligible, p)
		}
	}

	if len(eligible) == 0 {
		return lb.providers[0]
	}
	if len(eligible) == 1 {
		return eligible[0]
	}

	type providerScore struct {
		provider Provider
		score    float64
	}

	scores := make([]providerScore, 0, len(eligible))

	var minLatency, maxLatency, minCost, maxCost float64
	latencies := make([]float64, len(eligible))
	costs := make([]float64, len(eligible))

	for i, p := range eligible {
		metrics := lb.metrics[p.Name()]
		lat := metrics.GetAvgLatency()
		latencies[i] = lat

		models := p.Models()
		costPerToken := 0.0
		if len(models) > 0 {
			costPerToken = models[0].Cost.Input + models[0].Cost.Output
		}
		costs[i] = costPerToken

		if i == 0 {
			minLatency = lat
			maxLatency = lat
			minCost = costPerToken
			maxCost = costPerToken
		} else {
			if lat < minLatency {
				minLatency = lat
			}
			if lat > maxLatency {
				maxLatency = lat
			}
			if costPerToken < minCost {
				minCost = costPerToken
			}
			if costPerToken > maxCost {
				maxCost = costPerToken
			}
		}
	}

	for i, p := range eligible {
		metrics := lb.metrics[p.Name()]
		successRate := metrics.GetSuccessRate()

		var normLatency, normCost float64
		if maxLatency > minLatency {
			normLatency = (latencies[i] - minLatency) / (maxLatency - minLatency)
		}
		if maxCost > minCost {
			normCost = (costs[i] - minCost) / (maxCost - minCost)
		}

		score := (1-successRate)*0.3 + normLatency*0.3 + normCost*0.4
		scores = append(scores, providerScore{provider: p, score: score})
	}

	best := scores[0]
	for _, s := range scores[1:] {
		if s.score < best.score {
			best = s
		}
	}

	return best.provider
}

// RecordRequest 记录请求指标
func (lb *LoadBalancer) RecordRequest(providerName string, latency time.Duration, failed bool) {
	lb.mu.RLock()
	metrics, ok := lb.metrics[providerName]
	lb.mu.RUnlock()

	if ok {
		metrics.RecordRequest(latency, failed)
	}
}

// GetMetrics 获取 Provider 指标
func (lb *LoadBalancer) GetMetrics(providerName string) *ProviderMetrics {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.metrics[providerName]
}

// GetProviders 获取所有 Provider
func (lb *LoadBalancer) GetProviders() []Provider {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.providers
}

// AddProvider 添加 Provider
func (lb *LoadBalancer) AddProvider(p Provider) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.providers = append(lb.providers, p)
	lb.metrics[p.Name()] = &ProviderMetrics{}
	lb.weights[p.Name()] = 1
}

// RemoveProvider 移除 Provider
func (lb *LoadBalancer) RemoveProvider(name string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for i, p := range lb.providers {
		if p.Name() == name {
			lb.providers = append(lb.providers[:i], lb.providers[i+1:]...)
			delete(lb.metrics, name)
			delete(lb.weights, name)
			break
		}
	}
}

// Chat 实现 Provider 接口的 Chat 方法
func (lb *LoadBalancer) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	p := lb.Select()
	if p == nil {
		return nil, fmt.Errorf("no available provider")
	}

	start := time.Now()
	resp, err := p.Chat(ctx, req)
	latency := time.Since(start)

	lb.RecordRequest(p.Name(), latency, err != nil)

	return resp, err
}

// Stream 实现 Provider 接口的 Stream 方法
func (lb *LoadBalancer) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	p := lb.Select()
	if p == nil {
		return nil, fmt.Errorf("no available provider")
	}

	return p.Stream(ctx, req)
}

// Name 实现 Provider 接口的 Name 方法
func (lb *LoadBalancer) Name() string {
	return "load_balancer"
}

// Models 实现 Provider 接口的 Models 方法
func (lb *LoadBalancer) Models() []ModelInfo {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	if len(lb.providers) == 0 {
		return nil
	}
	return lb.providers[0].Models()
}

// Capabilities 实现 Provider 接口的 Capabilities 方法
func (lb *LoadBalancer) Capabilities() Capabilities {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	if len(lb.providers) == 0 {
		return Capabilities{}
	}
	return lb.providers[0].Capabilities()
}

// Cost 实现 Provider 接口的 Cost 方法
func (lb *LoadBalancer) Cost(modelID string, usage Usage) Cost {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	if len(lb.providers) == 0 {
		return Cost{}
	}
	return lb.providers[0].Cost(modelID, usage)
}
