package mcp

import (
	"encoding/json"
	"fmt"
)

// 协议常量
const (
	ProtocolVersion = "2024-11-05"
	jsonrpcVersion  = "2.0"
)

// 标准方法名
const (
	MethodInitialize  = "initialize"
	MethodListTools   = "tools/list"
	MethodCallTool    = "tools/call"
	MethodListResources = "resources/list"
	MethodListPrompts = "prompts/list"
)

// Request JSON-RPC 请求
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response JSON-RPC 响应
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError JSON-RPC 错误
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// Notification JSON-RPC 通知（无 ID）
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// InitializeParams initialize 请求参数
type InitializeParams struct {
	ProtocolVersion string      `json:"protocolVersion"`
	Capabilities    ClientCaps  `json:"capabilities"`
	ClientInfo      ClientInfo  `json:"clientInfo"`
}

// ClientCaps 客户端能力
type ClientCaps struct {
	Roots    *RootsCaps    `json:"roots,omitempty"`
	Sampling *SamplingCaps `json:"sampling,omitempty"`
}

// RootsCaps roots 能力
type RootsCaps struct {
	ListChanged bool `json:"listChanged"`
}

// SamplingCaps sampling 能力
type SamplingCaps struct{}

// ClientInfo 客户端信息
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult initialize 响应
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    ServerCaps   `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// ServerCaps 服务器能力
type ServerCaps struct {
	Tools     *ToolsCaps     `json:"tools,omitempty"`
	Resources *ResourcesCaps `json:"resources,omitempty"`
	Prompts   *PromptsCaps   `json:"prompts,omitempty"`
}

// ToolsCaps 工具能力
type ToolsCaps struct {
	ListChanged bool `json:"listChanged"`
}

// ResourcesCaps 资源能力
type ResourcesCaps struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

// PromptsCaps prompts 能力
type PromptsCaps struct {
	ListChanged bool `json:"listChanged"`
}

// ServerInfo 服务器信息
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ListToolsResult tools/list 响应
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// Tool MCP 工具定义
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema 工具输入 schema
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property schema 属性
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// CallToolParams tools/call 请求参数
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// CallToolResult tools/call 响应
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem 内容项
type ContentItem struct {
	Type string `json:"type"` // "text" | "image" | "resource"
	Text string `json:"text,omitempty"`
}

// NewRequest 创建 JSON-RPC 请求
func NewRequest(id int64, method string, params any) (*Request, error) {
	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		rawParams = data
	}

	return &Request{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}, nil
}

// ParseResponse 解析 JSON-RPC 响应
func ParseResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &resp, nil
}

// ParseResult 从响应中解析结果
func ParseResult[T any](resp *Response) (*T, error) {
	if resp.Error != nil {
		return nil, resp.Error
	}

	var result T
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}
	return &result, nil
}
