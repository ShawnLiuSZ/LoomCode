package tool

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const gitOutputLimit = 32 * 1024

type GitStatusTool struct {
	root string
}

func (t *GitStatusTool) SetRoot(root string) { t.root = root }

func (t *GitStatusTool) Name() string        { return "git_status" }
func (t *GitStatusTool) Description() string { return "Show working tree status in structured form" }
func (t *GitStatusTool) IsReadOnly() bool    { return true }

func (t *GitStatusTool) Schema() Schema {
	return Schema{
		Type:       "object",
		Properties: map[string]Property{},
	}
}

func (t *GitStatusTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	if err := t.checkGit(); err != nil {
		return nil, err
	}

	out, err := t.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	if len(strings.TrimSpace(out)) == 0 {
		return &Result{Content: "Working tree clean"}, nil
	}

	var modified, added, deleted, untracked, renamed, other []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if len(line) < 3 {
			continue
		}
		index := line[0]
		worktree := line[1]
		name := strings.TrimSpace(line[2:])

		switch {
		case index == '?' && worktree == '?':
			untracked = append(untracked, name)
		case index == 'R':
			renamed = append(renamed, name)
		case worktree == 'D' || index == 'D':
			deleted = append(deleted, name)
		case index == 'A':
			added = append(added, name)
		case index == 'M' || worktree == 'M':
			modified = append(modified, name)
		default:
			other = append(other, line)
		}
	}

	var sb strings.Builder
	if len(modified) > 0 {
		fmt.Fprintf(&sb, "Modified (%d):\n", len(modified))
		for _, f := range modified {
			sb.WriteString("  " + f + "\n")
		}
	}
	if len(added) > 0 {
		fmt.Fprintf(&sb, "Added (%d):\n", len(added))
		for _, f := range added {
			sb.WriteString("  " + f + "\n")
		}
	}
	if len(deleted) > 0 {
		fmt.Fprintf(&sb, "Deleted (%d):\n", len(deleted))
		for _, f := range deleted {
			sb.WriteString("  " + f + "\n")
		}
	}
	if len(untracked) > 0 {
		fmt.Fprintf(&sb, "Untracked (%d):\n", len(untracked))
		for _, f := range untracked {
			sb.WriteString("  " + f + "\n")
		}
	}
	if len(renamed) > 0 {
		fmt.Fprintf(&sb, "Renamed (%d):\n", len(renamed))
		for _, f := range renamed {
			sb.WriteString("  " + f + "\n")
		}
	}
	if len(other) > 0 {
		fmt.Fprintf(&sb, "Other (%d):\n", len(other))
		for _, f := range other {
			sb.WriteString("  " + f + "\n")
		}
	}

	return &Result{Content: sb.String()}, nil
}

func (t *GitStatusTool) checkGit() error {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository (or any of the parent directories)")
	}
	return nil
}

func (t *GitStatusTool) runGit(ctx context.Context, args ...string) (string, error) {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Env = EnvForSubprocess()
	SetProcessGroup(cmd)
	cmd.Cancel = func() error {
		return KillProcessGroup(cmd)
	}

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	result := string(out)
	if len(result) > gitOutputLimit {
		result = result[:gitOutputLimit] + "\n... (output truncated at 32KB)"
	}
	return result, nil
}

type GitDiffTool struct {
	root  string
	trust OutsideTrustChecker
}

func (t *GitDiffTool) SetRoot(root string) { t.root = root }

// SetTrust 设置工作区外文件访问信任检查器
func (t *GitDiffTool) SetTrust(trust OutsideTrustChecker) { t.trust = trust }

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string { return "Show changes in working tree or staged area" }
func (t *GitDiffTool) IsReadOnly() bool    { return true }

func (t *GitDiffTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"path":      {Type: "string", Description: "Optional file path to filter diff"},
			"staged":    {Type: "boolean", Description: "Show staged changes instead of working tree"},
			"max_lines": {Type: "integer", Description: "Max output lines (default 500)"},
		},
	}
}

func (t *GitDiffTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	if err := t.checkGit(); err != nil {
		return nil, err
	}

	gitArgs := []string{"diff", "--no-color"}
	if staged, _ := args["staged"].(bool); staged {
		gitArgs = append(gitArgs, "--cached")
	}
	if path, _ := args["path"].(string); path != "" {
		if t.root != "" {
			resolved, err := resolveWithinRoot(t.root, path, t.trust)
			if err != nil {
				return nil, err
			}
			path = resolved
		}
		gitArgs = append(gitArgs, "--", path)
	}

	out, err := t.runGit(ctx, gitArgs...)
	if err != nil {
		return nil, err
	}

	maxLines := 500
	if m, ok := args["max_lines"].(float64); ok && m > 0 {
		maxLines = int(m)
	}

	lines := strings.Split(out, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		out = strings.Join(lines, "\n") + fmt.Sprintf("\n... (truncated at %d lines)", maxLines)
	}

	if len(strings.TrimSpace(out)) == 0 {
		return &Result{Content: "No changes"}, nil
	}

	return &Result{Content: out}, nil
}

func (t *GitDiffTool) checkGit() error {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository (or any of the parent directories)")
	}
	return nil
}

func (t *GitDiffTool) runGit(ctx context.Context, args ...string) (string, error) {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Env = EnvForSubprocess()
	SetProcessGroup(cmd)
	cmd.Cancel = func() error {
		return KillProcessGroup(cmd)
	}

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	result := string(out)
	if len(result) > gitOutputLimit {
		result = result[:gitOutputLimit] + "\n... (output truncated at 32KB)"
	}
	return result, nil
}

type GitLogTool struct {
	root  string
	trust OutsideTrustChecker
}

func (t *GitLogTool) SetRoot(root string) { t.root = root }

// SetTrust 设置工作区外文件访问信任检查器
func (t *GitLogTool) SetTrust(trust OutsideTrustChecker) { t.trust = trust }

func (t *GitLogTool) Name() string        { return "git_log" }
func (t *GitLogTool) Description() string { return "Show recent commit history" }
func (t *GitLogTool) IsReadOnly() bool    { return true }

func (t *GitLogTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"count": {Type: "integer", Description: "Number of commits to show (default 20)"},
			"path":  {Type: "string", Description: "Optional file path to filter history"},
		},
	}
}

func (t *GitLogTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	if err := t.checkGit(); err != nil {
		return nil, err
	}

	count := 20
	if n, ok := args["count"].(float64); ok && n > 0 {
		count = int(n)
		if count > 100 {
			count = 100
		}
	}

	gitArgs := []string{"log", "--oneline", fmt.Sprintf("-%d", count)}
	if path, _ := args["path"].(string); path != "" {
		if t.root != "" {
			resolved, err := resolveWithinRoot(t.root, path, t.trust)
			if err != nil {
				return nil, err
			}
			path = resolved
		}
		gitArgs = append(gitArgs, "--", path)
	}

	out, err := t.runGit(ctx, gitArgs...)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "does not have any commits") {
			return &Result{Content: "No commits found"}, nil
		}
		return nil, err
	}

	if len(strings.TrimSpace(out)) == 0 {
		return &Result{Content: "No commits found"}, nil
	}

	return &Result{Content: out}, nil
}

func (t *GitLogTool) checkGit() error {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository (or any of the parent directories)")
	}
	return nil
}

func (t *GitLogTool) runGit(ctx context.Context, args ...string) (string, error) {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Env = EnvForSubprocess()
	SetProcessGroup(cmd)
	cmd.Cancel = func() error {
		return KillProcessGroup(cmd)
	}

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	result := string(out)
	if len(result) > gitOutputLimit {
		result = result[:gitOutputLimit] + "\n... (output truncated at 32KB)"
	}
	return result, nil
}

type GitCommitTool struct {
	root string
}

func (t *GitCommitTool) SetRoot(root string) { t.root = root }

func (t *GitCommitTool) Name() string        { return "git_commit" }
func (t *GitCommitTool) Description() string { return "Stage all changes and commit with a message" }
func (t *GitCommitTool) IsReadOnly() bool    { return false }

func (t *GitCommitTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"message": {Type: "string", Description: "The commit message"},
		},
		Required: []string{"message"},
	}
}

func (t *GitCommitTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	message, _ := args["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	if err := t.checkGit(); err != nil {
		return nil, err
	}

	if _, err := t.runGit(ctx, "add", "-A"); err != nil {
		return nil, fmt.Errorf("git add: %w", err)
	}

	out, err := t.runGit(ctx, "commit", "-m", message)
	if err != nil {
		return nil, err
	}

	return &Result{Content: strings.TrimSpace(out)}, nil
}

func (t *GitCommitTool) checkGit() error {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository (or any of the parent directories)")
	}
	return nil
}

func (t *GitCommitTool) runGit(ctx context.Context, args ...string) (string, error) {
	dir := t.root
	if dir == "" {
		dir = "."
	}
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Env = EnvForSubprocess()
	SetProcessGroup(cmd)
	cmd.Cancel = func() error {
		return KillProcessGroup(cmd)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(out)))
	}

	result := string(out)
	if len(result) > gitOutputLimit {
		result = result[:gitOutputLimit] + "\n... (output truncated at 32KB)"
	}
	return result, nil
}
