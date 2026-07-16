package control

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceTrust_LoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trust.json")

	wt := NewWorkspaceTrust()
	if err := wt.Load(path); err != nil {
		t.Fatalf("load empty trust file: %v", err)
	}

	if err := wt.SetDecision("/workspace", TrustAllowed); err != nil {
		t.Fatalf("set decision: %v", err)
	}
	if err := wt.AllowOutsideAccess("/outside/file.txt", true); err != nil {
		t.Fatalf("allow outside access: %v", err)
	}

	// 重新加载验证持久化
	wt2 := NewWorkspaceTrust()
	if err := wt2.Load(path); err != nil {
		t.Fatalf("reload trust file: %v", err)
	}
	if !wt2.IsTrusted("/workspace") {
		t.Error("workspace should be trusted after reload")
	}
	if !wt2.IsOutsideAccessAllowed("/outside/file.txt") {
		t.Error("outside path should be allowed after reload")
	}
}

func TestWorkspaceTrust_TemporaryAccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trust.json")

	wt := NewWorkspaceTrust()
	if err := wt.Load(path); err != nil {
		t.Fatalf("load empty trust file: %v", err)
	}

	if err := wt.AllowOutsideAccess("/tmp/session.txt", false); err != nil {
		t.Fatalf("allow temporary access: %v", err)
	}
	if !wt.IsOutsideAccessAllowed("/tmp/session.txt") {
		t.Error("temporary path should be allowed in-process")
	}

	// 临时授权不应持久化
	wt2 := NewWorkspaceTrust()
	if err := wt2.Load(path); err != nil {
		t.Fatalf("reload trust file: %v", err)
	}
	if wt2.IsOutsideAccessAllowed("/tmp/session.txt") {
		t.Error("temporary path should not persist")
	}
}

func TestWorkspaceTrust_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trust.json")

	wt := NewWorkspaceTrust()
	if err := wt.Load(path); err != nil {
		t.Fatalf("load empty trust file: %v", err)
	}
	if err := wt.SetDecision("/workspace", TrustAllowed); err != nil {
		t.Fatalf("set decision: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat trust file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("trust file mode = %o, want 0600", info.Mode().Perm())
	}
}
