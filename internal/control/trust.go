package control

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TrustDecision 工作区信任决策。
type TrustDecision string

const (
	// TrustAllowed 永久信任该工作区。
	TrustAllowed TrustDecision = "allowed"
	// TrustDenied 不信任该工作区（受限模式）。
	TrustDenied TrustDecision = "denied"
)

// OutsideAccess 记录对工作区外路径的访问授权。
type OutsideAccess struct {
	Path      string `json:"path"`
	Permanent bool   `json:"permanent"`
}

// trustRecord 持久化到磁盘的信任记录。
type trustRecord struct {
	Workspaces    map[string]TrustDecision `json:"workspaces"`
	OutsidePaths  []OutsideAccess          `json:"outside_paths"`
}

// WorkspaceTrust 管理工作区信任状态及工作区外文件访问授权。
// 设计为并发安全，可在 TUI 主线程与工具执行 goroutine 之间共享。
type WorkspaceTrust struct {
	mu sync.RWMutex

	path string

	workspaces   map[string]TrustDecision
	outsidePaths []OutsideAccess

	// temporaryPaths 仅在当前进程有效，退出后丢弃。
	temporaryPaths map[string]bool
}

// NewWorkspaceTrust 创建信任管理器（不自动加载文件）。
func NewWorkspaceTrust() *WorkspaceTrust {
	return &WorkspaceTrust{
		workspaces:     make(map[string]TrustDecision),
		temporaryPaths: make(map[string]bool),
	}
}

// Load 从 path 加载信任记录；文件不存在时返回空管理器。
func (t *WorkspaceTrust) Load(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read trust file: %w", err)
	}

	var rec trustRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return fmt.Errorf("parse trust file: %w", err)
	}

	t.workspaces = rec.Workspaces
	if t.workspaces == nil {
		t.workspaces = make(map[string]TrustDecision)
	}
	t.outsidePaths = rec.OutsidePaths
	return nil
}

// Save 将信任记录持久化到磁盘。
func (t *WorkspaceTrust) Save() error {
	t.mu.RLock()
	rec := trustRecord{
		Workspaces:   t.workspaces,
		OutsidePaths: t.outsidePaths,
	}
	path := t.path
	t.mu.RUnlock()

	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create trust dir: %w", err)
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trust: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write trust file: %w", err)
	}
	return nil
}

// Decision 返回工作区信任决策；未记录时返回空字符串。
func (t *WorkspaceTrust) Decision(workspace string) TrustDecision {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.workspaces[workspace]
}

// SetDecision 设置工作区信任决策并持久化。
func (t *WorkspaceTrust) SetDecision(workspace string, d TrustDecision) error {
	t.mu.Lock()
	t.workspaces[workspace] = d
	t.mu.Unlock()
	return t.Save()
}

// IsTrusted 报告工作区是否已被信任。
func (t *WorkspaceTrust) IsTrusted(workspace string) bool {
	return t.Decision(workspace) == TrustAllowed
}

// AllowOutsideAccess 授权访问工作区外的 path。
// permanent=true 时持久化；permanent=false 时仅在当前进程有效。
func (t *WorkspaceTrust) AllowOutsideAccess(path string, permanent bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	t.mu.Lock()
	if permanent {
		// 去重：若已存在则更新为永久。
		found := false
		for i := range t.outsidePaths {
			if t.outsidePaths[i].Path == abs {
				t.outsidePaths[i].Permanent = true
				found = true
				break
			}
		}
		if !found {
			t.outsidePaths = append(t.outsidePaths, OutsideAccess{Path: abs, Permanent: true})
		}
	} else {
		t.temporaryPaths[abs] = true
	}
	t.mu.Unlock()

	if permanent {
		return t.Save()
	}
	return nil
}

// PromptAndAllow 提示用户是否允许访问工作区外 path，并将用户选择记录到信任配置。
// 返回的 error 包含用户取消或输入错误；若用户拒绝访问，返回 nil error（调用方应继续拒绝操作）。
func (t *WorkspaceTrust) PromptAndAllow(path string) error {
	granted, permanent, err := PromptOutsideAccess(path)
	if err != nil {
		return err
	}
	if !granted {
		return nil
	}
	return t.AllowOutsideAccess(path, permanent)
}

// IsOutsideAccessAllowed 报告是否允许访问工作区外的 path。
func (t *WorkspaceTrust) IsOutsideAccessAllowed(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.temporaryPaths[abs] {
		return true
	}
	for _, p := range t.outsidePaths {
		if p.Path == abs {
			return true
		}
	}
	return false
}

// PromptTrust 通过 stdin 询问用户是否信任工作区。
// 返回决策与是否由用户显式确认（quit 时返回 error）。
func PromptTrust(workspace string) (TrustDecision, error) {
	fmt.Printf("\n是否信任工作区 %q？\n", workspace)
	fmt.Println("  Y - 信任（允许工具访问该工作区内的文件）")
	fmt.Println("  n - 不信任（受限模式，仅允许只读操作）")
	fmt.Println("  q - 退出程序")
	fmt.Print("请选择 [Y/n/q]: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}
		line = strings.TrimSpace(strings.ToLower(line))
		switch line {
		case "", "y", "yes":
			return TrustAllowed, nil
		case "n", "no":
			return TrustDenied, nil
		case "q", "quit":
			return "", fmt.Errorf("user quit")
		default:
			fmt.Print("请输入 Y/n/q: ")
		}
	}
}

// PromptOutsideAccess 通过 stdin 询问用户对工作区外路径的访问授权。
// 返回 (granted, permanent, error)。
func PromptOutsideAccess(path string) (bool, bool, error) {
	fmt.Printf("\n请求访问工作区外文件: %q\n", path)
	fmt.Println("  t - 临时访问（仅本次会话有效）")
	fmt.Println("  p - 永久访问（保存到信任配置）")
	fmt.Println("  n - 拒绝访问")
	fmt.Print("请选择 [t/p/n]: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return false, false, fmt.Errorf("read input: %w", err)
		}
		line = strings.TrimSpace(strings.ToLower(line))
		switch line {
		case "t", "temp", "temporary":
			return true, false, nil
		case "p", "perm", "permanent":
			return true, true, nil
		case "n", "no", "deny":
			return false, false, nil
		default:
			fmt.Print("请输入 t/p/n: ")
		}
	}
}
