package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// parseRemember 解析 /remember 的参数。
// "<键>: <值>" 形式拆成 key/val；否则整体作为内容，自动生成 note-N 键。
// 空白参数返回 ok=false。
func parseRemember(arg string, existingCount int) (key, val string, ok bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", false
	}
	if i := strings.Index(arg, ":"); i > 0 {
		k := strings.TrimSpace(arg[:i])
		v := strings.TrimSpace(arg[i+1:])
		if k != "" && v != "" {
			return k, v, true
		}
	}
	return fmt.Sprintf("note-%d", existingCount+1), arg, true
}

// handleRememberCmd 处理 /remember，把一条事实写入项目记忆（下次会话自动注入）。
func (a *App) handleRememberCmd(arg string) (tea.Model, tea.Cmd) {
	if a.memMgr == nil {
		a.messages = append(a.messages, chatMessage{
			Role: "system", Content: "记忆未启用（无法打开 ~/.loomcode/memory.db）", Timestamp: time.Now(),
		})
		return a, nil
	}

	existing, _ := a.memMgr.ListProjectMemories()
	key, val, ok := parseRemember(arg, len(existing))
	if !ok {
		a.messages = append(a.messages, chatMessage{
			Role: "system", Content: "用法: /remember <内容>  或  /remember <键>: <值>", Timestamp: time.Now(),
		})
		return a, nil
	}

	if err := a.memMgr.SaveProjectMemory(key, val); err != nil {
		a.messages = append(a.messages, chatMessage{
			Role: "system", Content: fmt.Sprintf("记忆保存失败: %v", err), Timestamp: time.Now(),
		})
		return a, nil
	}

	a.messages = append(a.messages, chatMessage{
		Role:      "system",
		Content:   fmt.Sprintf("已记住 [%s]：%s（下次会话起自动注入上下文）", key, val),
		Timestamp: time.Now(),
	})
	return a, nil
}
