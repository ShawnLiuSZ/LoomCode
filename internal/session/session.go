package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Message 会话消息
type Message struct {
	Timestamp time.Time `json:"timestamp"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
}

// Meta 会话元数据
type Meta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Model     string    `json:"model"`
	Provider  string    `json:"provider"`
}

// Session 会话
type Session struct {
	Meta
	Messages []Message `json:"-"`

	mu       sync.Mutex
	filePath string
}

// Manager 会话管理器
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	activeID string
	baseDir  string
}

// NewManager 创建会话管理器
func NewManager(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	m := &Manager{
		sessions: make(map[string]*Session),
		baseDir:  baseDir,
	}

	m.loadSessions()

	return m, nil
}

// Create 创建新会话
func (m *Manager) Create(name, model, provider string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("session_%d", now.UnixMilli())

	s := &Session{
		Meta: Meta{
			ID:        id,
			Name:      name,
			CreatedAt: now,
			UpdatedAt: now,
			Model:     model,
			Provider:  provider,
		},
		Messages: []Message{},
		filePath: filepath.Join(m.baseDir, id+".jsonl"),
	}

	m.sessions[id] = s
	m.activeID = id

	if err := s.saveMeta(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: save session meta %s: %v\n", id, err)
	}

	return s
}

// Get 获取会话
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Active 返回当前活跃会话
func (m *Manager) Active() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[m.activeID]
}

// SetActive 设置活跃会话
func (m *Manager) SetActive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session %q not found", id)
	}
	m.activeID = id
	return nil
}

// List 列出所有会话（按创建时间倒序）
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// MostRecent 返回最近更新的会话（用于默认启动时恢复上次模型选择）。
// 无会话时返回 nil。
func (m *Manager) MostRecent() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var recent *Session
	for _, s := range m.sessions {
		if recent == nil || s.UpdatedAt.After(recent.UpdatedAt) {
			recent = s
		}
	}
	return recent
}

// Delete 删除会话
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}

	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete session file: %w", err)
	}

	delete(m.sessions, id)
	if m.activeID == id {
		m.activeID = ""
	}
	return nil
}

// AddMessage 添加消息到活跃会话
func (m *Manager) AddMessage(msg Message) {
	m.mu.Lock()
	s := m.sessions[m.activeID]
	m.mu.Unlock()

	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	msg.Timestamp = time.Now()
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()

	m.appendToFile(s, msg)
}

// Save 持久化会话
func (m *Manager) Save(id string) error {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session %q not found", id)
	}

	return s.saveAll()
}

// UpdateModelProvider 更新活动会话的模型与 provider，并持久化 meta。
// 用于 /model 切换后让下次启动恢复上次选择的模型。
func (m *Manager) UpdateModelProvider(model, provider string) error {
	m.mu.Lock()
	s := m.sessions[m.activeID]
	m.mu.Unlock()

	if s == nil {
		return fmt.Errorf("no active session")
	}

	s.mu.Lock()
	s.Model = model
	s.Provider = provider
	s.UpdatedAt = time.Now()
	s.mu.Unlock()

	return s.saveMeta()
}

func (s *Session) saveMeta() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmpPath := s.filePath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(tmpPath)
	}()

	encoder := json.NewEncoder(f)
	if err := encoder.Encode(s.Meta); err != nil {
		return err
	}
	f.Close()

	return os.Rename(tmpPath, s.filePath)
}

// saveAll 完整保存（元数据 + 所有消息）
func (s *Session) saveAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmpPath := s.filePath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(tmpPath)
	}()

	encoder := json.NewEncoder(f)

	if err := encoder.Encode(s.Meta); err != nil {
		return err
	}

	for _, msg := range s.Messages {
		if err := encoder.Encode(msg); err != nil {
			return err
		}
	}

	f.Close()

	return os.Rename(tmpPath, s.filePath)
}

// appendToFile 追加消息
func (m *Manager) appendToFile(s *Session, msg Message) {
	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: open session file %s: %v\n", s.filePath, err)
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	if err := encoder.Encode(msg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: encode message to %s: %v\n", s.filePath, err)
	}
}

// loadSessions 从磁盘加载
func (m *Manager) loadSessions() {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		path := filepath.Join(m.baseDir, entry.Name())
		s, err := loadFromFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: load session %s: %v\n", path, err)
			continue
		}
		if s == nil {
			continue
		}

		m.sessions[s.ID] = s
	}
}

// loadFromFile 从文件加载
func loadFromFile(path string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)

	// 第一行：元数据
	var meta Meta
	if err := decoder.Decode(&meta); err != nil {
		return nil, fmt.Errorf("decode meta: %w", err)
	}

	// 后续行：消息
	var messages []Message
	lineNum := 1
	for decoder.More() {
		lineNum++
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s line %d: %v\n", path, lineNum, err)
			break
		}
		messages = append(messages, msg)
	}

	return &Session{
		Meta:     meta,
		Messages: messages,
		filePath: path,
	}, nil
}

// Count 返回会话数量
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
