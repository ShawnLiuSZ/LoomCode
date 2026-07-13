package tool

import (
	"os"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
)

func TestSetEnvProvider(t *testing.T) {
	orig := globalEnvProvider
	defer func() { globalEnvProvider = orig }()

	p := &testutil.TestEnvProvider{Env: []string{"KEY_A=val_a", "KEY_B=val_b"}}
	SetEnvProvider(p)

	env := EnvForSubprocess()
	if len(env) != 2 {
		t.Fatalf("env count = %d, want 2", len(env))
	}
	if env[0] != "KEY_A=val_a" {
		t.Errorf("env[0] = %q", env[0])
	}
	if env[1] != "KEY_B=val_b" {
		t.Errorf("env[1] = %q", env[1])
	}
}

func TestEnvForSubprocess_NilProvider(t *testing.T) {
	orig := globalEnvProvider
	globalEnvProvider = nil
	defer func() { globalEnvProvider = orig }()

	env := EnvForSubprocess()
	if env != nil {
		t.Errorf("expected nil env when no provider set, got %v", env)
	}
}

func TestEnvForSubprocess_EmptyEnv(t *testing.T) {
	orig := globalEnvProvider
	defer func() { globalEnvProvider = orig }()

	p := &testutil.TestEnvProvider{Env: []string{}}
	SetEnvProvider(p)

	env := EnvForSubprocess()
	if len(env) != 0 {
		t.Errorf("expected empty env, got %d items", len(env))
	}
}

func TestBashTool_InjectsEnv(t *testing.T) {
	orig := globalEnvProvider
	defer func() { globalEnvProvider = orig }()

	t.Setenv("LOOMCODE_TEST_KEY", "test_value")
	p := &testutil.TestEnvProvider{Env: os.Environ()}
	SetEnvProvider(p)

	tl := &BashTool{}
	result, err := tl.Execute(t.Context(), map[string]any{
		"command": "echo $LOOMCODE_TEST_KEY",
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Content != "test_value\n" {
		t.Errorf("expected 'test_value\\n', got %q", result.Content)
	}
}

func TestGrepTool_InjectsEnv(t *testing.T) {
	orig := globalEnvProvider
	defer func() { globalEnvProvider = orig }()

	p := &testutil.TestEnvProvider{Env: os.Environ()}
	SetEnvProvider(p)

	dir := t.TempDir()
	path := dir + "/test.txt"
	os.WriteFile(path, []byte("hello world\n"), 0644)

	tl := &GrepTool{}
	result, err := tl.Execute(t.Context(), map[string]any{
		"pattern": "hello",
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected grep results")
	}
}

func TestGlobTool_InjectsEnv(t *testing.T) {
	orig := globalEnvProvider
	defer func() { globalEnvProvider = orig }()

	p := &testutil.TestEnvProvider{Env: os.Environ()}
	SetEnvProvider(p)

	dir := t.TempDir()
	os.WriteFile(dir+"/test.go", []byte("package main"), 0644)

	tl := &GlobTool{}
	result, err := tl.Execute(t.Context(), map[string]any{
		"pattern": "*.go",
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected glob results")
	}
}

func TestBashTool_EnvInheritsAPIKeys(t *testing.T) {
	orig := globalEnvProvider
	defer func() { globalEnvProvider = orig }()

	t.Setenv("DEEPSEEK_API_KEY", "sk-test-deepseek-key")
	p := &testutil.TestEnvProvider{Env: os.Environ()}
	SetEnvProvider(p)

	tl := &BashTool{}
	result, err := tl.Execute(t.Context(), map[string]any{
		"command": "echo $DEEPSEEK_API_KEY",
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.Content != "sk-test-deepseek-key\n" {
		t.Errorf("expected 'sk-test-deepseek-key\\n', got %q", result.Content)
	}
}
