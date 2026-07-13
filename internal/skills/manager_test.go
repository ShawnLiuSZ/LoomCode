package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_Load(t *testing.T) {
	home := t.TempDir()

	// 创建 ~/.agents/skills/test-skill/SKILL.md
	agentsDir := filepath.Join(home, ".agents", "skills", "test-skill")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "SKILL.md"), []byte("# Test Skill\nA test skill for testing"), 0644)

	// 创建 ~/.loomcode/skills/loomcode-skill/SKILL.md
	loomcodeDir := filepath.Join(home, ".loomcode", "skills", "loomcode-skill")
	os.MkdirAll(loomcodeDir, 0755)
	os.WriteFile(filepath.Join(loomcodeDir, "SKILL.md"), []byte("# LoomCode Skill\nA loomcode-specific skill"), 0644)

	// 覆盖测试：同名 skill 在两个目录都存在
	overrideDir := filepath.Join(home, ".agents", "skills", "override-skill")
	os.MkdirAll(overrideDir, 0755)
	os.WriteFile(filepath.Join(overrideDir, "SKILL.md"), []byte("# Agents Override\nFrom agents"), 0644)

	loomcodeOverrideDir := filepath.Join(home, ".loomcode", "skills", "override-skill")
	os.MkdirAll(loomcodeOverrideDir, 0755)
	os.WriteFile(filepath.Join(loomcodeOverrideDir, "SKILL.md"), []byte("# LoomCode Override\nFrom loomcode"), 0644)

	// 设置 HOME 环境变量
	t.Setenv("HOME", home)

	m := NewManager()
	err := m.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if m.Count() != 3 {
		t.Errorf("Count() = %d, want 3", m.Count())
	}

	// 验证 loomcode-skill
	s, ok := m.Get("loomcode-skill")
	if !ok {
		t.Fatal("loomcode-skill not found")
	}
	if s.Source != "loomcode" {
		t.Errorf("loomcode-skill source = %q, want loomcode", s.Source)
	}

	// 验证 override-skill 来自 loomcode
	os2, ok := m.Get("override-skill")
	if !ok {
		t.Fatal("override-skill not found")
	}
	if os2.Source != "loomcode" {
		t.Errorf("override-skill source = %q, want loomcode (should override agents)", os2.Source)
	}
	if os2.Description != "LoomCode Override" {
		t.Errorf("override-skill description = %q, want 'LoomCode Override'", os2.Description)
	}
}

func TestManager_List(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".loomcode", "skills")
	os.MkdirAll(filepath.Join(dir, "skill-a"), 0755)
	os.WriteFile(filepath.Join(dir, "skill-a", "SKILL.md"), []byte("# Skill A"), 0644)
	os.MkdirAll(filepath.Join(dir, "skill-b"), 0755)
	os.WriteFile(filepath.Join(dir, "skill-b", "SKILL.md"), []byte("# Skill B"), 0644)

	m := NewManager()
	m.Load()

	list := m.List()
	if len(list) != 2 {
		t.Errorf("List() count = %d, want 2", len(list))
	}
	if list[0].Name != "skill-a" {
		t.Errorf("first skill = %q, want skill-a", list[0].Name)
	}
}

func TestManager_LoadEmptyDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := NewManager()
	err := m.Load()
	if err != nil {
		t.Fatalf("Load() should not error on missing dirs: %v", err)
	}
	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
}

func TestSkill_Content(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".loomcode", "skills", "readme-skill")
	os.MkdirAll(dir, 0755)
	content := "# README Skill\n\nThis is the full content of the skill."
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)

	m := NewManager()
	m.Load()

	s, ok := m.Get("readme-skill")
	if !ok {
		t.Fatal("skill not found")
	}

	c, err := s.Content()
	if err != nil {
		t.Fatalf("Content() error: %v", err)
	}
	if c != content {
		t.Errorf("Content() = %q, want %q", c, content)
	}
}

func TestReadDescription_NoFile(t *testing.T) {
	m := NewManager()
	desc := m.readDescription("/nonexistent/path")
	if desc != "" {
		t.Errorf("expected empty description, got %q", desc)
	}
}
