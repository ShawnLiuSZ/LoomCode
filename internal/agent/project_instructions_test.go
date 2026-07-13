package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

func TestLoadProjectInstructions_NoFiles(t *testing.T) {
	dir := t.TempDir()
	result := loadProjectInstructions(dir)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestLoadProjectInstructions_SingleFile(t *testing.T) {
	dir := t.TempDir()
	content := "# My Project\nFollow these rules."
	os.WriteFile(filepath.Join(dir, "LOOMCODE.md"), []byte(content), 0644)

	result := loadProjectInstructions(dir)
	if result != content {
		t.Errorf("got %q, want %q", result, content)
	}
}

func TestLoadProjectInstructions_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".loomcode"), 0755)
	os.WriteFile(filepath.Join(dir, "LOOMCODE.md"), []byte("# Base instructions"), 0644)
	os.WriteFile(filepath.Join(dir, "LOOMCODE.local.md"), []byte("# Local overrides"), 0644)
	os.WriteFile(filepath.Join(dir, ".loomcode", "instructions.md"), []byte("# Extra notes"), 0644)

	result := loadProjectInstructions(dir)
	if !strings.Contains(result, "# Base instructions") {
		t.Error("missing LOOMCODE.md content")
	}
	if !strings.Contains(result, "# Local overrides") {
		t.Error("missing LOOMCODE.local.md content")
	}
	if !strings.Contains(result, "# Extra notes") {
		t.Error("missing .loomcode/instructions.md content")
	}
}

func TestLoadProjectInstructions_PartialFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "LOOMCODE.local.md"), []byte("local only"), 0644)

	result := loadProjectInstructions(dir)
	if result != "local only" {
		t.Errorf("got %q, want %q", result, "local only")
	}
}

func TestLoadProjectInstructions_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "LOOMCODE.md"), []byte("   \n  "), 0644)

	result := loadProjectInstructions(dir)
	if result != "" {
		t.Errorf("expected empty for whitespace-only file, got %q", result)
	}
}

func TestBuildSystemPrompt_WithProjectInstructions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "LOOMCODE.md"), []byte("# Custom Rules\nDo X."), 0644)

	p := testutil.NewStubProvider(func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: "ok"}, nil
	})
	r := tool.NewRegistry()
	a := New(p, r)
	a.SetWorkDir(dir)

	prompt := a.buildSystemPrompt()
	if !strings.Contains(prompt, "## Project Instructions") {
		t.Error("system prompt should contain Project Instructions header")
	}
	if !strings.Contains(prompt, "Do X.") {
		t.Error("system prompt should contain LOOMCODE.md content")
	}
}

func TestBuildSystemPrompt_NoWorkDir(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	a := New(p, r)

	prompt := a.buildSystemPrompt()
	if strings.Contains(prompt, "Project Instructions") {
		t.Error("system prompt should not contain Project Instructions when no workDir")
	}
}

func TestBuildSystemPrompt_NoInstructionsFile(t *testing.T) {
	dir := t.TempDir()
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	a := New(p, r)
	a.SetWorkDir(dir)

	prompt := a.buildSystemPrompt()
	if strings.Contains(prompt, "Project Instructions") {
		t.Error("system prompt should not contain Project Instructions when no files found")
	}
}
