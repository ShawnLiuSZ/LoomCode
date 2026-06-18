package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

func TestNewRequest(t *testing.T) {
	req, err := NewRequest(1, "test/method", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q", req.JSONRPC)
	}
	if req.ID != 1 {
		t.Errorf("id = %d", req.ID)
	}
	if req.Method != "test/method" {
		t.Errorf("method = %q", req.Method)
	}
}

func TestNewRequest_NilParams(t *testing.T) {
	req, err := NewRequest(2, "no_params", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	if req.Params != nil {
		t.Error("params should be nil")
	}
}

func TestParseResponse(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`)
	resp, err := ParseResponse(data)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if resp.ID != 1 {
		t.Errorf("id = %d", resp.ID)
	}
}

func TestParseResponse_Error(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	resp, err := ParseResponse(data)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d", resp.Error.Code)
	}
}

func TestParseResult(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"result":{"name":"test"}}`)
	resp, _ := ParseResponse(data)

	type testResult struct {
		Name string `json:"name"`
	}
	result, err := ParseResult[testResult](resp)
	if err != nil {
		t.Fatalf("ParseResult error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("name = %q", result.Name)
	}
}

func TestRPCError_Error(t *testing.T) {
	e := &RPCError{Code: -32000, Message: "server error"}
	if e.Error() != "MCP error -32000: server error" {
		t.Errorf("Error() = %q", e.Error())
	}
}

// mockMCPServer 模拟 MCP 服务器（用于集成测试）

func startMockServer(t *testing.T) (*Client, func()) {
	t.Helper()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "mcp_server.sh")

	// 创建模拟服务器脚本
	script := `#!/bin/bash
# 读取 initialize 请求
read -r line
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":"mock-server","version":"1.0"}}}'

# 读取 initialized 通知（忽略）
read -r line

# 读取 tools/list 请求
read -r line
echo '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"echo","description":"Echo a message","inputSchema":{"type":"object","properties":{"message":{"type":"string","description":"Message to echo"}},"required":["message"]}}]}}'

# 读取 tools/call 请求
read -r line
echo '{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"Echo: hello world"}]}}'

# 保持运行
sleep 60
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	client := NewClient("bash", scriptPath)
	if err := client.Connect(); err != nil {
		client.Close()
		t.Fatalf("Connect error: %v", err)
	}

	return client, func() { client.Close() }
}

func TestClient_Connect(t *testing.T) {
	client, cleanup := startMockServer(t)
	defer cleanup()

	if client.ServerInfo().Name != "mock-server" {
		t.Errorf("ServerInfo.Name = %q", client.ServerInfo().Name)
	}
}

func TestClient_ListTools(t *testing.T) {
	client, cleanup := startMockServer(t)
	defer cleanup()

	tools, err := client.ListTools()
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools count = %d, want 1", len(tools))
	}
	if tools[0].Name != "echo" {
		t.Errorf("tool name = %q", tools[0].Name)
	}
}

func TestClient_CallTool(t *testing.T) {
	client, cleanup := startMockServer(t)
	defer cleanup()

	// 先 list tools（mock server 按顺序响应）
	client.ListTools()

	result, err := client.CallTool("echo", map[string]any{"message": "hello world"})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content count = %d", len(result.Content))
	}
	if result.Content[0].Text != "Echo: hello world" {
		t.Errorf("content = %q", result.Content[0].Text)
	}
}

func TestPluginManager_Connect(t *testing.T) {
	registry := tool.NewRegistry()
	mgr := NewPluginManager(registry)

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "plugin.sh")

	script := `#!/bin/bash
read -r line
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"test-plugin","version":"1.0"}}}'
read -r line
read -r line
echo '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"search","description":"Search files","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]}}'
read -r line
echo '{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"results"}]}}'
sleep 60
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	err := mgr.Connect("test", "bash", scriptPath)
	if err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer mgr.DisconnectAll()

	plugins := mgr.ListPlugins()
	if len(plugins) != 1 {
		t.Errorf("plugins count = %d, want 1", len(plugins))
	}
	if plugins[0] != "test" {
		t.Errorf("plugin name = %q", plugins[0])
	}

	if mgr.ToolCount() != 1 {
		t.Errorf("ToolCount() = %d, want 1", mgr.ToolCount())
	}

	// 验证工具已注册到 registry
	mcpToolName := "mcp_test_search"
	tl, ok := registry.Get(mcpToolName)
	if !ok {
		t.Fatalf("tool %q not found in registry", mcpToolName)
	}
	if tl.Name() != mcpToolName {
		t.Errorf("tool name = %q", tl.Name())
	}
}

func TestPluginManager_Disconnect(t *testing.T) {
	registry := tool.NewRegistry()
	mgr := NewPluginManager(registry)

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "disc.sh")
	script := `#!/bin/bash
read -r line
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"disc","version":"1.0"}}}'
read -r line
read -r line
echo '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"tool1","description":"desc","inputSchema":{"type":"object"}}]}}'
sleep 60
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	mgr.Connect("disconnect-test", "bash", scriptPath)

	if mgr.ToolCount() != 1 {
		t.Errorf("ToolCount before disconnect = %d, want 1", mgr.ToolCount())
	}

	mgr.Disconnect("disconnect-test")

	if mgr.ToolCount() != 0 {
		t.Errorf("ToolCount after disconnect = %d, want 0", mgr.ToolCount())
	}
	if len(mgr.ListPlugins()) != 0 {
		t.Error("plugins list should be empty after disconnect")
	}
}

func TestPluginManager_DuplicateConnect(t *testing.T) {
	registry := tool.NewRegistry()
	mgr := NewPluginManager(registry)

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "dup.sh")
	script := `#!/bin/bash
read -r line
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"dup","version":"1.0"}}}'
read -r line
read -r line
echo '{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}'
sleep 60
`
	os.WriteFile(scriptPath, []byte(script), 0755)

	mgr.Connect("dup", "bash", scriptPath)
	defer mgr.DisconnectAll()

	err := mgr.Connect("dup", "bash", scriptPath)
	if err == nil {
		t.Error("expected error for duplicate connection")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	// 验证 JSON 序列化往返
	req, _ := NewRequest(1, "test", map[string]string{"k": "v"})
	data, _ := json.Marshal(req)

	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if parsed.Method != "test" {
		t.Errorf("method = %q", parsed.Method)
	}
}

func TestMCPTool_Execute(t *testing.T) {
	client, cleanup := startMockServer(t)
	defer cleanup()

	client.ListTools()

	tl := &mcpTool{
		name:        "echo",
		description: "Echo tool",
		client:      client,
	}

	result, err := tl.Execute(nil, map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	fmt.Println(result)
	if result.Content != "Echo: hello world" {
		t.Errorf("Content = %q", result.Content)
	}
}
