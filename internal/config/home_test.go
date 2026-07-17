package config

import "testing"

// Low: HOME 不可用时，homeDir 不得回退到 cwd（否则用户配置路径退化为 ./.loomcode/loomcode.json，
// 可被同目录恶意配置注入）。
func TestHomeDir_NoCwdFallback(t *testing.T) {
	t.Setenv("HOME", "")
	home, ok := homeDir()
	if ok {
		t.Errorf("expected homeDir unavailable when HOME empty, got ok=true home=%q", home)
	}
	if home != "" {
		t.Errorf("homeDir must be empty (no cwd fallback) when unavailable, got %q", home)
	}
}
