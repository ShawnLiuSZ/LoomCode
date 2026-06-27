package tool

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// resolveWithinRoot 校验 path 落在 workspace root 之内并返回清理后的绝对路径。
// root 为空表示未配置工作区限制（向后兼容）。会解析符号链接以阻止逃逸。
func resolveWithinRoot(root, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, path)
	}
	abs = filepath.Clean(abs)

	if root == "" {
		return abs, nil
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	if !within(rootAbs, abs) {
		return "", fmt.Errorf("path %q escapes workspace root", path)
	}

	// 解析符号链接后再校验一次：文件已存在则解析文件本身，否则解析其父目录（用于创建新文件）。
	// root 自身也可能含符号链接（如 macOS /var → /private/var），需先规范化再比较。
	rootCanonical := rootAbs
	if r, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootCanonical = r
	}
	target := abs
	if _, statErr := os.Lstat(abs); statErr != nil {
		target = filepath.Dir(abs)
	}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		if !within(rootCanonical, resolved) {
			return "", fmt.Errorf("path %q escapes workspace root via symlink", path)
		}
	}

	return abs, nil
}

// within 判断 abs 是否在 rootAbs 之内（含 root 本身）。
func within(rootAbs, abs string) bool {
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// ReadFileTool 读取文件工具
type ReadFileTool struct {
	root       string
	permission PermissionChecker
}

// SetRoot 设置 workspace 根目录（路径限制）
func (t *ReadFileTool) SetRoot(root string) { t.root = root }

// SetPermissionChecker 设置权限检查器
func (t *ReadFileTool) SetPermissionChecker(checker PermissionChecker) { t.permission = checker }

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
	path, _ := args["path"].(string)
	abs, err := resolveWithinRoot(t.root, path)
	if err != nil {
		return nil, err
	}
	if t.permission != nil {
		if allowed, reason := t.permission.Check("read_file", args); !allowed {
			return nil, fmt.Errorf("read blocked: %s", reason)
		}
	}

	const maxReadSize = 1024 * 1024
	file, err := os.Open(abs)
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxReadSize+1))
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(data)
	if len(data) > maxReadSize {
		content = content[:maxReadSize] + "\n[truncated]"
	}

	return &Result{Content: content}, nil
}

// WriteFileTool 写入文件工具
type WriteFileTool struct {
	root       string
	permission PermissionChecker
	diagnoser  Diagnoser
}

// SetRoot 设置 workspace 根目录（路径限制）
func (t *WriteFileTool) SetRoot(root string) { t.root = root }

// SetPermissionChecker 设置权限检查器
func (t *WriteFileTool) SetPermissionChecker(checker PermissionChecker) { t.permission = checker }

// SetDiagnoser 设置写入后诊断器（可选，nil 时不诊断）
func (t *WriteFileTool) SetDiagnoser(d Diagnoser) { t.diagnoser = d }

func (t *WriteFileTool) diagnose(ctx context.Context, path string) string {
	return runDiagnoser(t.diagnoser, ctx, path)
}

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

	abs, err := resolveWithinRoot(t.root, path)
	if err != nil {
		return nil, err
	}
	if t.permission != nil {
		if allowed, reason := t.permission.Check("write_file", args); !allowed {
			return nil, fmt.Errorf("write blocked: %s", reason)
		}
	}

	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file %q: %w", path, err)
	}

	return &Result{Content: fmt.Sprintf("File written: %s", path) + t.diagnose(ctx, abs)}, nil
}

// EditFileTool 精确编辑文件工具
type EditFileTool struct {
	root       string
	permission PermissionChecker
	diagnoser  Diagnoser
}

// SetRoot 设置 workspace 根目录（路径限制）
func (t *EditFileTool) SetRoot(root string) { t.root = root }

// SetPermissionChecker 设置权限检查器
func (t *EditFileTool) SetPermissionChecker(checker PermissionChecker) { t.permission = checker }

// SetDiagnoser 设置编辑后诊断器（可选，nil 时不诊断）
func (t *EditFileTool) SetDiagnoser(d Diagnoser) { t.diagnoser = d }

func (t *EditFileTool) diagnose(ctx context.Context, path string) string {
	return runDiagnoser(t.diagnoser, ctx, path)
}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "Make a precise string replacement in a file" }
func (t *EditFileTool) IsReadOnly() bool    { return false }

func (t *EditFileTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"path":        {Type: "string", Description: "The path to the file to edit"},
			"old_text":    {Type: "string", Description: "The exact text to replace. Must match a unique location unless replace_all is set."},
			"new_text":    {Type: "string", Description: "The replacement text"},
			"replace_all": {Type: "boolean", Description: "Replace every occurrence instead of requiring a unique match (default false)"},
		},
		Required: []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	path, _ := args["path"].(string)
	oldText, _ := args["old_text"].(string)
	newText, _ := args["new_text"].(string)

	if oldText == "" {
		return nil, fmt.Errorf("old_text is required")
	}
	abs, err := resolveWithinRoot(t.root, path)
	if err != nil {
		return nil, err
	}
	if t.permission != nil {
		if allowed, reason := t.permission.Check("edit_file", args); !allowed {
			return nil, fmt.Errorf("edit blocked: %s", reason)
		}
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(data)
	replaceAll, _ := args["replace_all"].(bool)

	// 唯一匹配校验：避免改错位置（旧实现静默替换首个匹配，极易出错）。
	count := strings.Count(content, oldText)
	if count == 0 {
		return nil, fmt.Errorf("old_text not found in file")
	}
	if count > 1 && !replaceAll {
		return nil, fmt.Errorf("old_text matches %d locations; add surrounding context to make it unique, or set replace_all=true", count)
	}

	n := 1
	if replaceAll {
		n = -1
	}
	newContent := strings.Replace(content, oldText, newText, n)

	if err := os.WriteFile(abs, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file %q: %w", path, err)
	}

	msg := fmt.Sprintf("File edited: %s", path)
	if replaceAll && count > 1 {
		msg = fmt.Sprintf("File edited: %s (%d replacements)", path, count)
	}
	return &Result{Content: msg + t.diagnose(ctx, abs)}, nil
}
