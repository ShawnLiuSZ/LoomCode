package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ShawnLiuSZ/loomcode/internal/agent"
	"github.com/ShawnLiuSZ/loomcode/internal/config"
	"github.com/ShawnLiuSZ/loomcode/internal/control"
	"github.com/ShawnLiuSZ/loomcode/internal/dashboard"
	"github.com/ShawnLiuSZ/loomcode/internal/mcp"
	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/provider/deepseek"
	"github.com/ShawnLiuSZ/loomcode/internal/provider/mimo"
	"github.com/ShawnLiuSZ/loomcode/internal/provider/openai"
	"github.com/ShawnLiuSZ/loomcode/internal/session"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
	"github.com/ShawnLiuSZ/loomcode/internal/ui"
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
	flagSession  = flag.String("session", "", "Session ID to resume")
	flagVersion  = flag.Bool("version", false, "Show version")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	// 注入版本号到 UI 包
	ui.Version = version

	// 注册任务存储
	tool.SetTaskStore(tool.NewTaskStore())

	args := flag.Args()

	// 默认：无参数直接启动 TUI
	if len(args) == 0 {
		if *flagVersion {
			fmt.Printf("LoomCode CLI %s (commit: %s, built: %s)\n", version, commit, date)
			return
		}
		chatCommand()
		return
	}

	cmd := args[0]

	switch cmd {
	case "run":
		runCommand(args[1:])
	case "setup":
		setupCommand()
	case "chat", "tui":
		chatCommand()
	case "dashboard":
		dashboardCommand(args[1:])
	case "schema":
		fmt.Println(config.GenerateJSONSchema())
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Available commands: run, setup, chat, dashboard, schema")
		os.Exit(1)
	}
}

type runtime struct {
	cfg     *config.Config
	provCfg *config.ProviderConfig
	prov    provider.Provider
	tools   *tool.Registry
	perm    *control.Permission
	trust   *control.WorkspaceTrust
	plugins *mcp.PluginManager
	cpMgr   *tool.CheckpointManager
}

func initRuntime(chatMode bool) (*runtime, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("配置加载失败: %w", err)
	}

	tool.SetEnvProvider(&envProvider{cfg: cfg})

	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}

	// 加载工作区信任配置；首次启动时询问用户是否信任当前工作区。
	home, _ := os.UserHomeDir()
	trustPath := filepath.Join(home, ".loomcode", "trust.json")
	trust := control.NewWorkspaceTrust()
	if err := trust.Load(trustPath); err != nil {
		return nil, fmt.Errorf("加载信任配置失败: %w", err)
	}
	if !trust.IsTrusted(cwd) && trust.Decision(cwd) == "" {
		decision, err := control.PromptTrust(cwd)
		if err != nil {
			return nil, fmt.Errorf("信任确认失败: %w", err)
		}
		if err := trust.SetDecision(cwd, decision); err != nil {
			return nil, fmt.Errorf("保存信任决策失败: %w", err)
		}
	}

	provCfg, err := selectProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("provider 选择失败: %w", err)
	}

	p, err := createProvider(provCfg)
	if err != nil {
		return nil, fmt.Errorf("provider 创建失败: %w", err)
	}

	tools := tool.NewRegistry()
	tools.RegisterDefaults()

	perm := control.NewPermission(control.ModeAuto)
	var cpMgr *tool.CheckpointManager
	if chatMode {
		cpMgr = tool.NewCheckpointManager("")
	}
	configureToolPermissions(tools, perm, cpMgr, trust)

	pm := connectPlugins(context.Background(), cfg.Plugins, tools)

	return &runtime{
		cfg:     cfg,
		provCfg: provCfg,
		prov:    p,
		tools:   tools,
		perm:    perm,
		trust:   trust,
		plugins: pm,
		cpMgr:   cpMgr,
	}, nil
}

func runCommand(args []string) {
	var task string
	if len(args) > 0 {
		task = strings.Join(args, " ")
	} else {
		task = readStdin()
	}

	if task == "" {
		fmt.Fprintln(os.Stderr, "错误: 未提供任务")
		os.Exit(1)
	}

	r, err := initRuntime(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		fmt.Fprintln(os.Stderr, "提示: 运行 loomcode setup 生成配置文件")
		os.Exit(1)
	}
	if r.plugins != nil {
		defer r.plugins.DisconnectAll()
	}

	ag := agent.New(r.prov, r.tools)
	ag.SetMaxSteps(20)
	ag.SetModel(selectModel(r.provCfg))

	hm := tool.NewHookManager()
	registerAutoFormatHook(hm)
	ag.SetHooks(hm)

	fmt.Fprintf(os.Stderr, "Running with %s/%s...\n", r.provCfg.Name, selectModel(r.provCfg))
	fmt.Fprintln(os.Stderr, "---")

	ctx := context.Background()
	result, err := ag.Run(ctx, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}

func setupCommand() {
	fmt.Println("LoomCode CLI 配置向导")
	fmt.Println("=================")
	fmt.Println()

	w := config.NewWizard()
	cfg, envVars, err := w.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置失败: %v\n", err)
		os.Exit(1)
	}

	// 写入 ~/.loomcode/loomcode.json（全局配置目录）
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".loomcode")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建配置目录失败: %v\n", err)
		os.Exit(1)
	}
	configPath := filepath.Join(configDir, "loomcode.json")
	if err := config.WriteConfig(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "写入 %s 失败: %v\n", configPath, err)
		os.Exit(1)
	}
	fmt.Printf("✓ %s 已生成\n", configPath)

	// 写入 ~/.loomcode/.env（全局，已存在则追加）
	envPath := filepath.Join(configDir, ".env")
	if err := config.WriteEnvFile(envVars, envPath); err != nil {
		fmt.Fprintf(os.Stderr, "写入 %s 失败: %v\n", envPath, err)
		os.Exit(1)
	}
	fmt.Printf("✓ %s 已生成\n", envPath)

	// 写出 JSON Schema 供编辑器自动补全/校验
	if schemaPath, err := config.WriteSchemaFile(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write schema file: %v\n", err)
	} else {
		fmt.Printf("✓ 配置 Schema 已生成: %s\n", schemaPath)
	}

	fmt.Println()
	fmt.Println("配置完成！运行以下命令开始:")
	fmt.Println("  loomcode chat        # 启动交互式 TUI")
	fmt.Println("  loomcode run \"hello\" # 运行单个任务")
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

// connectPlugins 按配置连接所有 MCP 插件（stdio 或 SSE），把其工具注册进 tools。
// 单个插件连接失败不影响启动（打印告警后跳过）。返回 manager（无插件时返回 nil）。
func connectPlugins(ctx context.Context, plugins []config.PluginConfig, tools *tool.Registry) *mcp.PluginManager {
	if len(plugins) == 0 {
		return nil
	}
	pm := mcp.NewPluginManager(tools)
	for _, pc := range plugins {
		var err error
		switch pc.Kind() {
		case "sse":
			err = pm.ConnectSSE(ctx, pc.Name, pc.URL)
		case "stdio":
			err = pm.Connect(pc.Name, pc.Command, pc.Args...)
		default:
			fmt.Fprintf(os.Stderr, "Warning: MCP plugin %q has neither command nor url; skipped\n", pc.Name)
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP plugin %q connect failed: %v\n", pc.Name, err)
		}
	}
	return pm
}

// setToolTrust 将 TUI App 作为工作区外文件访问信任检查器注入到文件/Git 工具。
func setToolTrust(tools *tool.Registry, trust tool.OutsideTrustChecker) {
	if t, ok := tools.Get("read_file"); ok {
		if ft, ok := t.(*tool.ReadFileTool); ok {
			ft.SetTrust(trust)
		}
	}
	if t, ok := tools.Get("write_file"); ok {
		if ft, ok := t.(*tool.WriteFileTool); ok {
			ft.SetTrust(trust)
		}
	}
	if t, ok := tools.Get("edit_file"); ok {
		if ft, ok := t.(*tool.EditFileTool); ok {
			ft.SetTrust(trust)
		}
	}
	if t, ok := tools.Get("git_diff"); ok {
		if gt, ok := t.(*tool.GitDiffTool); ok {
			gt.SetTrust(trust)
		}
	}
	if t, ok := tools.Get("git_log"); ok {
		if gt, ok := t.(*tool.GitLogTool); ok {
			gt.SetTrust(trust)
		}
	}
}

// configureToolPermissions 给文件工具与 bash 接入权限护栏（C2），
// 并按"工作区内放行"模型配置 Auto 模式的默认白名单（H6）。
// cpMgr 非空时注入到写文件/编辑文件工具，启用编辑快照安全网。
// trust 用于工作区外文件访问的临时/永久授权提示。
func configureToolPermissions(tools *tool.Registry, perm *control.Permission, cpMgr *tool.CheckpointManager, trust *control.WorkspaceTrust) {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}

	// 工作区根：文件读写限制在 cwd 之内；Auto 模式下区内写入放行、区外拒绝。
	perm.Allowlist().SetAllowedPaths([]string{cwd})
	// Auto 模式下默认可用的安全 shell 命令；其余需用户显式加入或切到 review 模式。
	perm.Allowlist().SetShellCommands([]string{
		"ls", "cat", "head", "tail", "grep", "find",
		"pwd", "echo", "wc", "git", "go", "which",
	})

	if t, ok := tools.Get("bash"); ok {
		if bt, ok := t.(*tool.BashTool); ok {
			bt.SetPermissionChecker(perm)
		}
	}
	if t, ok := tools.Get("read_file"); ok {
		if ft, ok := t.(*tool.ReadFileTool); ok {
			ft.SetRoot(cwd)
			ft.SetPermissionChecker(perm)
			ft.SetTrust(trust)
		}
	}
	if t, ok := tools.Get("write_file"); ok {
		if ft, ok := t.(*tool.WriteFileTool); ok {
			ft.SetRoot(cwd)
			ft.SetPermissionChecker(perm)
			ft.SetTrust(trust)
			ft.SetDiagnoser(tool.GoDiagnoser{})
			if cpMgr != nil {
				ft.SetCheckpointManager(cpMgr)
			}
		}
	}
	if t, ok := tools.Get("edit_file"); ok {
		if ft, ok := t.(*tool.EditFileTool); ok {
			ft.SetRoot(cwd)
			ft.SetPermissionChecker(perm)
			ft.SetTrust(trust)
			ft.SetDiagnoser(tool.GoDiagnoser{})
			if cpMgr != nil {
				ft.SetCheckpointManager(cpMgr)
			}
		}
	}
	if t, ok := tools.Get("git_diff"); ok {
		if gt, ok := t.(*tool.GitDiffTool); ok {
			gt.SetRoot(cwd)
			gt.SetTrust(trust)
		}
	}
	if t, ok := tools.Get("git_log"); ok {
		if gt, ok := t.(*tool.GitLogTool); ok {
			gt.SetRoot(cwd)
			gt.SetTrust(trust)
		}
	}
}

// resolveAPIKey 解析 Provider 的 API Key。
// 若配置中直接填写 api_key 则优先使用；否则从 api_key_env 指定的环境变量读取。
func resolveAPIKey(provCfg *config.ProviderConfig) string {
	if provCfg.APIKey != "" {
		return provCfg.APIKey
	}
	if provCfg.APIKeyEnv != "" {
		return os.Getenv(provCfg.APIKeyEnv)
	}
	return ""
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
	reg.Register(&mimo.Adapter{})

	models := make([]provider.ModelConfigItem, len(provCfg.Models))
	for i, m := range provCfg.Models {
		models[i] = provider.ModelConfigItem{
			ID:              m.ID,
			Name:            m.Name,
			ContextWindow:   m.ContextWindow,
			CostInput:       m.Cost.Input,
			CostCachedInput: m.Cost.CachedInput,
			CostOutput:      m.Cost.Output,
			Reasoning:       m.Capabilities.Reasoning,
			ToolCall:        m.Capabilities.ToolCall,
			PrefixCache:     m.Capabilities.PrefixCache,
			Vision:          m.Capabilities.Vision,
			Voice:           m.Capabilities.Voice,
		}
	}

	apiKey := resolveAPIKey(provCfg)

	return reg.Create(provCfg.Kind, provider.Config{
		Name:         provCfg.Name,
		DisplayName:  provCfg.DisplayName,
		BaseURL:      provCfg.BaseURL,
		APIKey:       apiKey,
		AuthMethod:   provCfg.AuthMethod,
		DefaultModel: provCfg.DefaultModel,
		Models:       models,
	})
}

func registerAutoFormatHook(hm *tool.HookManager) {
	gofmtPath, err := exec.LookPath("gofmt")
	if err != nil {
		return
	}

	gofmtHandler := func(ctx context.Context, call tool.Call, result *tool.Result) error {
		if result != nil && result.Error != "" {
			return nil
		}
		path, _ := call.Args["path"].(string)
		if path == "" || !strings.HasSuffix(path, ".go") {
			return nil
		}
		cmd := exec.CommandContext(ctx, gofmtPath, "-w", path)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gofmt: %s: %w", strings.TrimSpace(string(output)), err)
		}
		return nil
	}

	makeGofmtHook := func(toolName string) tool.Hook {
		return tool.Hook{
			Name:     "auto-gofmt-" + toolName,
			Type:     tool.HookPostExecute,
			ToolName: toolName,
			Handler:  gofmtHandler,
		}
	}

	hm.Add(makeGofmtHook("write_file"))
	hm.Add(makeGofmtHook("edit_file"))
}

func chatCommand() {
	r, err := initRuntime(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if r.plugins != nil {
		defer r.plugins.DisconnectAll()
	}

	allProviders := []provider.Provider{r.prov}
	for _, pc := range r.cfg.Providers {
		if pc.Name == r.provCfg.Name {
			continue
		}
		if op, err := createProvider(&pc); err == nil {
			allProviders = append(allProviders, op)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: provider %q 创建失败，已跳过: %v\n", pc.Name, err)
		}
	}

	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".loomcode", "sessions")
	sessionMgr, err := session.NewManager(sessionDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: session manager init failed: %v\n", err)
	}

	if sessionMgr != nil {
		tool.SetSessionManagerForTools(r.tools, sessionMgr)
	}

	app := ui.NewApp(r.prov, r.tools)
	app.SetModel(selectModel(r.provCfg))
	app.SetProviders(allProviders)
	app.AddApprovalGuard(r.perm)
	app.SetCheckpointManager(r.cpMgr)
	app.SetTrust(r.trust)

	// TUI 模式下使用 App 的 TUI 提示替代 stdin 提示。
	setToolTrust(r.tools, app)

	hm := tool.NewHookManager()
	registerAutoFormatHook(hm)
	app.SetHooks(hm)

	if sessionMgr != nil {
		app.SetSessionManager(sessionMgr)

		if *flagSession != "" {
			if sess, ok := sessionMgr.Get(*flagSession); ok {
				if err := sessionMgr.SetActive(*flagSession); err != nil {
					log.Printf("activate session: %v", err)
				}
				app.RestoreSession(sess)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: session %q not found, starting new session\n", *flagSession)
			}
		} else {
			// 默认启动：从最近会话恢复上次选择的模型/provider（不恢复消息历史）
			if recent := sessionMgr.MostRecent(); recent != nil {
				app.RestoreModelFromSession(recent)
			}
		}
	}

	// 启用 WithMouseCellMotion：支持鼠标滚轮滚动 viewport，同时保留 Shift+拖动选中文本。
	program := tea.NewProgram(app, tea.WithAltScreen(), tea.WithInputTTY(), tea.WithMouseCellMotion())
	app.SetProgram(program)

	// 1.13 修复：捕获 SIGTERM/SIGHUP，终端关闭或 kill 时通知 TUI 保存退出，
	// 否则对话内容丢失。Bubble Tea 的 Ctrl+C 走 KeyMsg，系统信号需单独处理。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		program.Send(tea.Quit())
	}()

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI 运行错误: %v\n", err)
		os.Exit(1)
	}

	// 退出时打印 session ID，方便用户下次 --session 恢复
	if sid := app.SessionID(); sid != "" {
		fmt.Fprintf(os.Stderr, "\n🔁 使用 --session %s 可恢复当前会话\n", sid)
	}
}

func readStdin() string {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return ""
	}
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
	fmt.Fprintf(os.Stderr, "LoomCode CLI - 双螺旋 · 多模型 · 可扩展\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  loomcode [options] run <task>     Run a single task\n")
	fmt.Fprintf(os.Stderr, "  loomcode [options] setup          Run configuration wizard\n")
	fmt.Fprintf(os.Stderr, "  loomcode [options] chat           Interactive TUI\n")
	fmt.Fprintf(os.Stderr, "  loomcode [options] dashboard      Start web dashboard\n")
	fmt.Fprintf(os.Stderr, "  loomcode [options] schema         Print JSON Schema for loomcode.json\n")
	fmt.Fprintf(os.Stderr, "  loomcode [options]                Start interactive TUI (default)\n\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  loomcode                                    Start TUI\n")
	fmt.Fprintf(os.Stderr, "  loomcode --session session_1234567890       Resume session\n")
	fmt.Fprintf(os.Stderr, "  loomcode --provider deepseek --model deepseek-v4-pro\n")
	fmt.Fprintf(os.Stderr, "  loomcode run \"explain this code\"\n")
	fmt.Fprintf(os.Stderr, "  loomcode dashboard :9090                    Dashboard on port 9090\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}

// ExportEnvToSubprocess 将当前环境变量导出到子进程
// 工具执行（bash 等）时自动继承
func ExportEnvToSubprocess(cfg *config.Config) []string {
	apiKeys := make(map[string]bool)
	for _, p := range cfg.Providers {
		if p.APIKeyEnv != "" {
			apiKeys[p.APIKeyEnv] = true
		}
	}

	shellKeys := []string{
		"LOOMCODE_PROVIDER",
		"LOOMCODE_MODEL",
		"PATH",
		"HOME",
		"USER",
	}

	env := os.Environ()
	filtered := make([]string, 0, len(shellKeys))

	for _, e := range env {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			continue
		}
		key := e[:idx]

		if apiKeys[key] {
			continue
		}

		for _, sk := range shellKeys {
			if key == sk {
				val := e[idx+1:]
				if val == "" {
					continue
				}
				filtered = append(filtered, e)
				break
			}
		}
	}

	return filtered
}

// envProvider 实现 tool.EnvProvider 接口
type envProvider struct {
	cfg *config.Config
}

func (p *envProvider) EnvForSubprocess() []string {
	return ExportEnvToSubprocess(p.cfg)
}

// dashboardCommand 启动 Web Dashboard
func dashboardCommand(args []string) {
	addr := ":8080"
	if len(args) > 0 {
		addr = args[0]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := dashboard.NewServer(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dashboard 初始化失败: %v\n", err)
		os.Exit(1)
	}
	if err := server.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Dashboard 启动失败: %v\n", err)
		os.Exit(1)
	}
}
