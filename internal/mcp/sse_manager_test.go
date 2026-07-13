package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// mockMCPSSEServer 返回一个最小可用的 MCP-over-HTTP/SSE 服务器：
// GET /sse 提供 event-stream 并保持打开；POST /message 直接返回 JSON-RPC 响应。
func mockMCPSSEServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sse":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				w.Write([]byte("event: endpoint\ndata: /message\n\n"))
				f.Flush()
			}
			<-r.Context().Done() // 保持流打开，直到连接取消
		case "/message":
			body, _ := io.ReadAll(r.Body)
			var rpc struct {
				ID     int64  `json:"id"`
				Method string `json:"method"`
			}
			json.Unmarshal(body, &rpc)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			switch rpc.Method {
			case MethodInitialize:
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}}`, rpc.ID)
			case MethodListTools:
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"echo","description":"Echo back","inputSchema":{"type":"object","properties":{},"required":[]}}]}}`, rpc.ID)
			case MethodCallTool:
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"pong"}],"isError":false}}`, rpc.ID)
			default:
				w.Write([]byte("{}")) // notifications/initialized 等
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestPluginManager_ConnectSSE_RegistersTools(t *testing.T) {
	server := mockMCPSSEServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer server.Close()
	defer cancel()

	registry := tool.NewRegistry()
	mgr := NewPluginManager(registry)

	if err := mgr.ConnectSSE(ctx, "sse", server.URL); err != nil {
		t.Fatalf("ConnectSSE error: %v", err)
	}

	// 工具应以 mcp_<name>_<tool> 命名注册进 registry
	tl, ok := registry.Get("mcp_sse_echo")
	if !ok {
		t.Fatalf("expected mcp_sse_echo to be registered; plugins=%v count=%d", mgr.ListPlugins(), mgr.ToolCount())
	}

	// 并且可经统一接口调用
	res, err := tl.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.Content != "pong" {
		t.Errorf("tool result = %q, want \"pong\"", res.Content)
	}
}

func TestPluginManager_ConnectSSE_DuplicateRejected(t *testing.T) {
	server := mockMCPSSEServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer server.Close()
	defer cancel()

	mgr := NewPluginManager(tool.NewRegistry())
	if err := mgr.ConnectSSE(ctx, "dup", server.URL); err != nil {
		t.Fatalf("first ConnectSSE error: %v", err)
	}
	if err := mgr.ConnectSSE(ctx, "dup", server.URL); err == nil {
		t.Error("expected duplicate ConnectSSE to be rejected")
	}
}
