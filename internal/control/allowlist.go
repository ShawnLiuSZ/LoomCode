package control

import (
	"path/filepath"
	"strings"
)

// Allowlist 文件/命令白名单
type Allowlist struct {
	// Shell 命令白名单（精确匹配 + 前缀匹配）
	shellCommands []string

	// 文件路径白名单（目录前缀匹配）
	allowedPaths []string

	// 敏感文件模式（.env, credentials, secrets 等）
	sensitivePatterns []string

	// 禁止的 Shell 模式
	blockedShellPatterns []string
}

// NewAllowlist 创建白名单
func NewAllowlist() *Allowlist {
	return &Allowlist{
		sensitivePatterns: []string{
			".env",
			".env.local",
			".env.production",
			"credentials",
			"secrets",
			".pem",
			".key",
			"id_rsa",
			"id_ed25519",
			".git/config",
			"~/.aws/credentials",
		},
		blockedShellPatterns: []string{
			"rm -rf",
			"rm -r ",
			"sudo ",
			"chmod 777",
			"| sh",
			"> /dev/",
			"mkfs.",
			"dd if=",
			":(){ :|:& };:", // fork bomb
		},
	}
}

// SetShellCommands 设置允许的 Shell 命令
func (a *Allowlist) SetShellCommands(commands []string) {
	a.shellCommands = commands
}

// SetAllowedPaths 设置允许的文件路径
func (a *Allowlist) SetAllowedPaths(paths []string) {
	a.allowedPaths = paths
}

// AddSensitivePattern 添加敏感文件模式
func (a *Allowlist) AddSensitivePattern(pattern string) {
	a.sensitivePatterns = append(a.sensitivePatterns, pattern)
}

// IsAllowed 检查操作是否在白名单中
func (a *Allowlist) IsAllowed(toolName string, args map[string]any) bool {
	switch toolName {
	case "bash":
		return a.isShellAllowed(args)
	case "write_file", "edit_file":
		return a.isFileWriteAllowed(args)
	default:
		return true
	}
}

// CheckSensitive 检查文件路径是否敏感
// 返回 true 表示敏感，需要额外确认
func (a *Allowlist) CheckSensitive(path string) (bool, string) {
	base := filepath.Base(path)
	for _, pattern := range a.sensitivePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true, pattern
		}
		if strings.Contains(path, pattern) {
			return true, pattern
		}
	}
	return false, ""
}

// isShellAllowed 检查 Shell 命令是否允许
func (a *Allowlist) isShellAllowed(args map[string]any) bool {
	command, _ := args["command"].(string)
	if command == "" {
		return false
	}

	// 检查禁止模式
	for _, blocked := range a.blockedShellPatterns {
		if strings.Contains(command, blocked) {
			return false
		}
	}

	// 没有配置白名单 → 由 Gate 模式控制
	if len(a.shellCommands) == 0 {
		return false
	}

	// 检查允许列表
	for _, allowed := range a.shellCommands {
		if command == allowed || strings.HasPrefix(command, allowed) {
			return true
		}
	}

	return false
}

// isFileWriteAllowed 检查文件写入是否在允许路径
func (a *Allowlist) isFileWriteAllowed(args map[string]any) bool {
	path, _ := args["path"].(string)
	if path == "" {
		return false
	}

	// 检查敏感文件
	if sensitive, _ := a.CheckSensitive(path); sensitive {
		return false // 敏感文件始终需要审批
	}

	// 没有配置路径白名单 → 由 Gate 模式控制
	if len(a.allowedPaths) == 0 {
		return false
	}

	// 检查允许路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, allowed := range a.allowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, allowedAbs) {
			return true
		}
	}

	return false
}

// IsBlockedShell 检查 Shell 命令是否被禁止
func (a *Allowlist) IsBlockedShell(command string) (bool, string) {
	for _, blocked := range a.blockedShellPatterns {
		if strings.Contains(command, blocked) {
			return true, blocked
		}
	}
	return false, ""
}
