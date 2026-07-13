package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client LSP 客户端
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu         sync.Mutex
	nextID     atomic.Int64
	serverInfo *ServerInfo

	// 响应路由
	pendingMu sync.Mutex
	pending   map[int64]chan json.RawMessage
	done      chan struct{}

	// BUG-002 修复：生命周期 context，Close/异常退出时取消以杀掉子进程，避免孤儿进程。
	lifeCtx    context.Context
	lifeCancel context.CancelFunc
}

// ServerInfo 服务器信息
type ServerInfo struct {
	Name    string
	Version string
}

// NewClient 创建 LSP 客户端
func NewClient(command string, args ...string) *Client {
	// BUG-002 修复：用 CommandContext 绑定生命周期，Close 时取消 context 即可杀掉子进程，
	// 覆盖 panic/os.Exit 等无法正常调用 Process.Kill 的场景。
	lifeCtx, lifeCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(lifeCtx, command, args...)
	cmd.Env = filterEnvForSubprocess()
	return &Client{
		cmd:        cmd,
		pending:    make(map[int64]chan json.RawMessage),
		done:       make(chan struct{}),
		lifeCtx:    lifeCtx,
		lifeCancel: lifeCancel,
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

	// 启动后台 reader goroutine
	go c.readLoop()

	// 发送初始化请求
	params := map[string]any{
		"processId": os.Getpid(),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{},
			},
		},
	}
	result, err := c.call("initialize", params)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var info struct {
		ServerInfo *ServerInfo `json:"serverInfo"`
	}
	json.Unmarshal(result, &info) //nolint:errcheck // serverInfo is optional
	if info.ServerInfo != nil {
		c.serverInfo = info.ServerInfo
	}

	// 发送 initialized 通知
	c.sendNotification("initialized", nil)

	return nil
}

// readLoop 后台读取循环，按 ID 路由响应
func (c *Client) readLoop() {
	defer close(c.done)
	for {
		// L15 修复：LSP 帧格式为若干 header 行 + 一个空行 + body。
		// 必须循环读取 header 行直到遇到空行，而不能只读一行就假定下一行是空行，
		// 否则在 Content-Length 之外存在其它 header（如 Content-Type）时会错位解析。
		var contentLength int
		for {
			headerLine, err := c.stdout.ReadString('\n')
			if err != nil {
				return
			}
			// 去掉行尾 \r\n / \n，空行（仅 \r 或空）表示 header 结束。
			trimmed := strings.TrimRight(headerLine, "\r\n")
			if trimmed == "" {
				break
			}
			// 解析 Content-Length，继续读完剩余 header 行直到空行。
			_, _ = fmt.Sscanf(trimmed, "Content-Length: %d", &contentLength)
		}
		if contentLength <= 0 {
			continue
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(c.stdout, body); err != nil {
			return
		}

		var msg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  *LSPError       `json:"error,omitempty"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}

		// 响应（有 ID）
		if msg.ID != nil {
			c.pendingMu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.pendingMu.Unlock()

			if ok {
				if msg.Error != nil {
					ch <- nil
				} else {
					ch <- msg.Result
				}
			}
		}
		// 通知（无 ID）暂不处理
	}
}

// call 发送请求并等待响应（带超时）
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

	// 注册 pending 请求
	respCh := make(chan json.RawMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	// 写入请求（需要锁保护 stdin）
	c.mu.Lock()
	reqData, _ := json.Marshal(req)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(reqData))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		c.mu.Unlock()
		c.cancelPending(id)
		return nil, err
	}
	if _, err := c.stdin.Write(reqData); err != nil {
		c.mu.Unlock()
		c.cancelPending(id)
		return nil, err
	}
	c.mu.Unlock()

	// 等待响应（30 秒超时）
	select {
	case result := <-respCh:
		if result == nil {
			return nil, fmt.Errorf("server returned error")
		}
		return result, nil
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

// sendNotification 发送通知（无响应）。
// 必须使用 LSP 的 Content-Length 分帧（与 call 一致），不能用裸换行——否则会打乱服务端解析。
func (c *Client) sendNotification(method string, params any) {
	notif := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			log.Printf("lsp sendNotification %q: marshal params failed: %v", method, err)
			return
		}
		notif.Params = data
	}

	data, err := json.Marshal(notif)
	if err != nil {
		log.Printf("lsp sendNotification %q: marshal notif failed: %v", method, err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		log.Printf("lsp sendNotification %q: write header failed: %v", method, err)
		return
	}
	if _, err := c.stdin.Write(data); err != nil {
		log.Printf("lsp sendNotification %q: write body failed: %v", method, err)
	}
}

// Close 关闭客户端
// BUG-002 修复：取消生命周期 context 触发 CommandContext 杀掉子进程（与 Process.Kill 双保险），
// 并保证可安全重复调用（lifeCancel 置 nil 后跳过）。
func (c *Client) Close() error {
	if c.lifeCancel != nil {
		c.lifeCancel()
		c.lifeCancel = nil
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	return nil
}

// ServerInfo 返回服务器信息
func (c *Client) GetServerInfo() *ServerInfo {
	return c.serverInfo
}

// Notification JSON-RPC 通知（无 ID）
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

const jsonrpcVersion = "2.0"

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
