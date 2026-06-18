package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config 顶层配置结构
type Config struct {
	DefaultProvider string              `toml:"default_provider"`
	Providers       []ProviderConfig    `toml:"providers"`
	Plugins         []PluginConfig      `toml:"plugins"`
	Permissions     PermissionConfig    `toml:"permissions"`
	Search          SearchConfig        `toml:"search"`
	Experimental    ExperimentalConfig  `toml:"experimental"`
}

// ProviderConfig 单个 Provider 配置
type ProviderConfig struct {
	Name         string        `toml:"name"`
	DisplayName  string        `toml:"display_name"`
	Kind         string        `toml:"kind"`
	BaseURL      string        `toml:"base_url"`
	APIKeyEnv    string        `toml:"api_key_env"`
	AuthMethod   string        `toml:"auth_method"`
	DefaultModel string        `toml:"default_model"`
	Models       []ModelConfig `toml:"models"`
}

// ModelConfig 单个模型配置
type ModelConfig struct {
	ID            string     `toml:"id"`
	Name          string     `toml:"name"`
	Cost          CostConfig `toml:"cost"`
	ContextWindow int        `toml:"context_window"`
	Capabilities  CapConfig  `toml:"capabilities"`
}

// CostConfig 成本配置
type CostConfig struct {
	Input       float64 `toml:"input"`
	CachedInput float64 `toml:"cached_input"`
	Output      float64 `toml:"output"`
}

// CapConfig 模型能力配置
type CapConfig struct {
	Reasoning   bool `toml:"reasoning"`
	ToolCall    bool `toml:"tool_call"`
	PrefixCache bool `toml:"prefix_cache"`
	Vision      bool `toml:"vision"`
	Voice       bool `toml:"voice"`
}

// PluginConfig MCP 插件配置
type PluginConfig struct {
	Name    string   `toml:"name"`
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	Env     []string `toml:"env"`
}

// PermissionConfig 权限配置
type PermissionConfig struct {
	ShellAllowlist []string `toml:"shell_allowlist"`
}

// SearchConfig 搜索配置
type SearchConfig struct {
	Engine string `toml:"engine"`
}

// ExperimentalConfig 实验性功能
type ExperimentalConfig struct {
	MaxMode   bool `toml:"maxMode"`
	BatchTool bool `toml:"batchTool"`
}

// Load 从指定路径加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.resolveAPIKeys(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadDefault 按优先级查找并加载配置
// 优先级: CLI flags > ./helix.toml > ~/.helix/config.toml > 内置默认
func LoadDefault() (*Config, error) {
	paths := []string{
		"./helix.toml",
		filepath.Join(homeDir(), ".helix", "config.toml"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return Load(p)
		}
	}

	return DefaultConfig(), nil
}

// resolveAPIKeys 从环境变量注入 API Key
func (c *Config) resolveAPIKeys() error {
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.APIKeyEnv == "" {
			continue
		}
		key := os.Getenv(p.APIKeyEnv)
		if key == "" {
			return fmt.Errorf("provider %q requires environment variable %q to be set", p.Name, p.APIKeyEnv)
		}
	}
	return nil
}

// GetProvider 根据名称查找 Provider 配置
func (c *Config) GetProvider(name string) (*ProviderConfig, error) {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i], nil
		}
	}
	return nil, fmt.Errorf("provider %q not found", name)
}

// DefaultConfig 返回内置默认配置
func DefaultConfig() *Config {
	return &Config{
		DefaultProvider: "deepseek",
		Providers: []ProviderConfig{
			{
				Name:        "deepseek",
				DisplayName: "DeepSeek",
				Kind:        "deepseek",
				BaseURL:     "https://api.deepseek.com",
				APIKeyEnv:   "DEEPSEEK_API_KEY",
				DefaultModel: "deepseek-v4-flash",
				Models: []ModelConfig{
					{
						ID:   "deepseek-v4-flash",
						Name: "DeepSeek V4 Flash",
						Cost: CostConfig{Input: 0.14, CachedInput: 0.014, Output: 0.28},
						ContextWindow: 131072,
						Capabilities: CapConfig{ToolCall: true, PrefixCache: true},
					},
				},
			},
		},
	}
}

func homeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
