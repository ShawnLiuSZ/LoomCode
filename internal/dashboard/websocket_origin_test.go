package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAllowedOrigin_ExactHost(t *testing.T) {
	s := &Server{}
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
		if got := s.isAllowedOrigin(c.origin); got != c.want {
			t.Errorf("isAllowedOrigin(%q) = %v, want %v", c.origin, got, c.want)
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
		wantReject bool
	}{
		{"missing Origin header", false, "", true},
		{"empty Origin header", true, "", true},
		{"disallowed Origin", true, "http://evil.com", true},
		{"prefix-bypass Origin", true, "http://localhost.evil.com", true},
		{"allowed Origin", true, "http://localhost:8080", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws?token="+s.AuthToken(), nil)
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
	rec := httptest.NewRecorder()
	s.handleWebSocket(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("WebSocket without token: status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
