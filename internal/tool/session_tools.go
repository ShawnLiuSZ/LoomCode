package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ShawnLiuSZ/loomcode/internal/session"
)

// SessionManagerProvider 为会话工具提供只读访问能力。
// 由上层（CLI）注入，避免工具包直接持有会话管理器的生命周期。
type SessionManagerProvider interface {
	// List 返回所有会话元数据（按创建时间倒序）。
	List() []*session.Session
	// SessionWithMessages 返回指定 ID 的完整会话（含消息）。
	SessionWithMessages(id string) (*session.Session, error)
}

// ListSessionsTool 列出历史会话摘要。
type ListSessionsTool struct {
	mgr SessionManagerProvider
}

// SetSessionManager 注入会话管理器；传入 nil 表示清除来源。
func (t *ListSessionsTool) SetSessionManager(mgr SessionManagerProvider) {
	t.mgr = mgr
}

func (t *ListSessionsTool) Name() string     { return "list_sessions" }
func (t *ListSessionsTool) IsReadOnly() bool { return true }

func (t *ListSessionsTool) Description() string {
	return "List historical chat sessions with summaries. Useful when you want to recall prior tasks or reuse context from earlier conversations."
}

func (t *ListSessionsTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"limit": {
				Type:        "integer",
				Description: "Maximum number of sessions to return (default: 20).",
			},
		},
	}
}

func (t *ListSessionsTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	if t.mgr == nil {
		return &Result{Content: "No session manager configured."}, nil
	}

	limit := 20
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	sessions := t.mgr.List()
	if len(sessions) == 0 {
		return &Result{Content: "No historical sessions found."}, nil
	}
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	type summary struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Model     string    `json:"model"`
		Provider  string    `json:"provider"`
	}

	items := make([]summary, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, summary{
			ID:        s.ID,
			Name:      s.Name,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
			Model:     s.Model,
			Provider:  s.Provider,
		})
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sessions: %w", err)
	}
	return &Result{Content: string(data)}, nil
}

// ReadSessionTool 读取指定历史会话的摘要与最近消息。
type ReadSessionTool struct {
	mgr SessionManagerProvider
}

// SetSessionManager 注入会话管理器；传入 nil 表示清除来源。
func (t *ReadSessionTool) SetSessionManager(mgr SessionManagerProvider) {
	t.mgr = mgr
}

func (t *ReadSessionTool) Name() string     { return "read_session" }
func (t *ReadSessionTool) IsReadOnly() bool { return true }

func (t *ReadSessionTool) Description() string {
	return "Read the summary and recent messages of a specific historical session. Use this to learn from prior conversations."
}

func (t *ReadSessionTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"session_id": {
				Type:        "string",
				Description: "The ID of the session to read.",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of recent messages to include (default: 50).",
			},
		},
		Required: []string{"session_id"},
	}
}

func (t *ReadSessionTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	if t.mgr == nil {
		return &Result{Content: "No session manager configured."}, nil
	}

	id, ok := args["session_id"].(string)
	if !ok || id == "" {
		return &Result{Content: "Missing required argument: session_id."}, nil
	}

	limit := 50
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	s, err := t.mgr.SessionWithMessages(id)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Failed to read session: %v", err)}, nil
	}

	messages := s.Messages
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	// 对消息做轻量摘要：保留角色、工具名与内容前 500 字符，避免结果过大。
	type msgSummary struct {
		Timestamp time.Time `json:"timestamp,omitempty"`
		Role      string    `json:"role"`
		ToolName  string    `json:"tool_name,omitempty"`
		Content   string    `json:"content"`
	}

	items := make([]msgSummary, 0, len(messages))
	for _, m := range messages {
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		items = append(items, msgSummary{
			Timestamp: m.Timestamp,
			Role:      m.Role,
			ToolName:  m.ToolName,
			Content:   content,
		})
	}

	type output struct {
		ID        string       `json:"id"`
		Name      string       `json:"name"`
		CreatedAt time.Time    `json:"created_at"`
		UpdatedAt time.Time    `json:"updated_at"`
		Model     string       `json:"model"`
		Provider  string       `json:"provider"`
		Messages  []msgSummary `json:"messages"`
	}

	data, err := json.MarshalIndent(output{
		ID:        s.ID,
		Name:      s.Name,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Model:     s.Model,
		Provider:  s.Provider,
		Messages:  items,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal session: %w", err)
	}
	return &Result{Content: string(data)}, nil
}

// SetSessionManagerForTools 向已注册的工具注入会话管理器。
// 由于 list_sessions/read_session 默认注册为占位工具，需在 session manager 创建后调用本函数注入。
func SetSessionManagerForTools(r *Registry, mgr SessionManagerProvider) {
	if t, ok := r.Get("list_sessions"); ok {
		if st, ok := t.(*ListSessionsTool); ok {
			st.SetSessionManager(mgr)
		}
	}
	if t, ok := r.Get("read_session"); ok {
		if st, ok := t.(*ReadSessionTool); ok {
			st.SetSessionManager(mgr)
		}
	}
}


