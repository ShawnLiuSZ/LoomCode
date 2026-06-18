package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// App 交互式 TUI 应用
type App struct {
	width  int
	height int

	// 输入
	input string

	// 聊天历史
	messages []chatMessage

	// 状态
	running bool
	loading bool
	ready   bool
}

type chatMessage struct {
	Role    string // "user" | "assistant" | "system" | "tool"
	Content string
}

var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	systemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	inputStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	loadingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Padding(1, 2)
)

// NewApp 创建 TUI 应用
func NewApp() *App {
	return &App{
		messages: []chatMessage{
			{Role: "system", Content: "Welcome to Helix CLI - 双螺旋 · 多模型 · 可扩展"},
			{Role: "system", Content: "Type your task and press Enter. Type /quit to exit."},
		},
	}
}

// Init 初始化
func (a *App) Init() tea.Cmd {
	return nil
}

// Update 更新状态
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit

		case "enter":
			input := strings.TrimSpace(a.input)
			if input == "" {
				return a, nil
			}

			if input == "/quit" || input == "/exit" {
				return a, tea.Quit
			}

			// 添加用户消息
			a.messages = append(a.messages, chatMessage{Role: "user", Content: input})
			a.input = ""

			// 模拟处理（实际需连接 Agent）
			a.loading = true
			return a, simulateResponse(input)

		default:
			a.input += msg.String()
		}

	case responseMsg:
		a.loading = false
		a.messages = append(a.messages, chatMessage{Role: "assistant", Content: string(msg)})
	}
	return a, nil
}

// View 渲染界面
func (a *App) View() string {
	if !a.ready {
		return "Initializing...\n"
	}

	var sb strings.Builder

	// 标题
	sb.WriteString(headerStyle.Render("Helix CLI"))
	sb.WriteString("\n")

	// 消息历史
	visibleHeight := a.height - 5
	startIdx := 0
	if len(a.messages) > visibleHeight {
		startIdx = len(a.messages) - visibleHeight
	}

	for _, msg := range a.messages[startIdx:] {
		switch msg.Role {
		case "user":
			sb.WriteString(userStyle.Render("> " + msg.Content))
		case "assistant":
			sb.WriteString(assistantStyle.Render(msg.Content))
		case "system":
			sb.WriteString(systemStyle.Render(msg.Content))
		case "tool":
			sb.WriteString(toolStyle.Render("[tool] " + msg.Content))
		}
		sb.WriteString("\n")
	}

	// 加载指示器
	if a.loading {
		sb.WriteString(loadingStyle.Render("Thinking..."))
		sb.WriteString("\n")
	}

	// 输入区域
	inputView := inputStyle.Width(a.width - 4).Render("> " + a.input)
	sb.WriteString(inputView)
	sb.WriteString("\n")

	// 帮助
	sb.WriteString(helpStyle.Render("Enter: send | Ctrl+C: quit | /quit: exit"))

	return sb.String()
}

// responseMsg 模拟响应消息
type responseMsg string

func simulateResponse(input string) tea.Cmd {
	return func() tea.Msg {
		return responseMsg(fmt.Sprintf("Processing: %s (Agent integration coming soon)", input))
	}
}
