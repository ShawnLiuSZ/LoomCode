package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

// GoalStopCondition 停止条件管理器
type GoalStopCondition struct {
	mu        sync.RWMutex
	goal      string
	judge     provider.Provider
	enabled   bool
	lastCheck time.Time
}

// NewGoalStopCondition 创建停止条件管理器
func NewGoalStopCondition(judge provider.Provider) *GoalStopCondition {
	return &GoalStopCondition{
		judge: judge,
	}
}

// SetGoal 设置停止条件
func (g *GoalStopCondition) SetGoal(goal string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.goal = goal
	g.enabled = goal != ""
}

// GetGoal 获取当前停止条件
func (g *GoalStopCondition) GetGoal() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.goal
}

// IsEnabled 是否启用停止条件
func (g *GoalStopCondition) IsEnabled() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.enabled
}

// Clear 清除停止条件
func (g *GoalStopCondition) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.goal = ""
	g.enabled = false
}

// Evaluate 评估目标是否达成
// 返回: achieved(是否达成), reason(原因), error
func (g *GoalStopCondition) Evaluate(ctx context.Context, messages []provider.Message) (bool, string, error) {
	g.mu.RLock()
	goal := g.goal
	enabled := g.enabled
	g.mu.RUnlock()

	if !enabled || goal == "" {
		return false, "no goal set", nil
	}

	// 构建评估提示
	prompt := g.buildEvalPrompt(goal, messages)

	// 调用 judge 模型
	resp, err := g.judge.Chat(ctx, &provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: judgeSystemPrompt},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return false, "", fmt.Errorf("judge evaluation failed: %w", err)
	}

	// 解析评估结果
	achieved, reason := g.parseEvaluation(resp.Content)

	g.mu.Lock()
	g.lastCheck = time.Now()
	g.mu.Unlock()

	return achieved, reason, nil
}

// buildEvalPrompt 构建评估提示
func (g *GoalStopCondition) buildEvalPrompt(goal string, messages []provider.Message) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "## Goal\n%s\n\n", goal)
	sb.WriteString("## Conversation Summary\n")

	// 提取最近的对话内容（避免上下文过长）
	recentMessages := g.extractRecentMessages(messages, 10)
	for _, msg := range recentMessages {
		switch msg.Role {
		case "user":
			fmt.Fprintf(&sb, "User: %s\n", truncate(msg.Content, 200))
		case "assistant":
			if msg.Content != "" {
				fmt.Fprintf(&sb, "Assistant: %s\n", truncate(msg.Content, 200))
			}
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					fmt.Fprintf(&sb, "Tool Call: %s(%s)\n", tc.Function.Name, truncate(tc.Function.Arguments, 100))
				}
			}
		case "tool":
			fmt.Fprintf(&sb, "Tool Result: %s\n", truncate(msg.Content, 200))
		}
	}

	sb.WriteString("\n## Evaluation Question\n")
	sb.WriteString("Based on the conversation above, has the goal been ACHIEVED?\n")
	sb.WriteString("Consider:\n")
	sb.WriteString("1. Has the user's request been fully addressed?\n")
	sb.WriteString("2. Are there any pending tasks or incomplete work?\n")
	sb.WriteString("3. Would a reasonable person consider this task complete?\n\n")
	sb.WriteString("Answer with exactly one of:\n")
	sb.WriteString("- ACHIEVED: <brief reason why>\n")
	sb.WriteString("- NOT_ACHIEVED: <brief reason why not>\n")

	return sb.String()
}

// extractRecentMessages 提取最近的消息
func (g *GoalStopCondition) extractRecentMessages(messages []provider.Message, limit int) []provider.Message {
	if len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

// parseEvaluation 解析评估结果
func (g *GoalStopCondition) parseEvaluation(content string) (bool, string) {
	content = strings.ToUpper(strings.TrimSpace(content))

	if strings.HasPrefix(content, "ACHIEVED") {
		reason := extractReason(content, "ACHIEVED")
		return true, reason
	}

	if strings.HasPrefix(content, "NOT_ACHIEVED") {
		reason := extractReason(content, "NOT_ACHIEVED")
		return false, reason
	}

	// 默认未达成
	return false, "evaluation inconclusive"
}

// extractReason 提取原因
func extractReason(content, prefix string) string {
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, ":"); idx > 0 {
		reason := strings.TrimSpace(content[idx+1:])
		if reason != "" {
			return reason
		}
	}
	return prefix
}

// truncate 截断字符串（按 rune 边界）
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// judgeSystemPrompt judge 模型系统提示
const judgeSystemPrompt = `You are an impartial judge evaluating whether a goal has been achieved in a coding assistant conversation.

Your role is to:
1. Analyze the conversation objectively
2. Determine if the goal has been sufficiently achieved
3. Provide a brief, clear reason for your decision

Be fair but strict:
- The goal should be substantially achieved, not just partially
- Minor issues or suggestions for improvement don't prevent achievement
- If the assistant is still working or has errors, it's NOT_ACHIEVED
- If the user seems satisfied and the work is done, it's ACHIEVED

Always respond with exactly one of:
- ACHIEVED: <reason>
- NOT_ACHIEVED: <reason>`
