package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Checkpoint 记录一次文件修改前的快照。
type Checkpoint struct {
	ID           string    `json:"id"`            // 时间戳-based ID，全局唯一
	Timestamp    time.Time `json:"timestamp"`     // 快照时间
	OriginalPath string    `json:"original_path"` // 被修改的文件的绝对路径
	SnapshotPath string   `json:"snapshot_path"`  // 快照文件存储路径
	ToolName     string   `json:"tool_name"`      // 触发快照的工具名（write_file / edit_file）
	FileExisted  bool     `json:"file_existed"`   // 快照时文件是否存在（false=新建文件）
	FileSize     int64    `json:"file_size"`      // 快照时文件大小（字节）
}

// CheckpointManager 管理文件编辑快照，支持 /rewind 回退。
// 快照存储在 ~/.loomcode/checkpoints/ 下，每个快照为一个目录，
// 内含原始文件副本和元数据 JSON。
type CheckpointManager struct {
	baseDir    string // 快照根目录（~/.loomcode/checkpoints/）
	maxEntries int    // 最大快照数量，超出时删除最旧的
	mu         sync.Mutex
	idCounter  int64 // 同毫秒内的序号，保证 ID 唯一
}

// NewCheckpointManager 创建快照管理器。baseDir 为空时使用默认路径。
func NewCheckpointManager(baseDir string) *CheckpointManager {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.TempDir()
		}
		baseDir = filepath.Join(home, ".loomcode", "checkpoints")
	}
	cm := &CheckpointManager{
		baseDir:    baseDir,
		maxEntries: 100, // 默认保留最近 100 个快照
	}
	_ = os.MkdirAll(baseDir, 0700)
	return cm
}

// Snapshot 在文件被修改前创建快照。
// 如果文件不存在（新建文件场景），记录 FileExisted=false 且不写入快照内容。
// filePath 必须是绝对路径。
func (m *CheckpointManager) Snapshot(filePath, toolName string) (*Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	// 使用毫秒时间戳 + 原子计数器保证 ID 唯一（同毫秒内不冲突）
	seq := atomic.AddInt64(&m.idCounter, 1)
	id := fmt.Sprintf("%d_%03d", now.UnixMilli(), seq%1000)

	info, err := os.Stat(filePath)
	fileExisted := err == nil
	var fileSize int64
	if fileExisted {
		fileSize = info.Size()
	}

	cpDir := filepath.Join(m.baseDir, id)
	if err := os.MkdirAll(cpDir, 0700); err != nil {
		return nil, fmt.Errorf("create checkpoint dir: %w", err)
	}

	snapshotPath := filepath.Join(cpDir, "content")
	if fileExisted {
		data, err := os.ReadFile(filePath)
		if err != nil {
			os.RemoveAll(cpDir)
			return nil, fmt.Errorf("read file for snapshot: %w", err)
		}
		if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
			os.RemoveAll(cpDir)
			return nil, fmt.Errorf("write snapshot: %w", err)
		}
	} else {
		// 文件不存在（新建），写入空标记文件
		if err := os.WriteFile(snapshotPath, []byte{}, 0644); err != nil {
			os.RemoveAll(cpDir)
			return nil, fmt.Errorf("write empty snapshot: %w", err)
		}
	}

	cp := &Checkpoint{
		ID:           id,
		Timestamp:    now,
		OriginalPath: filePath,
		SnapshotPath: snapshotPath,
		ToolName:     toolName,
		FileExisted:  fileExisted,
		FileSize:     fileSize,
	}

	// 写入元数据
	metaPath := filepath.Join(cpDir, "meta.json")
	metaData, _ := json.MarshalIndent(cp, "", "  ")
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		// 元数据写入失败不影响快照本身，但记录告警
		fmt.Fprintf(os.Stderr, "warning: write checkpoint meta: %v\n", err)
	}

	// 清理超出上限的旧快照
	m.evictLocked()

	return cp, nil
}

// Restore 将指定快照的内容恢复到原始文件路径。
// 如果原始快照记录文件不存在（FileExisted=false），则删除当前文件（回退到"文件不存在"状态）。
func (m *CheckpointManager) Restore(checkpointID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp, err := m.loadMeta(checkpointID)
	if err != nil {
		return err
	}

	if !cp.FileExisted {
		// 原文件不存在 → 删除当前文件
		if err := os.Remove(cp.OriginalPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove file for restore: %w", err)
		}
		return nil
	}

	// 读取快照内容并写回原始路径
	data, err := os.ReadFile(cp.SnapshotPath)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	if err := os.WriteFile(cp.OriginalPath, data, 0644); err != nil {
		return fmt.Errorf("restore file: %w", err)
	}

	return nil
}

// List 返回最近的快照列表（按时间倒序）。
// filePathFilter 非空时只返回该文件的快照（绝对路径匹配）。
// limit <= 0 时使用默认值 20。
func (m *CheckpointManager) List(filePathFilter string, limit int) ([]Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit <= 0 {
		limit = 20
	}

	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var checkpoints []Checkpoint
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		cp, err := m.loadMeta(entry.Name())
		if err != nil {
			continue // 跳过损坏的快照
		}
		if filePathFilter != "" && cp.OriginalPath != filePathFilter {
			continue
		}
		checkpoints = append(checkpoints, *cp)
	}

	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.After(checkpoints[j].Timestamp)
	})

	if len(checkpoints) > limit {
		checkpoints = checkpoints[:limit]
	}

	return checkpoints, nil
}

// RestoreLast 恢复最近一个快照。返回恢复的快照信息。
func (m *CheckpointManager) RestoreLast() (*Checkpoint, error) {
	list, err := m.List("", 1)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no checkpoints available")
	}
	cp := list[0]
	if err := m.Restore(cp.ID); err != nil {
		return nil, err
	}
	return &cp, nil
}

// loadMeta 从磁盘加载快照元数据。
func (m *CheckpointManager) loadMeta(id string) (*Checkpoint, error) {
	metaPath := filepath.Join(m.baseDir, id, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		// 如果 meta.json 不存在，尝试从目录名推断时间并构造元数据
		return m.reconstructMeta(id)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parse checkpoint meta: %w", err)
	}
	return &cp, nil
}

// reconstructMeta 在 meta.json 缺失时从快照目录重建元数据。
func (m *CheckpointManager) reconstructMeta(id string) (*Checkpoint, error) {
	cpDir := filepath.Join(m.baseDir, id)
	snapshotPath := filepath.Join(cpDir, "content")

	info, err := os.Stat(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("checkpoint %q not found", id)
	}

	// 从 ID（时间戳毫秒）恢复时间
	var ts time.Time
	if tsMs, err := parseTimestampID(id); err == nil {
		ts = time.UnixMilli(tsMs)
	} else {
		ts = info.ModTime()
	}

	return &Checkpoint{
		ID:           id,
		Timestamp:    ts,
		OriginalPath: "", // 未知
		SnapshotPath: snapshotPath,
		FileExisted:  info.Size() > 0,
		FileSize:     info.Size(),
	}, nil
}

// parseTimestampID 尝试从 ID 中提取 Unix 毫秒时间戳。
// ID 格式为 "<毫秒时间戳>_<序号>"，取下划线前的部分。
func parseTimestampID(id string) (int64, error) {
	tsStr := id
	if idx := strings.Index(id, "_"); idx > 0 {
		tsStr = id[:idx]
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("not a timestamp: %w", err)
	}
	return ts, nil
}

// evictLocked 清理超出上限的旧快照。调用方需持有锁。
func (m *CheckpointManager) evictLocked() {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return
	}

	if len(entries) <= m.maxEntries {
		return
	}

	// 按目录名（时间戳）排序，删除最旧的
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	toDelete := len(entries) - m.maxEntries
	for i := 0; i < toDelete; i++ {
		os.RemoveAll(filepath.Join(m.baseDir, entries[i].Name()))
	}
}

// FormatCheckpointSummary 格式化快照列表为可读字符串（供 /rewind 命令展示）。
func FormatCheckpointSummary(checkpoints []Checkpoint) string {
	if len(checkpoints) == 0 {
		return "没有可用的快照。"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "最近 %d 个文件快照:\n\n", len(checkpoints))
	for i, cp := range checkpoints {
		// 取文件名作为简短显示
		shortPath := cp.OriginalPath
		if shortPath == "" {
			shortPath = "(unknown)"
		}
		// 截断过长的路径
		if len(shortPath) > 60 {
			shortPath = "..." + shortPath[len(shortPath)-57:]
		}

		status := "修改"
		if !cp.FileExisted {
			status = "新建"
		}

		fmt.Fprintf(&sb, "  %d. [%s] %s %s (%s, %dB)\n",
			i+1,
			cp.ID,
			cp.Timestamp.Format("15:04:05"),
			status,
			cp.ToolName,
			cp.FileSize,
		)
		fmt.Fprintf(&sb, "     文件: %s\n\n", shortPath)
	}
	sb.WriteString("使用 /rewind <ID> 恢复指定快照，或 /rewind last 恢复最近一个。")
	return sb.String()
}
