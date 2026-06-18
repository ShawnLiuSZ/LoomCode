package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
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
}

// NewClient 创建 MCP 客户端
func NewClient(command string, args ...string) *Client {
	return &Client{
		cmd: exec.Command(command, args...),
	}
}

// Connect 连接并初始化
func (c *Client) Connect() error {
	// 启动子进程
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

// Close 关闭连接
func (c *Client) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
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

// call 发送请求并等待响应
func (c *Client) call(method string, params any) (*Response, error) {
	id := c.nextID.Add(1)
	req, err := NewRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 发送请求
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// 读取响应
	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return ParseResponse(line)
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
