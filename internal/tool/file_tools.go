package tool

import (
	"context"
	"fmt"
	"os"
)

// ReadFileTool 读取文件工具
type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read the contents of a file" }
func (t *ReadFileTool) IsReadOnly() bool    { return true }

func (t *ReadFileTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"path": {Type: "string", Description: "The path to the file to read"},
		},
		Required: []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	return &Result{Content: string(data)}, nil
}

// WriteFileTool 写入文件工具
type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Create or overwrite a file with content" }
func (t *WriteFileTool) IsReadOnly() bool    { return false }

func (t *WriteFileTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"path":    {Type: "string", Description: "The path to the file to write"},
			"content": {Type: "string", Description: "The content to write to the file"},
		},
		Required: []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file %q: %w", path, err)
	}

	return &Result{Content: fmt.Sprintf("File written: %s", path)}, nil
}

// EditFileTool 精确编辑文件工具
type EditFileTool struct{}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "Make a precise string replacement in a file" }
func (t *EditFileTool) IsReadOnly() bool    { return false }

func (t *EditFileTool) Schema() Schema {
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

func (t *EditFileTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	path, _ := args["path"].(string)
	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)

	if path == "" || oldText == "" {
		return nil, fmt.Errorf("path and old_text are required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(data)
	count := 0
	newContent := content
	for {
		idx := findSubstring(newContent, oldText)
		if idx < 0 {
			break
		}
		newContent = newContent[:idx] + newText + newContent[idx+len(oldText):]
		count++
	}
	_ = count

	if newContent == content {
		return nil, fmt.Errorf("old_text not found in file")
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file %q: %w", path, err)
	}

	return &Result{Content: fmt.Sprintf("File edited: %s", path)}, nil
}

func findSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
