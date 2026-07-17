package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时 JSON 配置
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	content := `{
  "default_provider": "deepseek",
  "providers": [
    {
      "name": "deepseek",
      "display_name": "DeepSeek",
      "kind": "deepseek",
      "base_url": "https://api.deepseek.com",
      "api_key_env": "DEEPSEEK_API_KEY",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek V4 Flash",
          "context_window": 131072,
          "capabilities": {
            "tool_call": true,
            "prefix_cache": true
          }
        }
      ]
    }
  ]
}`
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
	path := filepath.Join(dir, "test.json")

	content := `{
  "providers": [
    {
      "name": "test",
      "kind": "openai",
      "base_url": "https://api.example.com/v1",
      "api_key_env": "MISSING_KEY",
      "models": [{ "id": "m1" }]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 不设置环境变量，应加载成功（打印警告但不阻止），
	// 模型仍可在 /model 中列出，仅实际调用时才会鉴权失败。
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() should succeed with missing API key (warn-only), got error: %v", err)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Models[0].ID != "m1" {
		t.Errorf("expected provider 'test' with model 'm1', got %+v", cfg.Providers)
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

	// 当前目录没有 loomcode.json，应返回默认配置
	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q", cfg.DefaultProvider)
	}
}

func TestLoadDefault_EmptyLocalFallsBack(t *testing.T) {
	// 模拟项目目录： loomcode.json 只有空数组，没有 provider
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	emptyLocal := filepath.Join(projectDir, "loomcode.json")
	if err := os.WriteFile(emptyLocal, []byte(`{"providers": []}`), 0644); err != nil {
		t.Fatal(err)
	}

	globalConfigDir := filepath.Join(globalDir, ".loomcode")
	if err := os.MkdirAll(globalConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	globalConfig := filepath.Join(globalConfigDir, "loomcode.json")
	content := `{
  "default_provider": "mimo",
  "providers": [
    {
      "name": "mimo",
      "display_name": "MiMo",
      "kind": "mimo",
      "base_url": "https://api.mimo.xiaomi.com/v1",
      "api_key_env": "MIMO_API_KEY",
      "models": [
        {
          "id": "mimo-v2.5-pro",
          "name": "MiMo V2.5 Pro",
          "context_window": 262144
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(globalConfig, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", globalDir)
	t.Setenv("MIMO_API_KEY", "test-key")
	t.Chdir(projectDir)

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if cfg.DefaultProvider != "mimo" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "mimo")
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "mimo" {
		t.Errorf("expected provider 'mimo', got %+v", cfg.Providers)
	}
}

func TestLoadDefault_AllEmptyFallsBackToDefault(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	emptyLocal := filepath.Join(projectDir, "loomcode.json")
	if err := os.WriteFile(emptyLocal, []byte(`{"providers": []}`), 0644); err != nil {
		t.Fatal(err)
	}
	globalConfigDir := filepath.Join(globalDir, ".loomcode")
	if err := os.MkdirAll(globalConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	emptyGlobal := filepath.Join(globalConfigDir, "loomcode.json")
	if err := os.WriteFile(emptyGlobal, []byte(`{"providers": []}`), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", globalDir)
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Chdir(projectDir)

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "deepseek")
	}
	if len(cfg.Providers) == 0 {
		t.Error("expected default providers")
	}
}

func TestLoadConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	content := `{
  "default_provider": "openai",
  "providers": [
    {
      "name": "openai",
      "kind": "openai",
      "base_url": "https://api.openai.com/v1",
      "api_key_env": "OPENAI_API_KEY",
      "models": [
        {
          "id": "gpt-4o",
          "context_window": 128000,
          "capabilities": { "tool_call": true }
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("OPENAI_API_KEY", "test-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "openai")
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "openai" {
		t.Errorf("expected provider 'openai', got %+v", cfg.Providers)
	}
}

func TestLoadDefault_ModelsJSON(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	globalConfigDir := filepath.Join(globalDir, ".loomcode")
	if err := os.MkdirAll(globalConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	modelsJSON := filepath.Join(globalConfigDir, "models.json")
	content := `{
  "default_provider": "mimo",
  "providers": [
    {
      "name": "mimo",
      "kind": "mimo",
      "base_url": "https://api.mimo.xiaomi.com/v1",
      "api_key_env": "MIMO_API_KEY",
      "models": [{ "id": "mimo-v2.5-pro", "context_window": 262144 }]
    }
  ]
}`
	if err := os.WriteFile(modelsJSON, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", globalDir)
	t.Setenv("MIMO_API_KEY", "test-key")
	t.Chdir(projectDir)

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if cfg.DefaultProvider != "mimo" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "mimo")
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "mimo" {
		t.Errorf("expected provider 'mimo', got %+v", cfg.Providers)
	}
}

func TestLoadDefault_LoomcodeJsonPriorityOverModelsJson(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	globalConfigDir := filepath.Join(globalDir, ".loomcode")
	if err := os.MkdirAll(globalConfigDir, 0755); err != nil {
		t.Fatal(err)
	}

	loomcodeJSON := filepath.Join(globalConfigDir, "loomcode.json")
	if err := os.WriteFile(loomcodeJSON, []byte(`{
  "default_provider": "openai",
  "providers": [
    {
      "name": "openai",
      "kind": "openai",
      "base_url": "https://api.openai.com/v1",
      "api_key_env": "OPENAI_API_KEY",
      "models": [{ "id": "gpt-4o", "context_window": 128000 }]
    }
  ]
}`), 0644); err != nil {
		t.Fatal(err)
	}

	modelsJSON := filepath.Join(globalConfigDir, "models.json")
	if err := os.WriteFile(modelsJSON, []byte(`{
  "default_provider": "mimo",
  "providers": [
    {
      "name": "mimo",
      "kind": "mimo",
      "base_url": "https://api.mimo.xiaomi.com/v1",
      "api_key_env": "MIMO_API_KEY",
      "models": [{ "id": "mimo-v2.5-pro", "context_window": 262144 }]
    }
  ]
}`), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", globalDir)
	t.Setenv("MIMO_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Chdir(projectDir)

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	// loomcode.json has higher priority than models.json
	if cfg.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %q, want %q (loomcode.json should win)", cfg.DefaultProvider, "openai")
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "openai" {
		t.Errorf("expected provider 'openai', got %+v", cfg.Providers)
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
	path := filepath.Join(dir, "bad.json")

	content := `{
  "providers": [
    {
      "name": "test",
      "kind": "unknown",
      "base_url": "https://api.test.com",
      "models": [{ "id": "m1" }]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}
