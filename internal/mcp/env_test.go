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

// TestMergeEnv_ExtraOverridesBase #1 修复：extra 应覆盖 base 同名 key。
func TestMergeEnv_ExtraOverridesBase(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/home/user", "USER=u", "LANG=C"}
	extra := map[string]string{
		"PATH":  "/custom/bin:/usr/bin",
		"USER":  "override-user",
		"NEW_KEY": "new-value",
	}

	merged := mergeEnv(base, extra)

	got := make(map[string]string)
	for _, e := range merged {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				got[e[:i]] = e[i+1:]
				break
			}
		}
	}

	if got["PATH"] != "/custom/bin:/usr/bin" {
		t.Errorf("PATH = %q, want /custom/bin:/usr/bin", got["PATH"])
	}
	if got["USER"] != "override-user" {
		t.Errorf("USER = %q, want override-user", got["USER"])
	}
	if got["NEW_KEY"] != "new-value" {
		t.Errorf("NEW_KEY = %q, want new-value", got["NEW_KEY"])
	}
	if got["HOME"] != "/home/user" {
		t.Errorf("HOME = %q, want /home/user", got["HOME"])
	}
	if got["LANG"] != "C" {
		t.Errorf("LANG = %q, want C", got["LANG"])
	}
}

func TestMergeEnv_EmptyExtra(t *testing.T) {
	base := []string{"PATH=/usr/bin"}
	merged := mergeEnv(base, nil)
	if len(merged) != 1 || merged[0] != "PATH=/usr/bin" {
		t.Errorf("expected base unchanged, got %v", merged)
	}
}
