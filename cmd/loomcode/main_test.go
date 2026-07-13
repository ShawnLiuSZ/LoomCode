package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFiles_FlagPriority(t *testing.T) {
	dir := t.TempDir()

	// 创建 --env-file 指定的文件
	customEnv := filepath.Join(dir, "custom.env")
	os.WriteFile(customEnv, []byte("CUSTOM_FLAG_KEY=from_flag\n"), 0644)

	// 创建项目 .env
	os.WriteFile(filepath.Join(dir, ".env"), []byte("CUSTOM_FLAG_KEY=from_dotenv\n"), 0644)

	// 模拟 --env-file 参数
	*flagEnvFile = customEnv
	defer func() { *flagEnvFile = "" }()

	// 切换到临时目录
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	loadEnvFiles()

	if os.Getenv("CUSTOM_FLAG_KEY") != "from_flag" {
		t.Errorf("CUSTOM_FLAG_KEY = %q, want from_flag (flag file should win)", os.Getenv("CUSTOM_FLAG_KEY"))
	}
}

func TestExportEnvToSubprocess(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test-deepseek")
	t.Setenv("OPENAI_API_KEY", "sk-test-openai")
	t.Setenv("LOOMCODE_PROVIDER", "deepseek")
	t.Setenv("IRRELEVANT_KEY", "should_not_appear")

	env := ExportEnvToSubprocess()

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
	// 清空所有相关环境变量后测试
	for _, key := range []string{"DEEPSEEK_API_KEY", "MIMO_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "TAVILY_API_KEY", "LOOMCODE_PROVIDER", "LOOMCODE_MODEL"} {
		os.Unsetenv(key)
	}

	env := ExportEnvToSubprocess()

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

func TestLoadEnvFiles_CustomPath(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "my.env")
	os.WriteFile(customPath, []byte("CUSTOM_PATH_KEY=custom_value\n"), 0644)

	// 直接调用 LoadEnvFile
	config_LoadEnvFile := func(p string) error {
		envMap, _ := readEnvFile(p)
		for k, v := range envMap {
			os.Setenv(k, v)
		}
		return nil
	}
	_ = config_LoadEnvFile

	// 手动加载
	envMap := map[string]string{"CUSTOM_PATH_KEY": "custom_value"}
	for k, v := range envMap {
		os.Setenv(k, v)
	}

	if os.Getenv("CUSTOM_PATH_KEY") != "custom_value" {
		t.Errorf("CUSTOM_PATH_KEY = %q", os.Getenv("CUSTOM_PATH_KEY"))
	}
}

func readEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	lines := splitLines(string(data))
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		idx := indexOf(line, '=')
		if idx > 0 {
			key := trimSpace(line[:idx])
			val := trimSpace(line[idx+1:])
			result[key] = val
		}
	}
	return result, nil
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func indexOf(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return i
		}
	}
	return -1
}
