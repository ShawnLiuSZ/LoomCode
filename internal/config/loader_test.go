package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeConfigs_ProjectOverridesGlobal(t *testing.T) {
	global := &Config{
		DefaultProvider: "deepseek",
		Env: map[string]string{
			"DEEPSEEK_API_KEY": "global-key",
		},
	}
	project := &Config{
		DefaultProvider: "openai",
		Env: map[string]string{
			"OPENAI_API_KEY": "project-key",
		},
	}

	merged := mergeConfigs(global, project)

	if merged.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %q, want %q", merged.DefaultProvider, "openai")
	}
	if merged.Env["DEEPSEEK_API_KEY"] != "global-key" {
		t.Errorf("expected global-key for DEEPSEEK_API_KEY, got %q", merged.Env["DEEPSEEK_API_KEY"])
	}
	if merged.Env["OPENAI_API_KEY"] != "project-key" {
		t.Errorf("expected project-key for OPENAI_API_KEY, got %q", merged.Env["OPENAI_API_KEY"])
	}
}

func TestMergeConfigs_ProjectOverridesEnv(t *testing.T) {
	global := &Config{
		Env: map[string]string{
			"API_KEY": "global-key",
		},
	}
	project := &Config{
		Env: map[string]string{
			"API_KEY": "project-key",
		},
	}

	merged := mergeConfigs(global, project)

	if merged.Env["API_KEY"] != "project-key" {
		t.Errorf("expected project-key, got %q", merged.Env["API_KEY"])
	}
}

func TestMergeConfigs_NilProject(t *testing.T) {
	global := &Config{
		DefaultProvider: "deepseek",
		Env:             map[string]string{"KEY": "val"},
	}

	merged := mergeConfigs(global, nil)

	if merged.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q, want deepseek", merged.DefaultProvider)
	}
	if merged.Env["KEY"] != "val" {
		t.Errorf("expected val for KEY, got %q", merged.Env["KEY"])
	}
}

func TestMergeConfigs_NilEnv(t *testing.T) {
	global := &Config{DefaultProvider: "deepseek"}
	project := &Config{Env: map[string]string{"KEY": "val"}}

	merged := mergeConfigs(global, project)

	if merged.Env == nil {
		t.Fatal("expected non-nil Env")
	}
	if merged.Env["KEY"] != "val" {
		t.Errorf("expected val, got %q", merged.Env["KEY"])
	}
}

func TestMergeConfigs_Permissions(t *testing.T) {
	global := &Config{Permissions: PermissionConfig{ShellAllowlist: []string{"ls"}}}
	project := &Config{Permissions: PermissionConfig{ShellAllowlist: []string{"git"}}}

	merged := mergeConfigs(global, project)

	if len(merged.Permissions.ShellAllowlist) != 1 || merged.Permissions.ShellAllowlist[0] != "git" {
		t.Errorf("expected [git], got %v", merged.Permissions.ShellAllowlist)
	}
}

func TestMergeConfigs_Search(t *testing.T) {
	global := &Config{Search: SearchConfig{Engine: "bing"}}
	project := &Config{Search: SearchConfig{Engine: "tavily"}}

	merged := mergeConfigs(global, project)

	if merged.Search.Engine != "tavily" {
		t.Errorf("Search.Engine = %q, want tavily", merged.Search.Engine)
	}
}

func TestMergeConfigs_Agent(t *testing.T) {
	global := &Config{Agent: AgentConfig{PlannerModel: "gpt-4"}}
	project := &Config{Agent: AgentConfig{PlannerModel: "deepseek-v4"}}

	merged := mergeConfigs(global, project)

	if merged.Agent.PlannerModel != "deepseek-v4" {
		t.Errorf("Agent.PlannerModel = %q, want deepseek-v4", merged.Agent.PlannerModel)
	}
}

func TestLoadWithProject_NoProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()
	home := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(home, ".loomcode"), 0755)

	cfgContent := `{"default_provider": "deepseek"}`
	os.WriteFile(filepath.Join(home, ".loomcode", "models.json"), []byte(cfgContent), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", origHome)

	cfg, err := LoadWithProject("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q, want deepseek", cfg.DefaultProvider)
	}
}

func TestLoadWithProject_EmptyProjectDir(t *testing.T) {
	// Empty projectDir should return global config
	tmpDir := t.TempDir()
	home := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(home, ".loomcode"), 0755)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", origHome)

	cfg, err := LoadWithProject("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get default config since no global config files exist
	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q, want deepseek", cfg.DefaultProvider)
	}
}

func TestLoadWithProject_ProjectOverridesGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up global config
	home := filepath.Join(tmpDir, "home")
	globalDir := filepath.Join(home, ".loomcode")
	os.MkdirAll(globalDir, 0755)
	globalContent := `{
  "default_provider": "deepseek",
  "providers": [{
    "name": "deepseek",
    "kind": "deepseek",
    "base_url": "https://api.deepseek.com",
    "api_key_env": "DEEPSEEK_API_KEY",
    "models": [{"id": "deepseek-v4-flash", "context_window": 131072}]
  }]
}`
	os.WriteFile(filepath.Join(globalDir, "models.json"), []byte(globalContent), 0644)

	// Set up project config with different provider
	projectDir := filepath.Join(tmpDir, "project")
	projectConfigDir := filepath.Join(projectDir, ".loomcode")
	os.MkdirAll(projectConfigDir, 0755)
	projectContent := `{
  "default_provider": "openai",
  "providers": [{
    "name": "openai",
    "kind": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key_env": "OPENAI_API_KEY",
    "models": [{"id": "gpt-4o", "context_window": 128000}]
  }]
}`
	os.WriteFile(filepath.Join(projectConfigDir, "settings.json"), []byte(projectContent), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", origHome)

	cfg, err := LoadWithProject(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %q, want openai", cfg.DefaultProvider)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "openai" {
		t.Errorf("expected provider openai, got %+v", cfg.Providers)
	}
}

func TestLoadWithProject_EnvMerge(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up global config
	home := filepath.Join(tmpDir, "home")
	globalDir := filepath.Join(home, ".loomcode")
	os.MkdirAll(globalDir, 0755)
	globalContent := `{
  "default_provider": "deepseek",
  "providers": [{
    "name": "deepseek",
    "kind": "deepseek",
    "base_url": "https://api.deepseek.com",
    "api_key_env": "DEEPSEEK_API_KEY",
    "models": [{"id": "deepseek-v4-flash", "context_window": 131072}]
  }],
  "env": {"DEEPSEEK_API_KEY": "global-key"}
}`
	os.WriteFile(filepath.Join(globalDir, "models.json"), []byte(globalContent), 0644)

	// Set up project config with additional env
	projectDir := filepath.Join(tmpDir, "project")
	projectConfigDir := filepath.Join(projectDir, ".loomcode")
	os.MkdirAll(projectConfigDir, 0755)
	projectContent := `{
  "default_provider": "deepseek",
  "providers": [{
    "name": "deepseek",
    "kind": "deepseek",
    "base_url": "https://api.deepseek.com",
    "api_key_env": "DEEPSEEK_API_KEY",
    "models": [{"id": "deepseek-v4-flash", "context_window": 131072}]
  }],
  "env": {"DEEPSEEK_API_KEY": "project-key", "EXTRA_KEY": "extra-val"}
}`
	os.WriteFile(filepath.Join(projectConfigDir, "settings.json"), []byte(projectContent), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", origHome)

	cfg, err := LoadWithProject(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Env["DEEPSEEK_API_KEY"] != "project-key" {
		t.Errorf("DEEPSEEK_API_KEY = %q, want project-key", cfg.Env["DEEPSEEK_API_KEY"])
	}
	if cfg.Env["EXTRA_KEY"] != "extra-val" {
		t.Errorf("EXTRA_KEY = %q, want extra-val", cfg.Env["EXTRA_KEY"])
	}
}
