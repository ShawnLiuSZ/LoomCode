package dashboard

import (
	"encoding/json"
	"net/http"
)

// handleIndex 处理首页
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

// handleSessions 处理会话列表
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions := []map[string]interface{}{
		{"id": "1", "name": "Session 1", "messages": 10},
		{"id": "2", "name": "Session 2", "messages": 5},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// handleCost 处理成本统计
func (s *Server) handleCost(w http.ResponseWriter, r *http.Request) {
	cost := map[string]interface{}{
		"total":   0.12,
		"today":   0.03,
		"history": []float64{0.01, 0.02, 0.03, 0.02, 0.04},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cost)
}

// handleStatus 处理 Provider 状态
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"deepseek": "connected",
		"mimo":     "connected",
		"openai":   "disconnected",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
