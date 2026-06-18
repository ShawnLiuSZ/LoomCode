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
