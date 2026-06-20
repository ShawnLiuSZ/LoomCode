package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Server Dashboard HTTP 服务器
type Server struct {
	addr   string
	mux    *http.ServeMux
}

// NewServer 创建 Dashboard 服务器
func NewServer(addr string) *Server {
	// 默认绑定 127.0.0.1（安全）
	if addr == "" || addr == ":8080" {
		addr = "127.0.0.1:8080"
	}
	// 如果只指定了端口（如 ":9090"），补上 127.0.0.1
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	s := &Server{
		addr: addr,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s
}

// routes 注册路由
func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/cost", s.handleCost)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/ws", s.handleWebSocket)
}

// Start 启动服务器（带超时配置）
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         s.addr,
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("Dashboard running at http://%s\n", s.addr)
	fmt.Println("注意：Dashboard 当前返回模拟数据（Mockup），仅用于 UI 原型展示。")

	// 优雅关闭
	go func() {
		<-context.Background().Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	return srv.ListenAndServe()
}
