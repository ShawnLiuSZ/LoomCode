package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ShawnLiuSZ/Helix/internal/agent"
	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/session"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// App 交互式 TUI 应用
type App struct {
	width  int
	height int

	// 输入
	input     string
	cursorPos int

	// 聊天历史
	messages []chatMessage

	// 状态
	ready   bool
	loading bool
	quitting bool

	// Agent
	agent       *agent.MultiAgent
	provider    provider.Provider
	tools       *tool.Registry

	// 会话
	sessionMgr  *session.Manager
	activeSess  *session.Session

	// 模式
	mode        agent.Mode
	modeDisplay string

	// 环境变量
	envVars     map[string]string
	envEditing  bool
	envEditKey  string
	envEditVal  string

	// 成本
	costTotal   float64
	costSession float64
	costLast    float64

	// 流式输出缓冲
	streamBuf   string
}

type chatMessage struct {
	Role      string
	Content   string
	ToolName  string
	Timestamp time.Time
}

// 样式
var (
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	assistantStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	systemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	toolStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	inputStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	loadingStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	headerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Padding(0, 1)
	statusBarStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("7")).Padding(0, 1)
	costGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	costYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	costRedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// NewApp 创建 TUI 应用
func NewApp(p provider.Provider, tools *tool.Registry) *App {
	ag := agent.NewMultiAgent(p, tools)

	return &App{
		agent:    ag,
		provider: p,
		tools:    tools,
		mode:     agent.ModeBuild,
		envVars:  loadEnvVars(),
		messages: []chatMessage{
			{Role: "system", Content: "Helix CLI — 双螺旋 · 多模型 · 可扩展", Timestamp: time.Now()},
			{Role: "system", Content: "模式: build | /help 查看命令 | Ctrl+C 退出", Timestamp: time.Now()},
		},
	}
}

// SetSessionManager 设置会话管理器
func (a *App) SetSessionManager(mgr *session.Manager) {
	a.sessionMgr = mgr
}

// Init 初始化
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
	)
}

// Update 更新状态
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

	case tea.KeyMsg:
		return a.handleKey(msg)

	case streamChunkMsg:
		a.streamBuf += string(msg)
		return a, listenStream(a.streamBuf)

	case streamDoneMsg:
		a.loading = false
		a.messages = append(a.messages, chatMessage{
			Role:      "assistant",
			Content:   a.streamBuf,
			Timestamp: time.Now(),
		})
		a.streamBuf = ""
		return a, nil

	case streamErrorMsg:
		a.loading = false
		a.messages = append(a.messages, chatMessage{
			Role:    "error",
			Content: fmt.Sprintf("Error: %s", string(msg)),
		})
		return a, nil
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		a.quitting = true
		return a, tea.Quit

	case "esc":
		a.input = ""
		return a, nil

	case "enter":
		return a.handleEnter()

	case "backspace":
		if len(a.input) > 0 {
			a.input = a.input[:len(a.input)-1]
		}
		return a, nil

	case "tab":
		a.cycleMode()
		return a, nil

	default:
		// 普通字符
		if len(msg.String()) == 1 {
			a.input += msg.String()
		}
		return a, nil
	}
}

func (a *App) handleEnter() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(a.input)
	if input == "" {
		return a, nil
	}
	a.input = ""

	// 斜杠命令
	if strings.HasPrefix(input, "/") {
		return a.handleCommand(input)
	}

	// 添加用户消息
	a.messages = append(a.messages, chatMessage{
		Role:      "user",
		Content:   input,
		Timestamp: time.Now(),
	})

	a.loading = true
	return a, a.runAgent(input)
}

func (a *App) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "/quit", "/exit":
		a.quitting = true
		return a, tea.Quit

	case "/help":
		help := `Commands:
  /help      Show this help
  /mode      Show current mode
  /build     Switch to build mode
  /plan      Switch to plan mode (read-only)
  /compose   Switch to compose mode
  /max       Switch to max mode (experimental)
  /clear     Clear chat history
  /cost      Show cost summary
  /sessions  List sessions
  /quit      Exit`
		a.messages = append(a.messages, chatMessage{Role: "system", Content: help})
		return a, nil

	case "/mode":
		msg := fmt.Sprintf("Current mode: %s", a.mode.String())
		a.messages = append(a.messages, chatMessage{Role: "system", Content: msg})
		return a, nil

	case "/build":
		a.mode = agent.ModeBuild
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "Switched to build mode"})
		return a, nil

	case "/plan":
		a.mode = agent.ModePlan
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "Switched to plan mode (read-only)"})
		return a, nil

	case "/compose":
		a.mode = agent.ModeCompose
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "Switched to compose mode"})
		return a, nil

	case "/max":
		a.mode = agent.ModeMax
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "Switched to max mode (experimental)"})
		return a, nil

	case "/clear":
		a.messages = a.messages[:0]
		return a, nil

	case "/cost":
		msg := fmt.Sprintf("Session: $%.4f | Last: $%.4f | Total: $%.4f",
			a.costSession, a.costLast, a.costTotal)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: msg})
		return a, nil

	case "/env":
		return a.handleEnvCommand(parts)

	default:
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd),
		})
		return a, nil
	}
}

func (a *App) cycleMode() {
	modes := []agent.Mode{agent.ModeBuild, agent.ModePlan, agent.ModeCompose}
	for i, m := range modes {
		if m == a.mode {
			a.mode = modes[(i+1)%len(modes)]
			break
		}
	}
	a.agent.SetMode(a.mode)
	a.messages = append(a.messages, chatMessage{
		Role:    "system",
		Content: fmt.Sprintf("Mode: %s", a.mode.String()),
	})
}

// View 渲染界面
func (a *App) View() string {
	if a.quitting {
		return "Goodbye!\n"
	}
	if !a.ready {
		return "Initializing...\n"
	}

	var sb strings.Builder

	// 标题栏
	title := a.renderTitle()
	sb.WriteString(title)
	sb.WriteString("\n")

	// 分隔线
	sb.WriteString(strings.Repeat("─", a.width))
	sb.WriteString("\n")

	// 消息区域
	msgArea := a.height - 5 // 减去标题+输入+状态栏
	sb.WriteString(a.renderMessages(msgArea))

	// 加载指示器
	if a.loading {
		spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		idx := int(time.Now().UnixMilli()/100) % len(spinChars)
		sb.WriteString(loadingStyle.Render(fmt.Sprintf(" %s Thinking...", spinChars[idx])))
		sb.WriteString("\n")
	}

	// 输入区域
	sb.WriteString(a.renderInput())
	sb.WriteString("\n")

	// 状态栏
	sb.WriteString(a.renderStatusBar())

	return sb.String()
}

func (a *App) renderTitle() string {
	modeColor := map[agent.Mode]string{
		agent.ModeBuild:   "🛠",
		agent.ModePlan:    "📋",
		agent.ModeCompose: "📦",
		agent.ModeMax:     "⚡",
	}

	icon := modeColor[a.mode]
	title := fmt.Sprintf("%s Helix CLI | %s mode | %s",
		icon, a.mode.String(), a.provider.Name())

	return headerStyle.Width(a.width).Render(title)
}

func (a *App) renderMessages(visibleLines int) string {
	var sb strings.Builder

	// 计算可见消息
	startIdx := 0
	totalLines := 0
	for i := len(a.messages) - 1; i >= 0; i-- {
		lines := strings.Count(a.messages[i].Content, "\n") + 1
		totalLines += lines
		if totalLines > visibleLines {
			startIdx = i + 1
			break
		}
	}

	for _, msg := range a.messages[startIdx:] {
		switch msg.Role {
		case "user":
			sb.WriteString(userStyle.Render("▸ " + msg.Content))
		case "assistant":
			for _, line := range strings.Split(msg.Content, "\n") {
				sb.WriteString(assistantStyle.Render("  " + line))
				sb.WriteString("\n")
			}
			continue
		case "system":
			sb.WriteString(systemStyle.Render("  " + msg.Content))
		case "tool":
			sb.WriteString(toolStyle.Render("  🔧 " + msg.Content))
		case "error":
			sb.WriteString(errorStyle.Render("  ✖ " + msg.Content))
		}
		sb.WriteString("\n")
	}

	// 流式输出缓冲
	if a.loading && a.streamBuf != "" {
		for _, line := range strings.Split(a.streamBuf, "\n") {
			sb.WriteString(assistantStyle.Render("  " + line))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (a *App) renderInput() string {
	prompt := fmt.Sprintf(" %s > ", a.mode.String())
	full := prompt + a.input

	return inputStyle.Width(a.width - 2).Render(full)
}

func (a *App) renderStatusBar() string {
	costDisplay := a.renderCost()

	left := fmt.Sprintf(" %s | Tab:切换模式 | /help:帮助", a.provider.Name())
	right := costDisplay

	padding := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 0 {
		padding = 0
	}

	bar := left + strings.Repeat(" ", padding) + right
	return statusBarStyle.Width(a.width).Render(bar)
}

func (a *App) renderCost() string {
	if a.costSession < 0.05 {
		return costGreenStyle.Render(fmt.Sprintf("$%.4f", a.costSession))
	}
	if a.costSession < 0.20 {
		return costYellowStyle.Render(fmt.Sprintf("$%.4f", a.costSession))
	}
	return costRedStyle.Render(fmt.Sprintf("$%.4f", a.costSession))
}

// runAgent 异步运行 Agent
func (a *App) runAgent(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := a.agent.Run(ctx, input)

		if err != nil {
			return streamErrorMsg(err.Error())
		}

		// 模拟流式输出：将结果分块发送
		chunks := splitIntoChunks(result, 50)
		cmds := make([]tea.Cmd, len(chunks))
		for i, chunk := range chunks {
			c := chunk
			cmds[i] = tea.Tick(time.Duration(i*20)*time.Millisecond, func(t time.Time) tea.Msg {
				return streamChunkMsg(c)
			})
		}

		return tea.Sequence(append(cmds, func() tea.Msg {
			return streamDoneMsg{}
		})...)
	}
}

func splitIntoChunks(s string, size int) []string {
	var chunks []string
	runes := []rune(s)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

// 消息类型
type streamChunkMsg string
type streamDoneMsg struct{}
type streamErrorMsg string

// listenStream 监听流式输出（用于 Bubble Tea 循环）
func listenStream(buf string) tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return streamChunkMsg("")
	})
}

// ============================================================
// 环境变量管理
// ============================================================

// loadEnvVars 加载已知环境变量
func loadEnvVars() map[string]string {
	vars := make(map[string]string)
	keys := []string{
		"DEEPSEEK_API_KEY",
		"MIMO_API_KEY",
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
	}

	for _, key := range keys {
		if val := getEnv(key); val != "" {
			vars[key] = maskValue(val)
		}
	}

	return vars
}

func getEnv(key string) string {
	return os.Getenv(key)
}

func maskValue(val string) string {
	if len(val) <= 8 {
		return strings.Repeat("*", len(val))
	}
	return val[:4] + strings.Repeat("*", len(val)-8) + val[len(val)-4:]
}

// handleEnvCommand 处理 /env 命令
func (a *App) handleEnvCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		// /env — 显示所有环境变量
		return a.showEnvVars()
	}

	subCmd := parts[1]
	switch subCmd {
	case "set":
		if len(parts) < 4 {
			a.messages = append(a.messages, chatMessage{
				Role:    "system",
				Content: "Usage: /env set <KEY> <VALUE>",
			})
			return a, nil
		}
		key := parts[2]
		val := strings.Join(parts[3:], " ")
		return a.setEnvVar(key, val)

	case "unset":
		if len(parts) < 3 {
			a.messages = append(a.messages, chatMessage{
				Role:    "system",
				Content: "Usage: /env unset <KEY>",
			})
			return a, nil
		}
		return a.unsetEnvVar(parts[2])

	case "reload":
		a.envVars = loadEnvVars()
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: "Environment variables reloaded",
		})
		return a, nil

	default:
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: fmt.Sprintf("Unknown env command: %s. Use: /env [set|unset|reload]", subCmd),
		})
		return a, nil
	}
}

func (a *App) showEnvVars() (tea.Model, tea.Cmd) {
	if len(a.envVars) == 0 {
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: "No environment variables configured.\n\nUse /env set <KEY> <VALUE> to add one.\nCommon keys: DEEPSEEK_API_KEY, MIMO_API_KEY, OPENAI_API_KEY",
		})
		return a, nil
	}

	var sb strings.Builder
	sb.WriteString("Environment Variables:\n\n")
	for key, val := range a.envVars {
		sb.WriteString(fmt.Sprintf("  %s = %s\n", key, val))
	}
	sb.WriteString("\nCommands: /env set <KEY> <VAL> | /env unset <KEY> | /env reload")

	a.messages = append(a.messages, chatMessage{Role: "system", Content: sb.String()})
	return a, nil
}

func (a *App) setEnvVar(key, val string) (tea.Model, tea.Cmd) {
	a.envVars[key] = maskValue(val)
	a.messages = append(a.messages, chatMessage{
		Role:    "system",
		Content: fmt.Sprintf("Set %s = %s\n\nNote: This only affects the current TUI session. For permanent changes, set the environment variable in your shell profile.", key, maskValue(val)),
	})
	return a, nil
}

func (a *App) unsetEnvVar(key string) (tea.Model, tea.Cmd) {
	delete(a.envVars, key)
	a.messages = append(a.messages, chatMessage{
		Role:    "system",
		Content: fmt.Sprintf("Unset %s", key),
	})
	return a, nil
}
