package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ShawnLiuSZ/Helix/internal/agent"
	"github.com/ShawnLiuSZ/Helix/internal/config"
	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/provider/deepseek"
	"github.com/ShawnLiuSZ/Helix/internal/provider/openai"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// 构建时注入的版本信息
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var (
	flagProvider = flag.String("provider", "", "Provider name (e.g. deepseek, openai)")
	flagModel    = flag.String("model", "", "Model ID (e.g. deepseek-v4-flash)")
	flagConfig   = flag.String("config", "", "Path to config file")
	flagVersion  = flag.Bool("version", false, "Show version")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()

	// 默认进入交互式 REPL（未来用 Bubble Tea 实现）
	if len(args) == 0 {
		if *flagVersion {
			fmt.Printf("Helix CLI %s (commit: %s, built: %s)\n", version, commit, date)
			return
		}
		fmt.Println("Helix CLI - 双螺旋 · 多模型 · 可扩展")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  helix run <task>     Run a single task")
		fmt.Println("  helix setup          Run configuration wizard")
		fmt.Println("  helix                Interactive REPL (coming soon)")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		return
	}

	cmd := args[0]

	switch cmd {
	case "run":
		runCommand(args[1:])
	case "setup":
		setupCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Available commands: run, setup")
		os.Exit(1)
	}
}

func runCommand(args []string) {
	var task string
	if len(args) > 0 {
		task = strings.Join(args, " ")
	} else {
		// 管道模式：从 stdin 读取
		task = readStdin()
	}

	if task == "" {
		fmt.Fprintln(os.Stderr, "Error: no task provided")
		os.Exit(1)
	}

	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 选择 Provider
	provCfg, err := selectProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 创建 Provider
	p, err := createProvider(provCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating provider: %v\n", err)
		os.Exit(1)
	}

	// 创建工具注册中心
	tools := tool.NewRegistry()
	tools.RegisterDefaults()

	// 创建 Agent
	ag := agent.New(p, tools)
	ag.SetMaxSteps(20)

	// 执行任务
	fmt.Fprintf(os.Stderr, "Running with %s/%s...\n", provCfg.Name, selectModel(provCfg))
	fmt.Fprintln(os.Stderr, "---")

	ctx := context.Background()
	result, err := ag.Run(ctx, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}

func setupCommand() {
	fmt.Println("Helix CLI Setup")
	fmt.Println("===============")
	fmt.Println()
	fmt.Println("To configure Helix, create a helix.toml file in your project directory.")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println()
	fmt.Println("  cp helix.example.toml helix.toml")
	fmt.Println("  # Edit helix.toml to add your API keys")
	fmt.Println()
	fmt.Println("Then set your API key environment variable:")
	fmt.Println()
	fmt.Println("  export DEEPSEEK_API_KEY=\"sk-...\"")
	fmt.Println()
	fmt.Println("Run 'helix run \"hello\"' to test your setup.")
}

func loadConfig() (*config.Config, error) {
	path := *flagConfig
	if path != "" {
		return config.Load(path)
	}
	return config.LoadDefault()
}

func selectProvider(cfg *config.Config) (*config.ProviderConfig, error) {
	name := *flagProvider
	if name == "" {
		name = cfg.DefaultProvider
	}
	if name == "" && len(cfg.Providers) > 0 {
		name = cfg.Providers[0].Name
	}
	return cfg.GetProvider(name)
}

func selectModel(provCfg *config.ProviderConfig) string {
	if *flagModel != "" {
		return *flagModel
	}
	if provCfg.DefaultModel != "" {
		return provCfg.DefaultModel
	}
	if len(provCfg.Models) > 0 {
		return provCfg.Models[0].ID
	}
	return "default"
}

func createProvider(provCfg *config.ProviderConfig) (provider.Provider, error) {
	reg := provider.NewRegistry()
	reg.Register(&openai.Adapter{})
	reg.Register(&deepseek.Adapter{})

	models := make([]provider.ModelConfigItem, len(provCfg.Models))
	for i, m := range provCfg.Models {
		models[i] = provider.ModelConfigItem{
			ID:            m.ID,
			Name:          m.Name,
			ContextWindow: m.ContextWindow,
			CostInput:     m.Cost.Input,
			CostCachedInput: m.Cost.CachedInput,
			CostOutput:    m.Cost.Output,
			Reasoning:     m.Capabilities.Reasoning,
			ToolCall:      m.Capabilities.ToolCall,
			PrefixCache:   m.Capabilities.PrefixCache,
			Vision:        m.Capabilities.Vision,
			Voice:         m.Capabilities.Voice,
		}
	}

	return reg.Create(provCfg.Kind, provider.Config{
		Name:         provCfg.Name,
		DisplayName:  provCfg.DisplayName,
		BaseURL:      provCfg.BaseURL,
		APIKey:       os.Getenv(provCfg.APIKeyEnv),
		AuthMethod:   provCfg.AuthMethod,
		DefaultModel: provCfg.DefaultModel,
		Models:       models,
	})
}

func readStdin() string {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return ""
	}

	reader := bufio.NewReader(os.Stdin)
	var sb strings.Builder
	for {
		line, err := reader.ReadString('\n')
		sb.WriteString(line)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

func usage() {
	fmt.Fprintf(os.Stderr, "Helix CLI - 双螺旋 · 多模型 · 可扩展\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  helix [options] run <task>     Run a single task\n")
	fmt.Fprintf(os.Stderr, "  helix [options] setup          Run configuration wizard\n")
	fmt.Fprintf(os.Stderr, "  helix [options]                Interactive REPL\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}
