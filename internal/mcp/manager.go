package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// PluginManager MCP 插件管理器
type PluginManager struct {
	mu      sync.RWMutex
	clients map[string]*Client // name → client
	tools   map[string]*mcpTool // tool name → wrapper
	registry *tool.Registry
}

// mcpTool MCP 工具适配器（实现 tool.Tool 接口）
type mcpTool struct {
	name        string
	description string
	schema      tool.Schema
	client      *Client
}

func (t *mcpTool) Name() string        { return t.name }
func (t *mcpTool) Description() string { return t.description }
func (t *mcpTool) IsReadOnly() bool    { return false } // MCP 工具默认为写入

func (t *mcpTool) Schema() tool.Schema {
	return t.schema
}

func (t *mcpTool) Execute(ctx context.Context, args map[string]any) (*tool.Result, error) {
	result, err := t.client.CallTool(t.name, args)
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
		clients:  make(map[string]*Client),
		tools:    make(map[string]*mcpTool),
		registry: registry,
	}
}

// Connect 连接 MCP 服务器
func (m *PluginManager) Connect(name, command string, args ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[name]; exists {
		return fmt.Errorf("plugin %q already connected", name)
	}

	client := NewClient(command, args...)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect %q: %w", name, err)
	}

	m.clients[name] = client

	// 发现并注册工具
	tools, err := client.ListTools()
	if err != nil {
		client.Close()
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
		m.registry.Register(mcpT)
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

	client.Close()
	delete(m.clients, name)
	return nil
}

// DisconnectAll 断开所有连接
func (m *PluginManager) DisconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		client.Close()
		delete(m.clients, name)
	}

	m.tools = make(map[string]*mcpTool)
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
