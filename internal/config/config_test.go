package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时 TOML 配置
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	content := `
default_provider = "deepseek"

[[providers]]
name         = "deepseek"
display_name = "DeepSeek"
kind         = "deepseek"
base_url     = "https://api.deepseek.com"
api_key_env  = "DEEPSEEK_API_KEY"

  [[providers.models]]
  id   = "deepseek-v4-flash"
  name = "DeepSeek V4 Flash"
  context_window = 131072

  [providers.models.capabilities]
  tool_call    = true
  prefix_cache = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DEEPSEEK_API_KEY", "test-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "deepseek")
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}

	p := cfg.Providers[0]
	if p.Name != "deepseek" {
		t.Errorf("provider name = %q, want %q", p.Name, "deepseek")
	}
	if len(p.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(p.Models))
	}
	if p.Models[0].ID != "deepseek-v4-flash" {
		t.Errorf("model ID = %q, want %q", p.Models[0].ID, "deepseek-v4-flash")
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	content := `
[[providers]]
name        = "test"
api_key_env = "MISSING_KEY"

  [[providers.models]]
  id = "m1"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 不设置环境变量，应该报错
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for missing API key env var")
	}
}

func TestGetProvider(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "a"},
			{Name: "b"},
		},
	}

	p, err := cfg.GetProvider("b")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "b" {
		t.Errorf("got %q, want %q", p.Name, "b")
	}

	_, err = cfg.GetProvider("c")
	if err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q", cfg.DefaultProvider)
	}
	if len(cfg.Providers) == 0 {
		t.Error("expected at least one provider in default config")
	}
}

func TestLoadDefault(t *testing.T) {
	// 设置测试用的 API key
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Setenv("MIMO_API_KEY", "test-key")

	// 当前目录没有 loomcode.toml，应返回默认配置
	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q", cfg.DefaultProvider)
	}
}

func TestValidate_DefaultProviderNotFound(t *testing.T) {
	cfg := &Config{
		DefaultProvider: "nonexistent",
		Providers: []ProviderConfig{
			{Name: "deepseek", Kind: "deepseek", BaseURL: "https://api.deepseek.com",
				Models: []ModelConfig{{ID: "m1"}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing default_provider")
	}
}

func TestValidate_ProviderNoName(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Kind: "openai", BaseURL: "https://api.openai.com", Models: []ModelConfig{{ID: "m1"}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for provider without name")
	}
}

func TestValidate_ProviderNoKind(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", BaseURL: "https://api.test.com", Models: []ModelConfig{{ID: "m1"}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for provider without kind")
	}
}

func TestValidate_ProviderUnknownKind(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", Kind: "unknown", BaseURL: "https://api.test.com",
				Models: []ModelConfig{{ID: "m1"}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for unknown provider kind")
	}
}

func TestValidate_ProviderNoBaseURL(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", Kind: "openai", Models: []ModelConfig{{ID: "m1"}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for provider without base_url")
	}
}

func TestValidate_ProviderInvalidURL(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", Kind: "openai", BaseURL: "not-a-url",
				Models: []ModelConfig{{ID: "m1"}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestValidate_ProviderNoModels(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", Kind: "openai", BaseURL: "https://api.openai.com"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for provider without models")
	}
}

func TestValidate_ModelNoID(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", Kind: "openai", BaseURL: "https://api.openai.com",
				Models: []ModelConfig{{Name: "no-id"}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for model without id")
	}
}

func TestValidate_ModelNegativeContextWindow(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", Kind: "openai", BaseURL: "https://api.openai.com",
				Models: []ModelConfig{{ID: "m1", ContextWindow: -100}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative context_window")
	}
}

func TestValidate_ModelNegativeCost(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{Name: "test", Kind: "openai", BaseURL: "https://api.openai.com",
				Models: []ModelConfig{{ID: "m1", Cost: CostConfig{Input: -1}}}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative cost")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		DefaultProvider: "deepseek",
		Providers: []ProviderConfig{
			{
				Name: "deepseek", Kind: "deepseek", BaseURL: "https://api.deepseek.com",
				Models: []ModelConfig{{ID: "m1", ContextWindow: 131072, Cost: CostConfig{Input: 0.14, Output: 0.28}}},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestLoad_Validation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")

	content := `
[[providers]]
name = "test"
kind = "unknown"
base_url = "https://api.test.com"

  [[providers.models]]
  id = "m1"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}
