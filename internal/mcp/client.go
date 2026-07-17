package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
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

// mergeEnv 合并环境变量，extra 覆盖 base 中的同名 key
func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	// 构建已存在的 key 集合
	existing := make(map[string]bool, len(base))
	for _, e := range base {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				existing[e[:i]] = true
				break
			}
		}
	}
	result := make([]string, 0, len(base)+len(extra))
	result = append(result, base...)
	for k, v := range extra {
		if !existing[k] {
			result = append(result, k+"="+v)
		}
	}
	return result
}

const (
	defaultMaxRetries = 3
	baseRetryDelay    = 1 * time.Second
	maxRetryDelay     = 30 * time.Second
	rpcTimeout        = 30 * time.Second
)

// errServerDisconnected 表示服务器断连（用于 isDisconnectedErr 的 errors.Is 匹配，L13）
var errServerDisconnected = errors.New("server disconnected")

// Client MCP 客户端（stdio transport）
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu          sync.Mutex
	nextID      atomic.Int64
	serverInfo  ServerInfo
	tools       []Tool
	initialized bool

	// 响应路由
	pendingMu sync.Mutex
	pending   map[int64]chan *Response
	done      chan struct{}

	// 重连配置
	command      string
	args         []string
	maxRetries   int
	reconnecting atomic.Bool

	// 额外环境变量（来自配置文件的 env 字段）
	extraEnv map[string]string

	// 子进程生命周期 context：connect 时创建，cleanup/Close 时取消，
	// 确保子进程（MCP server）在关闭时被杀掉，避免孤儿进程。
	lifeCtx    context.Context
	lifeCancel context.CancelFunc

	// 连接/关闭串行化锁，防止 Connect/Close/reconnect 并发竞态（L14）
	connectMu sync.Mutex
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

// SetEnv 设置额外环境变量
func (c *Client) SetEnv(env map[string]string) {
	c.extraEnv = env
}

// SetMaxRetries 设置最大重试次数
func (c *Client) SetMaxRetries(n int) {
	c.maxRetries = n
}

// Connect 连接并初始化
func (c *Client) Connect() error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()
	return c.connect()
}

// connect 内部连接实现
func (c *Client) connect() error {
	// 每次连接创建独立的生命周期 context，cleanup 时取消以杀掉子进程（避免孤儿进程）。
	c.lifeCtx, c.lifeCancel = context.WithCancel(context.Background())
	cmd := exec.CommandContext(c.lifeCtx, c.command, c.args...)
	cmd.Env = mergeEnv(filterEnvForSubprocess(), c.extraEnv)
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

	// 重建 done channel（cleanup 已关闭旧的）
	c.done = make(chan struct{})

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
			Name:    "LoomCode CLI",
			Version: "0.1.0",
		},
	}

	resp, err := c.callRaw(context.Background(), MethodInitialize, initParams)
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
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

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
		// 清空 pending：旧请求已无法通过新连接恢复
		c.pending = make(map[int64]chan *Response)
		c.pendingMu.Unlock()

		// 对于重连后的 pending 请求，通知失败
		for id, ch := range pending {
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
	return delay - jitter
}

// cleanup 清理当前连接资源。
// 注意：c.done 由 readLoop 捕获局部变量后 `defer close(done)` 统一关闭（BUG-001 修复：
// 必须捕获局部变量，否则 reconnect() 重建 c.done 后，旧 readLoop 会误关新的 done channel）。
// 这里【不要】再次 close，否则会与 readLoop 的 defer 形成 double-close → panic: close of closed channel。
// 取消 lifeCtx 会触发 exec.CommandContext 杀掉子进程（与 Process.Kill 双保险）。
func (c *Client) cleanup() {
	if c.lifeCancel != nil {
		c.lifeCancel()
		c.lifeCancel = nil
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
		c.stdin = nil
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
		c.cmd = nil
	}
}

// readLoop 后台读取循环，按 ID 路由响应
func (c *Client) readLoop() {
	// BUG-001 修复：捕获启动时的 done channel 到局部变量。
	// reconnect() 会重新赋值 c.done = make(chan struct{})，若直接 defer close(c.done)，
	// 旧 readLoop goroutine 退出时会关闭【新的】done channel，导致重连后所有 callRaw
	// 的 <-c.done 立即返回，误判为断连。关闭局部变量确保只关闭本次连接对应的旧 channel。
	done := c.done
	defer close(done)
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
func (c *Client) call(ctx context.Context, method string, params any) (*Response, error) {
	resp, err := c.callRaw(ctx, method, params)
	if err != nil && isDisconnectedErr(err) && c.maxRetries > 0 {
		if rerr := c.reconnect(); rerr != nil {
			return nil, fmt.Errorf("server disconnected and reconnect failed: %w", rerr)
		}
		// 重连成功，重试原始请求
		return c.callRaw(ctx, method, params)
	}
	return resp, err
}

// callRaw 发送请求并等待响应（不重连）
func (c *Client) callRaw(ctx context.Context, method string, params any) (*Response, error) {
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

	// 等待响应（N4：支持外部 ctx 取消，用户 Ctrl+C 时不再等满 30s）
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
		return nil, fmt.Errorf("%w", errServerDisconnected)
	case <-ctx.Done():
		c.cancelPending(id)
		return nil, ctx.Err()
	}
}

// isDisconnectedErr 判断是否为断连错误（L13：用 errors.Is 替代字符串匹配）
func isDisconnectedErr(err error) bool {
	if errors.Is(err, errServerDisconnected) {
		return true
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	return false
}

// cancelPending 清理超时/失败的 pending 请求，并 drain buffered 响应（L16）
func (c *Client) cancelPending(id int64) {
	c.pendingMu.Lock()
	ch, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	if ok {
		// drain 可能已到达的 buffered 响应，防止响应滞留 channel
		select {
		case <-ch:
		default:
		}
	}
}

// Close 关闭连接
func (c *Client) Close() error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()
	c.cleanup()
	return nil
}

// ServerInfo 返回服务器信息
func (c *Client) ServerInfo() ServerInfo {
	return c.serverInfo
}

// ListTools 列出可用工具
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
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
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
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

// sendNotification 发送通知（无响应，L11：错误日志化而非静默吞掉）
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
			log.Printf("mcp sendNotification %q: marshal params failed: %v", method, err)
			return
		}
		notif.Params = data
	}

	data, err := json.Marshal(notif)
	if err != nil {
		log.Printf("mcp sendNotification %q: marshal notif failed: %v", method, err)
		return
	}
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		log.Printf("mcp sendNotification %q: write failed: %v", method, err)
	}
}
