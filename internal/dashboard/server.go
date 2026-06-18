package dashboard

import (
	"fmt"
	"net/http"
)

// Server Dashboard HTTP 服务器
type Server struct {
	addr   string
	mux    *http.ServeMux
}

// NewServer 创建 Dashboard 服务器
func NewServer(addr string) *Server {
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

// Start 启动服务器
func (s *Server) Start() error {
	fmt.Printf("Dashboard running at http://localhost%s\n", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}
