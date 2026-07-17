package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateOldConfigs 迁移旧配置文件到新格式
// 1. 将 TOML 配置迁移到 JSON
// 2. 将 .env 中的 keys 迁移到 loomcode.json 的 env 字段
func MigrateOldConfigs(homeDir string) error {
	if homeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil // 无法获取 home 目录，跳过迁移
		}
		homeDir = home
	}

	configDir := filepath.Join(homeDir, ".loomcode")

	// 迁移 TOML 配置
	if err := migrateTOML(configDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: TOML migration failed: %v\n", err)
	}

	// 迁移 .env 文件
	if err := migrateEnvFile(configDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: .env migration failed: %v\n", err)
	}

	return nil
}

// migrateTOML 将 TOML 配置迁移到 JSON
func migrateTOML(configDir string) error {
	tomlFiles := []string{
		"loomcode.toml",
		"config.toml",
		"models.toml",
	}

	for _, tomlFile := range tomlFiles {
		tomlPath := filepath.Join(configDir, tomlFile)
		if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
			continue
		}

		// 备份原文件
		backupPath := tomlPath + ".bak"
		if err := copyFile(tomlPath, backupPath); err != nil {
			return fmt.Errorf("backup %s: %w", tomlPath, err)
		}

		// 读取 TOML（使用简化解析）
		data, err := os.ReadFile(tomlPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", tomlPath, err)
		}

		// 简单 TOML 到 JSON 转换（实际实现需要更完整）
		jsonData, err := simpleTOMLToJSON(data)
		if err != nil {
			return fmt.Errorf("convert %s: %w", tomlPath, err)
		}

		// 写入 JSON 文件
		jsonPath := filepath.Join(configDir, "models.json")
		if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
			return fmt.Errorf("write %s: %w", jsonPath, err)
		}

		// 删除原 TOML 文件
		os.Remove(tomlPath)

		fmt.Fprintf(os.Stderr, "Migrated %s → %s (backup: %s)\n", tomlFile, "models.json", backupPath)
	}

	return nil
}

// migrateEnvFile 将 .env 中的 keys 迁移到 settings.json 的 env 字段
func migrateEnvFile(configDir string) error {
	envPath := filepath.Join(configDir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil
	}

	// 读取 .env 文件
	envMap, err := readEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("read .env: %w", err)
	}

	if len(envMap) == 0 {
		return nil
	}

	// 备份原文件
	backupPath := envPath + ".bak"
	if err := copyFile(envPath, backupPath); err != nil {
		return fmt.Errorf("backup .env: %w", err)
	}

	// 读取或创建 settings.json
	settingsPath := filepath.Join(configDir, "settings.json")
	var config map[string]interface{}

	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &config)
	}

	if config == nil {
		config = make(map[string]interface{})
	}

	// 添加 env 字段
	config["env"] = envMap

	// 写入 settings.json
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(settingsPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}

	// 删除原 .env 文件
	os.Remove(envPath)

	fmt.Fprintf(os.Stderr, "Migrated .env → settings.json (backup: %s)\n", backupPath)
	return nil
}

// readEnvFile 读取 .env 文件
func readEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// 移除引号
		val = strings.Trim(val, "\"'")

		result[key] = val
	}

	return result, nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// simpleTOMLToJSON 简单的 TOML 到 JSON 转换（仅支持基本结构）
func simpleTOMLToJSON(data []byte) ([]byte, error) {
	// 这里需要一个简单的 TOML 解析器
	// 实际实现可以使用第三方库或更完整的解析
	// 暂时返回错误，让用户手动迁移
	return nil, fmt.Errorf("automatic TOML migration not supported, please convert manually")
}
