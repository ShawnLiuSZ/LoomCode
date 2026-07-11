package control

import "sync"

// GateMode 编辑门控模式
type GateMode int

const (
	ModeReview GateMode = iota // 每个编辑弹出确认
	ModeAuto                   // 自动应用，可撤销
	ModeYolo                   // 跳过所有确认
)

func (m GateMode) String() string {
	switch m {
	case ModeReview:
		return "review"
	case ModeAuto:
		return "auto"
	case ModeYolo:
		return "yolo"
	default:
		return "unknown"
	}
}

// Gate 编辑门控
type Gate struct {
	mu         sync.Mutex
	mode       GateMode
	allowlist  *Allowlist
	pendingOps []*Operation
}

// Operation 待审批的操作
type Operation struct {
	ToolName    string
	Description string
	Args        map[string]any
}

// NewGate 创建编辑门控
func NewGate(mode GateMode, allowlist *Allowlist) *Gate {
	return &Gate{
		mode:      mode,
		allowlist: allowlist,
	}
}

// SetMode 设置门控模式
func (g *Gate) SetMode(mode GateMode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.mode = mode
}

// Mode 返回当前模式
func (g *Gate) Mode() GateMode {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.mode
}

// Check 检查操作是否需要审批
// 返回 true 表示允许执行，false 表示被拦截
func (g *Gate) Check(toolName string, args map[string]any) (allowed bool, reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Yolo 模式：全部放行
	if g.mode == ModeYolo {
		return true, "yolo mode"
	}

	// 只读工具始终放行
	if isReadOnlyTool(toolName) {
		return true, "read-only tool"
	}

	// 检查白名单
	if g.allowlist != nil {
		if g.allowlist.IsAllowed(toolName, args) {
			return true, "allowlisted"
		}
	}

	// Review 模式：需要审批
	if g.mode == ModeReview {
		op := &Operation{
			ToolName:    toolName,
			Description: describeOperation(toolName, args),
			Args:        args,
		}
		g.pendingOps = append(g.pendingOps, op)
		return false, "requires approval"
	}

	// Auto 模式：白名单内的操作已在上面放行；其余（未白名单的 bash / 工作区外的写入）一律拒绝。
	// H6 安全修复：不再无条件放行，否则 Auto 等同 Yolo。
	if g.mode == ModeAuto {
		return false, "auto mode: not allowlisted"
	}

	return false, "blocked"
}

// PendingOps 返回待审批的操作列表
func (g *Gate) PendingOps() []*Operation {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pendingOps
}

// ClearPending 清空待审批列表
func (g *Gate) ClearPending() {
	g.pendingOps = nil
}

// isReadOnlyTool 判断是否只读工具
func isReadOnlyTool(name string) bool {
	readOnly := map[string]bool{
		"read_file": true,
		"grep":      true,
		"glob":      true,
	}
	return readOnly[name]
}

// describeOperation 生成操作描述
func describeOperation(toolName string, args map[string]any) string {
	switch toolName {
	case "write_file":
		path, _ := args["path"].(string)
		return "Write file: " + path
	case "edit_file":
		path, _ := args["path"].(string)
		old, _ := args["old_text"].(string)
		new, _ := args["new_text"].(string)
		return "Edit " + path + ": replace '" + truncate(old, 50) + "' with '" + truncate(new, 50) + "'"
	case "bash":
		cmd, _ := args["command"].(string)
		return "Run command: " + truncate(cmd, 100)
	default:
		return toolName
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
