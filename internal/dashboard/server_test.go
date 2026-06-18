package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRoutes(t *testing.T) {
	s := NewServer(":0")

	tests := []struct {
		path   string
		method string
	}{
		{"/", "GET"},
		{"/api/sessions", "GET"},
		{"/api/cost", "GET"},
		{"/api/status", "GET"},
		{"/ws", "GET"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)

		if w.Code == http.StatusMethodNotAllowed {
			t.Errorf("Route %s %s not found", tt.method, tt.path)
		}
	}
}
