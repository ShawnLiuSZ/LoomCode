package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ShawnLiuSZ/Helix/internal/agent"
	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/session"
	"github.com/ShawnLiuSZ/Helix/internal/skills"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// App 交互式 TUI 应用
type App struct {
	width  int
	height int

	// 输入
	input     string
	cursorPos int

	// 命令联想
	showSuggestions bool
	suggestions     []string
	suggestionIdx   int

	// 聊天历史
	messages []chatMessage

	// 状态
	ready    bool
	loading  bool
	quitting bool

	// Agent
	agent    *agent.MultiAgent
	provider provider.Provider
	tools    *tool.Registry
	model    string

	// 会话
	sessionMgr *session.Manager
	activeSess *session.Session

	// 模式
	mode        agent.Mode
	modeDisplay string

	// 环境变量
	envVars map[string]string

	// Skills
	skillsMgr *skills.Manager

	// 模型选择状态
	showModelPicker bool
	modelList      []string
	modelIdx       int

	// 成本
	costTotal   float64
	costSession float64
	costLast    float64

	// 流式输出缓冲
	streamBuf string
}

type chatMessage struct {
	Role      string
	Content   string
	ToolName  string
	Timestamp time.Time
}

// 所有可用命令
var allCommands = []string{
	"/help", "/mode", "/build", "/plan", "/compose", "/max",
	"/clear", "/cost", "/env", "/model", "/skills", "/sessions", "/quit",
}

// 样式
var (
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	assistantStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	systemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	toolStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	inputStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	cursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("7"))
	suggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	suggestionSel   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4"))
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

	// 加载 skills
	skillsMgr := skills.NewManager()
	skillsMgr.Load()

	return &App{
		agent:     ag,
		provider:  p,
		tools:     tools,
		mode:      agent.ModeBuild,
		envVars:   loadEnvVars(),
		skillsMgr: skillsMgr,
		messages: []chatMessage{
			{Role: "system", Content: "Helix CLI — 双螺旋 · 多模型 · 可扩展", Timestamp: time.Now()},
			{Role: "system", Content: "输入任务开始 | 输入 / 查看命令 | Tab 切换模式 | Ctrl+C 退出", Timestamp: time.Now()},
		},
	}
}

func (a *App) SetSessionManager(mgr *session.Manager) { a.sessionMgr = mgr }
func (a *App) SetModel(m string)                       { a.model = m; a.agent.SetModel(m) }

// RestoreSession 恢复历史会话
func (a *App) RestoreSession(sess *session.Session) {
	a.activeSess = sess
	a.messages = []chatMessage{
		{Role: "system", Content: fmt.Sprintf("已恢复会话: %s (%d 条消息)", sess.Name, len(sess.Messages)), Timestamp: time.Now()},
	}
	for _, msg := range sess.Messages {
		a.messages = append(a.messages, chatMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolName:  msg.ToolName,
			Timestamp: msg.Timestamp,
		})
	}
	a.messages = append(a.messages, chatMessage{
		Role: "system", Content: "会话已恢复，继续对话或输入 /help 查看命令", Timestamp: time.Now(),
	})
}

func (a *App) Init() tea.Cmd { return tea.EnterAltScreen }

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
		return a, nil
	case streamDoneMsg:
		a.loading = false
		a.messages = append(a.messages, chatMessage{Role: "assistant", Content: a.streamBuf, Timestamp: time.Now()})
		a.streamBuf = ""
		return a, nil
	case streamErrorMsg:
		a.loading = false
		errStr := string(msg)
		a.messages = append(a.messages, chatMessage{Role: "error", Content: errStr, Timestamp: time.Now()})
		return a, nil
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 处理多字节字符（中文、日文等）
	if msg.Type == tea.KeyRunes {
		for _, r := range msg.Runes {
			a.input += string(r)
		}
		a.cursorPos = len([]rune(a.input))
		if strings.HasPrefix(a.input, "/") {
			a.updateSuggestions()
		}
		return a, nil
	}

	key := msg.String()

	// 模型选择器模式
	if a.showModelPicker {
		switch key {
		case "down":
			a.modelIdx = (a.modelIdx + 1) % len(a.modelList)
			return a, nil
		case "up":
			a.modelIdx = (a.modelIdx - 1 + len(a.modelList)) % len(a.modelList)
			return a, nil
		case "enter":
			if a.modelIdx >= 0 && a.modelIdx < len(a.modelList) {
				a.model = a.modelList[a.modelIdx]
				a.agent.SetModel(a.model)
				a.messages = append(a.messages, chatMessage{
					Role: "system", Content: fmt.Sprintf("模型切换为: %s", a.model),
				})
			}
			a.showModelPicker = false
			return a, nil
		case "esc":
			a.showModelPicker = false
			return a, nil
		}
	}

	// 命令联想模式
	if a.showSuggestions {
		switch key {
		case "tab", "down":
			a.suggestionIdx = (a.suggestionIdx + 1) % len(a.suggestions)
			return a, nil
		case "shift+tab", "up":
			a.suggestionIdx = (a.suggestionIdx - 1 + len(a.suggestions)) % len(a.suggestions)
			return a, nil
		case "enter":
			if len(a.suggestions) > 0 {
				a.input = a.suggestions[a.suggestionIdx] + " "
				a.showSuggestions = false
				a.suggestions = nil
			}
			return a, nil
		case "esc":
			a.showSuggestions = false
			a.suggestions = nil
			return a, nil
		}
	}

	switch key {
	case "ctrl+c":
		a.quitting = true
		return a, tea.Quit
	case "esc":
		a.input = ""
		a.showSuggestions = false
		return a, nil
	case "enter":
		return a.handleEnter()
	case "backspace":
		runes := []rune(a.input)
		if len(runes) > 0 {
			runes = runes[:len(runes)-1]
			a.input = string(runes)
		}
		a.cursorPos = len(runes)
		a.updateSuggestions()
		return a, nil
	case "tab":
		if strings.HasPrefix(a.input, "/") {
			// 命令联想
			a.showSuggestions = true
			a.updateSuggestions()
		} else {
			a.cycleMode()
		}
		return a, nil
	default:
		if len(key) == 1 {
			a.input += key
			a.cursorPos = len([]rune(a.input))
			if strings.HasPrefix(a.input, "/") {
				a.updateSuggestions()
			}
		}
		return a, nil
	}
}

func (a *App) updateSuggestions() {
	if !strings.HasPrefix(a.input, "/") {
		a.showSuggestions = false
		a.suggestions = nil
		return
	}

	prefix := strings.ToLower(a.input)
	var matches []string
	for _, cmd := range allCommands {
		if strings.HasPrefix(strings.ToLower(cmd), prefix) {
			matches = append(matches, cmd)
		}
	}
	sort.Strings(matches)

	if len(matches) > 0 {
		a.suggestions = matches
		a.showSuggestions = true
		if a.suggestionIdx >= len(matches) {
			a.suggestionIdx = 0
		}
	} else {
		a.showSuggestions = false
		a.suggestions = nil
	}
}

func (a *App) handleEnter() (tea.Model, tea.Cmd) {
	a.showSuggestions = false

	input := strings.TrimSpace(a.input)
	if input == "" {
		return a, nil
	}
	a.input = ""

	if strings.HasPrefix(input, "/") {
		return a.handleCommand(input)
	}

	a.messages = append(a.messages, chatMessage{Role: "user", Content: input, Timestamp: time.Now()})
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
		help := `📋 命令列表:
  /help      显示帮助
  /mode      显示当前模式
  /build     切换到 build 模式
  /plan      切换到 plan 模式(只读)
  /compose   切换到 compose 模式
  /max       切换到 max 模式(实验)
  /model     显示/切换模型
  /skills    显示可用工具列表
  /clear     清空聊天
  /cost      显示成本
  /env       环境变量管理
  /sessions  会话列表
  /quit      退出

💡 提示:
  Tab 切换模式 | 输入 / 后 Tab 联想命令
  直接输入任务开始对话`
		a.messages = append(a.messages, chatMessage{Role: "system", Content: help})
		return a, nil

	case "/model":
		return a.handleModelCmd(parts)

	case "/skills":
		return a.handleSkillsCmd()

	case "/mode":
		a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("当前模式: %s | 模型: %s", a.mode.String(), a.model)})
		return a, nil

	case "/build":
		a.mode = agent.ModeBuild
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "切换到 build 模式"})
		return a, nil

	case "/plan":
		a.mode = agent.ModePlan
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "切换到 plan 模式(只读)"})
		return a, nil

	case "/compose":
		a.mode = agent.ModeCompose
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "切换到 compose 模式"})
		return a, nil

	case "/max":
		a.mode = agent.ModeMax
		a.agent.SetMode(a.mode)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "切换到 max 模式(实验)"})
		return a, nil

	case "/clear":
		a.messages = a.messages[:0]
		return a, nil

	case "/cost":
		msg := fmt.Sprintf("会话: $%.4f | 上次: $%.4f | 累计: $%.4f", a.costSession, a.costLast, a.costTotal)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: msg})
		return a, nil

	case "/env":
		return a.handleEnvCommand(parts)

	default:
		a.messages = append(a.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("未知命令: %s。输入 /help 查看可用命令。", cmd),
		})
		return a, nil
	}
}

func (a *App) handleModelCmd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		// 从 Provider 动态获取模型列表
		models := a.provider.Models()
		if len(models) == 0 {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "当前 Provider 没有注册模型"})
			return a, nil
		}

		// 启动交互式模型选择器
		a.modelList = make([]string, len(models))
		for i, m := range models {
			a.modelList[i] = m.ID
		}
		a.modelIdx = 0
		for i, id := range a.modelList {
			if id == a.model {
				a.modelIdx = i
				break
			}
		}
		a.showModelPicker = true

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("选择模型 (↑↓ 移动, Enter 确认, Esc 取消):\n\n"))
		sb.WriteString(fmt.Sprintf("当前: %s\n\n", a.model))
		sb.WriteString("可用模型:\n")
		for i, id := range a.modelList {
			marker := "  "
			if i == a.modelIdx {
				marker = "▶ "
			}
			sb.WriteString(fmt.Sprintf("%s%s\n", marker, id))
		}
		a.messages = append(a.messages, chatMessage{Role: "system", Content: sb.String()})
		return a, nil
	}

	newModel := parts[1]
	a.model = newModel
	a.agent.SetModel(newModel)
	a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("模型切换为: %s", newModel)})
	return a, nil
}

func (a *App) handleSkillsCmd() (tea.Model, tea.Cmd) {
	// 内置工具
	tools := a.tools.List()
	// 外部 skills
	skillList := a.skillsMgr.List()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 内置工具 (%d):\n\n", len(tools)))
	for _, t := range tools {
		icon := "✏️"
		if t.IsReadOnly() {
			icon = "📖"
		}
		sb.WriteString(fmt.Sprintf("  %s %s - %s\n", icon, t.Name(), t.Description()))
	}

	if len(skillList) > 0 {
		sb.WriteString(fmt.Sprintf("\n🧩 外部 Skills (%d):\n\n", len(skillList)))
		for _, s := range skillList {
			source := ""
			if s.Source == "helix" {
				source = " [helix]"
			}
			sb.WriteString(fmt.Sprintf("  📄 %s%s - %s\n", s.Name, source, s.Description))
		}
	}

	a.messages = append(a.messages, chatMessage{Role: "system", Content: sb.String()})
	return a, nil
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
	a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("模式: %s", a.mode.String())})
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
	sb.WriteString(a.renderTitle())
	sb.WriteString("\n")

	// 分隔线
	sb.WriteString(strings.Repeat("─", a.width))
	sb.WriteString("\n")

	// 消息区域
	suggLines := 0
	if a.showSuggestions && len(a.suggestions) > 0 {
		suggLines = len(a.suggestions) + 1
	}
	msgArea := a.height - 5 - suggLines
	if msgArea < 3 {
		msgArea = 3
	}
	sb.WriteString(a.renderMessages(msgArea))

	// 加载指示器
	if a.loading {
		spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		idx := int(time.Now().UnixMilli()/100) % len(spinChars)
		sb.WriteString(loadingStyle.Render(fmt.Sprintf(" %s 思考中...", spinChars[idx])))
		sb.WriteString("\n")
	}

	// 命令联想
	if a.showSuggestions && len(a.suggestions) > 0 {
		for i, s := range a.suggestions {
			if i == a.suggestionIdx {
				sb.WriteString(suggestionSel.Render(" ▶ " + s))
			} else {
				sb.WriteString(suggestionStyle.Render("   " + s))
			}
			sb.WriteString("\n")
		}
	}

	// 输入区域（带光标）
	sb.WriteString(a.renderInput())
	sb.WriteString("\n")

	// 状态栏
	sb.WriteString(a.renderStatusBar())

	return sb.String()
}

func (a *App) renderTitle() string {
	modeIcons := map[agent.Mode]string{
		agent.ModeBuild: "🛠", agent.ModePlan: "📋", agent.ModeCompose: "📦", agent.ModeMax: "⚡",
	}
	icon := modeIcons[a.mode]
	title := fmt.Sprintf("%s Helix CLI | %s | %s | %s",
		icon, a.mode.String(), a.provider.Name(), a.model)
	return headerStyle.Width(a.width).Render(title)
}

func (a *App) renderMessages(visibleLines int) string {
	var sb strings.Builder
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
			sb.WriteString(errorStyle.Render("  ✖ " + truncateStr(msg.Content, a.width-4)))
		}
		sb.WriteString("\n")
	}

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

	// 光标渲染：在 rune 位置后插入反色光标
	if a.cursorPos >= 0 {
		inputRunes := []rune(a.input)
		cp := a.cursorPos
		if cp > len(inputRunes) {
			cp = len(inputRunes)
		}

		// 计算光标在 prompt 之后的字节偏移
		cursorBytePos := len(prompt)
		for i := 0; i < cp; i++ {
			cursorBytePos += len(string(inputRunes[i]))
		}

		if cursorBytePos >= 0 && cursorBytePos <= len(full) {
			before := full[:cursorBytePos]
			after := full[cursorBytePos:]
			// 如果光标在末尾，高亮一个空格作为光标指示
			if cp == len(inputRunes) && len(inputRunes) > 0 {
				full = before + cursorStyle.Render(" ") + after
			} else if cursorBytePos < len(full) {
				// 光标在字符上，高亮该字符
				rest := full[cursorBytePos:]
				charRunes := []rune(rest)
				char := string(charRunes[0])
				charByteLen := len(char)
				afterChar := ""
				if cursorBytePos+charByteLen < len(full) {
					afterChar = full[cursorBytePos+charByteLen:]
				}
				full = before + cursorStyle.Render(char) + afterChar
			} else {
				full = before + cursorStyle.Render(" ") + after
			}
		}
	}

	return inputStyle.Width(a.width - 2).Render(full)
}

func (a *App) renderStatusBar() string {
	costDisplay := a.renderCost()
	left := fmt.Sprintf(" %s | %s | Tab:模式 | /:命令", a.provider.Name(), a.model)
	right := costDisplay

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	padding := a.width - leftW - rightW
	if padding < 1 {
		padding = 1
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

func (a *App) runAgent(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := a.agent.Run(ctx, input)
		if err != nil {
			return streamErrorMsg(err.Error())
		}
		chunks := splitIntoChunks(result, 50)
		cmds := make([]tea.Cmd, len(chunks))
		for i, chunk := range chunks {
			c := chunk
			cmds[i] = tea.Tick(time.Duration(i*20)*time.Millisecond, func(t time.Time) tea.Msg {
				return streamChunkMsg(c)
			})
		}
		return tea.Sequence(append(cmds, func() tea.Msg { return streamDoneMsg{} })...)
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

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// 消息类型
type streamChunkMsg string
type streamDoneMsg struct{}
type streamErrorMsg string

// ============================================================
// 环境变量管理
// ============================================================

func loadEnvVars() map[string]string {
	vars := make(map[string]string)
	for _, key := range []string{"DEEPSEEK_API_KEY", "MIMO_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if val := os.Getenv(key); val != "" {
			vars[key] = maskValue(val)
		}
	}
	return vars
}

func maskValue(val string) string {
	if len(val) <= 8 {
		return strings.Repeat("*", len(val))
	}
	return val[:4] + strings.Repeat("*", len(val)-8) + val[len(val)-4:]
}

func (a *App) handleEnvCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		return a.showEnvVars()
	}
	switch parts[1] {
	case "set":
		if len(parts) < 4 {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "用法: /env set <KEY> <VALUE>"})
			return a, nil
		}
		key := parts[2]
		val := strings.Join(parts[3:], " ")
		os.Setenv(key, val)
		a.envVars[key] = maskValue(val)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("已设置 %s = %s", key, maskValue(val))})
		return a, nil
	case "unset":
		if len(parts) < 3 {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "用法: /env unset <KEY>"})
			return a, nil
		}
		key := parts[2]
		delete(a.envVars, key)
		os.Unsetenv(key)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("已移除 %s", key)})
		return a, nil
	case "reload":
		a.envVars = loadEnvVars()
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "环境变量已重新加载"})
		return a, nil
	default:
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "用法: /env [set|unset|reload]"})
		return a, nil
	}
}

func (a *App) showEnvVars() (tea.Model, tea.Cmd) {
	if len(a.envVars) == 0 {
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "没有配置的环境变量。\n\n使用 /env set <KEY> <VALUE> 添加。"})
		return a, nil
	}
	var sb strings.Builder
	sb.WriteString("环境变量:\n\n")
	for key, val := range a.envVars {
		sb.WriteString(fmt.Sprintf("  %s = %s\n", key, val))
	}
	a.messages = append(a.messages, chatMessage{Role: "system", Content: sb.String()})
	return a, nil
}
