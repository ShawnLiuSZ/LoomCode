package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRoutes(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

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

func TestHandleIndex(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleIndex status = %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/html" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	for _, path := range []string{"/api/sessions", "/api/cost", "/api/status"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("%s without token: status = %d, want %d", path, w.Code, http.StatusForbidden)
		}
	}
}

func TestAPIWithValidToken(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/sessions?token="+s.AuthToken(), nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleSessions with valid token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAPIWithBearerToken(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+s.AuthToken())
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleSessions with Bearer token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleSessions(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/sessions?token="+s.AuthToken(), nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleSessions status = %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var sessions []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("sessions count = %d, want 2", len(sessions))
	}
	if sessions[0]["name"] != "Session 1" {
		t.Errorf("session name = %q", sessions[0]["name"])
	}
}

func TestHandleCost(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/cost?token="+s.AuthToken(), nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleCost status = %d, want %d", w.Code, http.StatusOK)
	}

	var cost map[string]any
	if err := json.NewDecoder(w.Body).Decode(&cost); err != nil {
		t.Fatalf("decode cost: %v", err)
	}
	if cost["total"] != 0.12 {
		t.Errorf("total cost = %v, want 0.12", cost["total"])
	}
}

func TestHandleStatus(t *testing.T) {
	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/status?token="+s.AuthToken(), nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleStatus status = %d, want %d", w.Code, http.StatusOK)
	}

	var status map[string]any
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status["deepseek"] != "connected" {
		t.Errorf("deepseek status = %v", status["deepseek"])
	}
}
