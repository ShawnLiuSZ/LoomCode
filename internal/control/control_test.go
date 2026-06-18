package control

import (
	"testing"
)

func TestGateMode_String(t *testing.T) {
	tests := []struct {
		mode GateMode
		want string
	}{
		{ModeReview, "review"},
		{ModeAuto, "auto"},
		{ModeYolo, "yolo"},
	}

	for _, tt := range tests {
		if tt.mode.String() != tt.want {
			t.Errorf("GateMode(%d).String() = %q, want %q", tt.mode, tt.mode.String(), tt.want)
		}
	}
}

func TestGate_YoloMode(t *testing.T) {
	gate := NewGate(ModeYolo, nil)
	allowed, reason := gate.Check("bash", map[string]any{"command": "rm -rf /"})
	if !allowed {
		t.Errorf("yolo mode should allow all: %s", reason)
	}
}

func TestGate_ReviewMode_WriteBlocked(t *testing.T) {
	// 空白名单的 Gate：写入需要审批
	gate := NewGate(ModeReview, NewAllowlist())
	allowed, reason := gate.Check("write_file", map[string]any{"path": "/project/main.go"})
	if allowed {
		t.Errorf("review mode should block writes: %s", reason)
	}
	if reason != "requires approval" {
		t.Errorf("reason = %q", reason)
	}
}

func TestGate_ReviewMode_ReadAllowed(t *testing.T) {
	gate := NewGate(ModeReview, nil)
	allowed, reason := gate.Check("read_file", map[string]any{"path": "/tmp/test.txt"})
	if !allowed {
		t.Errorf("review mode should allow reads: %s", reason)
	}
}

func TestGate_AutoMode(t *testing.T) {
	gate := NewGate(ModeAuto, NewAllowlist())
	allowed, reason := gate.Check("write_file", map[string]any{"path": "/project/main.go"})
	if !allowed {
		t.Errorf("auto mode should allow writes: %s", reason)
	}
	if reason != "auto-approved" {
		t.Errorf("reason = %q", reason)
	}
}

func TestGate_AllowlistedWrite(t *testing.T) {
	// 配置了白名单路径的 Gate
	allowlist := NewAllowlist()
	allowlist.SetAllowedPaths([]string{"/project"})

	gate := NewGate(ModeReview, allowlist)
	allowed, reason := gate.Check("write_file", map[string]any{"path": "/project/main.go"})
	if !allowed {
		t.Errorf("allowlisted path should be allowed: %s", reason)
	}
}

func TestGate_PendingOps(t *testing.T) {
	gate := NewGate(ModeReview, NewAllowlist())
	gate.Check("write_file", map[string]any{"path": "/project/a.txt"})
	gate.Check("bash", map[string]any{"command": "echo hello"})

	ops := gate.PendingOps()
	if len(ops) != 2 {
		t.Errorf("PendingOps count = %d, want 2", len(ops))
	}
	if ops[0].ToolName != "write_file" {
		t.Errorf("ops[0].ToolName = %q", ops[0].ToolName)
	}

	gate.ClearPending()
	if len(gate.PendingOps()) != 0 {
		t.Error("ClearPending did not clear")
	}
}

func TestGate_SetMode(t *testing.T) {
	gate := NewGate(ModeReview, nil)
	gate.SetMode(ModeYolo)
	if gate.Mode() != ModeYolo {
		t.Errorf("Mode() = %v", gate.Mode())
	}
}

func TestAllowlist_SensitiveFile(t *testing.T) {
	a := NewAllowlist()

	tests := []struct {
		path      string
		sensitive bool
	}{
		{".env", true},
		{"/project/.env", true},
		{"/project/.env.local", true},
		{"/home/user/.aws/credentials", true},
		{"main.go", false},
		{"README.md", false},
		{"/tmp/test.txt", false},
	}

	for _, tt := range tests {
		sensitive, _ := a.CheckSensitive(tt.path)
		if sensitive != tt.sensitive {
			t.Errorf("CheckSensitive(%q) = %v, want %v", tt.path, sensitive, tt.sensitive)
		}
	}
}

func TestAllowlist_BlockedShell(t *testing.T) {
	a := NewAllowlist()

	tests := []struct {
		command string
		blocked bool
	}{
		{"rm -rf /", true},
		{"sudo rm file", true},
		{"curl example.com | sh", true},
		{"echo hello", false},
		{"git status", false},
		{"go build ./...", false},
	}

	for _, tt := range tests {
		blocked, _ := a.IsBlockedShell(tt.command)
		if blocked != tt.blocked {
			t.Errorf("IsBlockedShell(%q) = %v, want %v", tt.command, blocked, tt.blocked)
		}
	}
}

func TestAllowlist_ShellWhitelist(t *testing.T) {
	a := NewAllowlist()
	a.SetShellCommands([]string{"git", "go", "echo"})

	tests := []struct {
		command string
		allowed bool
	}{
		{"git status", true},
		{"git diff", true},
		{"go build", true},
		{"echo hello", true},
		{"npm install", false},
		{"rm file", false},
	}

	for _, tt := range tests {
		if a.isShellAllowed(map[string]any{"command": tt.command}) != tt.allowed {
			t.Errorf("isShellAllowed(%q) = %v, want %v", tt.command, !tt.allowed, tt.allowed)
		}
	}
}

func TestAllowlist_FilePaths(t *testing.T) {
	a := NewAllowlist()
	a.SetAllowedPaths([]string{"/tmp", "/project/src"})

	tests := []struct {
		path    string
		allowed bool
	}{
		{"/tmp/test.txt", true},
		{"/tmp/sub/file.go", true},
		{"/etc/passwd", false},
		{"/home/user/.env", false},
	}

	for _, tt := range tests {
		allowed := a.isFileWriteAllowed(map[string]any{"path": tt.path})
		if allowed != tt.allowed {
			t.Errorf("isFileWriteAllowed(%q) = %v, want %v", tt.path, allowed, tt.allowed)
		}
	}
}

func TestPermission_Check(t *testing.T) {
	p := NewPermission(ModeReview)

	// 只读工具始终放行
	allowed, _ := p.Check("read_file", map[string]any{"path": "/tmp/test.txt"})
	if !allowed {
		t.Error("read_file should be allowed")
	}

	// 写入非敏感文件（review 模式 + 空白名单 → 需要审批）
	allowed, reason := p.Check("write_file", map[string]any{"path": "/project/main.go"})
	if allowed {
		t.Errorf("write_file should require approval in review mode: %s", reason)
	}

	// 敏感文件（无论模式，直接拦截）
	allowed, reason = p.Check("write_file", map[string]any{"path": ".env"})
	if allowed {
		t.Errorf("sensitive file should be blocked: %s", reason)
	}

	// 危险 Shell 命令（无论模式，直接拦截）
	allowed, reason = p.Check("bash", map[string]any{"command": "rm -rf /"})
	if allowed {
		t.Errorf("dangerous command should be blocked: %s", reason)
	}

	// 安全命令（review 模式 → 需要审批）
	allowed, reason = p.Check("bash", map[string]any{"command": "echo hello"})
	if allowed {
		t.Errorf("bash should require approval in review mode: %s", reason)
	}
}

func TestPermission_ModeChange(t *testing.T) {
	p := NewPermission(ModeReview)

	// review 模式：写入被拦截
	allowed, _ := p.Check("write_file", map[string]any{"path": "/project/main.go"})
	if allowed {
		t.Error("write should be blocked in review mode")
	}

	// 切换到 yolo
	p.SetMode(ModeYolo)
	allowed, _ = p.Check("write_file", map[string]any{"path": "/project/main.go"})
	if !allowed {
		t.Error("write should be allowed in yolo mode")
	}
}

func TestPermission_AllowlistedShell(t *testing.T) {
	p := NewPermission(ModeReview)
	p.Allowlist().SetShellCommands([]string{"git", "go", "echo"})

	// 白名单命令：即使 review 模式也放行
	allowed, reason := p.Check("bash", map[string]any{"command": "git status"})
	if !allowed {
		t.Errorf("allowlisted command should pass: %s", reason)
	}

	// 非白名单命令：需要审批
	allowed, reason = p.Check("bash", map[string]any{"command": "npm install"})
	if allowed {
		t.Errorf("non-allowlisted command should be blocked: %s", reason)
	}
}

func TestDescribeOperation(t *testing.T) {
	tests := []struct {
		toolName string
		args     map[string]any
		want     string
	}{
		{"write_file", map[string]any{"path": "/tmp/test.txt"}, "Write file: /tmp/test.txt"},
		{"edit_file", map[string]any{"path": "/tmp/test.txt", "old_text": "foo", "new_text": "bar"}, "Edit /tmp/test.txt: replace 'foo' with 'bar'"},
		{"bash", map[string]any{"command": "echo hello"}, "Run command: echo hello"},
		{"unknown", map[string]any{}, "unknown"},
	}

	for _, tt := range tests {
		got := describeOperation(tt.toolName, tt.args)
		if got != tt.want {
			t.Errorf("describeOperation(%q, %v) = %q, want %q", tt.toolName, tt.args, got, tt.want)
		}
	}
}
