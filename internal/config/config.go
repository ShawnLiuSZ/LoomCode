package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Config 顶层配置结构
type Config struct {
	DefaultProvider string             `json:"default_provider,omitempty"`
	Providers       []ProviderConfig   `json:"providers,omitempty"`
	Plugins         []PluginConfig     `json:"plugins,omitempty"`
	Env             map[string]string  `json:"env,omitempty"` // API keys: project > global > env vars
	Permissions     PermissionConfig   `json:"permissions,omitempty"`
	Search          SearchConfig       `json:"search,omitempty"`
	Experimental    ExperimentalConfig `json:"experimental,omitempty"`
	Agent           AgentConfig        `json:"agent,omitempty"`
}

// ProviderConfig 单个 Provider 配置
type ProviderConfig struct {
	Name         string        `json:"name,omitempty"`
	DisplayName  string        `json:"display_name,omitempty"`
	Kind         string        `json:"kind,omitempty"`
	BaseURL      string        `json:"base_url,omitempty"`
	APIKey       string        `json:"api_key,omitempty"`
	APIKeyEnv    string        `json:"api_key_env,omitempty"`
	AuthMethod   string        `json:"auth_method,omitempty"`
	DefaultModel string        `json:"default_model,omitempty"`
	Models       []ModelConfig `json:"models,omitempty"`
}

// ModelConfig 单个模型配置
type ModelConfig struct {
	ID            string     `json:"id,omitempty"`
	Name          string     `json:"name,omitempty"`
	Cost          CostConfig `json:"cost,omitempty"`
	ContextWindow int        `json:"context_window,omitempty"`
	Capabilities  CapConfig  `json:"capabilities,omitempty"`
}

// CostConfig 成本配置
type CostConfig struct {
	Input       float64 `json:"input,omitempty"`
	CachedInput float64 `json:"cached_input,omitempty"`
	Output      float64 `json:"output,omitempty"`
}

// CapConfig 模型能力配置
type CapConfig struct {
	Reasoning   bool `json:"reasoning,omitempty"`
	ToolCall    bool `json:"tool_call,omitempty"`
	PrefixCache bool `json:"prefix_cache,omitempty"`
	Vision      bool `json:"vision,omitempty"`
	Voice       bool `json:"voice,omitempty"`
}

// PluginConfig MCP 插件配置。
// command 非空 → stdio 传输；url 非空 → HTTP/SSE 传输（url 优先）。
type PluginConfig struct {
	Name    string   `json:"name,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	URL     string   `json:"url,omitempty"`
}

// Kind 返回插件的传输类型："sse" / "stdio" / ""（未配置）。
func (p PluginConfig) Kind() string {
	switch {
	case p.URL != "":
		return "sse"
	case p.Command != "":
		return "stdio"
	default:
		return ""
	}
}

// PermissionConfig 权限配置
type PermissionConfig struct {
	ShellAllowlist []string `json:"shell_allowlist,omitempty"`
}

// SearchConfig 搜索配置
type SearchConfig struct {
	Engine string `json:"engine,omitempty"`
}

// ExperimentalConfig 实验性功能
type ExperimentalConfig struct {
	MaxMode   bool `json:"maxMode,omitempty"`
	BatchTool bool `json:"batchTool,omitempty"`
}

// AgentConfig Agent 层配置（planner/executor 分离 session 等）。
// 参考 DeepSeek-Reasonix SPEC.md §3.5：planner 和 executor 在两个独立 session 中运行，
// session 之间不混合，每个 session prepend-only 增长，保持 prefix cache 命中。
type AgentConfig struct {
	// PlannerModel 规划器模型（可选）。
	// 非空时启用 planner/executor 分离 session 架构：
	// planner 用此模型在独立 session 中规划，executor 用默认模型在独立 session 中执行。
	// 空时退化为单 session 模式（与 MultiAgent 行为一致）。
	PlannerModel string `json:"planner_model,omitempty"`
}

// Load 从指定路径加载配置（仅支持 JSON）
func Load(path string) (*Config, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" {
		return nil, fmt.Errorf("unsupported config format: %s (only .json supported)", ext)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse json config: %w", err)
	}

	if err := cfg.resolveAPIKeys(cfg.Env, nil); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// LoadDefault 按优先级查找并加载配置（仅支持 JSON）
//
// 优先级（高到低）：
//   1. --config flag
//   2. ./loomcode.json（项目级主配置）
//   3. ~/.loomcode/loomcode.json（全局主配置）
//   4. ./models.json（项目级模型配置）
//   5. ~/.loomcode/models.json（全局模型配置）
//   6. 内置默认
func LoadDefault() (*Config, error) {
	home, _ := os.UserHomeDir()

	type configPath struct {
		path       string
		configType string // "main" or "models"
	}

	var paths []configPath

	// 项目级
	paths = append(paths,
		configPath{"./loomcode.json", "main"},
		configPath{"./models.json", "models"},
	)

	// 全局级
	if home != "" {
		dir := filepath.Join(home, ".loomcode")
		paths = append(paths,
			configPath{filepath.Join(dir, "loomcode.json"), "main"},
			configPath{filepath.Join(dir, "models.json"), "models"},
		)
	}

	for _, entry := range paths {
		if _, err := os.Stat(entry.path); err != nil {
			continue
		}
		cfg, err := Load(entry.path)
		if err != nil {
			return nil, err
		}
		if len(cfg.Providers) == 0 {
			continue
		}
		return cfg, nil
	}

	return DefaultConfig(), nil
}

// expandEnvVar expands ${ENV_VAR} references in a string.
// If the value contains ${...}, it extracts the env var name and resolves it.
// Returns the expanded value, or the original value if no expansion is needed.
func expandEnvVar(value string, envMaps ...map[string]string) string {
	if len(value) < 4 || value[:2] != "${" || value[len(value)-1] != '}' {
		return value
	}
	envName := value[2 : len(value)-1]
	if envName == "" {
		return value
	}

	// Check provided env maps first (project > global)
	for _, m := range envMaps {
		if m == nil {
			continue
		}
		if val, ok := m[envName]; ok && val != "" {
			return val
		}
	}

	// Fall back to system environment
	if val := os.Getenv(envName); val != "" {
		return val
	}

	return value // return unresolved reference as-is
}

// resolveAPIKeys 检查各 Provider 的 API Key 是否已配置。
// 支持 ${ENV_VAR} 语法直接展开环境变量。
// 优先级：api_key(${...}展开) > api_key_env(project env > global env > system env)
func (c *Config) resolveAPIKeys(projectEnv, globalEnv map[string]string) error {
	for i := range c.Providers {
		p := &c.Providers[i]

		// 1. 如果 api_key 包含 ${ENV_VAR}，展开它
		if p.APIKey != "" {
			p.APIKey = expandEnvVar(p.APIKey, projectEnv, globalEnv)
			continue
		}

		// 2. 通过 api_key_env 查找（向后兼容）
		if p.APIKeyEnv == "" {
			continue
		}
		keyName := p.APIKeyEnv

		// 2a. 项目级 env 配置
		if val, ok := projectEnv[keyName]; ok && val != "" {
			p.APIKey = val
			continue
		}

		// 2b. 全局 env 配置
		if val, ok := globalEnv[keyName]; ok && val != "" {
			p.APIKey = val
			continue
		}

		// 2c. 系统环境变量
		if val := os.Getenv(keyName); val != "" {
			p.APIKey = val
			continue
		}

		fmt.Fprintf(os.Stderr, "Warning: provider %q 环境变量 %q 未设置，模型可浏览但无法调用\n", p.Name, keyName)
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

	// 校验 plugins：name 唯一、command/url 至少有一个（N5）
	seenPlugin := make(map[string]bool)
	for i, pl := range c.Plugins {
		if pl.Name == "" {
			return fmt.Errorf("plugins[%d]: name is required", i)
		}
		if seenPlugin[pl.Name] {
			return fmt.Errorf("plugins[%d]: duplicate plugin name %q", i, pl.Name)
		}
		seenPlugin[pl.Name] = true
		if pl.Command == "" && pl.URL == "" {
			return fmt.Errorf("plugin %q: command or url is required", pl.Name)
		}
		if pl.URL != "" && !isValidURL(pl.URL) {
			return fmt.Errorf("plugin %q: url %q is not a valid URL", pl.Name, pl.URL)
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
				Name:         "deepseek",
				DisplayName:  "DeepSeek",
				Kind:         "deepseek",
				BaseURL:      "https://api.deepseek.com",
				APIKeyEnv:    "DEEPSEEK_API_KEY",
				DefaultModel: "deepseek-v4-flash",
				Models: []ModelConfig{
					{
						ID:            "deepseek-v4-flash",
						Name:          "DeepSeek V4 Flash",
						Cost:          CostConfig{Input: 0.14, CachedInput: 0.014, Output: 0.28},
						ContextWindow: 131072,
						Capabilities:  CapConfig{ToolCall: true, PrefixCache: true},
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
