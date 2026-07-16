package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/session"
	"github.com/ShawnLiuSZ/loomcode/internal/testutil"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

func newTestApp() *App {
	p := testutil.NewStubProvider(nil)
	tools := tool.NewRegistry()
	return NewApp(p, tools)
}

func TestTextareaInput(t *testing.T) {
	app := newTestApp()

	// 初始化窗口大小
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// 模拟输入 "hi"
	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})

	result := m.(*App)
	if result.textArea.Value() != "hi" {
		t.Errorf("expected textarea value 'hi', got %q", result.textArea.Value())
	}
}

func TestEnterSendsMessage(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// 输入文字
	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})

	// 按 Enter 发送
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	result := m.(*App)
	// textarea 应该被清空
	if result.textArea.Value() != "" {
		t.Errorf("expected textarea to be empty after enter, got %q", result.textArea.Value())
	}
	// 消息应该被添加（welcome + user）
	if len(result.messages) < 2 {
		t.Errorf("expected at least 2 messages after enter, got %d", len(result.messages))
	}
	// 最后一条应该是 user 消息
	lastMsg := result.messages[len(result.messages)-1]
	if lastMsg.Role != "user" {
		t.Errorf("expected last message role 'user', got %q", lastMsg.Role)
	}
	if lastMsg.Content != "hi" {
		t.Errorf("expected last message content 'hi', got %q", lastMsg.Content)
	}
}

func TestEscClearsInput(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// 输入文字
	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})

	// 按 Esc 清空
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	result := m.(*App)
	if result.textArea.Value() != "" {
		t.Errorf("expected textarea to be empty after esc, got %q", result.textArea.Value())
	}
}

func TestCtrlCQuits(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	var m tea.Model = app
	// 1.12 修复：Ctrl+C 需要二次确认。第一次按显示提示，不退出。
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := m.(*App)
	if result.quitting {
		t.Error("expected quitting to be false after first ctrl+c (confirm required)")
	}
	if result.confirmQuit != true {
		t.Error("expected confirmQuit to be true after first ctrl+c")
	}
	// 第一次 Ctrl+C 应返回 Tick 命令（3 秒重置），而非 tea.Quit
	if cmd == nil {
		t.Error("expected a tick command after first ctrl+c")
	}

	// 第二次按 Ctrl+C 确认退出
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result = m.(*App)
	if !result.quitting {
		t.Error("expected quitting to be true after second ctrl+c")
	}
	if cmd == nil {
		t.Error("expected a quit command after second ctrl+c")
	}
}

func TestTabCyclesMode(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	result := m.(*App)
	// Should cycle from build to plan
	if result.mode.String() != "plan" {
		t.Errorf("expected mode 'plan' after tab, got %q", result.mode.String())
	}
}

func TestModelPickerShowsAllProvidersWithArrowMovement(t *testing.T) {
	// 创建两个 provider：stub（当前）和 mimo
	stubProv := testutil.NewStubProvider(nil)
	mimoProv := testutil.NewStubProvider(nil)
	mimoProv.NameVal = "mimo"
	mimoProv.ModelsVal = []provider.ModelInfo{
		{ID: "mimo-v2.5", Name: "MiMo V2.5", ContextWindow: 1048576},
		{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro", ContextWindow: 262144},
	}

	tools := tool.NewRegistry()
	app := NewApp(stubProv, tools)
	app.SetProviders([]provider.Provider{stubProv, mimoProv})
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// 验证：初始 provider 的模型列表应该有 0 个模型（stubProvider 默认没有 Models）
	// 但需要给 stub 加一些模型
	stubProv.ModelsVal = []provider.ModelInfo{
		{ID: "ds-v4-flash", Name: "DeepSeek V4 Flash", ContextWindow: 131072},
		{ID: "ds-v4-pro", Name: "DeepSeek V4 Pro", ContextWindow: 131072},
	}

	// 重新调用 SetProviders 更新
	app.SetProviders([]provider.Provider{stubProv, mimoProv})

	// 模拟输入 /model 并按 Enter
	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})

	t.Logf("textarea value before enter: %q", m.(*App).textArea.Value())

	// 直接调用 handleModelCmd 验证核心逻辑
	app2 := m.(*App)
	app2.handleModelCmd([]string{"/model"})
	t.Logf("showModelPicker after direct call: %v", app2.showModelPicker)
	t.Logf("modelList length: %d", len(app2.modelList))
	for i, e := range app2.modelList {
		t.Logf("  [%d] %s/%s", i, e.ProviderName, e.ModelID)
	}

	// 验证模型选择器被打开
	if !app2.showModelPicker {
		t.Fatal("expected showModelPicker to be true after /model command")
	}

	// 验证模型列表包含所有 4 个模型
	expectedCount := 4 // 2 stub + 2 mimo
	if len(app2.modelList) != expectedCount {
		t.Fatalf("expected %d models in picker, got %d", expectedCount, len(app2.modelList))
	}

	// 验证模型列表顺序：stub 模型在前，mimo 模型在后
	if app2.modelList[0].ProviderName != "stub" || app2.modelList[0].ModelID != "ds-v4-flash" {
		t.Errorf("expected first entry stub/ds-v4-flash, got %s/%s", app2.modelList[0].ProviderName, app2.modelList[0].ModelID)
	}
	if app2.modelList[2].ProviderName != "mimo" || app2.modelList[2].ModelID != "mimo-v2.5" {
		t.Errorf("expected third entry mimo/mimo-v2.5, got %s/%s", app2.modelList[2].ProviderName, app2.modelList[2].ModelID)
	}

	// 验证初始选中索引为 0（箭头指向第一个条目）
	if app2.modelIdx != 0 {
		t.Errorf("expected initial modelIdx 0, got %d", app2.modelIdx)
	}

	// 验证渲染输出包含模型选择器界面
	rendered := app2.renderMessages(30, "", "")
	if !strings.Contains(rendered, "▶ ds-v4-flash") {
		t.Errorf("expected rendered picker to show arrow on first model, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[mimo]") {
		t.Errorf("expected rendered picker to show mimo provider section, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "mimo-v2.5-pro") {
		t.Errorf("expected rendered picker to show mimo-v2.5-pro, got:\n%s", rendered)
	}

	// 模拟按向下箭头 — 并通过 Update 测试键盘事件处理
	// 启用 showModelPicker
	m.(*App).showModelPicker = true
	m.(*App).modelList = app2.modelList
	m.(*App).modelIdx = 0
	m.(*App).allProviders = app2.allProviders
	m.(*App).allProviderModels = app2.allProviderModels
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	result := m.(*App)
	if result.modelIdx != 1 {
		t.Errorf("expected modelIdx 1 after down arrow, got %d", result.modelIdx)
	}
	rendered = result.renderMessages(30, "", "")
	if !strings.Contains(rendered, "▶ ds-v4-pro") {
		t.Errorf("expected arrow on ds-v4-pro after down, got:\n%s", rendered)
	}

	// 按 Enter 确认选择ds-v4-pro
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result = m.(*App)
	if result.showModelPicker {
		t.Error("expected model picker to close after enter")
	}
	if result.model != "ds-v4-pro" {
		t.Errorf("expected model ds-v4-pro, got %s", result.model)
	}

	// 验证有"模型切换"系统消息
	found := false
	for _, msg := range result.messages {
		if strings.Contains(msg.Content, "stub") && strings.Contains(msg.Content, "ds-v4-pro") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected system message about model switch to stub/ds-v4-pro")
	}

	// 再测试跨 provider 切换 — 直接操作状态模拟
	app2.showModelPicker = true
	app2.modelIdx = 2 // 选中 mimo/mimo-v2.5
	app2.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if app2.model != "mimo-v2.5" {
		t.Errorf("expected model mimo-v2.5 after switching, got %s", app2.model)
	}
	if app2.provider.Name() != "mimo" {
		t.Errorf("expected provider mimo after cross-provider switch, got %s", app2.provider.Name())
	}
	if app2.showModelPicker {
		t.Error("expected model picker to close after enter")
	}
}

func TestSlashTriggersSuggestions(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// 输入 "/"
	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

	result := m.(*App)
	if !result.showSuggestions {
		t.Error("expected suggestions to be shown after typing /")
	}
	if len(result.suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}
}

func TestStreamMessagesAccumulateAndFinalize(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	app.loading = true

	var m tea.Model = app
	m, _ = m.Update(streamChunkMsg{Content: "hello "})
	m, _ = m.Update(streamChunkMsg{Content: "world"})

	result := m.(*App)
	if result.streamBuf != "hello world" {
		t.Errorf("expected streamBuf 'hello world', got %q", result.streamBuf)
	}
	if !result.loading {
		t.Error("expected loading to remain true while streaming")
	}

	m, _ = m.Update(streamDoneMsg{})
	result = m.(*App)
	if result.loading {
		t.Error("expected loading to be false after stream done")
	}
	if result.streamBuf != "" {
		t.Errorf("expected streamBuf to be cleared, got %q", result.streamBuf)
	}
	lastMsg := result.messages[len(result.messages)-1]
	if lastMsg.Role != "assistant" || lastMsg.Content != "hello world" {
		t.Errorf("expected final assistant message 'hello world', got %q/%q", lastMsg.Role, lastMsg.Content)
	}
}

func TestStreamErrorClearsBuffer(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	app.loading = true
	app.streamBuf = "partial"

	var m tea.Model = app
	m, _ = m.Update(streamErrorMsg("something went wrong"))

	result := m.(*App)
	if result.loading {
		t.Error("expected loading to be false after stream error")
	}
	if result.streamBuf != "" {
		t.Errorf("expected streamBuf to be cleared after error, got %q", result.streamBuf)
	}
	lastMsg := result.messages[len(result.messages)-1]
	if lastMsg.Role != "error" {
		t.Errorf("expected last message role 'error', got %q", lastMsg.Role)
	}
}

func TestResumeCmd_EntersPickerAndRendersViewport(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	mgr, err := session.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("create session manager: %v", err)
	}
	app.SetSessionManager(mgr)
	mgr.Create("old session", "model", "provider")

	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/resume")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	result := m.(*App)
	if !result.showResumePicker {
		t.Error("expected resume picker to be shown")
	}
	if !strings.Contains(result.viewport.View(), "old session") {
		t.Errorf("expected viewport to render resume picker with session name, got:\n%s", result.viewport.View())
	}
}

func TestSaveSession_RenamesDefaultSessionFromFirstUserMessage(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	mgr, err := session.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("create session manager: %v", err)
	}
	app.SetSessionManager(mgr)

	// 创建一个名为 default 的会话并恢复它（模拟 /resume 后首个任务）
	sess := mgr.Create("default", "model", "provider")
	app.RestoreSession(sess)

	// 模拟用户发送任务
	app.messages = append(app.messages, chatMessage{Role: "user", Content: "implement login feature"})
	app.saveSession()

	restored, ok := mgr.Get(sess.ID)
	if !ok {
		t.Fatal("session not found after save")
	}
	if restored.Name != "implement login feature" {
		t.Errorf("expected session name renamed to first user message, got %q", restored.Name)
	}
}
