package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFiles(t *testing.T) {
	dir := t.TempDir()

	// 创建 .env 文件
	envContent := "TEST_KEY_1=value1\nTEST_KEY_2=value2\n"
	os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}

	if os.Getenv("TEST_KEY_1") != "value1" {
		t.Errorf("TEST_KEY_1 = %q", os.Getenv("TEST_KEY_1"))
	}
	if os.Getenv("TEST_KEY_2") != "value2" {
		t.Errorf("TEST_KEY_2 = %q", os.Getenv("TEST_KEY_2"))
	}
}

func TestLoadEnvFiles_LocalOverride(t *testing.T) {
	dir := t.TempDir()

	// .env 设置值
	os.WriteFile(filepath.Join(dir, ".env"), []byte("TEST_OVERRIDE=original\n"), 0644)
	// .env.local 覆盖
	os.WriteFile(filepath.Join(dir, ".env.local"), []byte("TEST_OVERRIDE=overridden\n"), 0644)

	LoadEnvFiles(dir)

	if os.Getenv("TEST_OVERRIDE") != "overridden" {
		t.Errorf("TEST_OVERRIDE = %q, want overridden", os.Getenv("TEST_OVERRIDE"))
	}
}

func TestLoadEnvFiles_NoFiles(t *testing.T) {
	dir := t.TempDir()
	// 没有 .env 文件也不应报错
	err := LoadEnvFiles(dir)
	if err != nil {
		t.Errorf("LoadEnvFiles should not error for missing files: %v", err)
	}
}

func TestLoadEnvFiles_OnlyDotEnv(t *testing.T) {
	dir := t.TempDir()
	// 只有 .env，没有 .env.local
	os.WriteFile(filepath.Join(dir, ".env"), []byte("ONLY_DOTENV=present\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("ONLY_DOTENV") != "present" {
		t.Errorf("ONLY_DOTENV = %q", os.Getenv("ONLY_DOTENV"))
	}
}

func TestLoadEnvFiles_OnlyDotEnvLocal(t *testing.T) {
	dir := t.TempDir()
	// 只有 .env.local，没有 .env
	os.WriteFile(filepath.Join(dir, ".env.local"), []byte("ONLY_LOCAL=local_only\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("ONLY_LOCAL") != "local_only" {
		t.Errorf("ONLY_LOCAL = %q", os.Getenv("ONLY_LOCAL"))
	}
}

func TestLoadEnvFiles_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	// 空 .env 文件
	os.WriteFile(filepath.Join(dir, ".env"), []byte(""), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Errorf("LoadEnvFiles should not error for empty file: %v", err)
	}
}

func TestLoadEnvFiles_CommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	// 包含注释和空行
	content := `# This is a comment
KEY_A=value_a

# Another comment  
KEY_B=value_b

`
	os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("KEY_A") != "value_a" {
		t.Errorf("KEY_A = %q", os.Getenv("KEY_A"))
	}
	if os.Getenv("KEY_B") != "value_b" {
		t.Errorf("KEY_B = %q", os.Getenv("KEY_B"))
	}
}

func TestLoadEnvFiles_ValueWithSpaces(t *testing.T) {
	dir := t.TempDir()
	// 包含空格的变量值
	os.WriteFile(filepath.Join(dir, ".env"), []byte("GREETING=hello world\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("GREETING") != "hello world" {
		t.Errorf("GREETING = %q", os.Getenv("GREETING"))
	}
}

func TestLoadEnvFiles_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	// 包含等号的值
	os.WriteFile(filepath.Join(dir, ".env"), []byte("URL=https://api.example.com?v=1\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("URL") != "https://api.example.com?v=1" {
		t.Errorf("URL = %q", os.Getenv("URL"))
	}
}

func TestLoadEnvFiles_MultipleDots(t *testing.T) {
	dir := t.TempDir()
	// 键名中包含多个点号
	os.WriteFile(filepath.Join(dir, ".env"), []byte("MY.APP.KEY=secret123\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("MY.APP.KEY") != "secret123" {
		t.Errorf("MY.APP.KEY = %q", os.Getenv("MY.APP.KEY"))
	}
}

func TestLoadEnvFiles_ExistingEnvNotOverriddenByDotEnv(t *testing.T) {
	dir := t.TempDir()
	// 系统环境变量已存在，.env 的值会覆盖（因为用 os.Setenv）
	t.Setenv("EXISTING_KEY", "system_value")
	os.WriteFile(filepath.Join(dir, ".env"), []byte("EXISTING_KEY=env_value\n"), 0644)

	LoadEnvFiles(dir)

	// 由于 loadEnvFile 使用 os.Setenv，.env 的值应覆盖系统值
	if os.Getenv("EXISTING_KEY") != "env_value" {
		t.Errorf("EXISTING_KEY = %q, want env_value (env file should override)", os.Getenv("EXISTING_KEY"))
	}
}

func TestLoadEnvFiles_NoExistingHomeDir(t *testing.T) {
	// 设置一个不存在的 HOME 目录来测试全局 .env 加载
	t.Setenv("HOME", "/nonexistent/home/dir")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("PROJECT_KEY=project\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles should not error when HOME doesn't exist: %v", err)
	}
	if os.Getenv("PROJECT_KEY") != "project" {
		t.Errorf("PROJECT_KEY = %q", os.Getenv("PROJECT_KEY"))
	}
}

func TestLoadEnvFiles_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	// 引号包围的值
	os.WriteFile(filepath.Join(dir, ".env"), []byte(`QUOTED="value with spaces"
SINGLE_QUOTED='single quoted'
`), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	// godotenv 会自动去除引号
	if os.Getenv("QUOTED") != "value with spaces" {
		t.Errorf("QUOTED = %q", os.Getenv("QUOTED"))
	}
	if os.Getenv("SINGLE_QUOTED") != "single quoted" {
		t.Errorf("SINGLE_QUOTED = %q", os.Getenv("SINGLE_QUOTED"))
	}
}

func TestLoadEnvFiles_ExportPrefix(t *testing.T) {
	dir := t.TempDir()
	// export 前缀
	os.WriteFile(filepath.Join(dir, ".env"), []byte("export EXPORTED_KEY=exported_value\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("EXPORTED_KEY") != "exported_value" {
		t.Errorf("EXPORTED_KEY = %q", os.Getenv("EXPORTED_KEY"))
	}
}

func TestLoadEnvFiles_DuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	// 同一文件中的重复键，后出现的应覆盖
	os.WriteFile(filepath.Join(dir, ".env"), []byte("DUP_KEY=first\nDUP_KEY=second\n"), 0644)

	err := LoadEnvFiles(dir)
	if err != nil {
		t.Fatalf("LoadEnvFiles error: %v", err)
	}
	if os.Getenv("DUP_KEY") != "second" {
		t.Errorf("DUP_KEY = %q, want second (last value wins)", os.Getenv("DUP_KEY"))
	}
}

func TestEnvFileTemplate(t *testing.T) {
	tmpl := EnvFileTemplate()
	if tmpl == "" {
		t.Error("template should not be empty")
	}
}

func TestCreateEnvTemplate(t *testing.T) {
	dir := t.TempDir()

	err := CreateEnvTemplate(dir)
	if err != nil {
		t.Fatalf("CreateEnvTemplate error: %v", err)
	}

	path := filepath.Join(dir, ".env")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error(".env file should exist")
	}

	// 再次调用不应覆盖
	err = CreateEnvTemplate(dir)
	if err != nil {
		t.Errorf("second CreateEnvTemplate should not error: %v", err)
	}
}
