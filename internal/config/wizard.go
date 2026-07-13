package config

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Wizard guides the user through interactive configuration setup.
type Wizard struct {
	reader *bufio.Reader
}

// NewWizard creates a new interactive configuration wizard.
func NewWizard() *Wizard {
	return &Wizard{reader: bufio.NewReader(os.Stdin)}
}

// providerPreset holds defaults for a built-in provider.
type providerPreset struct {
	Kind        string
	Name        string
	DisplayName string
	BaseURL     string
	APIKeyEnv   string
	Models      []ModelConfig
}

// providerPresets defines built-in provider defaults (DeepSeek/MiMo/OpenAI).
var providerPresets = map[int]providerPreset{
	1: {
		Kind: "deepseek", Name: "deepseek", DisplayName: "DeepSeek",
		BaseURL: "https://api.deepseek.com", APIKeyEnv: "DEEPSEEK_API_KEY",
		Models: []ModelConfig{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextWindow: 131072,
				Cost:         CostConfig{Input: 0.14, CachedInput: 0.014, Output: 0.28},
				Capabilities: CapConfig{ToolCall: true, PrefixCache: true}},
			{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", ContextWindow: 131072,
				Cost:         CostConfig{Input: 0.55, CachedInput: 0.055, Output: 2.19},
				Capabilities: CapConfig{Reasoning: true, ToolCall: true, PrefixCache: true}},
		},
	},
	2: {
		Kind: "mimo", Name: "mimo", DisplayName: "MiMo",
		BaseURL: "https://api.mimo.xiaomi.com/v1", APIKeyEnv: "MIMO_API_KEY",
		Models: []ModelConfig{
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro", ContextWindow: 262144,
				Capabilities: CapConfig{Reasoning: true, ToolCall: true, Voice: true}},
		},
	},
	3: {
		Kind: "openai", Name: "openai", DisplayName: "OpenAI",
		BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY",
		Models: []ModelConfig{
			{ID: "gpt-4o", Name: "GPT-4o", ContextWindow: 128000,
				Capabilities: CapConfig{ToolCall: true, Vision: true}},
			{ID: "gpt-4o-mini", Name: "GPT-4o mini", ContextWindow: 128000,
				Capabilities: CapConfig{ToolCall: true, Vision: true}},
		},
	},
}

// Run executes the wizard, returning the generated Config and env vars.
// Steps: select provider → enter API key → select model → optional Max Mode.
func (w *Wizard) Run() (*Config, map[string]string, error) {
	// Step 1: Select provider
	fmt.Println()
	fmt.Println("选择 AI 提供商:")
	fmt.Println("  1. DeepSeek (推荐，性价比高)")
	fmt.Println("  2. MiMo (小米 AI，支持语音)")
	fmt.Println("  3. OpenAI (GPT 系列)")
	fmt.Println("  4. 自定义 (OpenAI 兼容 API)")
	choice, err := w.readChoice("请输入选项 (1-4): ", 4)
	if err != nil {
		return nil, nil, err
	}

	var preset providerPreset
	if choice == 4 {
		preset, err = w.collectCustomProvider()
		if err != nil {
			return nil, nil, err
		}
	} else {
		preset = providerPresets[choice]
	}

	// Step 2: Enter API key
	fmt.Println()
	apiKey, err := w.readLine(fmt.Sprintf("请输入 API Key (%s): ", preset.APIKeyEnv))
	if err != nil {
		return nil, nil, err
	}
	if apiKey == "" {
		return nil, nil, fmt.Errorf("API Key 不能为空")
	}
	envVars := map[string]string{preset.APIKeyEnv: apiKey}

	// Step 3: Select model
	modelID, models, err := w.selectModel(preset)
	if err != nil {
		return nil, nil, err
	}

	// Step 4: Ask about Max Mode (experimental)
	fmt.Println()
	maxMode, err := w.readYesNo("启用 Max Mode (实验性，更强的推理能力)? (y/N): ")
	if err != nil {
		return nil, nil, err
	}

	// Step 5: Generate config
	cfg := &Config{
		DefaultProvider: preset.Name,
		Providers: []ProviderConfig{{
			Name:         preset.Name,
			DisplayName:  preset.DisplayName,
			Kind:         preset.Kind,
			BaseURL:      preset.BaseURL,
			APIKeyEnv:    preset.APIKeyEnv,
			DefaultModel: modelID,
			Models:       models,
		}},
	}
	if maxMode {
		cfg.Experimental.MaxMode = true
	}

	return cfg, envVars, nil
}

// collectCustomProvider gathers details for a custom OpenAI-compatible provider.
func (w *Wizard) collectCustomProvider() (providerPreset, error) {
	name, err := w.readLine("请输入 Provider 名称 (例如 my-openai): ")
	if err != nil {
		return providerPreset{}, err
	}
	if name == "" {
		name = "custom"
	}
	baseURL, err := w.readLine("请输入 Base URL (例如 https://api.example.com/v1): ")
	if err != nil {
		return providerPreset{}, err
	}
	if baseURL == "" {
		return providerPreset{}, fmt.Errorf("base URL 不能为空")
	}
	apiKeyEnv, err := w.readLine("请输入 API Key 环境变量名 (例如 MY_API_KEY): ")
	if err != nil {
		return providerPreset{}, err
	}
	if apiKeyEnv == "" {
		apiKeyEnv = "CUSTOM_API_KEY"
	}
	return providerPreset{
		Kind: "openai", Name: name, DisplayName: name,
		BaseURL: baseURL, APIKeyEnv: apiKeyEnv,
	}, nil
}

// selectModel prompts the user to pick a model from the preset or enter a custom one.
// Returns the selected model ID and the model list to embed in the config.
func (w *Wizard) selectModel(preset providerPreset) (string, []ModelConfig, error) {
	fmt.Println()
	fmt.Println("选择模型:")
	for i, m := range preset.Models {
		label := ""
		if i == 0 {
			label = " (推荐)"
		}
		fmt.Printf("  %d. %s%s\n", i+1, m.ID, label)
	}
	fmt.Printf("  %d. 自定义模型 ID\n", len(preset.Models)+1)

	choice, err := w.readChoice("请输入选项: ", len(preset.Models)+1)
	if err != nil {
		return "", nil, err
	}

	if choice <= len(preset.Models) {
		return preset.Models[choice-1].ID, preset.Models, nil
	}

	// Custom model
	customID, err := w.readLine("请输入模型 ID: ")
	if err != nil {
		return "", nil, err
	}
	if customID == "" {
		return "", nil, fmt.Errorf("模型 ID 不能为空")
	}
	customModel := ModelConfig{
		ID: customID, Name: customID, ContextWindow: 32768,
		Capabilities: CapConfig{ToolCall: true},
	}
	return customID, []ModelConfig{customModel}, nil
}

// readLine reads a single line from stdin, trimming surrounding whitespace.
func (w *Wizard) readLine(prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := w.reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("读取输入失败: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// readChoice reads a numeric choice in [1, max], retrying up to 3 times.
func (w *Wizard) readChoice(prompt string, max int) (int, error) {
	for attempt := 0; attempt < 3; attempt++ {
		s, err := w.readLine(prompt)
		if err != nil {
			return 0, err
		}
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n >= 1 && n <= max {
			return n, nil
		}
		fmt.Printf("无效选项，请输入 1-%d\n", max)
	}
	return 0, fmt.Errorf("连续 3 次无效输入")
}

// readYesNo reads a yes/no answer, defaulting to false.
func (w *Wizard) readYesNo(prompt string) (bool, error) {
	s, err := w.readLine(prompt)
	if err != nil {
		return false, err
	}
	s = strings.ToLower(s)
	return s == "y" || s == "yes", nil
}

// WriteConfig marshals cfg to TOML and writes it to the given path.
func WriteConfig(cfg *Config, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建 %s 失败: %w", path, err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("编码配置失败: %w", err)
	}
	return nil
}

// WriteEnvFile writes environment variables to a .env file.
// If the file already exists, new entries are appended; otherwise a new file is created.
// Keys are written in sorted order for stable output.
func WriteEnvFile(envVars map[string]string, path string) error {
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := envVars[k]
		// Quote values containing spaces, #, or quote chars to keep them valid in .env.
		if strings.ContainsAny(v, " #\"'") {
			fmt.Fprintf(&sb, "%s=%q\n", k, v)
		} else {
			fmt.Fprintf(&sb, "%s=%s\n", k, v)
		}
	}

	// Append to existing file; otherwise create a new one with a header.
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("打开 %s 失败: %w", path, err)
		}
		defer f.Close()
		if _, err := f.WriteString("\n# Added by helix setup\n" + sb.String()); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", path, err)
		}
		return nil
	}

	content := "# Helix CLI Environment Configuration\n# Generated by helix setup\n\n" + sb.String()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", path, err)
	}
	return nil
}
