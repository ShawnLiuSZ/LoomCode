package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SSEClient MCP SSE 客户端（HTTP SSE transport）
type SSEClient struct {
	baseURL      string
	httpClient   *http.Client
	eventCh      chan SSEEvent
	mu           sync.Mutex
	nextID       atomic.Int64
	serverInfo   ServerInfo
	tools        []Tool
	initialized  bool
	sessionID    string
	lastDataTime atomic.Int64
	notifyCh     chan struct{}
	reconnecting atomic.Int32
	notifyMu     sync.Mutex
	closeOnce    sync.Once
}

// SSEEvent SSE 事件
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// NewSSEClient 创建 SSE 客户端
func NewSSEClient(baseURL string) *SSEClient {
	return &SSEClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		eventCh:    make(chan SSEEvent, 100),
		notifyCh:   make(chan struct{}, 1),
	}
}

// Connect 连接并初始化
func (c *SSEClient) Connect(ctx context.Context) error {
	// 1. 获取 SSE 端点
	endpoint, err := c.discoverEndpoint(ctx)
	if err != nil {
		return fmt.Errorf("discover endpoint: %w", err)
	}

	// 2. 启动 SSE 监听
	go c.listenSSE(ctx, endpoint)

	// 3. 发送 initialize
	initParams := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ClientCaps{
			Roots: &RootsCaps{ListChanged: true},
		},
		ClientInfo: ClientInfo{
			Name:    "LoomCode CLI",
			Version: "0.1.0",
		},
	}

	resp, err := c.call(ctx, MethodInitialize, initParams)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		return fmt.Errorf("parse init result: %w", err)
	}

	c.serverInfo = initResult.ServerInfo
	c.initialized = true

	// 4. 发送 initialized 通知
	c.sendNotification(ctx, "notifications/initialized", nil)

	return nil
}

// discoverEndpoint 发现 SSE 端点
func (c *SSEClient) discoverEndpoint(ctx context.Context) (string, error) {
	// 尝试从根路径获取端点信息
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/sse", nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// 如果直接访问 /sse 失败，尝试根路径
		req, err = http.NewRequestWithContext(ctx, "GET", c.baseURL, nil)
		if err != nil {
			return "", err
		}
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return "", err
		}
	}
	defer resp.Body.Close()

	// 检查是否是 SSE 响应
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return c.baseURL + "/sse", nil
	}

	// 尝试解析响应获取端点
	var info struct {
		Endpoints struct {
			SSE string `json:"sse"`
		} `json:"endpoints"`
		SSE string `json:"sse"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &info); err == nil {
		if info.Endpoints.SSE != "" {
			return info.Endpoints.SSE, nil
		}
		if info.SSE != "" {
			return info.SSE, nil
		}
	}

	// 默认使用 /sse
	return c.baseURL + "/sse", nil
}

// listenSSE 监听 SSE 事件
func (c *SSEClient) listenSSE(ctx context.Context, endpoint string) {
	const (
		initialDelay = 1 * time.Second
		maxDelay     = 30 * time.Second
		multiplier   = 2.0
		jitterPct    = 0.25
		maxAttempts  = 10
	)

	delay := initialDelay
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if attempts > maxAttempts {
			log.Printf("SSE client: max reconnect attempts (%d) exceeded, giving up", maxAttempts)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			delay, attempts = c.reconnectWithBackoff(ctx, delay, initialDelay, maxDelay, multiplier, jitterPct, maxAttempts, &attempts)
			continue
		}

		if c.sessionID != "" {
			req.Header.Set("Mcp-Session-Id", c.sessionID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			delay, attempts = c.reconnectWithBackoff(ctx, delay, initialDelay, maxDelay, multiplier, jitterPct, maxAttempts, &attempts)
			continue
		}

		if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
			c.sessionID = sid
		}

		attempts = 0
		delay = initialDelay
		c.lastDataTime.Store(time.Now().UnixMilli())

		// 为本次 SSE 流创建独立 cancel，让 heartbeat 超时能关闭 body
		sseCtx, sseCancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go c.heartbeatMonitor(sseCtx, resp.Body, done)

		c.readSSEStream(sseCtx, resp.Body)
		sseCancel()
		resp.Body.Close()
		close(done)

		c.reconnecting.Store(1)
		c.notifyMu.Lock()
		close(c.notifyCh)
		c.notifyCh = make(chan struct{}, 1)
		c.notifyMu.Unlock()

		delay, attempts = c.reconnectWithBackoff(ctx, delay, initialDelay, maxDelay, multiplier, jitterPct, maxAttempts, &attempts)
	}
}

func (c *SSEClient) reconnectWithBackoff(ctx context.Context, delay, initialDelay, maxDelay time.Duration, multiplier, jitterPct float64, maxAttempts int, attempts *int) (time.Duration, int) {
	*attempts++
	if *attempts > maxAttempts {
		return delay, *attempts
	}

	jitter := delay.Seconds() * jitterPct * (2*rand.Float64() - 1)
	sleepDuration := time.Duration(float64(delay.Seconds())+jitter) * time.Second
	if sleepDuration < initialDelay {
		sleepDuration = initialDelay
	}

	select {
	case <-ctx.Done():
		return delay, *attempts
	case <-time.After(sleepDuration):
	}

	nextDelay := time.Duration(float64(delay) * multiplier)
	if nextDelay > maxDelay {
		nextDelay = maxDelay
	}
	return nextDelay, *attempts
}

func (c *SSEClient) heartbeatMonitor(ctx context.Context, body io.ReadCloser, done chan struct{}) {
	const heartbeatTimeout = 60 * time.Second
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			lastTime := time.UnixMilli(c.lastDataTime.Load())
			if time.Since(lastTime) > heartbeatTimeout {
				// 关闭 body 让 readSSEStream 的 scanner.Scan() 返回 error 退出
				body.Close()
				return
			}
		}
	}
}

func (c *SSEClient) readSSEStream(ctx context.Context, body io.Reader) {
	scanner := bufio.NewScanner(body)
	var event SSEEvent

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			event.Event = trimSSEField(line, "event:")
		} else if strings.HasPrefix(line, "data:") {
			event.Data = trimSSEField(line, "data:")
		} else if strings.HasPrefix(line, "id:") {
			event.ID = trimSSEField(line, "id:")
		} else if line == "" {
			// 空行表示事件结束
			if event.Data != "" {
				c.lastDataTime.Store(time.Now().UnixMilli())
				c.safeSendEvent(event)
			}
			event = SSEEvent{}
		}
	}
}

// Close 关闭连接。用 sync.Once 防止重复 close panic。
func (c *SSEClient) Close() error {
	c.closeOnce.Do(func() {
		close(c.eventCh)
	})
	return nil
}

// safeSendEvent 向 eventCh 非阻塞发送事件，recover 防止 Close 后 send-on-closed panic。
func (c *SSEClient) safeSendEvent(event SSEEvent) {
	defer func() { _ = recover() }() // 防止 Close 后 send-on-closed channel panic
	select {
	case c.eventCh <- event:
	default:
	}
}

// trimSSEField 去除 SSE 字段前缀，兼容 "field: value" 和 "field:value" 两种形式。
func trimSSEField(line, prefix string) string {
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimPrefix(rest, " ") // 有空格则去一个
	return rest
}

// ServerInfo 返回服务器信息
func (c *SSEClient) ServerInfo() ServerInfo {
	return c.serverInfo
}

// ListTools 列出可用工具
func (c *SSEClient) ListTools(ctx context.Context) ([]Tool, error) {
	resp, err := c.call(ctx, MethodListTools, nil)
	if err != nil {
		return nil, err
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}

	c.tools = result.Tools
	return result.Tools, nil
}

// CallTool 调用工具
func (c *SSEClient) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.call(ctx, MethodCallTool, params)
	if err != nil {
		return nil, err
	}

	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tool result: %w", err)
	}

	return &result, nil
}

// call 发送请求并等待响应
func (c *SSEClient) call(ctx context.Context, method string, params any) (*Response, error) {
	id := c.nextID.Add(1)
	req, err := NewRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 发送 POST 请求
	data, _ := json.Marshal(req)
	postURL := c.baseURL + "/message"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", postURL, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 对于 POST 请求，响应可能直接返回
	if resp.StatusCode == http.StatusOK {
		var response Response
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &response); err == nil {
			return &response, nil
		}
	}

	// 否则从 SSE 事件中获取响应
	return c.waitForResponse(ctx, id)
}

// waitForResponse 等待响应
func (c *SSEClient) waitForResponse(ctx context.Context, id int64) (*Response, error) {
	return c.waitForResponseWithRetry(ctx, id)
}

// waitForResponseWithRetry 等待响应，支持重连后重试。
// 每次循环重新获取 c.notifyCh，避免重连期间通道被替换后错过信号（L5）。
func (c *SSEClient) waitForResponseWithRetry(ctx context.Context, id int64) (*Response, error) {
	timeout := time.After(30 * time.Second)

	for {
		// 每次循环重新获取 notifyCh，防止重连时通道被 close+替换导致等待者错过信号
		c.notifyMu.Lock()
		notifyCh := c.notifyCh
		c.notifyMu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for response")
		case <-notifyCh:
			// 连接断开通知：等待重连（sleep 2s 给 SSE 流重建时间），然后重新尝试
			time.Sleep(2 * time.Second)
			continue
		case event := <-c.eventCh:
			if event.Event == "message" || event.Event == "" {
				var msg struct {
					ID     int64           `json:"id"`
					Result json.RawMessage `json:"result"`
					Error  *RPCError       `json:"error"`
				}
				if err := json.Unmarshal([]byte(event.Data), &msg); err == nil {
					if msg.ID == id {
						return &Response{
							JSONRPC: jsonrpcVersion,
							ID:      msg.ID,
							Result:  msg.Result,
							Error:   msg.Error,
						}, nil
					}
				}
			}
		}
	}
}

// sendNotification 发送通知（无响应）
func (c *SSEClient) sendNotification(ctx context.Context, method string, params any) {
	notif := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
	}

	if params != nil {
		data, _ := json.Marshal(params)
		notif.Params = data
	}

	data, _ := json.Marshal(notif)
	postURL := c.baseURL + "/message"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", postURL, strings.NewReader(string(data)))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	if _, err := c.httpClient.Do(httpReq); err != nil {
		log.Printf("send notification %s failed: %v", method, err)
	}
}

// ParseSSEURL 解析 SSE URL
func ParseSSEURL(rawURL string) (baseURL, endpoint string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}

	baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	endpoint = u.Path

	if endpoint == "" {
		endpoint = "/sse"
	}

	return baseURL, endpoint, nil
}
