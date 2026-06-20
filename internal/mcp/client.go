package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// filterEnvForSubprocess 过滤子进程所需的环境变量
func filterEnvForSubprocess() []string {
	keys := []string{
		"DEEPSEEK_API_KEY", "MIMO_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY",
		"PATH", "HOME", "USER", "LANG",
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
}

// NewClient 创建 MCP 客户端
func NewClient(command string, args ...string) *Client {
	cmd := exec.Command(command, args...)
	cmd.Env = filterEnvForSubprocess()
	return &Client{
		cmd:     cmd,
		pending: make(map[int64]chan *Response),
		done:    make(chan struct{}),
	}
}

// Connect 连接并初始化
func (c *Client) Connect() error {
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	c.stdout = bufio.NewReader(stdout)

	if err := c.cmd.Start(); err != nil {
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

	resp, err := c.call(MethodInitialize, initParams)
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

// call 发送请求并等待响应（带超时）
func (c *Client) call(method string, params any) (*Response, error) {
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
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		c.mu.Unlock()
		c.cancelPending(id)
		return nil, fmt.Errorf("write request: %w", err)
	}
	c.mu.Unlock()

	// 等待响应（30 秒超时）
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("server error: %s", resp.Error.Message)
		}
		return resp, nil
	case <-time.After(30 * time.Second):
		c.cancelPending(id)
		return nil, fmt.Errorf("request timeout (30s)")
	case <-c.done:
		c.cancelPending(id)
		return nil, fmt.Errorf("server disconnected")
	}
}

// cancelPending 清理超时的 pending 请求
func (c *Client) cancelPending(id int64) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	delete(c.pending, id)
}

// Close 关闭连接
func (c *Client) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
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
		data, _ := json.Marshal(params)
		notif.Params = data
	}

	data, _ := json.Marshal(notif)
	data = append(data, '\n')
	c.stdin.Write(data)
}
