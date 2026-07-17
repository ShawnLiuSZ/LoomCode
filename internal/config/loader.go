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

// loadGlobalConfig loads the global config from ~/.loomcode/.
// Merges loomcode.json (env, plugins, etc.) with models.json (providers, default_provider).
func loadGlobalConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	dir := filepath.Join(home, ".loomcode")

	// Load models.json (primary: providers + default_provider)
	modelsPath := filepath.Join(dir, "models.json")
	modelsCfg, _ := Load(modelsPath)

	// Load loomcode.json (env, plugins, permissions, etc.)
	loomcodePath := filepath.Join(dir, "loomcode.json")
	loomcodeCfg, _ := Load(loomcodePath)

	// Merge: models.json provides providers, loomcode.json provides env/plugins
	merged := mergeConfigs(modelsCfg, loomcodeCfg)

	if merged == nil {
		return DefaultConfig(), nil
	}

	return merged, nil
}

// loadProjectConfig loads the project-level config from <projectDir>/.claude/loomcode.json.
func loadProjectConfig(projectDir string) (*Config, error) {
	if projectDir == "" {
		return nil, nil
	}

	projectPath := filepath.Join(projectDir, ".claude", "loomcode.json")
	if _, err := os.Stat(projectPath); err == nil {
		return Load(projectPath)
	}

	return nil, nil
}

// mergeConfigs merges two configs, with project overriding global.
func mergeConfigs(global, project *Config) *Config {
	if project == nil {
		return global
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
