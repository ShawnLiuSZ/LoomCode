package ui

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glamourstyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/ShawnLiuSZ/loomcode/internal/agent"
	"github.com/ShawnLiuSZ/loomcode/internal/consts"
	"github.com/ShawnLiuSZ/loomcode/internal/control"
	"github.com/ShawnLiuSZ/loomcode/internal/memory"
	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/session"
	"github.com/ShawnLiuSZ/loomcode/internal/skills"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// Version 版本号（由 main 包注入）
var Version = "dev"

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
	mode agent.Mode

	// 环境变量
	envVars map[string]string

	// Skills
	skillsMgr *skills.Manager

	// 长期记忆（/remember 写入，启动时自动注入）
	memMgr *memory.Manager

	// 模型选择状态
	showModelPicker   bool
	modelList         []modelPickerEntry
	modelIdx          int
	allProviders      []provider.Provider
	allProviderModels map[string][]provider.ModelInfo

	// 成本
	costTotal   atomic.Uint64
	costSession atomic.Uint64
	costLast    atomic.Uint64
	costBudget  float64

	// 会话保存状态
	savedMsgCount int // 已保存的消息数量

	// 上下文使用
	tokensUsed   int
	tokensWindow int

	// Cache 命中率（来自 agent EventLog，nil 时不显示）
	eventLog *agent.EventLog

	// Agent 活动状态
	lastStep int
	lastTool string

	// 流式输出缓冲
	streamMu    sync.Mutex
	streamBuf   string
	streamChars int // 当前步流式已接收字符数，用于实时估算生成 token

	// BubbleTea program reference
	program *tea.Program

	// Approval
	pendingApproval *pendingWrite

	// 请求取消
	cancelFunc context.CancelFunc

	// 任务队列
	taskQueue []string

	// Markdown renderer
	glamourRenderer *glamour.TermRenderer

	// 终端背景色（true=深色，false=亮色），影响 glamour style 选择
	hasDarkBackground bool

	// Ctrl+C 二次确认退出（1.12）
	confirmQuit     bool
	confirmQuitTime time.Time

	// 渲染缓存（消息内容 → 渲染结果）
	renderCache map[string]string

	// 编辑快照管理器（/rewind 回退安全网）
	checkpointMgr *tool.CheckpointManager
}

type chatMessage struct {
	Role      string
	Content   string
	ToolName  string
	Timestamp time.Time
}

// modelPickerEntry 模型选择器条目（包含 provider 信息）
type modelPickerEntry struct {
	ProviderName string
	ModelID      string
}

type pendingWrite struct {
	toolName   string
	path       string
	oldContent string
	newContent string
	decision   chan bool
}

type approvalMsg struct{}

// confirmQuitResetMsg 3 秒后重置 Ctrl+C 二次确认状态
type confirmQuitResetMsg struct{}

// 所有可用命令
var allCommands = []string{
	"/help", "/mode", "/build", "/plan", "/compose", "/max",
	"/goal", "/clear", "/cost", "/budget", "/env", "/model", "/skills", "/sessions", "/compact", "/queue", "/steps", "/remember", "/quit",
}

// 样式
var (
	userStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	assistantStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	systemStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	toolStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	suggestionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	suggestionSel    = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4"))
	headerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Padding(0, 1)
	statusBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("7")).Padding(0, 1)
	costGreenStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	costYellowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	costRedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	activityStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Italic(true)
	contextWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)

	approvalTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	diffAddStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	diffDelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	diffHeaderStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	approvalHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true)
)

// NewApp 创建 TUI 应用
func NewApp(p provider.Provider, tools *tool.Registry) *App {
	ag := agent.NewMultiAgent(p, tools)

	// 加载 skills
	skillsMgr := skills.NewManager()
	if err := skillsMgr.Load(); err != nil {
		log.Printf("load skills: %v", err)
	}
	ag.SetSkillsManager(skillsMgr)

	// 接入长期记忆（项目知识/用户偏好注入系统提示）。best-effort：打不开则跳过。
	var memMgr *memory.Manager
	if home, err := os.UserHomeDir(); err == nil {
		if store, err := memory.NewStore(filepath.Join(home, ".loomcode", "memory.db")); err == nil {
			memMgr = memory.NewManager(store)
			ag.SetMemory(memMgr)
		}
	}

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
	// 修复光标渲染乱码：用 Background 替代 Reverse(true)
	ta.Cursor.Style = lipgloss.NewStyle().Background(lipgloss.Color("7"))
	ta.Cursor.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))

	// 创建 glamour renderer（1.3：背景色在 Init 中检测，此处先默认深色，WindowSizeMsg 时按实际更新）
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourstyles.DarkStyle),
		glamour.WithWordWrap(80),
	)

	app := &App{
		agent:           ag,
		provider:        p,
		tools:           tools,
		mode:            agent.ModeBuild,
		envVars:         loadEnvVars(),
		skillsMgr:       skillsMgr,
		memMgr:          memMgr,
		textArea:        ta,
		glamourRenderer: renderer,
		renderCache:     make(map[string]string),
		messages: []chatMessage{
			{Role: "welcome", Content: "", Timestamp: time.Now()},
		},
	}

	// 接通 EventLog，用于状态栏显示 prefix cache 命中率
	app.eventLog = ag.EventLog()

	ag.SetCostCallback(func(cost float64) {
		app.costLast.Store(costUint64FromFloat(cost))
		for {
			old := app.costSession.Load()
			newVal := costUint64FromFloat(costFloatFromUint64(old) + cost)
			if app.costSession.CompareAndSwap(old, newVal) {
				break
			}
		}
		for {
			old := app.costTotal.Load()
			newVal := costUint64FromFloat(costFloatFromUint64(old) + cost)
			if app.costTotal.CompareAndSwap(old, newVal) {
				break
			}
		}
		sessionCost := costFloatFromUint64(app.costSession.Load())
		if app.costBudget > 0 && sessionCost >= app.costBudget && app.cancelFunc != nil {
			app.cancelFunc()
			app.messages = append(app.messages, chatMessage{
				Role:    "system",
				Content: fmt.Sprintf("预算已用尽 ($%.4f/$%.4f)。使用 /budget <amount> 调整预算。", sessionCost, app.costBudget),
			})
		}
	})

	return app
}

func (a *App) SetSessionManager(mgr *session.Manager) { a.sessionMgr = mgr }
func (a *App) SetModel(m string)                      { a.model = m; a.agent.SetModel(m) }
func (a *App) SetWorkDir(d string)                    { a.agent.SetWorkDir(d) }
func (a *App) SetProgram(p *tea.Program)              { a.program = p }
func (a *App) SetHooks(hm *tool.HookManager)          { a.agent.SetHooks(hm) }

// SetCheckpointManager 注入编辑快照管理器，启用 /rewind 回退安全网。
func (a *App) SetCheckpointManager(mgr *tool.CheckpointManager) { a.checkpointMgr = mgr }

// SetEventLog 注入 agent EventLog,用于在状态栏显示 prefix cache 命中率。
// 传入 nil 则不显示 cache 字段。
func (a *App) SetEventLog(l *agent.EventLog) { a.eventLog = l }

func (a *App) AddApprovalGuard(perm *control.Permission) {
	a.agent.AddGuard(func(tc tool.Call) error {
		if tc.Name != "write_file" && tc.Name != "edit_file" {
			return nil
		}
		if perm != nil {
			if allowed, _ := perm.Check(tc.Name, tc.Args); allowed {
				return nil
			}
		}
		path, _ := tc.Args["path"].(string)
		pw := &pendingWrite{
			toolName: tc.Name,
			path:     path,
			decision: make(chan bool, 1),
		}
		if tc.Name == "write_file" {
			if data, err := os.ReadFile(path); err == nil {
				pw.oldContent = string(data)
			}
			pw.newContent, _ = tc.Args["content"].(string)
		} else {
			pw.oldContent, _ = tc.Args["old_text"].(string)
			pw.newContent, _ = tc.Args["new_text"].(string)
		}
		if a.program != nil {
			// 必须先把 pw 挂到 a.pendingApproval，handleKey 才会往 pw.decision 喂值；
			// 否则下方 <-pw.decision 会永久阻塞，导致 TUI 卡死。
			a.pendingApproval = pw
			a.program.Send(approvalMsg{})
		}
		if <-pw.decision {
			return nil
		}
		return fmt.Errorf("rejected by user")
	})
}

// SetProviders 设置所有 provider 列表（用于 /model 跨 provider 切换）
func (a *App) SetProviders(providers []provider.Provider) {
	a.allProviders = providers
	a.allProviderModels = make(map[string][]provider.ModelInfo, len(providers))
	for _, p := range providers {
		a.allProviderModels[p.Name()] = p.Models()
	}
}

// persistModelChange 持久化模型/provider 切换到活动会话，让下次启动能恢复。
// activeSess 为 nil（用户还没输入过消息）时跳过，下次启动走默认模型。
func (a *App) persistModelChange() {
	if a.sessionMgr == nil || a.activeSess == nil {
		return
	}
	_ = a.sessionMgr.UpdateModelProvider(a.model, a.provider.Name())
}

// RestoreModelFromSession 从会话恢复模型/provider 选择（不恢复消息历史）。
// 用于默认启动时记住上次选择的模型。仅当会话的 provider 仍已注册时生效。
func (a *App) RestoreModelFromSession(sess *session.Session) {
	if sess == nil || sess.Provider == "" || sess.Model == "" {
		return
	}
	for _, p := range a.allProviders {
		if p.Name() == sess.Provider {
			a.provider = p
			a.agent.SetProvider(p)
			a.model = sess.Model
			a.agent.SetModel(sess.Model)
			return
		}
	}
}

// saveSession 将新消息保存到活动会话
func (a *App) saveSession() {
	if a.sessionMgr == nil {
		return
	}
	// 懒创建默认会话：仅当用户实际输入过消息时才创建，避免仅含 welcome/system 消息的空会话文件（L9）。
	if a.activeSess == nil {
		hasUserMsg := false
		for _, m := range a.messages {
			if m.Role == "user" {
				hasUserMsg = true
				break
			}
		}
		if !hasUserMsg {
			return
		}
		a.activeSess = a.sessionMgr.Create("default", a.model, a.provider.Name())
		if a.activeSess == nil {
			// 会话创建失败（元信息持久化失败），中止本次保存以免后续追加到无效文件。
			return
		}
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
	if err := a.sessionMgr.Save(a.activeSess.ID); err != nil {
		log.Printf("save session: %v", err)
	}
}

// RestoreSession 恢复历史会话
func (a *App) RestoreSession(sess *session.Session) {
	a.activeSess = sess
	a.messages = []chatMessage{
		{Role: "system", Content: fmt.Sprintf("已恢复会话: %s (%d 条消息)", sess.Name, len(sess.Messages)), Timestamp: time.Now()},
	}

	// 恢复会话保存的 model/provider（若仍存在于已注册 provider 列表中）
	if sess.Provider != "" && sess.Model != "" {
		for _, p := range a.allProviders {
			if p.Name() == sess.Provider {
				a.provider = p
				a.agent.SetProvider(p)
				a.model = sess.Model
				a.agent.SetModel(sess.Model)
				break
			}
		}
	}

	var history []provider.Message
	for _, msg := range sess.Messages {
		a.messages = append(a.messages, chatMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolName:  msg.ToolName,
			Timestamp: msg.Timestamp,
		})
		// 仅以 user/assistant 文本重建 LLM 对话历史（session 不存 tool_calls 结构，
		// 跳过 tool/system/空内容以免产生畸形消息序列）。
		if (msg.Role == "user" || msg.Role == "assistant") && msg.Content != "" {
			history = append(history, provider.Message{Role: msg.Role, Content: msg.Content})
		}
	}
	a.agent.SetHistory(history) // 让模型在恢复会话后仍记得之前的对话
	a.messages = append(a.messages, chatMessage{
		Role: "system", Content: "会话已恢复，继续对话或输入 /help 查看命令", Timestamp: time.Now(),
	})
}

func (a *App) Init() tea.Cmd {
	// 1.2 修复：检测终端背景色，检测失败时回退到深色（多数终端默认深色）。
	// 不再强制 SetHasDarkBackground(true)，否则亮色终端上浅色文字不可见。
	a.hasDarkBackground = true // 回退默认值
	if bg := lipgloss.HasDarkBackground(); bg {
		a.hasDarkBackground = true
	} else {
		// HasDarkBackground 返回 false 可能是"确认亮色"也可能是"检测失败"，
		// 这里保守地采用 true 作为回退（与原行为一致），但允许 WindowSizeMsg 再次更新。
		// 若终端明确报告亮色（OSC 11），在此处已正确设置。
		a.hasDarkBackground = false
	}
	lipgloss.SetHasDarkBackground(a.hasDarkBackground)
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

		// 1.4 修复：最小终端尺寸检查，防止 height-8 为负数导致 panic / 渲染异常。
		if a.height < 10 || a.width < 40 {
			// 尺寸过小时只更新尺寸，不重建 viewport/renderer（渲染会出错）
			return a, nil
		}

		// 更新 textarea 宽度
		a.textArea.SetWidth(msg.Width - 4)

		// 更新 viewport
		if a.viewport.Width == 0 {
			a.viewport = viewport.New(msg.Width, a.height-8)
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
		} else {
			a.viewport.Width = msg.Width
			a.viewport.Height = a.height - 8
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
			a.viewport.GotoBottom()
		}

		// 更新 glamour renderer（1.3：按实际背景色选择 style）
		a.glamourRenderer = a.newGlamourRenderer(msg.Width - 4)

		// 1.9 修复：窗口缩放后渲染缓存按旧宽度渲染，必须清空。
		a.renderCache = make(map[string]string)

	case tea.KeyMsg:
		return a.handleKey(msg)

	case streamChunkMsg:
		a.streamMu.Lock()
		a.streamBuf += string(msg)
		a.streamChars += len(msg)
		buf := a.streamBuf
		a.streamMu.Unlock()
		a.viewport.SetContent(a.renderMessages(a.height-8, buf))
		a.viewport.GotoBottom()
		return a, nil

	case streamDoneMsg:
		a.streamMu.Lock()
		content := a.streamBuf
		a.streamBuf = ""
		a.streamChars = 0
		a.streamMu.Unlock()
		a.loading = false
		a.cancelFunc = nil
		a.messages = append(a.messages, chatMessage{Role: "assistant", Content: content, Timestamp: time.Now()})
		a.saveSession()
		a.viewport.SetContent(a.renderMessages(a.height-8, ""))
		a.viewport.GotoBottom()
		return a, a.processQueue()

	case streamErrorMsg:
		a.streamMu.Lock()
		a.streamBuf = ""
		a.streamMu.Unlock()
		a.loading = false
		a.cancelFunc = nil
		errStr := friendlyError(string(msg))
		a.messages = append(a.messages, chatMessage{Role: "error", Content: errStr, Timestamp: time.Now()})
		a.saveSession()
		a.viewport.SetContent(a.renderMessages(a.height-8, ""))
		a.viewport.GotoBottom()
		return a, a.processQueue()

	case approvalMsg:
		a.viewport.SetContent(a.renderMessages(a.height-8, ""))
		a.viewport.GotoBottom()
		return a, nil

	case confirmQuitResetMsg:
		// 1.12：3 秒超时后重置 Ctrl+C 二次确认
		a.confirmQuit = false
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

	if a.pendingApproval != nil {
		switch key {
		case "enter":
			pw := a.pendingApproval
			a.pendingApproval = nil
			pw.decision <- true
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
			a.viewport.GotoBottom()
			return a, nil
		case "esc":
			pw := a.pendingApproval
			a.pendingApproval = nil
			pw.decision <- false
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "操作已拒绝"})
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
			a.viewport.GotoBottom()
			return a, nil
		default:
			return a, nil
		}
	}

	// 模型选择器模式
	if a.showModelPicker {
		switch key {
		case "down":
			a.modelIdx = (a.modelIdx + 1) % len(a.modelList)
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
			a.viewport.GotoBottom()
			return a, nil
		case "up":
			a.modelIdx = (a.modelIdx - 1 + len(a.modelList)) % len(a.modelList)
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
			a.viewport.GotoBottom()
			return a, nil
		case "enter":
			if a.modelIdx >= 0 && a.modelIdx < len(a.modelList) {
				entry := a.modelList[a.modelIdx]
				// 如果切换到不同 provider，需要切换 provider
				if entry.ProviderName != a.provider.Name() {
					for _, p := range a.allProviders {
						if p.Name() == entry.ProviderName {
							a.provider = p
							a.agent.SetProvider(p) // 同步切换 agent 内部 provider
							break
						}
					}
				}
				a.model = entry.ModelID
				a.agent.SetModel(a.model)
				a.persistModelChange() // 持久化切换，下次启动恢复
				a.messages = append(a.messages, chatMessage{
					Role: "system", Content: fmt.Sprintf("模型切换为: %s/%s", entry.ProviderName, entry.ModelID),
				})
				a.viewport.SetContent(a.renderMessages(a.height-8, ""))
				a.viewport.GotoBottom()
			}
			a.showModelPicker = false
			return a, nil
		case "esc":
			a.showModelPicker = false
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
			a.viewport.GotoBottom()
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
				// 选中建议后直接执行（无需再按一次回车）
				// 先取值再清空，否则 nil slice 索引会 panic
				selected := a.suggestions[a.suggestionIdx]
				a.textArea.Reset()
				a.showSuggestions = false
				a.suggestions = nil
				return a.handleCommand(selected)
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
		// 1.12 修复：Ctrl+C 二次确认退出，防止误按丢失上下文。
		// 首次按 Ctrl+C 显示提示，3 秒内再按一次才真正退出。
		if a.confirmQuit {
			a.saveSession()
			a.quitting = true
			return a, tea.Quit
		}
		a.confirmQuit = true
		a.confirmQuitTime = time.Now()
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: "再按一次 Ctrl+C 确认退出（3 秒内有效）",
		})
		a.viewport.SetContent(a.renderMessages(a.height-8, ""))
		// 3 秒后自动重置 confirmQuit
		return a, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return confirmQuitResetMsg{}
		})

	case "esc":
		if a.loading && a.cancelFunc != nil {
			a.cancelFunc()
			a.cancelFunc = nil
			a.loading = false
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "请求已取消"})
			a.viewport.SetContent(a.renderMessages(a.height-8, ""))
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
		a.viewport.HalfPageUp()
		return a, nil

	case "pgdown":
		a.viewport.HalfPageDown()
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

	// 如果正在执行任务，加入队列
	if a.loading {
		a.taskQueue = append(a.taskQueue, input)
		queueLen := len(a.taskQueue)
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: fmt.Sprintf("已加入队列 (%d 待处理)", queueLen),
		})
		a.viewport.SetContent(a.renderMessages(a.height-8, ""))
		a.viewport.GotoBottom()
		return a, nil
	}

	a.messages = append(a.messages, chatMessage{Role: "user", Content: input, Timestamp: time.Now()})
	a.loading = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	a.viewport.SetContent(a.renderMessages(a.height-8, ""))
	a.viewport.GotoBottom()
	return a, a.runAgent(ctx, input)
}

func (a *App) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "/quit", "/exit":
		a.saveSession()
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
  /steps     查看/设置最大步数
  /model     显示/切换模型
  /skills    显示可用工具列表
  /clear     清空聊天
  /cost      显示成本
  /budget    设置/查看预算上限
  /compact   压缩上下文历史
  /remember  记住一条项目事实(下次会话自动注入)
  /env       环境变量管理
  /rewind    查看/恢复文件编辑快照
  /sessions  会话列表
  /queue     查看任务队列
  /quit      退出

提示:
  Tab 切换模式 | 输入 / 后 Tab 联想命令
  直接输入任务开始对话
  执行中发送新任务会自动排队
  Shift+Enter 换行 | 粘贴多行代码

Goal/Stop Condition:
  /goal "实现用户认证模块"  设置停止条件
  /goal                      显示当前停止条件
  /goal clear                清除停止条件

Budget:
  /budget 1.00      设置预算上限为 $1.00
  /budget            查看当前预算
  /budget clear      清除预算`
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

	case "/remember":
		return a.handleRememberCmd(strings.TrimSpace(strings.TrimPrefix(cmd, "/remember")))

	case "/clear":
		a.messages = a.messages[:0]
		a.agent.ResetConversation() // 同时清空模型侧对话历史
		a.viewport.SetContent("")
		return a, nil

	case "/cost":
		msg := fmt.Sprintf("会话: $%.4f | 上次: $%.4f | 累计: $%.4f", costFloatFromUint64(a.costSession.Load()), costFloatFromUint64(a.costLast.Load()), costFloatFromUint64(a.costTotal.Load()))
		a.messages = append(a.messages, chatMessage{Role: "system", Content: msg})
		return a, nil

	case "/budget":
		return a.handleBudgetCmd(parts)

	case "/queue":
		if len(a.taskQueue) == 0 {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "队列为空"})
		} else {
			var sb strings.Builder
			fmt.Fprintf(&sb, "队列中有 %d 个任务:\n\n", len(a.taskQueue))
			for i, task := range a.taskQueue {
				fmt.Fprintf(&sb, "  %d. %s\n", i+1, task)
			}
			a.messages = append(a.messages, chatMessage{Role: "system", Content: sb.String()})
		}
		return a, nil

	case "/steps":
		if len(parts) < 2 {
			msg := fmt.Sprintf("当前最大步数: %d\n\n使用 /steps <n> 设置新值", a.agent.GetMaxSteps())
			a.messages = append(a.messages, chatMessage{Role: "system", Content: msg})
			return a, nil
		}
		n := 0
		if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil || n < 1 {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: "无效数字，请输入正整数"})
			return a, nil
		}
		a.agent.SetMaxSteps(n)
		a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("最大步数已设置为 %d", n)})
		return a, nil

	case "/sessions":
		return a.handleSessionsCmd(parts)

	case "/compact":
		return a.handleCompactCmd()

	case "/env":
		return a.handleEnvCommand(parts)

	case "/rewind":
		return a.handleRewindCmd(parts)

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

func (a *App) handleBudgetCmd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		if a.costBudget <= 0 {
			a.messages = append(a.messages, chatMessage{
				Role:    "system",
				Content: "当前未设置预算。\n\n使用 /budget <amount> 设置预算，例如 /budget 1.00",
			})
		} else {
			a.messages = append(a.messages, chatMessage{
				Role:    "system",
				Content: fmt.Sprintf("预算: $%.2f | 已用: $%.4f | 剩余: $%.4f\n\n使用 /budget clear 清除预算", a.costBudget, costFloatFromUint64(a.costSession.Load()), a.costBudget-costFloatFromUint64(a.costSession.Load())),
			})
		}
		return a, nil
	}

	if parts[1] == "clear" {
		a.costBudget = 0
		a.agent.SetCostBudget(0)
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: "已清除预算",
		})
		return a, nil
	}

	var amount float64
	if _, err := fmt.Sscanf(parts[1], "%f", &amount); err != nil || amount <= 0 {
		a.messages = append(a.messages, chatMessage{
			Role:    "system",
			Content: "无效金额，请输入正数，例如 /budget 1.00",
		})
		return a, nil
	}

	a.costBudget = amount
	a.agent.SetCostBudget(amount)
	a.messages = append(a.messages, chatMessage{
		Role:    "system",
		Content: fmt.Sprintf("预算已设置为 $%.2f。当前已用 $%.4f", amount, costFloatFromUint64(a.costSession.Load())),
	})
	return a, nil
}

func (a *App) handleModelCmd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		// 收集所有 provider 的所有模型
		var entries []modelPickerEntry
		currentProviderName := a.provider.Name()

		// 先添加当前 provider 的模型
		if a.allProviderModels != nil {
			if models, ok := a.allProviderModels[currentProviderName]; ok {
				for _, m := range models {
					entries = append(entries, modelPickerEntry{
						ProviderName: currentProviderName,
						ModelID:      m.ID,
					})
				}
			}
			// 再添加其他 provider 的模型
			for _, p := range a.allProviders {
				if p.Name() == currentProviderName {
					continue
				}
				if models, ok := a.allProviderModels[p.Name()]; ok {
					for _, m := range models {
						entries = append(entries, modelPickerEntry{
							ProviderName: p.Name(),
							ModelID:      m.ID,
						})
					}
				}
			}
		}

		// 如果没有 allProviderModels，回退到只显示当前 provider 的模型
		if len(entries) == 0 {
			models := a.provider.Models()
			if len(models) == 0 {
				a.messages = append(a.messages, chatMessage{Role: "system", Content: "当前 Provider 没有注册模型"})
				a.viewport.SetContent(a.renderMessages(a.height-8, ""))
				a.viewport.GotoBottom()
				return a, nil
			}
			for _, m := range models {
				entries = append(entries, modelPickerEntry{
					ProviderName: currentProviderName,
					ModelID:      m.ID,
				})
			}
		}

		a.modelList = entries
		a.modelIdx = 0
		for i, e := range a.modelList {
			if e.ModelID == a.model && e.ProviderName == currentProviderName {
				a.modelIdx = i
				break
			}
		}
		a.showModelPicker = true
		a.viewport.SetContent(a.renderMessages(a.height-8, ""))
		a.viewport.GotoBottom()
		return a, nil
	}

	newModel := parts[1]

	// 查找模型所属的 provider 并切换
	if a.allProviderModels != nil {
		for _, p := range a.allProviders {
			if models, ok := a.allProviderModels[p.Name()]; ok {
				for _, m := range models {
					if m.ID == newModel {
						if a.provider.Name() != p.Name() {
							a.provider = p
							a.agent.SetProvider(p) // 同步切换 agent 内部 provider
						}
						break
					}
				}
			}
		}
	}

	a.model = newModel
	a.agent.SetModel(newModel)
	a.persistModelChange() // 持久化切换，下次启动恢复
	a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("模型切换为: %s/%s", a.provider.Name(), newModel)})
	a.viewport.SetContent(a.renderMessages(a.height-8, ""))
	a.viewport.GotoBottom()
	return a, nil
}

func (a *App) handleSkillsCmd() (tea.Model, tea.Cmd) {
	tools := a.tools.List()
	skillList := a.skillsMgr.List()

	var sb strings.Builder
	fmt.Fprintf(&sb, "内置工具 (%d):\n\n", len(tools))
	for _, t := range tools {
		icon := "✏️"
		if t.IsReadOnly() {
			icon = "📖"
		}
		fmt.Fprintf(&sb, "  %s %s - %s\n", icon, t.Name(), t.Description())
	}

	if len(skillList) > 0 {
		fmt.Fprintf(&sb, "\n外部 Skills (%d):\n\n", len(skillList))
		for _, s := range skillList {
			source := ""
			if s.Source == "loomcode" {
				source = " [loomcode]"
			}
			fmt.Fprintf(&sb, "  📄 %s%s - %s\n", s.Name, source, s.Description)
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
			if sess == nil {
				a.messages = append(a.messages, chatMessage{
					Role: "system", Content: "创建新会话失败：无法持久化会话元信息，请检查磁盘/目录权限。",
				})
				break
			}
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
			if err := a.sessionMgr.SetActive(parts[2]); err != nil {
				log.Printf("activate session: %v", err)
			}
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
	fmt.Fprintf(&sb, "会话列表 (%d):\n\n", len(sessions))
	for _, s := range sessions {
		marker := "  "
		if a.activeSess != nil && a.activeSess.ID == s.ID {
			marker = "▶ "
		}
		fmt.Fprintf(&sb, "%s%s — %s (%d 条消息, %s)\n",
			marker, s.ID, s.Name, len(s.Messages), s.UpdatedAt.Format("01-02 15:04"))
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
			fmt.Fprintf(&summary, "- 用户: %s\n", truncateRunes(msg.Content, 100))
		case "assistant":
			fmt.Fprintf(&summary, "- 助手: %s\n", truncateRunes(msg.Content, 100))
		}
	}

	// 重建消息列表
	newMessages := []chatMessage{
		a.messages[0], // system
		{Role: "system", Content: summary.String(), Timestamp: time.Now()},
	}
	newMessages = append(newMessages, a.messages[len(a.messages)-keepRecent:]...)
	a.messages = newMessages

	a.viewport.SetContent(a.renderMessages(a.height-8, ""))
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

	// Approval diff
	if a.pendingApproval != nil {
		sb.WriteString("\n")
		sb.WriteString(a.renderApproval())
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
	title := fmt.Sprintf("%s LoomCode CLI | %s | %s | %s",
		icon, a.mode.String(), a.provider.Name(), a.model)
	return headerStyle.Width(a.width).Render(title)
}

func (a *App) renderMessages(visibleLines int, streamBuf string) string {
	var sb strings.Builder

	for _, msg := range a.messages {
		switch msg.Role {
		case "welcome":
			sb.WriteString(a.renderWelcome())
			sb.WriteString("\n")
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

	// 模型选择器：实时渲染（不持久化到 messages）
	if a.showModelPicker {
		sb.WriteString("\n")
		sb.WriteString(systemStyle.Render("选择模型 (↑↓ 移动, Enter 确认, Esc 取消):"))
		sb.WriteString("\n\n")
		sb.WriteString(systemStyle.Render(fmt.Sprintf("当前: %s/%s", a.provider.Name(), a.model)))
		sb.WriteString("\n\n")
		// 按 provider 分组显示
		currentProviderName := a.provider.Name()
		// 先显示当前 provider
		sb.WriteString(systemStyle.Render(fmt.Sprintf("[%s]", currentProviderName)))
		sb.WriteString("\n")
		for i, e := range a.modelList {
			if e.ProviderName != currentProviderName {
				continue
			}
			marker := "  "
			if i == a.modelIdx {
				marker = "▶ "
			}
			sb.WriteString(systemStyle.Render(fmt.Sprintf("%s%s", marker, e.ModelID)))
			sb.WriteString("\n")
		}
		// 再显示其他 provider
		for _, p := range a.allProviders {
			if p.Name() == currentProviderName {
				continue
			}
			hasModels := false
			for _, e := range a.modelList {
				if e.ProviderName == p.Name() {
					hasModels = true
					break
				}
			}
			if !hasModels {
				continue
			}
			sb.WriteString("\n")
			sb.WriteString(systemStyle.Render(fmt.Sprintf("[%s]", p.Name())))
			sb.WriteString("\n")
			for i, e := range a.modelList {
				if e.ProviderName != p.Name() {
					continue
				}
				marker := "  "
				if i == a.modelIdx {
					marker = "▶ "
				}
				sb.WriteString(systemStyle.Render(fmt.Sprintf("%s%s", marker, e.ModelID)))
				sb.WriteString("\n")
			}
		}
	}

	// 流式输出缓冲
	a.streamMu.Lock()
	buf := a.streamBuf
	a.streamMu.Unlock()
	if a.loading && buf != "" {
		rendered := a.renderMarkdown(buf)
		for _, line := range strings.Split(rendered, "\n") {
			sb.WriteString(assistantStyle.Render("  " + line))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderWelcome 渲染欢迎界面（Logo + Tips + 状态）
func (a *App) renderWelcome() string {
	var sb strings.Builder

	// 单色 Logo（Loom = 织机）
	loomcodeStyle := lipgloss.NewStyle().Bold(true)
	blue := lipgloss.Color("39") // #00BFFF
	verStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	tipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	logo := []struct {
		text  string
		color lipgloss.TerminalColor
	}{
		{"  ██╗        █████╗    █████╗   ███╗   ███╗", blue},
		{"  ██║       ██╔══██╗  ██╔══██╗  ████╗ ████║", blue},
		{"  ██║       ██║  ██║  ██║  ██║  ██╔████╔██║", blue},
		{"  ██║       ██║  ██║  ██║  ██║  ██║╚██╔╝██║", blue},
		{"  ███████╗  ╚█████╔╝  ╚█████╔╝  ██║ ╚═╝ ██║", blue},
		{"  ╚══════╝   ╚════╝    ╚════╝   ╚═╝     ╚═╝", blue},
	}

	tips := []string{
		"开始使用的小提示",
		"Shift + Enter 换行",
		"输入 / 查看可用命令",
		fmt.Sprintf("Model: %s/%s", a.provider.Name(), a.model),
		fmt.Sprintf("Mode: %s", a.mode.String()),
	}

	maxLines := len(logo)
	if len(tips)+1 > maxLines {
		maxLines = len(tips) + 1
	}

	for i := 0; i < maxLines; i++ {
		left := ""
		if i < len(logo) {
			left = loomcodeStyle.Foreground(logo[i].color).Render(logo[i].text)
		}
		right := ""
		if i == 0 {
			right = verStyle.Render("LoomCode CLI v" + Version)
		} else if i-1 < len(tips) {
			right = tipStyle.Render(tips[i-1])
		}
		fmt.Fprintf(&sb, "%s  %s\n", left, right)
	}

	return sb.String()
}

// newGlamourRenderer 根据终端背景色创建 glamour 渲染器。
// 1.3 修复：不再硬编码 DarkStyle，亮色终端使用 LightStyle。
func (a *App) newGlamourRenderer(width int) *glamour.TermRenderer {
	style := glamourstyles.DarkStyle
	if !a.hasDarkBackground {
		style = glamourstyles.LightStyle
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	return r
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

	// 缓存结果（满了则清空一半，避免内存无限增长）
	if len(a.renderCache) >= consts.MaxRenderCacheEntries {
		for k := range a.renderCache {
			delete(a.renderCache, k)
			if len(a.renderCache) <= consts.MaxRenderCacheEntries/2 {
				break
			}
		}
	}
	a.renderCache[content] = result

	return result
}

func (a *App) renderStatusBar() string {
	costDisplay := a.renderCost()
	contextDisplay := a.renderContextUsage()
	left := fmt.Sprintf(" %s | %s | Tab:模式 | /:命令", a.provider.Name(), a.model)

	// 右侧分段拼接：context | tokens | cache | cost（为空时跳过，避免空分隔符）
	var rightParts []string
	rightParts = append(rightParts, contextDisplay)
	if tok := a.renderTokens(); tok != "" {
		rightParts = append(rightParts, tok)
	}
	if cache := a.renderCacheHit(); cache != "" {
		rightParts = append(rightParts, cache)
	}
	rightParts = append(rightParts, costDisplay)
	right := strings.Join(rightParts, " | ")

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	padding := a.width - leftW - rightW
	if padding < 1 {
		padding = 1
	}

	bar := left + strings.Repeat(" ", padding) + right
	return statusBarStyle.Width(a.width).Render(bar)
}

// renderCacheHit 渲染 prefix cache 命中率。
// eventLog 为 nil 或尚无输入 token 时返回空串（状态栏不显示该字段）。
func (a *App) renderCacheHit() string {
	if a.eventLog == nil {
		return ""
	}
	cached, total := a.eventLog.CacheStats()
	if total <= 0 {
		return ""
	}
	pct := cached * 100 / total
	return fmt.Sprintf("cache: %d%%", pct)
}

// renderTokens 渲染 token 计数。
// 显示累计输入/输出 token；流式生成中追加当前步实时估算（基于已接收字符数 / 3）。
// eventLog 为 nil 或尚无任何 token 时返回空串（状态栏不显示该字段）。
func (a *App) renderTokens() string {
	if a.eventLog == nil {
		return ""
	}
	input, output, _ := a.eventLog.TokenStats()
	if input == 0 && output == 0 {
		return ""
	}
	a.streamMu.Lock()
	streaming := a.streamChars
	loading := a.loading
	a.streamMu.Unlock()
	out := formatTokens(output)
	// 流式生成中：在累计输出基础上追加当前步实时估算，让用户看到 token 在涨
	if loading && streaming > 0 {
		out = fmt.Sprintf("%s+~%d", out, streaming/3)
	}
	return fmt.Sprintf("tok: ↑%s ↓%s", formatTokens(input), out)
}

// formatTokens 把 token 数格式化为紧凑显示：>=1000 用 "1.2k"，否则原数。
func formatTokens(n int64) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// costFloatFromUint64 把 atomic Uint64 中存储的 float64 位模式还原为 float64 值。
func costFloatFromUint64(v uint64) float64 {
	return math.Float64frombits(v)
}

// costUint64FromFloat 把 float64 值转换为 Uint64 位模式，用于 atomic 存储。
func costUint64FromFloat(v float64) uint64 {
	return math.Float64bits(v)
}

func (a *App) renderCost() string {
	sessionCost := costFloatFromUint64(a.costSession.Load())
	var display string
	if a.costBudget > 0 {
		display = fmt.Sprintf("$%.4f/$%.2f", sessionCost, a.costBudget)
	} else {
		display = fmt.Sprintf("$%.4f", sessionCost)
	}
	if sessionCost < consts.CostGreenThreshold {
		return costGreenStyle.Render(display)
	}
	if sessionCost < consts.CostYellowThreshold {
		return costYellowStyle.Render(display)
	}
	return costRedStyle.Render(display)
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

func (a *App) renderApproval() string {
	pw := a.pendingApproval
	var sb strings.Builder

	sb.WriteString(approvalTitleStyle.Render(fmt.Sprintf("  ⚠ %s: %s", pw.toolName, pw.path)))
	sb.WriteString("\n")

	if pw.toolName == "write_file" {
		sb.WriteString(renderWriteDiff(pw))
	} else {
		sb.WriteString(renderEditDiff(pw))
	}

	sb.WriteString("\n")
	sb.WriteString(approvalHelpStyle.Render("  Enter 确认 | Esc 拒绝"))
	sb.WriteString("\n")
	return sb.String()
}

func renderWriteDiff(pw *pendingWrite) string {
	var sb strings.Builder
	lines := strings.Split(pw.newContent, "\n")
	if len(lines) > 30 {
		lines = lines[:30]
	}
	if pw.oldContent != "" {
		sb.WriteString(diffHeaderStyle.Render("  --- (current)"))
		sb.WriteString("\n")
		oldLines := strings.Split(pw.oldContent, "\n")
		if len(oldLines) > 10 {
			oldLines = oldLines[:10]
		}
		for _, l := range oldLines {
			sb.WriteString(diffDelStyle.Render("  - " + l))
			sb.WriteString("\n")
		}
		sb.WriteString(diffHeaderStyle.Render("  +++ (new)"))
		sb.WriteString("\n")
	} else {
		sb.WriteString(diffHeaderStyle.Render("  +++ (new file)"))
		sb.WriteString("\n")
	}
	for _, l := range lines {
		sb.WriteString(diffAddStyle.Render("  + " + l))
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderEditDiff(pw *pendingWrite) string {
	var sb strings.Builder
	sb.WriteString(diffHeaderStyle.Render("  --- old_text"))
	sb.WriteString("\n")
	oldLines := strings.Split(pw.oldContent, "\n")
	if len(oldLines) > 20 {
		oldLines = oldLines[:20]
	}
	for _, l := range oldLines {
		sb.WriteString(diffDelStyle.Render("  - " + l))
		sb.WriteString("\n")
	}
	sb.WriteString(diffHeaderStyle.Render("  +++ new_text"))
	sb.WriteString("\n")
	newLines := strings.Split(pw.newContent, "\n")
	if len(newLines) > 20 {
		newLines = newLines[:20]
	}
	for _, l := range newLines {
		sb.WriteString(diffAddStyle.Render("  + " + l))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (a *App) runAgent(ctx context.Context, input string) tea.Cmd {
	return func() tea.Msg {
		textCh, errCh := a.agent.RunStream(ctx, input)
		var streamErr error

		for textCh != nil || errCh != nil {
			select {
			case text, ok := <-textCh:
				if !ok {
					textCh = nil
					break
				}
				if a.program != nil {
					a.program.Send(streamChunkMsg(text))
				}
			case err, ok := <-errCh:
				if !ok {
					errCh = nil
					break
				}
				streamErr = err
				if a.program != nil {
					a.program.Send(streamErrorMsg(err.Error()))
				}
			}
		}

		if streamErr == nil && a.program != nil {
			a.program.Send(streamDoneMsg{})
		}
		return nil
	}
}

// 消息类型
type streamChunkMsg string
type streamDoneMsg struct{}
type streamErrorMsg string

// truncateRunes 按 rune 边界截断字符串，避免切在多字节字符中间
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// friendlyError 将常见错误映射为用户友好提示
func friendlyError(err string) string {
	lower := strings.ToLower(err)
	switch {
	case strings.Contains(lower, "api key") || strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		return "API Key 无效或未设置。运行 loomcode setup 或检查 .env 文件中的 API_KEY 配置。"
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests"):
		return "请求过于频繁，请稍后重试。"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return "请求超时，请检查网络连接或稍后重试。"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "dial tcp"):
		return "无法连接到服务器，请检查网络。"
	case strings.Contains(lower, "503"):
		return "服务暂时不可用，请稍后重试。"
	case strings.Contains(lower, "500"):
		return "服务器内部错误，请稍后重试。"
	case strings.Contains(lower, "ssl") || strings.Contains(lower, "tls"):
		return "SSL/TLS 连接失败，请检查网络或代理配置。"
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
		if !isValidEnvKey(key) {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("无效的环境变量名: %q（仅允许字母、数字、下划线，且不能以数字开头）", key)})
			return a, nil
		}
		val := strings.Join(parts[3:], " ")
		_ = os.Setenv(key, val)
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
		_ = os.Unsetenv(key)
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
		// val 在 loadEnvVars/set 时已脱敏，此处直接输出，避免二次 mask 导致掩码错乱
		fmt.Fprintf(&sb, "  %s = %s\n", key, val)
	}
	a.messages = append(a.messages, chatMessage{Role: "system", Content: sb.String()})
	return a, nil
}

// handleRewindCmd 处理 /rewind 命令，支持查看快照列表和恢复指定快照。
func (a *App) handleRewindCmd(parts []string) (tea.Model, tea.Cmd) {
	if a.checkpointMgr == nil {
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "快照管理器未启用。"})
		return a, nil
	}

	// agent 运行期间禁止恢复，避免并发写文件导致 TOCTOU 竞态（N6）
	if a.loading {
		a.messages = append(a.messages, chatMessage{Role: "system", Content: "Agent 正在运行，请等待完成或按 Esc 取消后再执行 /rewind。"})
		return a, nil
	}

	// /rewind — 列出最近快照
	if len(parts) < 2 {
		checkpoints, err := a.checkpointMgr.List("", 20)
		if err != nil {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("读取快照失败: %v", err)})
			return a, nil
		}
		a.messages = append(a.messages, chatMessage{Role: "system", Content: tool.FormatCheckpointSummary(checkpoints)})
		return a, nil
	}

	// /rewind last — 恢复最近一个快照
	if parts[1] == "last" {
		cp, err := a.checkpointMgr.RestoreLast()
		if err != nil {
			a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("恢复失败: %v", err)})
			return a, nil
		}
		a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("✓ 已恢复最近快照 [%s]，文件: %s", cp.ID, cp.OriginalPath)})
		return a, nil
	}

	// /rewind <ID> — 恢复指定快照
	cpID := parts[1]
	if err := a.checkpointMgr.Restore(cpID); err != nil {
		a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("恢复失败: %v", err)})
		return a, nil
	}
	a.messages = append(a.messages, chatMessage{Role: "system", Content: fmt.Sprintf("✓ 已恢复快照 [%s]", cpID)})
	return a, nil
}

// processQueue 处理队列中的下一个任务
func (a *App) processQueue() tea.Cmd {
	if len(a.taskQueue) == 0 {
		return nil
	}
	next := a.taskQueue[0]
	a.taskQueue = a.taskQueue[1:]

	a.messages = append(a.messages, chatMessage{Role: "user", Content: next, Timestamp: time.Now()})
	a.loading = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	a.viewport.SetContent(a.renderMessages(a.height-8, ""))
	a.viewport.GotoBottom()

	return a.runAgent(ctx, next)
}

// isValidEnvKey 校验环境变量名：仅允许字母、数字、下划线，且不能以数字开头
func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 && r >= '0' && r <= '9' {
			return false
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
