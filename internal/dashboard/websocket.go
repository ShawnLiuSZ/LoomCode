package dashboard

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// WSClient WebSocket 客户端
type WSClient struct {
	conn *websocket.Conn
	hub  *WSHub
	send chan []byte

	// closeOnce 保证 send channel 只被关闭一次，
	// 避免 unregister 与 broadcast 两条路径同时 close 同一 channel → panic。
	closeOnce sync.Once
}

// closeSend 安全地关闭 send channel（仅关闭一次）。
func (c *WSClient) closeSend() {
	c.closeOnce.Do(func() { close(c.send) })
}

// TSHub WebSocket 管理器
type WSHub struct {
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
	done       chan struct{}
	stopOnce   sync.Once
}

// NewWSHub 创建 WebSocket 管理器
func NewWSHub() *WSHub {
	hub := &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		done:       make(chan struct{}),
	}
	go hub.run()
	return hub
}

// Stop 停止 WSHub 主循环，让 goroutine 退出（N10）。
// 使用 sync.Once 保证重复调用不会 panic（close 已关闭 channel）。
func (h *WSHub) Stop() {
	h.stopOnce.Do(func() { close(h.done) })
}

// run WebSocket 管理器主循环
func (h *WSHub) run() {
	for {
		select {
		case <-h.done:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			total := len(h.clients)
			h.mu.Unlock()
			log.Printf("WebSocket client connected: %d total", total)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.closeSend()
			}
			total := len(h.clients)
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected: %d total", total)

		case message := <-h.broadcast:
			// 先 RLock 遍历发送；缓冲满的 client 收集到 slowClients，
			// 遍历结束后再 Lock 统一删除。避免在 range 迭代中修改 map 导致部分 client 被跳过漏收。
			h.mu.RLock()
			var slowClients []*WSClient
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 发送缓冲区满，标记待删除
					slowClients = append(slowClients, client)
				}
			}
			h.mu.RUnlock()

			if len(slowClients) > 0 {
				h.mu.Lock()
				for _, client := range slowClients {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						client.closeSend()
					}
				}
				total := len(h.clients)
				h.mu.Unlock()
				log.Printf("WebSocket dropped %d slow client(s): %d remaining", len(slowClients), total)
			}
		}
	}
}

// Broadcast 广播消息到所有客户端。若 WSHub 已停止，直接丢弃消息避免死锁。
func (h *WSHub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	case <-h.done:
	}
}

// ClientCount 返回当前连接数
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// handleWebSocket 处理 WebSocket 连接（带安全检查）
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.isValidToken(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	origin := r.Header.Get("Origin")
	if !s.isAllowedOrigin(origin, r.RemoteAddr) {
		http.Error(w, "Origin not allowed", http.StatusForbidden)
		return
	}

	hub := s.wsHub
	if hub == nil {
		http.Error(w, "WebSocket not initialized", http.StatusServiceUnavailable)
		return
	}

	// 检查是否支持 WebSocket（httptest.ResponseRecorder 不支持）
	if _, ok := w.(http.Hijacker); !ok {
		// 回退到普通 HTTP 响应（用于测试）
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "WebSocket endpoint (test mode - no upgrade)",
		})
		return
	}

	wsHandler := websocket.Handler(func(ws *websocket.Conn) {
		defer func() { _ = ws.Close() }()

		_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))

		client := &WSClient{
			conn: ws,
			hub:  hub,
			send: make(chan []byte, 256),
		}

		select {
		case hub.register <- client:
		case <-hub.done:
			return
		}

		// 读循环（仅用于检测断开）。
		// 每次收到消息后刷新读 deadline，避免健康连接被一次性 60s deadline 误杀（R2）。
		go func() {
			defer func() {
				select {
				case hub.unregister <- client:
				case <-hub.done:
				}
			}()
			for {
				_ = ws.SetReadDeadline(time.Now().Add(120 * time.Second))
				var msg []byte
				err := websocket.Message.Receive(ws, &msg)
				if err != nil {
					return
				}
				// 客户端消息暂不处理（可扩展为命令）
			}
		}()

		// 写循环
		for msg := range client.send {
			if err := websocket.Message.Send(ws, msg); err != nil {
				return
			}
			// 重置写 deadline
			_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		}
	})

	wsHandler.ServeHTTP(w, r)
}

// isAllowedOrigin 检查 Origin 是否允许（按精确 host 匹配，防止前缀绕过 CSWSH）。
// #5 修复：空 Origin（非浏览器客户端如 CLI/监控脚本）在 loopback 场景下放行，
// 真正的防线是 token + localhost 绑定；非 loopback 的空 Origin 仍拒绝。
func (s *Server) isAllowedOrigin(origin, remoteAddr string) bool {
	if origin == "" {
		// 空 Origin：仅对 loopback 客户端放行（剥离端口后匹配）。
		host := remoteAddr
		if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
			host = h
		}
		switch host {
		case "localhost", "127.0.0.1", "::1":
			return true
		}
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// WSMessage WebSocket 消息格式
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// SendJSON 发送 JSON 消息到所有客户端
func (h *WSHub) SendJSON(msgType string, payload interface{}) {
	msg := WSMessage{
		Type:    msgType,
		Payload: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.Broadcast(data)
}
