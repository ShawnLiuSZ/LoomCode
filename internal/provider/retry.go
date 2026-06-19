package provider

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"
)

// RetryConfig HTTP 重试配置
type RetryConfig struct {
	MaxRetries  int           // 最大重试次数（默认 3）
	BaseDelay   time.Duration // 基础延迟（默认 1s）
	MaxDelay    time.Duration // 最大延迟（默认 30s）
	RetryOn     []int         // 需要重试的 HTTP 状态码
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		RetryOn:    []int{429, 500, 502, 503, 504},
	}
}

// RetryableHTTPClient 支持重试的 HTTP 客户端
type RetryableHTTPClient struct {
	client *http.Client
	config RetryConfig
}

// NewRetryableClient 创建重试客户端（带连接池）
func NewRetryableClient(timeout time.Duration) *RetryableHTTPClient {
	return &RetryableHTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: defaultTransport,
		},
		config: DefaultRetryConfig(),
	}
}

// defaultTransport 共享的连接池 Transport
var defaultTransport = &http.Transport{
	MaxIdleConns:        20,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
	DisableKeepAlives:   false,
}

// NewRetryableClientWithConfig 创建带配置的重试客户端
func NewRetryableClientWithConfig(timeout time.Duration, cfg RetryConfig) *RetryableHTTPClient {
	return &RetryableHTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: defaultTransport,
		},
		config: cfg,
	}
}

// Do 执行请求（带重试）
func (c *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.backoff(attempt)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
		}

		// 每次重试需要重新读取 body（如果有）
		if attempt > 0 && req.Body != nil {
			// 注意：这里假设 body 是可重复读取的（bytes.Reader）
			if seeker, ok := req.Body.(interface{ Seek(int64, int) (int64, error) }); ok {
				seeker.Seek(0, 0)
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// 检查是否需要重试的状态码
		if c.shouldRetry(resp.StatusCode) {
			resp.Body.Close()
			lastErr = fmt.Errorf("retryable status %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.config.MaxRetries, lastErr)
}

// backoff 指数退避延迟计算
func (c *RetryableHTTPClient) backoff(attempt int) time.Duration {
	delay := float64(c.config.BaseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(c.config.MaxDelay) {
		delay = float64(c.config.MaxDelay)
	}
	return time.Duration(delay)
}

// shouldRetry 判断状态码是否需要重试
func (c *RetryableHTTPClient) shouldRetry(statusCode int) bool {
	for _, code := range c.config.RetryOn {
		if statusCode == code {
			return true
		}
	}
	return false
}

// DoWithRetry 便捷函数：使用默认配置执行带重试的请求
func DoWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	client := NewRetryableClient(120 * time.Second)
	return client.Do(req.WithContext(ctx))
}
