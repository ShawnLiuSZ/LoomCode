package lsp

import (
	"os"
	"testing"
)

func TestFilterEnvForSubprocess(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	t.Setenv("MIMO_API_KEY", "sk-mimo")
	t.Setenv("SOME_OTHER_VAR", "not_relevant")

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
	apiKeyPrefixes := []string{"DEEPSEEK_API_KEY=", "MIMO_API_KEY="}
	for _, e := range env {
		for _, prefix := range apiKeyPrefixes {
			if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
				t.Errorf("%s should NOT be in filtered env", prefix[:len(prefix)-1])
			}
		}
	}

	if !found["PATH"] {
		t.Error("PATH should be in filtered env")
	}
	if !found["LANG"] {
		t.Error("LANG should be in filtered env")
	}

	for _, e := range env {
		if len(e) >= len("SOME_OTHER_VAR") && e[:len("SOME_OTHER_VAR")+1] == "SOME_OTHER_VAR=" {
			t.Error("SOME_OTHER_VAR should NOT be in filtered env")
		}
	}
}

func TestFilterEnvForSubprocess_SystemVars(t *testing.T) {
	env := filterEnvForSubprocess()

	systemVars := []string{"PATH", "HOME", "USER", "LANG"}
	for _, sv := range systemVars {
		found := false
		for _, e := range env {
			if len(e) > len(sv) && e[:len(sv)+1] == sv+"=" {
				found = true
				break
			}
		}
		if !found {
			t.Logf("%s not found (may be absent on this system)", sv)
		}
	}
}

func TestFilterEnvForSubprocess_EmptyEnv(t *testing.T) {
	// 清空 API keys
	for _, key := range []string{"DEEPSEEK_API_KEY", "MIMO_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		os.Unsetenv(key)
	}

	env := filterEnvForSubprocess()

	// 仍应包含系统变量
	if len(env) == 0 {
		t.Error("filtered env should not be empty (system vars)")
	}
}
