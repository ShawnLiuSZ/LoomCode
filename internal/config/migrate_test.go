package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".loomcode")
	os.MkdirAll(configDir, 0755)

	// 创建 .env 文件
	envContent := `DEEPSEEK_API_KEY=sk-test123
MIMO_API_KEY=sk-mimo456
`
	os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0644)

	// 执行迁移
	err := migrateEnvFile(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证 loomcode.json 创建
	loomcodePath := filepath.Join(configDir, "loomcode.json")
	if _, err := os.Stat(loomcodePath); os.IsNotExist(err) {
		t.Fatal("loomcode.json not created")
	}

	// 验证内容
	data, _ := os.ReadFile(loomcodePath)
	if !strings.Contains(string(data), "DEEPSEEK_API_KEY") {
		t.Error("DEEPSEEK_API_KEY not found in loomcode.json")
	}
	if !strings.Contains(string(data), "sk-test123") {
		t.Error("value sk-test123 not found in loomcode.json")
	}

	// 验证 .env 备份创建
	if _, err := os.Stat(filepath.Join(configDir, ".env.bak")); os.IsNotExist(err) {
		t.Error(".env.bak not created")
	}

	// 验证原 .env 已删除
	if _, err := os.Stat(filepath.Join(configDir, ".env")); !os.IsNotExist(err) {
		t.Error("original .env was not removed")
	}
}

func TestMigrateEnvFileNoEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".loomcode")
	os.MkdirAll(configDir, 0755)

	// 没有 .env 文件，应跳过
	err := migrateEnvFile(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 不应创建 loomcode.json
	if _, err := os.Stat(filepath.Join(configDir, "loomcode.json")); !os.IsNotExist(err) {
		t.Error("loomcode.json should not be created when no .env exists")
	}
}

func TestMigrateEnvFileEmptyEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".loomcode")
	os.MkdirAll(configDir, 0755)

	// 创建空 .env（只有注释和空行）
	envContent := `# this is a comment

`
	os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0644)

	err := migrateEnvFile(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 空 env 不应创建 loomcode.json
	if _, err := os.Stat(filepath.Join(configDir, "loomcode.json")); !os.IsNotExist(err) {
		t.Error("loomcode.json should not be created for empty .env")
	}
}

func TestMigrateEnvFilePreservesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".loomcode")
	os.MkdirAll(configDir, 0755)

	// 创建已有的 loomcode.json
	existing := `{"default_provider": "deepseek"}`
	os.WriteFile(filepath.Join(configDir, "loomcode.json"), []byte(existing), 0644)

	// 创建 .env
	envContent := `NEW_API_KEY=sk-new789
`
	os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0644)

	err := migrateEnvFile(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证合并后的 loomcode.json 包含原有字段和新 env
	data, _ := os.ReadFile(filepath.Join(configDir, "loomcode.json"))
	content := string(data)
	if !strings.Contains(content, "default_provider") {
		t.Error("existing default_provider field was lost")
	}
	if !strings.Contains(content, "NEW_API_KEY") {
		t.Error("NEW_API_KEY not found in loomcode.json")
	}
}

func TestMigrateEnvFileWithQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".loomcode")
	os.MkdirAll(configDir, 0755)

	envContent := `API_KEY="sk-quoted"
ANOTHER='single-quoted'
`
	os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0644)

	err := migrateEnvFile(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(configDir, "loomcode.json"))
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	env, ok := config["env"].(map[string]interface{})
	if !ok {
		t.Fatal("env field not found or wrong type")
	}

	if got := env["API_KEY"]; got != "sk-quoted" {
		t.Errorf("API_KEY = %v, want sk-quoted", got)
	}
	if got := env["ANOTHER"]; got != "single-quoted" {
		t.Errorf("ANOTHER = %v, want single-quoted", got)
	}
}

func TestMigrateOldConfigsNoHome(t *testing.T) {
	// 空 homeDir 应安全跳过（获取 UserHomeDir 后发现无 .loomcode）
	err := MigrateOldConfigs("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `# Comment line
KEY1=value1
KEY2="quoted value"
KEY3='single quoted'
  KEY4  =  spaced

# Another comment
EMPTY=
`
	os.WriteFile(envPath, []byte(content), 0644)

	result, err := readEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := map[string]string{
		"KEY1": "value1",
		"KEY2": "quoted value",
		"KEY3": "single quoted",
		"KEY4": "spaced",
		"EMPTY": "",
	}

	for k, want := range tests {
		if got := result[k]; got != want {
			t.Errorf("readEnvFile[%s] = %q, want %q", k, got, want)
		}
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	content := "hello world"
	os.WriteFile(src, []byte(content), 0644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != content {
		t.Errorf("copyFile: got %q, want %q", string(data), content)
	}
}
