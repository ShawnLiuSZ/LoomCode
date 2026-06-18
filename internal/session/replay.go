package session

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// Replayer 会话重放器
type Replayer struct {
	session  *Session
	output   io.Writer
	delay    time.Duration
	verbose  bool
}

// NewReplayer 创建会话重放器
func NewReplayer(session *Session, output io.Writer) *Replayer {
	return &Replayer{
		session: session,
		output:  output,
		delay:   100 * time.Millisecond,
		verbose: false,
	}
}

// SetDelay 设置重放延迟
func (r *Replayer) SetDelay(delay time.Duration) {
	r.delay = delay
}

// SetVerbose 设置详细模式
func (r *Replayer) SetVerbose(verbose bool) {
	r.verbose = verbose
}

// Replay 重放会话
func (r *Replayer) Replay(ctx context.Context) error {
	if r.session == nil {
		return fmt.Errorf("no session to replay")
	}

	r.writeHeader()

	for i, msg := range r.session.Messages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := r.replayMessage(i, msg); err != nil {
			return err
		}

		if r.delay > 0 {
			time.Sleep(r.delay)
		}
	}

	r.writeFooter()
	return nil
}

// replayMessage 重放单条消息
func (r *Replayer) replayMessage(index int, msg Message) error {
	switch msg.Role {
	case "user":
		r.writeUserMessage(msg)
	case "assistant":
		r.writeAssistantMessage(msg)
	case "tool":
		r.writeToolMessage(msg)
	case "system":
		if r.verbose {
			r.writeSystemMessage(msg)
		}
	}
	return nil
}

// writeHeader 写入头部
func (r *Replayer) writeHeader() {
	fmt.Fprintln(r.output)
	fmt.Fprintln(r.output, "═══════════════════════════════════════════════════════════════")
	fmt.Fprintf(r.output, "  会话重放: %s\n", r.session.Name)
	fmt.Fprintf(r.output, "  模型: %s | Provider: %s\n", r.session.Model, r.session.Provider)
	fmt.Fprintf(r.output, "  创建时间: %s\n", r.session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(r.output, "  消息数量: %d\n", len(r.session.Messages))
	fmt.Fprintln(r.output, "═══════════════════════════════════════════════════════════════")
	fmt.Fprintln(r.output)
}

// writeFooter 写入尾部
func (r *Replayer) writeFooter() {
	fmt.Fprintln(r.output)
	fmt.Fprintln(r.output, "═══════════════════════════════════════════════════════════════")
	fmt.Fprintln(r.output, "  重放完成")
	fmt.Fprintln(r.output, "═══════════════════════════════════════════════════════════════")
}

// writeUserMessage 写入用户消息
func (r *Replayer) writeUserMessage(msg Message) {
	fmt.Fprintln(r.output)
	fmt.Fprintf(r.output, "▸ 用户 (%s)\n", msg.Timestamp.Format("15:04:05"))
	fmt.Fprintln(r.output, "───────────────────────────────────────────────────────────────")
	r.writeWrapped(msg.Content, "  ")
	fmt.Fprintln(r.output)
}

// writeAssistantMessage 写入助手消息
func (r *Replayer) writeAssistantMessage(msg Message) {
	fmt.Fprintln(r.output)
	fmt.Fprintf(r.output, "▸ 助手\n")
	fmt.Fprintln(r.output, "───────────────────────────────────────────────────────────────")
	if msg.Content != "" {
		r.writeWrapped(msg.Content, "  ")
	}
	if msg.ToolName != "" {
		fmt.Fprintf(r.output, "  🔧 工具调用: %s\n", msg.ToolName)
	}
	fmt.Fprintln(r.output)
}

// writeToolMessage 写入工具消息
func (r *Replayer) writeToolMessage(msg Message) {
	if r.verbose {
		fmt.Fprintln(r.output)
		fmt.Fprintf(r.output, "▸ 工具结果 (%s)\n", msg.ToolName)
		fmt.Fprintln(r.output, "───────────────────────────────────────────────────────────────")
		r.writeWrapped(msg.Content, "  ")
		fmt.Fprintln(r.output)
	}
}

// writeSystemMessage 写入系统消息
func (r *Replayer) writeSystemMessage(msg Message) {
	fmt.Fprintln(r.output)
	fmt.Fprintf(r.output, "▸ 系统\n")
	fmt.Fprintln(r.output, "───────────────────────────────────────────────────────────────")
	r.writeWrapped(msg.Content, "  ")
	fmt.Fprintln(r.output)
}

// writeWrapped 写入自动换行的文本
func (r *Replayer) writeWrapped(text string, prefix string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		fmt.Fprintf(r.output, "%s%s\n", prefix, line)
	}
}

// ReplayOptions 重放选项
type ReplayOptions struct {
	Delay   time.Duration
	Verbose bool
	Start   int
	End     int
}

// ReplaySession 重放会话的便捷函数
func ReplaySession(ctx context.Context, session *Session, opts *ReplayOptions) error {
	if opts == nil {
		opts = &ReplayOptions{
			Delay:   100 * time.Millisecond,
			Verbose: false,
		}
	}

	replayer := NewReplayer(session, nil)
	replayer.SetDelay(opts.Delay)
	replayer.SetVerbose(opts.Verbose)

	return replayer.Replay(ctx)
}

// GetSessionInfo 获取会话信息
func GetSessionInfo(session *Session) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("会话: %s\n", session.Name))
	sb.WriteString(fmt.Sprintf("模型: %s\n", session.Model))
	sb.WriteString(fmt.Sprintf("Provider: %s\n", session.Provider))
	sb.WriteString(fmt.Sprintf("创建时间: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("更新时间: %s\n", session.UpdatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("消息数量: %d\n", len(session.Messages)))

	return sb.String()
}
