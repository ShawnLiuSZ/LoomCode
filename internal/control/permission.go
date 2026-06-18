package control

import "fmt"

// Permission 权限管理器
type Permission struct {
	allowlist *Allowlist
	gate      *Gate
}

// NewPermission 创建权限管理器
func NewPermission(mode GateMode) *Permission {
	allowlist := NewAllowlist()
	return &Permission{
		allowlist: allowlist,
		gate:      NewGate(mode, allowlist),
	}
}

// SetMode 设置门控模式
func (p *Permission) SetMode(mode GateMode) {
	p.gate.SetMode(mode)
}

// Mode 返回当前模式
func (p *Permission) Mode() GateMode {
	return p.gate.Mode()
}

// Allowlist 返回白名单
func (p *Permission) Allowlist() *Allowlist {
	return p.allowlist
}

// Check 检查操作是否允许
func (p *Permission) Check(toolName string, args map[string]any) (allowed bool, reason string) {
	// 先检查敏感文件
	if path, ok := args["path"].(string); ok {
		if sensitive, pattern := p.allowlist.CheckSensitive(path); sensitive {
			return false, fmt.Sprintf("sensitive file matched pattern: %s", pattern)
		}
	}

	// 检查 Shell 命令
	if toolName == "bash" {
		if cmd, ok := args["command"].(string); ok {
			if blocked, pattern := p.allowlist.IsBlockedShell(cmd); blocked {
				return false, fmt.Sprintf("blocked shell pattern: %s", pattern)
			}
		}
	}

	return p.gate.Check(toolName, args)
}

// PendingOps 返回待审批操作
func (p *Permission) PendingOps() []*Operation {
	return p.gate.PendingOps()
}

// ClearPending 清空待审批
func (p *Permission) ClearPending() {
	p.gate.ClearPending()
}
