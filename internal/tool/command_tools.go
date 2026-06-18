package tool

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// BashTool Shell 命令执行工具
type BashTool struct {
	permission PermissionChecker
}

// PermissionChecker 权限检查器接口
type PermissionChecker interface {
	Check(toolName string, args map[string]any) (allowed bool, reason string)
}

// SetPermissionChecker 设置权限检查器
func (t *BashTool) SetPermissionChecker(checker PermissionChecker) {
	t.permission = checker
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Execute a shell command" }
func (t *BashTool) IsReadOnly() bool    { return false }

func (t *BashTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"command": {Type: "string", Description: "The shell command to execute"},
		},
		Required: []string{"command"},
	}
}

func (t *BashTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// 权限检查
	if t.permission != nil {
		if allowed, reason := t.permission.Check("bash", args); !allowed {
			return nil, fmt.Errorf("command blocked: %s", reason)
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Env = EnvForSubprocess()
	output, err := cmd.CombinedOutput()

	result := &Result{Content: string(output)}
	if err != nil {
		if len(output) > 0 {
			result.Content = string(output)
		}
		result.Error = err.Error()
		return result, nil
	}

	return result, nil
}

// GrepTool 内容搜索工具
type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search for a pattern in files" }
func (t *GrepTool) IsReadOnly() bool    { return true }

func (t *GrepTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"pattern": {Type: "string", Description: "The regex pattern to search for"},
			"path":    {Type: "string", Description: "The file or directory to search in"},
		},
		Required: []string{"pattern", "path"},
	}
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)

	if pattern == "" || path == "" {
		return nil, fmt.Errorf("pattern and path are required")
	}

	cmd := exec.CommandContext(ctx, "grep", "-rn", pattern, path)
	cmd.Env = EnvForSubprocess()
	output, err := cmd.CombinedOutput()

	content := string(output)
	if len(content) == 0 {
		content = fmt.Sprintf("No matches found for '%s' in %s", pattern, path)
	}
	if err != nil {
		// grep returns exit 1 when no matches found
		if len(output) == 0 {
			content = fmt.Sprintf("No matches found for '%s' in %s", pattern, path)
		}
	}

	return &Result{Content: content}, nil
}

// GlobTool 文件匹配工具
type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Find files matching a glob pattern" }
func (t *GlobTool) IsReadOnly() bool    { return true }

func (t *GlobTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"pattern": {Type: "string", Description: "The glob pattern (e.g. '**/*.go')"},
			"path":    {Type: "string", Description: "The directory to search in"},
		},
		Required: []string{"pattern"},
	}
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)

	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	if path == "" {
		path = "."
	}

	cmd := exec.CommandContext(ctx, "find", path, "-name", pattern, "-type", "f")
	cmd.Env = EnvForSubprocess()
	output, _ := cmd.CombinedOutput()

	content := strings.TrimSpace(string(output))
	if len(content) == 0 {
		content = fmt.Sprintf("No files matching '%s'", pattern)
	}
	// find 命令在没有找到文件时会返回错误，这是正常情况

	return &Result{Content: content}, nil
}
