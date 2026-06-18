package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client LSP 客户端
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu       sync.Mutex
	nextID   atomic.Int64
	serverInfo *ServerInfo
}

// ServerInfo 服务器信息
type ServerInfo struct {
	Name    string
	Version string
}

// NewClient 创建 LSP 客户端
func NewClient(command string, args ...string) *Client {
	return &Client{
		cmd: exec.Command(command, args...),
	}
}

// Connect 启动 LSP 服务器并初始化
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

	// 发送 initialize 请求
	params := InitializeParams{
		ProcessID:  nil,
		RootURI:    nil,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCaps{
				Completion:       &CompletionCaps{DynamicRegistration: false},
				Hover:            &HoverCaps{DynamicRegistration: false},
				Definition:       &DefinitionCaps{DynamicRegistration: false},
				DocumentSymbol:   &DocumentSymbolCaps{DynamicRegistration: false},
				CodeAction:       &CodeActionCaps{DynamicRegistration: false},
			},
		},
	}

	result, err := c.call("initialize", params)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parse init result: %w", err)
	}

	c.serverInfo = &ServerInfo{
		Name:    initResult.ServerInfo.Name,
		Version: initResult.ServerInfo.Version,
	}

	// 发送 initialized 通知
	c.notify("initialized", &InitializedParams{})

	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	// 发送 shutdown
	c.call("shutdown", nil)
	// 发送 exit 通知
	c.notify("exit", nil)

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// DidOpen 通知服务器文件已打开
func (c *Client) DidOpen(uri, languageID, text string) {
	c.notify("textDocument/didOpen", DidOpenParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	})
}

// DidChange 通知服务器文件已变更
func (c *Client) DidChange(uri string, version int, changes []TextDocumentContentChangeEvent) {
	c.notify("textDocument/didChange", DidChangeParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: changes,
	})
}

// Completion 请求代码补全
func (c *Client) Completion(uri string, line, character int) ([]CompletionItem, error) {
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	result, err := c.call("textDocument/completion", params)
	if err != nil {
		return nil, err
	}

	var completionResult struct {
		Items []CompletionItem `json:"items"`
		IsIncomplete bool      `json:"isIncomplete"`
	}
	if err := json.Unmarshal(result, &completionResult); err != nil {
		return nil, fmt.Errorf("parse completion: %w", err)
	}

	return completionResult.Items, nil
}

// Hover 请求悬停信息
func (c *Client) Hover(uri string, line, character int) (*Hover, error) {
	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	result, err := c.call("textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	var hover Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, fmt.Errorf("parse hover: %w", err)
	}

	return &hover, nil
}

// Definition 请求跳转定义
func (c *Client) Definition(uri string, line, character int) ([]Location, error) {
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	result, err := c.call("textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("parse definition: %w", err)
	}

	return locations, nil
}

// DocumentSymbol 请求文档符号
func (c *Client) DocumentSymbol(uri string) ([]DocumentSymbol, error) {
	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	result, err := c.call("textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []DocumentSymbol
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, fmt.Errorf("parse symbols: %w", err)
	}

	return symbols, nil
}

// call 发送请求并等待响应
func (c *Client) call(method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	req := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}

	if params != nil {
		data, _ := json.Marshal(params)
		req.Params = data
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 发送
	reqData, _ := json.Marshal(req)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(reqData))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return nil, err
	}
	if _, err := c.stdin.Write(reqData); err != nil {
		return nil, err
	}

	// 读取响应（LSP 使用 Content-Length 头部）
	headerLine, err := c.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var contentLength int
	fmt.Sscanf(headerLine, "Content-Length: %d", &contentLength)

	// 跳过空行
	c.stdout.ReadString('\n')

	// 读取 body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *LSPError       `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp.Result, nil
}

// notify 发送通知（无响应）
func (c *Client) notify(method string, params any) {
	data, _ := json.Marshal(params)
	notif := struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
		Params:  data,
	}

	notifData, _ := json.Marshal(notif)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(notifData))
	c.stdin.Write([]byte(header))
	c.stdin.Write(notifData)
}
