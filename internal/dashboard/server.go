package dashboard

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

// Server Dashboard HTTP 服务器
type Server struct {
	addr      string
	mux       *http.ServeMux
	wsHub     *WSHub
	authToken string
}

// NewServer 创建 Dashboard 服务器（含随机 auth token）。
func NewServer(addr string) *Server {
	token := generateAuthToken()
	s := &Server{
		addr:      loopbackAddr(addr),
		mux:       http.NewServeMux(),
		wsHub:     NewWSHub(),
		authToken: token,
	}
	s.routes()
	return s
}

// AuthToken 返回启动时生成的认证 token，供外部获取并传给浏览器。
func (s *Server) AuthToken() string {
	return s.authToken
}

// generateAuthToken 生成 32 字节随机 hex token。
func generateAuthToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("dashboard: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// loopbackAddr 强制将监听地址绑定到回环网卡（禁止暴露到全网卡）。
// 非回环 host（如 0.0.0.0、局域网 IP）会被改回 127.0.0.1，仅保留端口。
func loopbackAddr(addr string) string {
	if addr == "" {
		return "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "127.0.0.1:8080"
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return net.JoinHostPort(host, port)
	default:
		return net.JoinHostPort("127.0.0.1", port)
	}
}

// routes 注册路由
func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/sessions", s.requireAuth(s.handleSessions))
	s.mux.HandleFunc("/api/cost", s.requireAuth(s.handleCost))
	s.mux.HandleFunc("/api/status", s.requireAuth(s.handleStatus))
	s.mux.HandleFunc("/ws", s.handleWebSocket)
}

// requireAuth 是中间件：检查 query param token 是否匹配服务端生成的 authToken。
// 不匹配时返回 403；匹配时放行。首页 / 不需要 auth（由前端 JS 附加 token）。
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isValidToken(r) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// isValidToken 校验请求中的 token：优先读 query param ?token=xxx，其次读 Authorization: Bearer xxx。
// 5.1 修复：使用 subtle.ConstantTimeCompare 防止时序攻击（原 == 比较会因匹配长度差异泄露 token 前缀）。
func (s *Server) isValidToken(r *http.Request) bool {
	if t := r.URL.Query().Get("token"); t != "" {
		return subtle.ConstantTimeCompare([]byte(t), []byte(s.authToken)) == 1
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		if len(auth) > 7 && auth[:7] == "Bearer " {
			return subtle.ConstantTimeCompare([]byte(auth[7:]), []byte(s.authToken)) == 1
		}
	}
	return false
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

	fmt.Printf("Dashboard running at http://%s?token=%s\n", s.addr, s.authToken)
	fmt.Println("注意：Dashboard 当前返回模拟数据（Mockup），仅用于 UI 原型展示。")

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("dashboard shutdown: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
