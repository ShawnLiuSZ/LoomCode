package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

// MemoryProvider 提供需注入系统提示的长期记忆（项目知识、用户偏好）。
// memory.Manager 已实现该接口（BuildContextPrompt）。
type MemoryProvider interface {
	BuildContextPrompt() string
}

// keepRecentMessages 压缩时保留的最近消息条数（在轮次边界上对齐）
const keepRecentMessages = 8

// maxDirEntries 系统提示中目录清单的最大条目数
const maxDirEntries = 40

// buildEnvContext 构建环境上下文块，为模型提供工作目录、平台、日期、
// git 分支与顶层目录结构等"锚定信息"，避免模型先用工具去摸索这些基本事实。
func (a *Agent) buildEnvContext() string {
	var sb strings.Builder
	sb.WriteString("\n## Environment\n")
	fmt.Fprintf(&sb, "- OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&sb, "- Date: %s\n", time.Now().Format("2006-01"))

	if a.workDir == "" {
		return sb.String()
	}
	fmt.Fprintf(&sb, "- Working directory: %s\n", a.workDir)

	if branch := gitBranch(a.workDir); branch != "" {
		fmt.Fprintf(&sb, "- Git branch: %s\n", branch)
	}

	if entries := listDir(a.workDir); len(entries) > 0 {
		sb.WriteString("- Top-level entries:\n")
		for _, e := range entries {
			sb.WriteString("  - " + e + "\n")
		}
	}
	return sb.String()
}

// gitBranch 直接读取 .git/HEAD 解析当前分支，避免 exec git（更快、无依赖）。
func gitBranch(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	if ref := strings.TrimPrefix(head, "ref: refs/heads/"); ref != head {
		return ref
	}
	// detached HEAD：返回短哈希
	if len(head) >= 7 {
		return head[:7]
	}
	return ""
}

// listDir 返回顶层非隐藏条目，目录加 "/" 后缀，按名称排序并限量。
func listDir(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			name += "/"
		}
		out = append(out, name)
	}
	sort.Strings(out)
	if len(out) > maxDirEntries {
		out = out[:maxDirEntries]
		out = append(out, fmt.Sprintf("... (%d+ entries truncated)", maxDirEntries))
	}
	return out
}

// archiveDroppedMessages 把压缩时被丢弃的消息段归档到
// ~/.loomcode/archive/<session_id>/<timestamp>.jsonl，便于事后追溯。
// 任何失败都只记录日志告警，绝不中断压缩流程。
// Agent 暂未持久化 session_id，这里用时间戳生成临时唯一标识，避免改动结构体。
func (a *Agent) archiveDroppedMessages(start, cut int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("archiveDroppedMessages panic: %v", r)
		}
	}()
	if cut <= start {
		return
	}
	dropped := a.messages[start:cut]
	if len(dropped) == 0 {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("archive: cannot resolve home dir: %v", err)
		return
	}
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	archiveDir := filepath.Join(home, ".loomcode", "archive", sessionID)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		log.Printf("archive: mkdir %s failed: %v", archiveDir, err)
		return
	}
	fname := filepath.Join(archiveDir, time.Now().Format("20060102-150405")+".jsonl")
	data, err := json.Marshal(dropped)
	if err != nil {
		log.Printf("archive: marshal failed: %v", err)
		return
	}
	if err := os.WriteFile(fname, data, 0o644); err != nil {
		log.Printf("archive: write %s failed: %v", fname, err)
		return
	}
}

// leadingSystemCount 返回前导 system 消息数量（静态 system + 动态 system）。
// 压缩与截断都跳过这些消息，以保持 [system, ...] prefix 稳定，最大化 prefix cache 命中率。
func leadingSystemCount(msgs []provider.Message) int {
	n := 0
	for n < len(msgs) && msgs[n].Role == "system" {
		n++
	}
	return n
}

// compactMessages 缓存友好的上下文压缩：当估算 token 超过窗口阈值时，
// 把"较旧的完整轮次"摘要为单条固定的 summary 块插在 system 之后，
// 保持 [system, summary] 前缀稳定 —— 从而最大化 provider 的 prefix cache 命中率。
//
// 与旧的 truncateMessages（每步丢弃最旧轮次、持续位移前缀、击穿缓存）相反：
// 压缩后 token 大幅下降，后续多步不再触发，前缀保持稳定。
// 若摘要调用失败，则回退到机械截断，保证永不超窗、永不崩溃。
func (a *Agent) compactMessages(ctx context.Context, ctxWindow int) {
	if ctxWindow <= 0 {
		return // 未知模型，不压缩
	}
	maxInput := ctxWindow * 80 / 100
	if a.estimateTokens(a.messages) <= maxInput {
		return
	}

	// 跳过所有前导 system 消息（静态 system + 动态 system），保持 prefix 稳定。
	start := leadingSystemCount(a.messages)
	if len(a.messages) <= keepRecentMessages+start {
		return // 内容太少，无可压缩
	}

	cut := len(a.messages) - keepRecentMessages
	if cut <= start {
		return
	}
	// 保证保留区不以孤立的 tool 结果开头（否则 assistant.tool_calls 缺失会触发 API 400）
	for cut < len(a.messages) && a.messages[cut].Role == "tool" {
		cut++
	}
	if cut >= len(a.messages) {
		a.truncateMessages(ctxWindow) // 找不到干净边界，回退
		return
	}

	summary, usage, err := a.summarize(ctx, a.messages[start:cut])
	if err != nil {
		a.truncateMessages(ctxWindow) // 摘要失败，回退到机械截断
		return
	}
	if a.onCost != nil {
		c := a.provider.Cost(a.model, usage)
		a.costAccumulated += c.TotalCost
		a.onCost(c.TotalCost)
	}

	// 在丢弃旧消息前归档，便于事后追溯；失败仅告警，不影响压缩流程。
	a.archiveDroppedMessages(start, cut)

	// H22 修复：容量需 +1，因为下面会额外插入一条 summary system 消息；
	// 原公式 len-(cut-start)+start 少算 1，导致 append 时触发一次扩容重新分配。
	rebuilt := make([]provider.Message, 0, len(a.messages)-(cut-start)+start+1)
	// 保留所有前导 system 消息（静态 + 动态），维持 prefix 与运行环境上下文。
	rebuilt = append(rebuilt, a.messages[:start]...)
	rebuilt = append(rebuilt, provider.Message{Role: "system", Content: summary})
	rebuilt = append(rebuilt, a.messages[cut:]...)
	a.messages = rebuilt

	// 压缩改变了 prefix，旧 prefix cache 已失效；重置调度器计时，
	// 使下一次请求建立新的缓存基线。
	if a.cacheScheduler != nil {
		a.cacheScheduler.Reset()
	}
}

const compactionSystemPrompt = `You are a context-compression assistant for a coding agent.
Summarize the conversation segment below into a compact but information-dense note the agent can rely on to keep working.
Preserve: the user's goal, key decisions, files created/modified (with purpose), important findings, commands run and their outcomes, and any unfinished work or next steps.
Drop chit-chat and redundant detail. Output plain text only, no preamble.`

// summarize 调用一次（非流式、无工具）LLM 将旧消息段压缩为文本摘要。
func (a *Agent) summarize(ctx context.Context, msgs []provider.Message) (string, provider.Usage, error) {
	var transcript strings.Builder
	for _, m := range msgs {
		role := m.Role
		content := m.Content
		if len(m.ToolCalls) > 0 {
			names := make([]string, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				names = append(names, tc.Function.Name)
			}
			content = strings.TrimSpace(content + " [called tools: " + strings.Join(names, ", ") + "]")
		}
		if content == "" {
			continue
		}
		transcript.WriteString(role)
		transcript.WriteString(": ")
		transcript.WriteString(content)
		transcript.WriteString("\n")
	}

	resp, err := a.provider.Chat(ctx, &provider.ChatRequest{
		Model: a.model,
		Messages: []provider.Message{
			{Role: "system", Content: compactionSystemPrompt},
			{Role: "user", Content: "Conversation segment to summarize:\n\n" + transcript.String()},
		},
	})
	if err != nil {
		return "", provider.Usage{}, err
	}
	if resp == nil {
		return "[Earlier conversation summary]\n(no response)", provider.Usage{}, nil
	}
	return "[Earlier conversation summary]\n" + resp.Content, resp.Usage, nil
}
