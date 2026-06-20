package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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

	// 输入（Bubbles textarea）
	textArea textarea.Model

	// 命令联想
	showSuggestions bool
	suggestions     []string
	suggestionIdx   int

	// 聊天历史
	messages []chatMessage

	// 可滚动 viewport
	viewport viewport.Model

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

	// 会话保存状态
	savedMsgCount int // 已保存的消息数量

	// 上下文使用
	tokensUsed    int
	tokensWindow  int

	// Agent 活动状态
	lastStep    int
	lastTool    string

	// 流式输出缓冲
	streamMu  sync.Mutex
	streamBuf string

	// BubbleTea program reference
	program *tea.Program

	// 请求取消
	cancelFunc context.CancelFunc

	// Markdown renderer
	glamourRenderer *glamour.TermRenderer

	// 渲染缓存（消息内容 → 渲染结果）
	renderCache map[string]string
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
	"/goal", "/clear", "/cost", "/env", "/model", "/skills", "/sessions", "/compact", "/quit",
}

// 样式
var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	systemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	inputStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	suggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	suggestionSel   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4"))
	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	loadingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Padding(0, 1)
	statusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("7")).Padding(0, 1)
	costGreenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	costYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	costRedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	activityStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Italic(true)
	contextWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)

// NewApp 创建 TUI 应用
func NewApp(p provider.Provider, tools *tool.Registry) *App {
	ag := agent.NewMultiAgent(p, tools)

	// 加载 skills
	skillsMgr := skills.NewManager()
	skillsMgr.Load()
	ag.SetSkillsManager(skillsMgr)

	// 创建 textarea
	ta := textarea.New()
	ta.Placeholder = "输入任务... (Shift+Enter 换行, Enter 发送)"
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()

	// 创建 glamour renderer
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	app := &App{
		agent:          ag,
		provider:       p,
		tools:          tools,
		mode:           agent.ModeBuild,
		envVars:        loadEnvVars(),
		skillsMgr:      skillsMgr,
		textArea:       ta,
		glamourRenderer: renderer,
		renderCache:    make(map[string]string),
		messages: []chatMessage{
			{Role: "system", Content: "Helix CLI — 双螺旋 · 多模型 · 可扩展", Timestamp: time.Now()},
			{Role: "system", Content: "输入任务开始 | 输入 / 查看命令 | Tab 切换模式 | Ctrl+C 退出", Timestamp: time.Now()},
		},
	}

	ag.SetCostCallback(func(cost float64) {
		app.costLast = cost
		app.costSession += cost
		app.costTotal += cost
	})

	return app
}

func (a *App) SetSessionManager(mgr *session.Manager) { a.sessionMgr = mgr }
func (a *App) SetModel(m string)                       { a.model = m; a.agent.SetModel(m) }
func (a *App) SetProgram(p *tea.Program)               { a.program = p }

// saveSession 将新消息保存到活动会话
func (a *App) saveSession() {
	if a.sessionMgr == nil || a.activeSess == nil {
		return
	}
	// 只保存新消息（上次保存之后的）
	for i := a.savedMsgCount; i < len(a.messages); i++ {
		msg := a.messages[i]
		a.sessionMgr.AddMessage(session.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	a.savedMsgCount = len(a.messages)
	a.sessionMgr.Save(a.activeSess.ID)
}

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

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tea.EnterAltScreen,
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		// 更新 textarea 宽度
		a.textArea.SetWidth(msg.Width - 4)

		// 更新 viewport
		if a.viewport.Width == 0 {
			a.viewport = viewport.New(msg.Width, a.height-8)
			a.viewport.SetContent(a.renderMessages(a.height - 8))
		} else {
			a.viewport.Width = msg.Width
			a.viewport.Height = a.height - 8
			a.viewport.SetContent(a.renderMessages(a.height - 8))
			a.viewport.GotoBottom()
		}

		// 更新 glamour renderer
		a.glamourRenderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(msg.Width-4),
		)

	case tea.KeyMsg:
		return a.handleKey(msg)

	case streamChunkMsg:
		a.streamMu.Lock()
		a.streamBuf += string(msg)
		a.streamMu.Unlock()
		a.viewport.SetContent(a.renderMessages(a.height - 8))
		a.viewport.GotoBottom()
		return a, nil

	case streamDoneMsg:
		a.streamMu.Lock()
		content := a.streamBuf
		a.streamBuf = ""
		a.streamMu.Unlock()
		a.loading = false
		a.cancelFunc = nil
		a.messages = append(a.messages, chatMessage{Role: "assistant", Content: content, Timestamp: time.Now()})
		a.saveSession()
		a.viewport.SetContent(a.renderMessages(a.height - 8))
		a.viewport.GotoBottom()
		return a, nil

	case streamErrorMsg:
		a.loading = false
		a.cancelFunc = nil
		errStr := friendlyError(string(msg))
		a.messages = append(a.messages, chatMessage{Role: "error", Content: errStr, Timestamp: time.Now()})
		a.saveSession()
		a.viewport.SetContent(a.renderMessages(a.height - 8))
		a.viewport.GotoBottom()
		return a, nil
	}

	// 更新 textarea
	var taCmd tea.Cmd
	a.textArea, taCmd = a.textArea.Update(msg)
	cmds = append(cmds, taCmd)

	// 更新 viewport
	var vpCmd tea.Cmd
	a.viewport, vpCmd = a.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return a, tea.Batch(cmds...)
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
				a.viewport.SetContent(a.renderMessages(a.height - 8))
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
				a.textArea.SetValue(a.suggestions[a.suggestionIdx] + " ")
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
		a.saveSession()
		a.quitting = true
		return a, tea.Quit

	case "esc":
		if a.loading && a.cancelFunc != nil {
			a.cancelFunc()
			a.cancelFunc = nil
			a.loading = false
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "请求已取消"})
			a.viewport.SetContent(a.renderMessages(a.height - 8))
			return a, nil
		}
		a.textArea.Reset()
		a.showSuggestions = false
		return a, nil

	case "enter":
		return a.handleEnter()

	case "tab":
		input := a.textArea.Value()
		if strings.HasPrefix(input, "/") {
			a.showSuggestions = true
			a.updateSuggestions()
		} else {
			a.cycleMode()
		}
		return a, nil

	case "pgup":
		a.viewport.HalfViewUp()
		return a, nil

	case "pgdown":
		a.viewport.HalfViewDown()
		return a, nil
	}

	// 转发其他按键到 textarea（字母、退格、方向键等）
	var cmd tea.Cmd
	a.textArea, cmd = a.textArea.Update(msg)
	if strings.HasPrefix(a.textArea.Value(), "/") {
		a.updateSuggestions()
	} else {
		a.showSuggestions = false
	}
	return a, cmd
}

func (a *App) updateSuggestions() {
	input := a.textArea.Value()
	if !strings.HasPrefix(input, "/") {
		a.showSuggestions = false
		a.suggestions = nil
		return
	}

	prefix := strings.ToLower(input)
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

	input := strings.TrimSpace(a.textArea.Value())
	if input == "" {
		return a, nil
	}
	a.textArea.Reset()

	if strings.HasPrefix(input, "/") {
		return a.handleCommand(input)
	}

	a.messages = append(a.messages, chatMessage{Role: "user", Content: input, Timestamp: time.Now()})
	a.loading = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	a.viewport.SetContent(a.renderMessages(a.height - 8))
	a.viewport.GotoBottom()
	return a, a.runAgent(ctx, input)
}

func (a *App) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "/quit", "/exit":
		a.quitting = true
		return a, tea.Quit

	case "/help":
		help := `命令列表:
  /help      显示帮助
  /mode      显示当前模式
  /build     切换到 build 模式
  /plan      切换到 plan 模式(只读)
  /compose   切换到 compose 模式
  /max       切换到 max 模式(实验)
  /goal      设置停止条件
  /model     显示/切换模型
  /skills    显示可用工具列表
  /clear     清空聊天
  /cost      显示成本
  /compact   压缩上下文历史
  /env       环境变量管理
  /sessions  会话列表
  /quit      退出

提示:
  Tab 切换模式 | 输入 / 后 Tab 联想命令
  直接输入任务开始对话
  Shift+Enter 换行 | 粘贴多行代码

Goal/Stop Condition:
  /goal "实现用户认证模块"  设置停止条件
  /goal                      显示当前停止条件
  /goal clear                清除停止条件`
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

	case "/goal":
		return a.handleGoalCmd(parts)

	case "/clear":
		a.messages = a.messages[:0]
		a.viewport.SetContent("")
		return a, nil

	case "/cost":
		msg := fmt.Sprintf("会话: $%.4f | 上次: $%.4f | 累计: $%.4f", a.costSession, a.costLast, a.costTotal)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: msg})
		return a, nil

	case "/sessions":
		return a.handleSessionsCmd(parts)

	case "/compact":
		return a.handleCompactCmd()

	case "/env":
		return a.handleEnvCommand(parts)

	default:
		a.messages = append(a.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("未知命令: %s。输入 /help 查看可用命令。", cmd),
		})
		return a, nil
	}
}

func (a *App) handleGoalCmd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		goal := a.agent.GetGoal()
		if goal == "" {
			a.messages = append(a.messages, chatMessage{
				Role:    "system",
				Content: "当前未设置停止条件。\n\n使用 /goal \"<条件>\" 设置停止条件。",
			})
		} else {
			a.messages = append(a.messages, chatMessage{
				Role:    "system",
				Content: fmt.Sprintf("当前停止条件:\n%s\n\n使用 /goal clear 清除", goal),
			})
		}
		return a, nil
	}

	if parts[1] == "clear" {
		a.agent.ClearGoal()
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: "已清除停止条件",
		})
		return a, nil
	}

	goal := strings.Join(parts[1:], " ")
	goal = strings.Trim(goal, "\"'")
	if goal == "" {
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: "请提供停止条件，例如: /goal \"实现用户认证模块\"",
		})
		return a, nil
	}

	a.agent.SetGoal(goal)
	a.messages = append(a.messages, chatMessage{
		Role:    "system",
		Content: fmt.Sprintf("已设置停止条件:\n%s\n\nAgent 将在达成目标后自动停止。", goal),
	})
	return a, nil
}

func (a *App) handleModelCmd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		models := a.provider.Models()
		if len(models) == 0 {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "当前 Provider 没有注册模型"})
			return a, nil
		}

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
	tools := a.tools.List()
	skillList := a.skillsMgr.List()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("内置工具 (%d):\n\n", len(tools)))
	for _, t := range tools {
		icon := "✏️"
		if t.IsReadOnly() {
			icon = "📖"
		}
		sb.WriteString(fmt.Sprintf("  %s %s - %s\n", icon, t.Name(), t.Description()))
	}

	if len(skillList) > 0 {
		sb.WriteString(fmt.Sprintf("\n外部 Skills (%d):\n\n", len(skillList)))
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

func (a *App) handleSessionsCmd(parts []string) (tea.Model, tea.Cmd) {
	if a.sessionMgr == nil {
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "会话管理器未初始化"})
		return a, nil
	}

	if len(parts) >= 2 {
		switch parts[1] {
		case "new":
			name := fmt.Sprintf("session_%d", time.Now().UnixMilli())
			if len(parts) >= 3 {
				name = strings.Join(parts[2:], " ")
			}
			sess := a.sessionMgr.Create(name, a.model, a.provider.Name())
			a.activeSess = sess
			a.messages = append(a.messages, chatMessage{
				Role: "system", Content: fmt.Sprintf("已创建新会话: %s (ID: %s)", name, sess.ID),
			})
			return a, nil
		case "switch":
			if len(parts) < 3 {
				a.messages = append(a.messages, chatMessage{
					Role: "system", Content: "用法: /sessions switch <ID>",
				})
				return a, nil
			}
			sess, ok := a.sessionMgr.Get(parts[2])
			if !ok {
				a.messages = append(a.messages, chatMessage{
					Role: "system", Content: fmt.Sprintf("会话 %q 不存在", parts[2]),
				})
				return a, nil
			}
			a.sessionMgr.SetActive(parts[2])
			a.RestoreSession(sess)
			return a, nil
		}
	}

	sessions := a.sessionMgr.List()
	if len(sessions) == 0 {
		a.messages = append(a.messages, chatMessage{
			Role: "system", Content: "暂无会话。使用 /sessions new <名称> 创建新会话。",
		})
		return a, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("会话列表 (%d):\n\n", len(sessions)))
	for _, s := range sessions {
		marker := "  "
		if a.activeSess != nil && a.activeSess.ID == s.ID {
			marker = "▶ "
		}
		sb.WriteString(fmt.Sprintf("%s%s — %s (%d 条消息, %s)\n",
			marker, s.ID, s.Name, len(s.Messages), s.UpdatedAt.Format("01-02 15:04")))
	}
	sb.WriteString("\n使用 /sessions switch <ID> 切换会话")
	a.messages = append(a.messages, chatMessage{Role: "system", Content: sb.String()})
	return a, nil
}

func (a *App) handleCompactCmd() (tea.Model, tea.Cmd) {
	if len(a.messages) <= 4 {
		a.messages = append(a.messages, chatMessage{
			Role: "system", Content: "上下文已经很简洁，无需压缩。",
		})
		return a, nil
	}

	// 保留 system 消息和最近 6 条，压缩中间的历史
	keepRecent := 6
	if len(a.messages) <= keepRecent+2 {
		return a, nil
	}

	// 提取中间消息的摘要
	middle := a.messages[1 : len(a.messages)-keepRecent]
	var summary strings.Builder
	summary.WriteString("[上下文已压缩] 历史对话摘要:\n")
	for _, msg := range middle {
		switch msg.Role {
		case "user":
			content := msg.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			summary.WriteString(fmt.Sprintf("- 用户: %s\n", content))
		case "assistant":
			content := msg.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			summary.WriteString(fmt.Sprintf("- 助手: %s\n", content))
		}
	}

	// 重建消息列表
	newMessages := []chatMessage{
		a.messages[0], // system
		{Role: "system", Content: summary.String(), Timestamp: time.Now()},
	}
	newMessages = append(newMessages, a.messages[len(a.messages)-keepRecent:]...)
	a.messages = newMessages

	a.viewport.SetContent(a.renderMessages(a.height - 8))
	a.messages = append(a.messages, chatMessage{
		Role: "system", Content: fmt.Sprintf("上下文已压缩：保留最近 %d 条消息，中间 %d 条已摘要。", keepRecent, len(middle)),
	})
	return a, nil
}

func (a *App) cycleMode() {
	modes := []agent.Mode{agent.ModeBuild, agent.ModePlan, agent.ModeCompose, agent.ModeMax}
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

	// 消息区域（viewport）
	sb.WriteString(a.viewport.View())
	sb.WriteString("\n")

	// Agent 活动状态
	if a.loading {
		activity := "思考中..."
		if a.lastTool != "" {
			activity = fmt.Sprintf("第 %d 步 · 🔧 %s", a.lastStep, a.lastTool)
		}
		sb.WriteString(activityStyle.Render(fmt.Sprintf(" %s", activity)))
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

	// 输入区域（textarea）
	sb.WriteString(a.textArea.View())
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

	for _, msg := range a.messages {
		switch msg.Role {
		case "user":
			sb.WriteString(userStyle.Render("▸ " + msg.Content))
			sb.WriteString("\n")
		case "assistant":
			// 尝试用 glamour 渲染 markdown
			rendered := a.renderMarkdown(msg.Content)
			for _, line := range strings.Split(rendered, "\n") {
				sb.WriteString(assistantStyle.Render("  " + line))
				sb.WriteString("\n")
			}
		case "system":
			sb.WriteString(systemStyle.Render("  " + msg.Content))
			sb.WriteString("\n")
		case "tool":
			sb.WriteString(toolStyle.Render("  🔧 " + msg.Content))
			sb.WriteString("\n")
		case "error":
			for _, line := range strings.Split(msg.Content, "\n") {
				sb.WriteString(errorStyle.Render("  ✖ " + line))
				sb.WriteString("\n")
			}
		}
	}

	// 流式输出缓冲
	if a.loading && a.streamBuf != "" {
		rendered := a.renderMarkdown(a.streamBuf)
		for _, line := range strings.Split(rendered, "\n") {
			sb.WriteString(assistantStyle.Render("  " + line))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderMarkdown 用 glamour 渲染 markdown，带缓存，失败时回退到纯文本
func (a *App) renderMarkdown(content string) string {
	if a.glamourRenderer == nil {
		return content
	}

	// 检查缓存
	if cached, ok := a.renderCache[content]; ok {
		return cached
	}

	rendered, err := a.glamourRenderer.Render(content)
	if err != nil {
		return content
	}
	result := strings.TrimRight(rendered, "\n")

	// 缓存结果（限制缓存大小，避免内存无限增长）
	if len(a.renderCache) < 500 {
		a.renderCache[content] = result
	}

	return result
}

func (a *App) renderStatusBar() string {
	costDisplay := a.renderCost()
	contextDisplay := a.renderContextUsage()
	left := fmt.Sprintf(" %s | %s | Tab:模式 | /:命令", a.provider.Name(), a.model)
	right := contextDisplay + " | " + costDisplay

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

func (a *App) renderContextUsage() string {
	if a.tokensWindow <= 0 {
		return fmt.Sprintf("%dk/?", a.tokensUsed/1000)
	}
	ratio := float64(a.tokensUsed) / float64(a.tokensWindow)
	if ratio > 0.8 {
		return contextWarnStyle.Render(fmt.Sprintf("%dk/%dk ⚠️", a.tokensUsed/1000, a.tokensWindow/1000))
	}
	return fmt.Sprintf("%dk/%dk", a.tokensUsed/1000, a.tokensWindow/1000)
}

func (a *App) runAgent(ctx context.Context, input string) tea.Cmd {
	return func() tea.Msg {
		textCh, errCh := a.agent.RunStream(ctx, input)

		for {
			select {
			case text, ok := <-textCh:
				if !ok {
					if a.program != nil {
						a.program.Send(streamDoneMsg{})
					}
					return nil
				}
				if a.program != nil {
					a.program.Send(streamChunkMsg(text))
				}
			case err, ok := <-errCh:
				if ok && err != nil {
					if a.program != nil {
						a.program.Send(streamErrorMsg(err.Error()))
					}
					return nil
				}
				if a.program != nil {
					a.program.Send(streamDoneMsg{})
				}
				return nil
			}
		}
	}
}

// 消息类型
type streamChunkMsg string
type streamDoneMsg struct{ cost float64 }
type streamErrorMsg string

// friendlyError 将常见错误映射为用户友好提示
func friendlyError(err string) string {
	lower := strings.ToLower(err)
	switch {
	case strings.Contains(lower, "api key") || strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		return "API Key 无效或未设置。运行 helix setup 或检查 .env 文件中的 API_KEY 配置。"
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests"):
		return "请求过于频繁，请稍后重试。"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return "请求超时，请检查网络连接或稍后重试。"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "dial tcp"):
		return "无法连接到服务器，请检查网络。"
	case strings.Contains(lower, "model") && (strings.Contains(lower, "not found") || strings.Contains(lower, "does not exist")):
		return "模型不存在，请检查模型名称或使用 /model 切换模型。"
	case strings.Contains(lower, "max steps"):
		return "达到最大推理步数限制。使用 /goal 设置停止条件，或增大步数限制。"
	case strings.Contains(lower, "context canceled") || strings.Contains(lower, "context deadline"):
		return "请求已取消。"
	case strings.Contains(lower, "no task provided"):
		return "请提供任务描述。"
	default:
		return err
	}
}

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
