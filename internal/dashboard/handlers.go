// Package dashboard 提供 Web Dashboard 服务。
//
// 注意：当前所有接口返回硬编码的模拟数据（Mockup），
// 用于展示 UI 原型。生产环境需接入真实的 session/cost 数据。
package dashboard

import (
	"encoding/json"
	"log"
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
	if _, err := w.Write(data); err != nil {
		log.Printf("dashboard: write response: %v", err)
	}
}

// handleSessions 处理会话列表（Mockup）
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions := []map[string]interface{}{
		{"id": "1", "name": "Session 1", "messages": 10, "_mockup": true},
		{"id": "2", "name": "Session 2", "messages": 5, "_mockup": true},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sessions); err != nil {
		log.Printf("dashboard: encode sessions: %v", err)
	}
}

// handleCost 处理成本统计（Mockup）
func (s *Server) handleCost(w http.ResponseWriter, r *http.Request) {
	cost := map[string]interface{}{
		"total":   0.12,
		"today":   0.03,
		"history": []float64{0.01, 0.02, 0.03, 0.02, 0.04},
		"_mockup": true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cost); err != nil {
		log.Printf("dashboard: encode cost: %v", err)
	}
}

// handleStatus 处理 Provider 状态（Mockup）
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"deepseek": "connected",
		"mimo":     "connected",
		"openai":   "disconnected",
		"_mockup":  true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("dashboard: encode status: %v", err)
	}
}
