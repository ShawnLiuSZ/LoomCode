package memory

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Entry 记忆条目
type Entry struct {
	ID        string    `json:"id"`
	Layer     Layer     `json:"layer"`
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Layer 记忆层级
type Layer int

const (
	LayerCheckpoint Layer = iota // 会话级检查点
	LayerProject                 // 项目级 MEMORY.md
	LayerGlobal                  // 跨项目用户偏好
	LayerHistory                 // SQLite 原始对话
)

func (l Layer) String() string {
	switch l {
	case LayerCheckpoint:
		return "checkpoint"
	case LayerProject:
		return "project"
	case LayerGlobal:
		return "global"
	case LayerHistory:
		return "history"
	default:
		return "unknown"
	}
}

// Store SQLite FTS5 记忆存储
type Store struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

// NewStore 创建记忆存储
func NewStore(path string) (*Store, error) {
	// H24 修复：默认 ":memory:" 在连接池中每个连接是独立的数据库，
	// 导致写入在一个连接、查询在另一个连接时读不到数据。
	// 对内存库使用 cache=shared 并限制单连接，保证所有操作落在同一个共享内存库。
	openPath := path
	if path == ":memory:" {
		openPath = "file::memory:?cache=shared"
	}

	db, err := sql.Open("sqlite", openPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// 内存共享库必须限制单连接，否则不同连接仍可能拿到独立实例。
	if path == ":memory:" {
		db.SetMaxOpenConns(1)
	}

	// 启用 busy_timeout（内存库无需 WAL；WAL 对 :memory: 无意义且可能报错）
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	store := &Store{db: db, path: path}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

// NewMemoryStore 创建内存存储（测试用）
func NewMemoryStore() (*Store, error) {
	return NewStore(":memory:")
}

// Close 关闭存储
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate 创建表结构
func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			layer INTEGER NOT NULL,
			key TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			key, content, content=memories, content_rowid=rowid
		)`,
		`CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content);
			INSERT INTO memories_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content);
		END`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q, err)
		}
	}

	return nil
}

// Save 保存记忆条目
func (s *Store) Save(entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO memories (id, layer, key, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Layer, entry.Key, entry.Content, entry.CreatedAt, entry.UpdatedAt,
	)
	return err
}

// Get 获取记忆条目
func (s *Store) Get(id string) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(
		`SELECT id, layer, key, content, created_at, updated_at FROM memories WHERE id = ?`, id,
	)

	var e Entry
	err := row.Scan(&e.ID, &e.Layer, &e.Key, &e.Content, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// List 按层级列出记忆
func (s *Store) List(layer Layer) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, layer, key, content, created_at, updated_at
		 FROM memories WHERE layer = ? ORDER BY updated_at DESC`, layer,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanEntries(rows)
}

// ListAll 列出所有记忆
func (s *Store) ListAll() ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, layer, key, content, created_at, updated_at
		 FROM memories ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanEntries(rows)
}

// Search 全文搜索
func (s *Store) Search(query string, limit int) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT m.id, m.layer, m.key, m.content, m.created_at, m.updated_at
		 FROM memories m
		 JOIN memories_fts fts ON m.rowid = fts.rowid
		 WHERE memories_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`, query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanEntries(rows)
}

// Delete 删除记忆
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	return err
}

// DeleteByLayer 按层级删除
func (s *Store) DeleteByLayer(layer Layer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM memories WHERE layer = ?`, layer)
	return err
}

// UpsertByKey 按 key 更新或插入（同层级）
func (s *Store) UpsertByKey(layer Layer, key, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("%s_%s", layer.String(), key)

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO memories (id, layer, key, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, COALESCE((SELECT created_at FROM memories WHERE id = ?), ?), ?)`,
		id, layer, key, content, id, now, now,
	)
	return err
}

// Count 返回记忆总数
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count); err != nil {
		return 0
	}
	return count
}

// scanEntries 扫描行到 Entry 列表
func scanEntries(rows *sql.Rows) ([]*Entry, error) {
	var entries []*Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Layer, &e.Key, &e.Content, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}
