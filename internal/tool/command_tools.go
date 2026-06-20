package tool

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
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

// maxOutputSize bash 输出最大 1MB
const maxOutputSize = 1024 * 1024

// bashTimeout bash 命令独立超时 60 秒
const bashTimeout = 60 * time.Second

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

	// 创建独立超时 context（60 秒），不依赖父 context
	bashCtx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(bashCtx, "bash", "-c", command)
	cmd.Env = EnvForSubprocess()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // 创建进程组

	// 使用 Pipe 限制输出大小
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // 合并 stderr 到 stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	// 读取输出（限制大小）
	limitedReader := io.LimitReader(stdout, maxOutputSize)
	output, _ := io.ReadAll(limitedReader)

	// 等待命令完成
	waitErr := cmd.Wait()

	// 如果输出超限，添加提示
	if len(output) >= maxOutputSize {
		output = append(output, []byte("\n... (output truncated at 1MB)")...)
	}

	result := &Result{Content: string(output)}
	if waitErr != nil {
		if len(output) > 0 {
			result.Content = string(output)
		}
		result.Error = waitErr.Error()
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
