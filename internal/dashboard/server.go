package dashboard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Server Dashboard HTTP 服务器
type Server struct {
	addr   string
	mux    *http.ServeMux
	wsHub  *WSHub
}

// NewServer 创建 Dashboard 服务器
func NewServer(addr string) *Server {
	s := &Server{
		addr:  loopbackAddr(addr),
		mux:   http.NewServeMux(),
		wsHub: NewWSHub(),
	}
	s.routes()
	return s
}

// loopbackAddr 强制将监听地址绑定到回环网卡（dashboard 无鉴权，禁止暴露到全网卡）。
// 非回环 host（如 0.0.0.0、局域网 IP）会被改回 127.0.0.1，仅保留端口。
func loopbackAddr(addr string) string {
	if addr == "" {
		return "127.0.0.1:8080"
	}
	// 统一用 SplitHostPort 解析（含 ":8080"、"[::1]:8080" 等形式）。
	// 解析失败（如裸 IPv6 "::1" 无端口、格式异常）一律回退到默认回环地址，避免拼出畸形地址。
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "127.0.0.1:8080"
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return net.JoinHostPort(host, port)
	default:
		// 空 host（":8080"）或任何非回环 host（0.0.0.0、局域网 IP、:: 等）→ 强制回环。
		return net.JoinHostPort("127.0.0.1", port)
	}
}

// routes 注册路由
func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/cost", s.handleCost)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/ws", s.handleWebSocket)
}

// Start 启动服务器（带超时配置）。ctx 取消时触发优雅关闭。
func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.addr,
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("Dashboard running at http://%s\n", s.addr)
	fmt.Println("注意：Dashboard 当前返回模拟数据（Mockup），仅用于 UI 原型展示。")

	// 优雅关闭：ctx 取消时 Shutdown，让 ListenAndServe 返回。
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
