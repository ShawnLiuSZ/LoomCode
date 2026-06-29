package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/skills"
	"github.com/ShawnLiuSZ/Helix/internal/testutil"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

func TestSetSkillsManager_RegistersSkillTool(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	a := New(p, tool.NewRegistry())

	if _, ok := a.tools.Get("skill"); ok {
		t.Fatal("skill tool should not exist before SetSkillsManager")
	}

	a.SetSkillsManager(skills.NewManager())

	if _, ok := a.tools.Get("skill"); !ok {
		t.Error("SetSkillsManager should register the skill tool so the model can load skills")
	}
}

func TestBuildSystemPrompt_SkillGuidanceMentionsTool(t *testing.T) {
	// Real skill on a temp HOME so the skill tool has something to list.
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillDir := filepath.Join(home, ".helix", "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Make a git commit"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := skills.NewManager()
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}

	p := testutil.NewStubProvider(nil)
	a := New(p, tool.NewRegistry())
	a.SetSkillsManager(mgr)

	prompt := a.buildSystemPrompt()
	// Must point the model at the `skill` tool, not the human-only /skills UI command.
	if strings.Contains(prompt, "/skills") {
		t.Error("system prompt should not tell the model to use the /skills UI command")
	}
	if !strings.Contains(prompt, "skill") {
		t.Error("system prompt should reference the skill tool")
	}
	// Skills 列表已移至按需工具，不应再注入 system prompt。
	if strings.Contains(prompt, "commit") {
		t.Errorf("system prompt should not list skill names; use the skill tool instead, got:\n%s", prompt)
	}

	// skill 工具无参数应列出已加载的 skill（渐进式披露入口）。
	tl, ok := a.tools.Get("skill")
	if !ok {
		t.Fatal("skill tool should be registered after SetSkillsManager")
	}
	res, err := tl.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(res.Content, "commit") {
		t.Errorf("skill tool with no args should list available skills, got: %s", res.Content)
	}
}
