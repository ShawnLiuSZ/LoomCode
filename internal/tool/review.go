package tool

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ReviewTool 编辑预览工具
type ReviewTool struct {
	mu      sync.Mutex
	pending []PendingEdit
	enabled bool
}

// PendingEdit 待确认的编辑
type PendingEdit struct {
	ID      int
	File    string
	OldText string
	NewText string
	Applied bool
}

// NewReviewTool 创建编辑预览工具
func NewReviewTool() *ReviewTool {
	return &ReviewTool{
		pending: make([]PendingEdit, 0),
		enabled: true,
	}
}

// SetEnabled 设置是否启用预览模式
func (t *ReviewTool) SetEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled = enabled
}

// IsEnabled 是否启用预览模式
func (t *ReviewTool) IsEnabled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.enabled
}

// AddPending 添加待确认的编辑
func (t *ReviewTool) AddPending(edit PendingEdit) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	edit.ID = len(t.pending) + 1
	t.pending = append(t.pending, edit)
	return edit.ID
}

// GetPending 获取所有待确认的编辑
func (t *ReviewTool) GetPending() []PendingEdit {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]PendingEdit, len(t.pending))
	copy(result, t.pending)
	return result
}

// ClearPending 清除所有待确认的编辑
func (t *ReviewTool) ClearPending() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = make([]PendingEdit, 0)
}

// Apply 应用指定 ID 的编辑
func (t *ReviewTool) Apply(id int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, edit := range t.pending {
		if edit.ID == id {
			if edit.Applied {
				return fmt.Errorf("edit %d already applied", id)
			}

			// 读取文件
			content, err := os.ReadFile(edit.File)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			// 替换文本
			newContent := strings.Replace(string(content), edit.OldText, edit.NewText, 1)
			if newContent == string(content) {
				return fmt.Errorf("old text not found in file")
			}

			// 写入文件
			if err := os.WriteFile(edit.File, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}

			t.pending[i].Applied = true
			return nil
		}
	}

	return fmt.Errorf("edit %d not found", id)
}

// ApplyAll 应用所有待确认的编辑
func (t *ReviewTool) ApplyAll() error {
	t.mu.Lock()
	edits := make([]PendingEdit, len(t.pending))
	copy(edits, t.pending)
	t.mu.Unlock()

	for _, edit := range edits {
		if !edit.Applied {
			if err := t.Apply(edit.ID); err != nil {
				return fmt.Errorf("apply edit %d: %w", edit.ID, err)
			}
		}
	}

	return nil
}

// Reject 拒绝指定 ID 的编辑
func (t *ReviewTool) Reject(id int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, edit := range t.pending {
		if edit.ID == id {
			// 从列表中移除
			t.pending = append(t.pending[:i], t.pending[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("edit %d not found", id)
}

// RejectAll 拒绝所有待确认的编辑
func (t *ReviewTool) RejectAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = make([]PendingEdit, 0)
}

// Preview 生成编辑预览
func (t *ReviewTool) Preview(edit PendingEdit) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "📄 File: %s\n", edit.File)
	sb.WriteString("─────────────────────────────────────\n")
	sb.WriteString("SEARCH:\n")
	sb.WriteString(truncateLines(edit.OldText, 10))
	sb.WriteString("\n─────────────────────────────────────\n")
	sb.WriteString("REPLACE:\n")
	sb.WriteString(truncateLines(edit.NewText, 10))
	sb.WriteString("\n─────────────────────────────────────\n")

	return sb.String()
}

// PreviewAll 预览所有待确认的编辑
func (t *ReviewTool) PreviewAll() string {
	pending := t.GetPending()
	if len(pending) == 0 {
		return "没有待确认的编辑"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "📋 待确认编辑 (%d 个):\n\n", len(pending))

	for _, edit := range pending {
		sb.WriteString(t.Preview(edit))
		sb.WriteString("\n")
	}

	sb.WriteString("命令:\n")
	sb.WriteString("  /apply      - 应用所有编辑\n")
	sb.WriteString("  /apply <id> - 应用指定编辑\n")
	sb.WriteString("  /reject     - 拒绝所有编辑\n")
	sb.WriteString("  /reject <id>- 拒绝指定编辑\n")

	return sb.String()
}

// truncateLines 截断到指定行数
func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

// ReviewEditTool 编辑文件工具（带预览）
type ReviewEditTool struct {
	review *ReviewTool
}

// NewReviewEditTool 创建带预览的编辑工具
func NewReviewEditTool(review *ReviewTool) *ReviewEditTool {
	return &ReviewEditTool{review: review}
}

func (t *ReviewEditTool) Name() string { return "edit_file" }
func (t *ReviewEditTool) Description() string {
	return "Make a precise string replacement in a file (with preview)"
}
func (t *ReviewEditTool) IsReadOnly() bool { return false }

func (t *ReviewEditTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"path":     {Type: "string", Description: "The path to the file to edit"},
			"old_text": {Type: "string", Description: "The text to replace"},
			"new_text": {Type: "string", Description: "The replacement text"},
		},
		Required: []string{"path", "old_text", "new_text"},
	}
}

func (t *ReviewEditTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	path, _ := args["path"].(string)
	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)

	if path == "" || oldText == "" {
		return nil, fmt.Errorf("path and old_text are required")
	}

	// 如果预览模式启用，添加到待确认列表
	if t.review.IsEnabled() {
		id := t.review.AddPending(PendingEdit{
			File:    path,
			OldText: oldText,
			NewText: newText,
		})

		preview := t.review.Preview(PendingEdit{
			File:    path,
			OldText: oldText,
			NewText: newText,
		})

		return &Result{
			Content: fmt.Sprintf("已添加到待确认列表 (ID: %d)\n\n%s\n使用 /apply 确认应用", id, preview),
		}, nil
	}

	// 否则直接应用
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(data)
	if !strings.Contains(content, oldText) {
		return nil, fmt.Errorf("old_text not found in file")
	}

	newContent := strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file %q: %w", path, err)
	}

	return &Result{Content: fmt.Sprintf("File edited: %s", path)}, nil
}
