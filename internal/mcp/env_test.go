package mcp

import (
	"os"
	"testing"
)

func TestFilterEnvForSubprocess(t *testing.T) {
	// 设置测试环境变量
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("ANTHROPIC_API_KEY", "sk-anthropic")
	t.Setenv("IRRELEVANT_VAR", "should_not_appear")

	env := filterEnvForSubprocess()

	found := make(map[string]bool)
	for _, e := range env {
		for _, key := range []string{"PATH", "HOME", "USER", "LANG"} {
			if len(e) > len(key) && e[:len(key)+1] == key+"=" {
				found[key] = true
			}
		}
	}

	// API keys should NOT be in filtered env (security)
	for _, e := range env {
		for _, key := range []string{"DEEPSEEK_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
			if len(e) > len(key) && e[:len(key)+1] == key+"=" {
				t.Errorf("%s should NOT be in filtered env", key)
			}
		}
	}

	if !found["PATH"] {
		t.Error("PATH should be in filtered env")
	}
	if !found["HOME"] {
		t.Error("HOME should be in filtered env")
	}

	// 验证无关变量不出现
	for _, e := range env {
		if len(e) >= len("IRRELEVANT_VAR") && e[:len("IRRELEVANT_VAR")+1] == "IRRELEVANT_VAR=" {
			t.Error("IRRELEVANT_VAR should NOT be in filtered env")
		}
	}
}

func TestFilterEnvForSubprocess_EmptyKeys(t *testing.T) {
	// 清除所有 API key
	for _, key := range []string{"DEEPSEEK_API_KEY", "MIMO_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		os.Unsetenv(key)
	}

	env := filterEnvForSubprocess()

	// PATH, HOME, USER 等系统变量应始终存在
	foundPath := false
	for _, e := range env {
		if len(e) >= 5 && e[:5] == "PATH=" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Error("PATH should always be in filtered env")
	}
}

func TestFilterEnvForSubprocess_NoDuplicates(t *testing.T) {
	env := filterEnvForSubprocess()

	seen := make(map[string]bool)
	for _, e := range env {
		if seen[e] {
			t.Errorf("duplicate env entry: %s", e)
		}
		seen[e] = true
	}
}
