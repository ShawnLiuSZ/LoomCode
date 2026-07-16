package main

import (
	"os"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/config"
)

func TestExportEnvToSubprocess(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test-deepseek")
	t.Setenv("OPENAI_API_KEY", "sk-test-openai")
	t.Setenv("LOOMCODE_PROVIDER", "deepseek")
	t.Setenv("IRRELEVANT_KEY", "should_not_appear")

	cfg := &config.Config{
		Providers: []config.ProviderConfig{
			{APIKeyEnv: "DEEPSEEK_API_KEY"},
			{APIKeyEnv: "OPENAI_API_KEY"},
		},
	}

	env := ExportEnvToSubprocess(cfg)

	found := make(map[string]bool)
	for _, e := range env {
		for _, key := range []string{"DEEPSEEK_API_KEY", "OPENAI_API_KEY", "LOOMCODE_PROVIDER", "IRRELEVANT_KEY"} {
			if len(e) > len(key) && e[:len(key)+1] == key+"=" {
				found[key] = true
			}
		}
	}

	if found["DEEPSEEK_API_KEY"] {
		t.Error("DEEPSEEK_API_KEY should NOT be in exported env")
	}
	if found["OPENAI_API_KEY"] {
		t.Error("OPENAI_API_KEY should NOT be in exported env")
	}
	if !found["LOOMCODE_PROVIDER"] {
		t.Error("LOOMCODE_PROVIDER should be in exported env")
	}
	if found["IRRELEVANT_KEY"] {
		t.Error("IRRELEVANT_KEY should NOT be in exported env")
	}
}

func TestExportEnvToSubprocess_Empty(t *testing.T) {
	for _, key := range []string{"DEEPSEEK_API_KEY", "MIMO_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "TAVILY_API_KEY", "LOOMCODE_PROVIDER", "LOOMCODE_MODEL"} {
		os.Unsetenv(key)
	}

	cfg := &config.Config{
		Providers: []config.ProviderConfig{},
	}

	env := ExportEnvToSubprocess(cfg)

	// PATH、HOME、USER 等系统变量应始终存在
	foundPath := false
	for _, e := range env {
		if len(e) >= 5 && e[:5] == "PATH=" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Error("PATH should always be in exported env")
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-env-key")

	tests := []struct {
		name     string
		cfg      config.ProviderConfig
		expected string
	}{
		{
			name:     "api_key takes precedence",
			cfg:      config.ProviderConfig{APIKey: "sk-direct-key", APIKeyEnv: "DEEPSEEK_API_KEY"},
			expected: "sk-direct-key",
		},
		{
			name:     "fallback to api_key_env",
			cfg:      config.ProviderConfig{APIKey: "", APIKeyEnv: "DEEPSEEK_API_KEY"},
			expected: "sk-env-key",
		},
		{
			name:     "empty when both missing",
			cfg:      config.ProviderConfig{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveAPIKey(&tt.cfg); got != tt.expected {
				t.Errorf("resolveAPIKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}
