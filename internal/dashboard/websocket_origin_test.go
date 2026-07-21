package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAllowedOrigin_ExactHost(t *testing.T) {
	s := &Server{}
	// remoteAddr 用一个 loopback 地址（仅对空 Origin 分支有影响）。
	const remote = "127.0.0.1:54321"
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost", true},
		{"http://localhost:8080", true},
		{"http://127.0.0.1", true},
		{"http://127.0.0.1:3000", true},
		{"https://localhost", true},
		{"http://localhost.evil.com", false},
		{"http://127.0.0.1.attacker.com", false},
		{"http://localhostXSS", false},
		{"http://evil.com", false},
		{"http://notlocalhost", false},
		{"ftp://localhost", false},
		{"garbage", false},
	}
	for _, c := range cases {
		if got := s.isAllowedOrigin(c.origin, remote); got != c.want {
			t.Errorf("isAllowedOrigin(%q) = %v, want %v", c.origin, got, c.want)
		}
	}
}

// TestIsAllowedOrigin_EmptyOriginLoopback #5 修复：空 Origin 对 loopback 放行，非 loopback 拒绝。
func TestIsAllowedOrigin_EmptyOriginLoopback(t *testing.T) {
	s := &Server{}
	cases := []struct {
		remote string
		want   bool
	}{
		{"127.0.0.1:54321", true},
		{"[::1]:54321", true},
		{"localhost:54321", true},
		{"192.168.1.1:54321", false},
		{"10.0.0.1:54321", false},
		{"", false}, // 无 RemoteAddr 也拒绝
	}
	for _, c := range cases {
		if got := s.isAllowedOrigin("", c.remote); got != c.want {
			t.Errorf("isAllowedOrigin(\"\", %q) = %v, want %v", c.remote, got, c.want)
		}
	}
}

func TestHandleWebSocket_RejectsMissingOrigin(t *testing.T) {
	s, err := NewServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cases := []struct {
		name       string
		setOrigin  bool
		origin     string
		remoteAddr string
		wantReject bool
	}{
		// #5 修复后：空 Origin + loopback 放行（不再拒绝）。
		{"missing Origin header from loopback", false, "", "127.0.0.1:54321", false},
		{"empty Origin header from loopback", true, "", "127.0.0.1:54321", false},
		// 空 Origin + 非 loopback 仍拒绝。
		{"missing Origin header from remote", false, "", "192.168.1.1:54321", true},
		{"empty Origin header from remote", true, "", "192.168.1.1:54321", true},
		// 非空 Origin 走原逻辑。
		{"disallowed Origin", true, "http://evil.com", "127.0.0.1:54321", true},
		{"prefix-bypass Origin", true, "http://localhost.evil.com", "127.0.0.1:54321", true},
		{"allowed Origin", true, "http://localhost:8080", "127.0.0.1:54321", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws?token="+s.AuthToken(), nil)
			req.RemoteAddr = c.remoteAddr
			if c.setOrigin {
				req.Header.Set("Origin", c.origin)
			}
			rec := httptest.NewRecorder()
			s.handleWebSocket(rec, req)
			rejected := rec.Code == http.StatusForbidden
			if rejected != c.wantReject {
				t.Errorf("status=%d rejected=%v, wantReject=%v", rec.Code, rejected, c.wantReject)
			}
		})
	}
}

func TestHandleWebSocket_RejectsMissingToken(t *testing.T) {
	s, err := NewServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	s.handleWebSocket(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("WebSocket without token: status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
