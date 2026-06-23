package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLSPError_Error(t *testing.T) {
	e := &LSPError{Code: -32000, Message: "server error"}
	if e.Error() != "LSP error -32000: server error" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestPosition_RoundTrip(t *testing.T) {
	pos := Position{Line: 10, Character: 5}
	data, _ := json.Marshal(pos)

	var parsed Position
	json.Unmarshal(data, &parsed)

	if parsed.Line != 10 || parsed.Character != 5 {
		t.Errorf("round-trip failed: %+v", parsed)
	}
}

func TestRange_RoundTrip(t *testing.T) {
	r := Range{
		Start: Position{Line: 1, Character: 2},
		End:   Position{Line: 3, Character: 4},
	}
	data, _ := json.Marshal(r)

	var parsed Range
	json.Unmarshal(data, &parsed)

	if parsed.Start.Line != 1 || parsed.End.Character != 4 {
		t.Errorf("round-trip failed: %+v", parsed)
	}
}

func TestLocation_RoundTrip(t *testing.T) {
	loc := Location{
		URI: "file:///test.go",
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 10},
		},
	}
	data, _ := json.Marshal(loc)

	var parsed Location
	json.Unmarshal(data, &parsed)

	if parsed.URI != "file:///test.go" {
		t.Errorf("URI = %q", parsed.URI)
	}
}

func TestInitializeParams_Marshal(t *testing.T) {
	params := InitializeParams{
		ProcessID: nil,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCaps{
				Completion: &CompletionCaps{DynamicRegistration: false},
			},
		},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed InitializeParams
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Capabilities.TextDocument == nil {
		t.Error("TextDocument capabilities should not be nil")
	}
}

func TestCompletionParams_Marshal(t *testing.T) {
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///main.go"},
		Position:     Position{Line: 10, Character: 5},
	}

	data, _ := json.Marshal(params)

	var parsed CompletionParams
	json.Unmarshal(data, &parsed)

	if parsed.TextDocument.URI != "file:///main.go" {
		t.Errorf("URI = %q", parsed.TextDocument.URI)
	}
}

func TestCompletionItem_Marshal(t *testing.T) {
	items := []CompletionItem{
		{Label: "fmt.Println", Kind: 3, Detail: "Print with newline"},
		{Label: "fmt.Printf", Kind: 3, Detail: "Print formatted"},
	}

	data, _ := json.Marshal(items)

	var parsed []CompletionItem
	json.Unmarshal(data, &parsed)

	if len(parsed) != 2 {
		t.Fatalf("items count = %d, want 2", len(parsed))
	}
	if parsed[0].Label != "fmt.Println" {
		t.Errorf("label = %q", parsed[0].Label)
	}
}

func TestHover_Marshal(t *testing.T) {
	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "**Println** formats and writes to standard output.",
		},
	}

	data, _ := json.Marshal(hover)

	var parsed Hover
	json.Unmarshal(data, &parsed)

	if parsed.Contents.Kind != "markdown" {
		t.Errorf("Kind = %q", parsed.Contents.Kind)
	}
}

func TestDocumentSymbol_Marshal(t *testing.T) {
	symbols := []DocumentSymbol{
		{
			Name: "main",
			Kind: 12, // Function
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 10, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 4},
			},
			Children: []DocumentSymbol{
				{Name: "helper", Kind: 12},
			},
		},
	}

	data, _ := json.Marshal(symbols)

	var parsed []DocumentSymbol
	json.Unmarshal(data, &parsed)

	if len(parsed) != 1 {
		t.Fatalf("symbols count = %d", len(parsed))
	}
	if parsed[0].Name != "main" {
		t.Errorf("name = %q", parsed[0].Name)
	}
	if len(parsed[0].Children) != 1 {
		t.Errorf("children count = %d", len(parsed[0].Children))
	}
}

func TestInitializeResult_Marshal(t *testing.T) {
	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    1,
			},
			HoverProvider:      true,
			DefinitionProvider: true,
		},
		ServerInfo: ServerInfoResult{
			Name:    "gopls",
			Version: "0.15.0",
		},
	}

	data, _ := json.Marshal(result)

	var parsed InitializeResult
	json.Unmarshal(data, &parsed)

	if parsed.ServerInfo.Name != "gopls" {
		t.Errorf("ServerInfo.Name = %q", parsed.ServerInfo.Name)
	}
	if !parsed.Capabilities.HoverProvider {
		t.Error("HoverProvider should be true")
	}
}

func TestDidOpenParams_Marshal(t *testing.T) {
	params := DidOpenParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///main.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main\n\nfunc main() {}\n",
		},
	}

	data, _ := json.Marshal(params)

	var parsed DidOpenParams
	json.Unmarshal(data, &parsed)

	if parsed.TextDocument.LanguageID != "go" {
		t.Errorf("LanguageID = %q", parsed.TextDocument.LanguageID)
	}
}

// startMockLSPServer 创建模拟 LSP 服务器
func startMockLSPServer(t *testing.T) (*Client, func()) {
	t.Helper()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "lsp_server.sh")

	// 模拟 LSP 服务器：读取 Content-Length 分帧，返回 JSON-RPC 响应
	script := `#!/bin/bash
# LSP 服务器模拟：读取请求并返回响应

read_lsp_message() {
    while IFS= read -r line; do
        case "$line" in
            Content-Length:*)
                content_length="${line#Content-Length: }"
                content_length="${content_length%%$'\r'}"
                # 读取空行
                read -r blank_line
                # 读取 body
                body=$(dd bs=1 count=$content_length 2>/dev/null)
                echo "$body"
                return
                ;;
        esac
    done
}

write_lsp_message() {
    local body="$1"
    local length=${#body}
    printf "Content-Length: %d\r\n\r\n%s" "$length" "$body"
}

# 读取 initialize 请求
body=$(read_lsp_message)
# 返回 initialize 响应
response='{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"hoverProvider":true,"definitionProvider":true},"serverInfo":{"name":"mock-lsp","version":"1.0"}}}'
write_lsp_message "$response"

# 读取 initialized 通知
read_lsp_message > /dev/null

# 进入请求循环
while true; do
    body=$(read_lsp_message) || break
    # 解析 method
    method=$(echo "$body" | grep -o '"method":"[^"]*"' | head -1 | cut -d'"' -f4)
    id=$(echo "$body" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)

    case "$method" in
        "textDocument/hover")
            response="{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"contents\":{\"kind\":\"markdown\",\"value\":\"**func main()**\"}}}"
            ;;
        "textDocument/definition")
            response="{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"uri\":\"file:///main.go\",\"range\":{\"start\":{\"line\":5,\"character\":0},\"end\":{\"line\":5,\"character\":4}}}}"
            ;;
        "shutdown")
            response="{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":null}"
            write_lsp_message "$response"
            exit 0
            ;;
        *)
            response="{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":null}"
            ;;
    esac
    write_lsp_message "$response"
done
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
	client, cleanup := startMockLSPServer(t)
	defer cleanup()

	info := client.GetServerInfo()
	if info == nil {
		t.Fatal("expected server info")
	}
	if info.Name != "mock-lsp" {
		t.Errorf("ServerInfo.Name = %q, want %q", info.Name, "mock-lsp")
	}
	if info.Version != "1.0" {
		t.Errorf("ServerInfo.Version = %q, want %q", info.Version, "1.0")
	}
}

func TestClient_Hover(t *testing.T) {
	client, cleanup := startMockLSPServer(t)
	defer cleanup()

	params := map[string]any{
		"textDocument": map[string]any{"uri": "file:///main.go"},
		"position":     map[string]any{"line": 5, "character": 0},
	}

	result, err := client.call("textDocument/hover", params)
	if err != nil {
		t.Fatalf("hover error: %v", err)
	}

	var hover Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		t.Fatalf("unmarshal hover: %v", err)
	}
	if hover.Contents.Value != "**func main()**" {
		t.Errorf("hover contents = %q", hover.Contents.Value)
	}
}

func TestClient_Definition(t *testing.T) {
	client, cleanup := startMockLSPServer(t)
	defer cleanup()

	params := map[string]any{
		"textDocument": map[string]any{"uri": "file:///main.go"},
		"position":     map[string]any{"line": 10, "character": 5},
	}

	result, err := client.call("textDocument/definition", params)
	if err != nil {
		t.Fatalf("definition error: %v", err)
	}

	var loc Location
	if err := json.Unmarshal(result, &loc); err != nil {
		t.Fatalf("unmarshal location: %v", err)
	}
	if loc.URI != "file:///main.go" {
		t.Errorf("URI = %q", loc.URI)
	}
	if loc.Range.Start.Line != 5 {
		t.Errorf("Range.Start.Line = %d, want 5", loc.Range.Start.Line)
	}
}

func TestClient_Shutdown(t *testing.T) {
	client, cleanup := startMockLSPServer(t)
	defer cleanup()

	params := struct{}{}
	_, err := client.call("shutdown", params)
	if err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestClient_Close(t *testing.T) {
	client, cleanup := startMockLSPServer(t)
	defer cleanup()

	// 关闭不应 panic
	if err := client.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}

	// 二次关闭也不应 panic
	_ = client.Close()
}

func TestClient_FilterEnv(t *testing.T) {
	env := filterEnvForSubprocess()
	// 应包含 PATH
	hasPath := false
	for _, e := range env {
		if len(e) > 5 && e[:5] == "PATH=" {
			hasPath = true
			break
		}
	}
	if !hasPath {
		t.Error("filtered env should contain PATH")
	}

	// 不应包含 API key
	for _, e := range env {
		if len(e) > 9 && e[:9] == "API_KEY=" {
			t.Error("filtered env should not contain API_KEY")
		}
	}
}

func TestClient_Timeout(t *testing.T) {
	// 测试 close 后 call 应返回 disconnect 错误
	client, cleanup := startMockLSPServer(t)
	defer cleanup()

	// 正常调用应成功
	params := map[string]any{
		"textDocument": map[string]any{"uri": "file:///main.go"},
		"position":     map[string]any{"line": 5, "character": 0},
	}
	_, err := client.call("textDocument/hover", params)
	if err != nil {
		t.Fatalf("hover before close: %v", err)
	}

	// 关闭后调用应失败
	client.Close()
	time.Sleep(100 * time.Millisecond)

	_, err = client.call("textDocument/hover", params)
	if err == nil {
		t.Error("expected error after close")
	}
}
