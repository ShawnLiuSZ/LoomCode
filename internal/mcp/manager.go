package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// mcpClient 抽象 stdio 与 SSE(HTTP) 两种传输，使 manager 能用同一套逻辑
// 发现并注册工具。ctx 由调用方传入，支持取消（N4）。
type mcpClient interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error)
	Close() error
}

// sseClientAdapter 把基于 ctx 的 SSEClient 适配成 mcpClient。
// 工具调用透传调用方 ctx，支持用户取消；SSE 监听生命周期由 ConnectSSE 传入的 ctx 控制。
type sseClientAdapter struct{ c *SSEClient }

func (s *sseClientAdapter) ListTools(ctx context.Context) ([]Tool, error) {
	return s.c.ListTools(ctx)
}
func (s *sseClientAdapter) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	return s.c.CallTool(ctx, name, args)
}
func (s *sseClientAdapter) Close() error { return s.c.Close() }

// PluginManager MCP 插件管理器
type PluginManager struct {
	mu       sync.RWMutex
	clients  map[string]mcpClient // name → client（stdio 或 SSE）
	tools    map[string]*mcpTool  // tool name → wrapper
	registry *tool.Registry
}

// mcpTool MCP 工具适配器（实现 tool.Tool 接口）
type mcpTool struct {
	name        string
	description string
	schema      tool.Schema
	client      mcpClient
}

func (t *mcpTool) Name() string        { return t.name }
func (t *mcpTool) Description() string { return t.description }
func (t *mcpTool) IsReadOnly() bool    { return false } // MCP 工具默认为写入

func (t *mcpTool) Schema() tool.Schema {
	return t.schema
}

func (t *mcpTool) Execute(ctx context.Context, args map[string]any) (*tool.Result, error) {
	result, err := t.client.CallTool(ctx, t.name, args)
	if err != nil {
		return nil, fmt.Errorf("mcp call %q: %w", t.name, err)
	}

	if result.IsError {
		errMsg := ""
		for _, item := range result.Content {
			errMsg += item.Text
		}
		return &tool.Result{Error: errMsg}, nil
	}

	content := ""
	for _, item := range result.Content {
		if item.Type == "text" {
			content += item.Text
		}
	}

	return &tool.Result{Content: content}, nil
}

// NewPluginManager 创建插件管理器
func NewPluginManager(registry *tool.Registry) *PluginManager {
	return &PluginManager{
		clients:  make(map[string]mcpClient),
		tools:    make(map[string]*mcpTool),
		registry: registry,
	}
}

// Connect 连接 MCP 服务器（stdio 传输）
func (m *PluginManager) Connect(name, command string, args []string, env map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[name]; exists {
		return fmt.Errorf("plugin %q already connected", name)
	}

	client := NewClient(command, args...)
	if len(env) > 0 {
		client.SetEnv(env)
	}
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect %q: %w", name, err)
	}

	m.clients[name] = client
	return m.registerClientTools(name, client)
}

// ConnectSSE 连接 MCP 服务器（HTTP SSE 传输）。
// ctx 控制 SSE 监听协程的生命周期，应传入长生命周期的 context（如应用级 ctx）。
func (m *PluginManager) ConnectSSE(ctx context.Context, name, baseURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[name]; exists {
		return fmt.Errorf("plugin %q already connected", name)
	}

	sse := NewSSEClient(baseURL)
	if err := sse.Connect(ctx); err != nil {
		return fmt.Errorf("connect SSE %q: %w", name, err)
	}

	client := &sseClientAdapter{c: sse}
	m.clients[name] = client
	return m.registerClientTools(name, client)
}

// registerClientTools 发现 client 的工具并注册到 registry（stdio/SSE 共用）。
// 调用方需持有 m.mu。失败时回收 client 与登记项。
func (m *PluginManager) registerClientTools(name string, client mcpClient) error {
	tools, err := client.ListTools(context.Background())
	if err != nil {
		_ = client.Close()
		delete(m.clients, name)
		return fmt.Errorf("list tools for %q: %w", name, err)
	}

	for _, t := range tools {
		mcpT := &mcpTool{
			name:        fmt.Sprintf("mcp_%s_%s", name, t.Name),
			description: fmt.Sprintf("[MCP:%s] %s", name, t.Description),
			client:      client,
			schema: tool.Schema{
				Type:       t.InputSchema.Type,
				Properties: convertProperties(t.InputSchema.Properties),
				Required:   t.InputSchema.Required,
			},
		}

		m.tools[mcpT.name] = mcpT
		if err := m.registry.Register(mcpT); err != nil {
			return fmt.Errorf("register tool %q: %w", mcpT.name, err)
		}
	}

	return nil
}

// Disconnect 断开连接
func (m *PluginManager) Disconnect(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[name]
	if !ok {
		return fmt.Errorf("plugin %q not connected", name)
	}

	// 注销工具
	for toolName, t := range m.tools {
		if t.client == client {
			delete(m.tools, toolName)
		}
	}

	_ = client.Close()
	delete(m.clients, name)
	return nil
}

// DisconnectAll 断开所有连接。
// 先在锁内快照 clients 列表并清空 map，再解锁逐个 Close，
// 避免某个 Close 阻塞（如 stdio 子进程不响应）导致整个 DisconnectAll 卡住（N8）。
func (m *PluginManager) DisconnectAll() {
	m.mu.Lock()
	clients := make([]mcpClient, 0, len(m.clients))
	for name, client := range m.clients {
		clients = append(clients, client)
		delete(m.clients, name)
	}
	m.tools = make(map[string]*mcpTool)
	m.mu.Unlock()

	for _, client := range clients {
		_ = client.Close()
	}
}

// ListPlugins 列出已连接的插件
func (m *PluginManager) ListPlugins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// ToolCount 返回已注册的 MCP 工具数
func (m *PluginManager) ToolCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tools)
}

// convertProperties 转换 MCP schema 属性到 tool.Property
func convertProperties(props map[string]Property) map[string]tool.Property {
	result := make(map[string]tool.Property, len(props))
	for name, p := range props {
		result[name] = tool.Property{
			Type:        p.Type,
			Description: p.Description,
		}
	}
	return result
}
