package memory

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// historyCounter 保证高频保存时 ID 唯一。
var historyCounter int64

// Manager 四层记忆管理器
type Manager struct {
	store *Store
}

// NewManager 创建记忆管理器
func NewManager(store *Store) *Manager {
	return &Manager{store: store}
}

// Store 返回底层存储
func (m *Manager) Store() *Store {
	return m.store
}

// SaveCheckpoint 保存会话检查点
func (m *Manager) SaveCheckpoint(sessionID string, content string) error {
	return m.store.UpsertByKey(LayerCheckpoint, sessionID, content)
}

// GetCheckpoint 获取会话检查点
func (m *Manager) GetCheckpoint(sessionID string) (string, error) {
	id := fmt.Sprintf("checkpoint_%s", sessionID)
	entry, err := m.store.Get(id)
	if err != nil {
		return "", err
	}
	return entry.Content, nil
}

// SaveProjectMemory 保存项目记忆
func (m *Manager) SaveProjectMemory(key, content string) error {
	return m.store.UpsertByKey(LayerProject, key, content)
}

// GetProjectMemory 获取项目记忆
func (m *Manager) GetProjectMemory(key string) (string, error) {
	id := fmt.Sprintf("project_%s", key)
	entry, err := m.store.Get(id)
	if err != nil {
		return "", err
	}
	return entry.Content, nil
}

// ListProjectMemories 列出所有项目记忆
func (m *Manager) ListProjectMemories() ([]*Entry, error) {
	return m.store.List(LayerProject)
}

// SaveGlobalPreference 保存全局偏好
func (m *Manager) SaveGlobalPreference(key, content string) error {
	return m.store.UpsertByKey(LayerGlobal, key, content)
}

// GetGlobalPreference 获取全局偏好
func (m *Manager) GetGlobalPreference(key string) (string, error) {
	id := fmt.Sprintf("global_%s", key)
	entry, err := m.store.Get(id)
	if err != nil {
		return "", err
	}
	return entry.Content, nil
}

// SaveHistory 保存对话历史
func (m *Manager) SaveHistory(role, content string) error {
	now := time.Now()
	n := atomic.AddInt64(&historyCounter, 1)
	id := fmt.Sprintf("history_%d_%d", now.UnixNano(), n)
	return m.store.Save(&Entry{
		ID:      id,
		Layer:   LayerHistory,
		Key:     role,
		Content: content,
	})
}

// SearchHistory 搜索对话历史
func (m *Manager) SearchHistory(query string, limit int) ([]*Entry, error) {
	return m.store.Search(query, limit)
}

// BuildContextPrompt 构建上下文提示（注入到 LLM 前缀）
func (m *Manager) BuildContextPrompt() string {
	var sb strings.Builder

	// 项目记忆
	projectMemories, err := m.store.List(LayerProject)
	if err == nil && len(projectMemories) > 0 {
		sb.WriteString("## Project Knowledge\n\n")
		for _, entry := range projectMemories {
			fmt.Fprintf(&sb, "- **%s**: %s\n", entry.Key, entry.Content)
		}
		sb.WriteString("\n")
	}

	// 全局偏好
	globalPrefs, err := m.store.List(LayerGlobal)
	if err == nil && len(globalPrefs) > 0 {
		sb.WriteString("## User Preferences\n\n")
		for _, entry := range globalPrefs {
			fmt.Fprintf(&sb, "- **%s**: %s\n", entry.Key, entry.Content)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Dream 从历史中提取知识（模拟实现）
func (m *Manager) Dream() ([]string, error) {
	// 搜索最近的对话，提取关键模式
	entries, err := m.store.Search("important remember note", 50)
	if err != nil {
		return nil, err
	}

	var insights []string
	seen := make(map[string]bool)

	for _, e := range entries {
		// 简单启发式：包含特定关键词的内容
		lower := strings.ToLower(e.Content)
		if strings.Contains(lower, "remember") ||
			strings.Contains(lower, "important") ||
			strings.Contains(lower, "note:") {
			if !seen[e.Content] {
				insights = append(insights, e.Content)
				seen[e.Content] = true
			}
		}
	}

	return insights, nil
}

// Distill 识别重复模式并打包（模拟实现）
func (m *Manager) Distill() ([]string, error) {
	entries, err := m.store.List(LayerHistory)
	if err != nil {
		return nil, err
	}

	// 统计工具使用频率
	toolCounts := make(map[string]int)
	for _, e := range entries {
		lower := strings.ToLower(e.Content)
		for _, tool := range []string{"read_file", "write_file", "bash", "grep", "glob"} {
			if strings.Contains(lower, tool) {
				toolCounts[tool]++
			}
		}
	}

	var patterns []string
	for tool, count := range toolCounts {
		if count >= 3 {
			patterns = append(patterns, fmt.Sprintf("Frequent tool: %s (used %d times)", tool, count))
		}
	}

	return patterns, nil
}
