package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnvFiles 按优先级加载 .env 文件
// 加载顺序（后加载覆盖先加载）：
// 1. ~/.helix/.env（全局）
// 2. ./.env（项目）
// 3. ./.env.local（项目本地，不提交）
func LoadEnvFiles(projectDir string) error {
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".helix", ".env")
		if err := loadEnvFile(globalPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to load global .env: %v", err)
		}
	}

	projectPath := filepath.Join(projectDir, ".env")
	if err := loadEnvFile(projectPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to load project .env: %v", err)
	}

	localPath := filepath.Join(projectDir, ".env.local")
	if err := loadEnvFile(localPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to load local .env: %v", err)
	}

	return nil
}

// LoadEnvFile 加载指定路径的 .env 文件
func LoadEnvFile(path string) error {
	return loadEnvFile(path)
}

// loadEnvFile 加载单个 .env 文件（不存在不报错）
func loadEnvFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	// godotenv.Load 默认不覆盖已存在的环境变量
	// 使用 godotenv.Read 读取后再手动 setenv 实现覆盖
	envMap, err := godotenv.Read(path)
	if err != nil {
		return err
	}
	for key, val := range envMap {
		if err := os.Setenv(key, val); err != nil {
			log.Printf("Warning: failed to set env %s: %v", key, err)
		}
	}
	return nil
}

// EnvFileTemplate 生成 .env 模板内容
func EnvFileTemplate() string {
	return `# Helix CLI Environment Configuration
# Copy this file to .env and fill in your API keys

# DeepSeek API Key
DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxx

# MiMo API Key (optional)
MIMO_API_KEY=

# OpenAI API Key (optional)
OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxx

# Anthropic API Key (optional)
ANTHROPIC_API_KEY=

# Custom provider API keys can be added here
# MY_CUSTOM_API_KEY=
`
}

// CreateEnvTemplate 创建 .env 模板文件（如果不存在）
func CreateEnvTemplate(dir string) error {
	path := filepath.Join(dir, ".env")
	if _, err := os.Stat(path); err == nil {
		return nil // 已存在，不覆盖
	}
	return os.WriteFile(path, []byte(EnvFileTemplate()), 0644)
}
