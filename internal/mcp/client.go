package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// filterEnvForSubprocess 过滤子进程所需的环境变量
// 只传必要的系统变量，不传 API key（安全）
func filterEnvForSubprocess() []string {
	keys := []string{
		"PATH", "HOME", "USER", "LANG", "TMPDIR",
	}
	var filtered []string
	for _, e := range os.Environ() {
		for _, key := range keys {
			if len(e) > len(key) && e[:len(key)+1] == key+"=" {
				filtered = append(filtered, e)
				break
			}
		}
	}
	return filtered
}

const (
	defaultMaxRetries = 3
	baseRetryDelay    = 1 * time.Second
	maxRetryDelay     = 30 * time.Second
	rpcTimeout        = 30 * time.Second
)

// Client MCP 客户端（stdio transport）
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu       sync.Mutex
	nextID   atomic.Int64
	serverInfo ServerInfo
	tools     []Tool
	initialized bool

	// 响应路由
	pendingMu sync.Mutex
	pending   map[int64]chan *Response
	done      chan struct{}

	// 重连配置
	command     string
	args        []string
	maxRetries  int
	reconnecting atomic.Bool
}

// NewClient 创建 MCP 客户端
func NewClient(command string, args ...string) *Client {
	return &Client{
		command:    command,
		args:       args,
		maxRetries: defaultMaxRetries,
		pending:    make(map[int64]chan *Response),
		done:       make(chan struct{}),
	}
}

// SetMaxRetries 设置最大重试次数
func (c *Client) SetMaxRetries(n int) {
	c.maxRetries = n
}

// Connect 连接并初始化
func (c *Client) Connect() error {
	return c.connect()
}

// connect 内部连接实现
func (c *Client) connect() error {
	cmd := exec.Command(c.command, c.args...)
	cmd.Env = filterEnvForSubprocess()
	c.cmd = cmd

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	c.stdout = bufio.NewReader(stdout)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	// 启动后台 reader goroutine
	go c.readLoop()

	// 发送 initialize
	initParams := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ClientCaps{
			Roots: &RootsCaps{ListChanged: true},
		},
		ClientInfo: ClientInfo{
			Name:    "Helix CLI",
			Version: "0.1.0",
		},
	}

	resp, err := c.callRaw(MethodInitialize, initParams)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		return fmt.Errorf("parse init result: %w", err)
	}

	c.serverInfo = initResult.ServerInfo
	c.initialized = true

	// 发送 initialized 通知
	c.sendNotification("notifications/initialized", nil)

	return nil
}

// reconnect 尝试重连 MCP 服务器
func (c *Client) reconnect() error {
	if !c.reconnecting.CompareAndSwap(false, true) {
		return fmt.Errorf("reconnection already in progress")
	}
	defer c.reconnecting.Store(false)

	// 清理旧进程
	c.cleanup()

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay(attempt)
			time.Sleep(delay)
		}

		if err := c.connect(); err != nil {
			lastErr = err
			continue
		}

		// 重连成功，重新注册所有 pending 请求
		c.pendingMu.Lock()
		pending := make(map[int64]chan *Response, len(c.pending))
		for id, ch := range c.pending {
			pending[id] = ch
		}
		c.pendingMu.Unlock()

		// 对于重连后的 pending 请求，发送新的请求
		for id, ch := range pending {
			// 由于重连后协议状态已重置，pending 请求需要重新发送
			// 但调用方不知道重连发生了，所以直接通知失败
			select {
			case ch <- &Response{
				JSONRPC: jsonrpcVersion,
				ID:      id,
				Error:   &RPCError{Code: -32000, Message: "server reconnected, request lost"},
			}:
			default:
			}
		}

		return nil
	}

	return fmt.Errorf("reconnect failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// retryDelay 计算指数退避延迟（带 jitter）
func (c *Client) retryDelay(attempt int) time.Duration {
	delay := baseRetryDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > maxRetryDelay {
			delay = maxRetryDelay
			break
		}
	}
	// 添加 jitter（±25%）
	jitter := time.Duration(rand.Int63n(int64(delay) / 2))
	return delay - jitter/2 + jitter/2
}

// cleanup 清理当前连接资源
func (c *Client) cleanup() {
	if c.stdin != nil {
		c.stdin.Close()
		c.stdin = nil
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
		c.cmd = nil
	}
}

// readLoop 后台读取循环，按 ID 路由响应
func (c *Client) readLoop() {
	defer close(c.done)
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			return
		}

		resp, err := ParseResponse(line)
		if err != nil {
			continue
		}

		// 路由响应到对应的 pending 请求
		c.pendingMu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.pendingMu.Unlock()

		if ok {
			ch <- resp
		}
	}
}

// call 发送请求并等待响应（带超时和自动重连）
func (c *Client) call(method string, params any) (*Response, error) {
	resp, err := c.callRaw(method, params)
	if err != nil && isDisconnectedErr(err) && c.maxRetries > 0 {
		if rerr := c.reconnect(); rerr != nil {
			return nil, fmt.Errorf("server disconnected and reconnect failed: %w", rerr)
		}
		// 重连成功，重试原始请求
		return c.callRaw(method, params)
	}
	return resp, err
}

// callRaw 发送请求并等待响应（不重连）
func (c *Client) callRaw(method string, params any) (*Response, error) {
	id := c.nextID.Add(1)
	req, err := NewRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	// 注册 pending 请求
	respCh := make(chan *Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	// 写入请求（需要锁保护 stdin）
	c.mu.Lock()
	data, err := json.Marshal(req)
	if err != nil {
		c.mu.Unlock()
		c.cancelPending(id)
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		c.mu.Unlock()
		c.cancelPending(id)
		return nil, fmt.Errorf("write request: %w", err)
	}
	c.mu.Unlock()

	// 等待响应
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("server error: %s", resp.Error.Message)
		}
		return resp, nil
	case <-time.After(rpcTimeout):
		c.cancelPending(id)
		return nil, fmt.Errorf("request timeout (%v)", rpcTimeout)
	case <-c.done:
		c.cancelPending(id)
		return nil, fmt.Errorf("server disconnected")
	}
}

// isDisconnectedErr 判断是否为断连错误
func isDisconnectedErr(err error) bool {
	msg := err.Error()
	return msg == "server disconnected" ||
		msg == "write request: io: read/write on closed pipe" ||
		msg == "write request: write |1: broken pipe"
}

// cancelPending 清理超时的 pending 请求
func (c *Client) cancelPending(id int64) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	delete(c.pending, id)
}

// Close 关闭连接
func (c *Client) Close() error {
	c.cleanup()
	return nil
}

// ServerInfo 返回服务器信息
func (c *Client) ServerInfo() ServerInfo {
	return c.serverInfo
}

// ListTools 列出可用工具
func (c *Client) ListTools() ([]Tool, error) {
	resp, err := c.call(MethodListTools, nil)
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
func (c *Client) CallTool(name string, args map[string]any) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.call(MethodCallTool, params)
	if err != nil {
		return nil, err
	}

	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tool result: %w", err)
	}

	return &result, nil
}

// sendNotification 发送通知（无响应）
func (c *Client) sendNotification(method string, params any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	notif := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return
		}
		notif.Params = data
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return
	}
	data = append(data, '\n')
	c.stdin.Write(data)
}
