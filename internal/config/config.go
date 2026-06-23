package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// LoadDefault 按优先级查找并加载配置
// 优先级: CLI flags > ./helix.toml > ~/.helix/config.toml > 内置默认
func LoadDefault() (*Config, error) {
	paths := []string{"./helix.toml"}
	// 仅当 HOME 可用时才纳入用户级配置路径；否则跳过，避免退化为 cwd 相对路径被恶意配置注入。
	if home, ok := homeDir(); ok {
		paths = append(paths, filepath.Join(home, ".helix", "config.toml"))
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

// knownProviderKinds 已知的 Provider 类型
var knownProviderKinds = map[string]bool{
	"deepseek": true,
	"mimo":     true,
	"openai":   true,
}

// Validate 校验配置合法性
func (c *Config) Validate() error {
	// 校验 DefaultProvider 引用是否存在
	if c.DefaultProvider != "" {
		found := false
		for _, p := range c.Providers {
			if p.Name == c.DefaultProvider {
				found = true
				break
			}
		}
		if !found && len(c.Providers) > 0 {
			return fmt.Errorf("default_provider %q not found in providers", c.DefaultProvider)
		}
	}

	for i, p := range c.Providers {
		if p.Name == "" {
			return fmt.Errorf("providers[%d]: name is required", i)
		}
		if p.Kind == "" {
			return fmt.Errorf("provider %q: kind is required", p.Name)
		}
		if !knownProviderKinds[p.Kind] {
			return fmt.Errorf("provider %q: unknown kind %q (supported: deepseek, mimo, openai)", p.Name, p.Kind)
		}
		if p.BaseURL == "" {
			return fmt.Errorf("provider %q: base_url is required", p.Name)
		}
		if !isValidURL(p.BaseURL) {
			return fmt.Errorf("provider %q: base_url %q is not a valid URL", p.Name, p.BaseURL)
		}
		if len(p.Models) == 0 {
			return fmt.Errorf("provider %q: at least one model is required", p.Name)
		}

		for j, m := range p.Models {
			if m.ID == "" {
				return fmt.Errorf("provider %q, models[%d]: id is required", p.Name, j)
			}
			if m.ContextWindow < 0 {
				return fmt.Errorf("provider %q, model %q: context_window must be non-negative, got %d", p.Name, m.ID, m.ContextWindow)
			}
			if m.Cost.Input < 0 || m.Cost.Output < 0 || m.Cost.CachedInput < 0 {
				return fmt.Errorf("provider %q, model %q: cost values must be non-negative", p.Name, m.ID)
			}
		}
	}

	return nil
}

// isValidURL 校验 URL 格式
func isValidURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
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

// homeDir 返回用户主目录；不可用时返回 ("", false)，调用方应跳过用户级路径而非回退到 cwd。
func homeDir() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	return home, true
}
