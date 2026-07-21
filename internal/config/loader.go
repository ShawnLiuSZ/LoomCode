package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoadWithProject loads and merges configs with project > global priority.
func LoadWithProject(projectDir string) (*Config, error) {
	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("load global config: %w", err)
	}

	projectCfg, err := loadProjectConfig(projectDir)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	merged := mergeConfigs(globalCfg, projectCfg)

	if err := merged.resolveAPIKeys(merged.Env, nil); err != nil {
		return nil, err
	}

	if err := merged.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return merged, nil
}

// loadOrWarn 加载配置文件，文件不存在返回 nil（静默），文件存在但解析失败则告警并返回 nil。
// #6 修复：原 `modelsCfg, _ := Load(...)` 吞掉所有错误，JSON 畸形时静默回退默认值，用户无从察觉配置写错。
func loadOrWarn(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		// 文件不存在属于正常情况（可选配置），静默跳过。
		if os.IsNotExist(err) {
			return nil
		}
		// 文件存在但读取/解析/校验失败：向 stderr 告警，避免静默回退。
		fmt.Fprintf(os.Stderr, "Warning: failed to load config %q: %v\n", path, err)
		return nil
	}
	return cfg
}

// loadGlobalConfig loads the global config from ~/.loomcode/.
// Merges settings.json (env, plugins, permissions, etc.) with models.json (providers, default_provider).
func loadGlobalConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	dir := filepath.Join(home, ".loomcode")

	// Load models.json (primary: providers + default_provider)
	modelsPath := filepath.Join(dir, "models.json")
	modelsCfg := loadOrWarn(modelsPath)

	// Load settings.json (env, plugins, permissions, etc.)
	settingsPath := filepath.Join(dir, "settings.json")
	settingsCfg := loadOrWarn(settingsPath)

	// Merge: models.json provides providers, settings.json provides env/plugins
	merged := mergeConfigs(modelsCfg, settingsCfg)

	if merged == nil {
		return DefaultConfig(), nil
	}

	return merged, nil
}

// loadProjectConfig loads the project-level config from <projectDir>/.loomcode/.
// Searches for settings.json (shared) and settings.local.json (local override).
func loadProjectConfig(projectDir string) (*Config, error) {
	if projectDir == "" {
		return nil, nil
	}

	dir := filepath.Join(projectDir, ".loomcode")

	// Load settings.json (shared config, can be committed to git)
	settingsPath := filepath.Join(dir, "settings.json")
	settingsCfg := loadOrWarn(settingsPath)

	// Load settings.local.json (local override, gitignored)
	localPath := filepath.Join(dir, "settings.local.json")
	localCfg := loadOrWarn(localPath)

	// Merge: local overrides shared
	return mergeConfigs(settingsCfg, localCfg), nil
}

// mergeConfigs merges two configs, with project overriding global.
func mergeConfigs(global, project *Config) *Config {
	if project == nil {
		return global
	}
	if global == nil {
		return project
	}

	merged := *global // shallow copy

	if project.DefaultProvider != "" {
		merged.DefaultProvider = project.DefaultProvider
	}
	if len(project.Providers) > 0 {
		merged.Providers = project.Providers
	}
	if len(project.Plugins) > 0 {
		merged.Plugins = project.Plugins
	}
	if len(project.Env) > 0 {
		if merged.Env == nil {
			merged.Env = make(map[string]string)
		}
		for k, v := range project.Env {
			merged.Env[k] = v
		}
	}
	if len(project.Permissions.ShellAllowlist) > 0 {
		merged.Permissions = project.Permissions
	}
	if project.Search.Engine != "" {
		merged.Search = project.Search
	}
	if project.Agent.PlannerModel != "" {
		merged.Agent = project.Agent
	}

	return &merged
}
