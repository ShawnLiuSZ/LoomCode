package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ShawnLiuSZ/Helix/internal/consts"
)

// RetryConfig HTTP 重试配置
type RetryConfig struct {
	MaxRetries int           // 最大重试次数（默认 3）
	BaseDelay  time.Duration // 基础延迟（默认 1s）
	MaxDelay   time.Duration // 最大延迟（默认 30s）
	RetryOn    []int         // 需要重试的 HTTP 状态码
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

	// 若请求体未提供 GetBody（如 bytes.NewReader），一次性读入内存并记录，
	// 以便重试时重新发送 body。否则重试会发送空 body，导致上游返回 400。
	if req.Body != nil && req.GetBody == nil {
		if buf, rerr := io.ReadAll(req.Body); rerr == nil {
			_ = req.Body.Close()
			req.Body = io.NopCloser(bytes.NewReader(buf))
			req.ContentLength = int64(len(buf))
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(buf)), nil
			}
		}
	}

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay(attempt, nil)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
		}

		// 每次重试需要重新读取 body（如果有）
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err == nil {
				req.Body = body
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// 检查是否需要重试的状态码
		if c.shouldRetry(resp.StatusCode) {
			lastErr = fmt.Errorf("retryable status %d", resp.StatusCode)

			// 429: 尊重 Retry-After 头
			if resp.StatusCode == http.StatusTooManyRequests {
				if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")); retryAfter > 0 {
					resp.Body.Close()
					select {
					case <-req.Context().Done():
						return nil, req.Context().Err()
					case <-time.After(retryAfter):
					}
					continue
				}
			}

			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.config.MaxRetries, lastErr)
}

// retryDelay 计算重试延迟（支持 Retry-After 覆盖 + jitter）
func (c *RetryableHTTPClient) retryDelay(attempt int, retryAfter *time.Duration) time.Duration {
	var delay time.Duration
	if retryAfter != nil && *retryAfter > 0 {
		delay = *retryAfter
	} else {
		delay = c.backoff(attempt)
	}
	return c.addJitter(delay)
}

// backoff 指数退避延迟计算
func (c *RetryableHTTPClient) backoff(attempt int) time.Duration {
	delay := float64(c.config.BaseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(c.config.MaxDelay) {
		delay = float64(c.config.MaxDelay)
	}
	return time.Duration(delay)
}

// addJitter 添加 ±25% 随机抖动
func (c *RetryableHTTPClient) addJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return delay
	}
	// [-25%, +25%] 双边抖动
	jitterRange := int64(delay) / 2
	if jitterRange <= 0 {
		return delay
	}
	offset := time.Duration(rand.Int63n(jitterRange)) - time.Duration(jitterRange/2)
	return delay + offset
}

// parseRetryAfter 解析 Retry-After 头
// 支持两种格式：秒数（"120"）和 HTTP-date（"Wed, 21 Oct 2015 07:28:00 GMT"）
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	// 尝试解析为秒数
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		d := time.Duration(seconds) * time.Second
		// 安全上限：最多等待 5 分钟
		if d > 5*time.Minute {
			d = 5 * time.Minute
		}
		return d
	}

	// 尝试解析为 HTTP-date
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		// 安全上限：最多等待 5 分钟
		if d > 5*time.Minute {
			d = 5 * time.Minute
		}
		return d
	}

	return 0
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
	client := NewRetryableClient(consts.DefaultHTTPTimeout)
	return client.Do(req.WithContext(ctx))
}
