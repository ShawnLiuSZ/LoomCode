package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// EventType 事件类型
type EventType int

const (
	// EventToolCall 工具调用
	EventToolCall EventType = iota
	// EventToolResult 工具结果
	EventToolResult
	// EventError 错误
	EventError
	// EventCost 成本
	EventCost
	// EventCacheHit 缓存命中
	EventCacheHit
	// EventMessage 消息
	EventMessage
	// EventGoalCheck Goal 检查
	EventGoalCheck
)

// String 返回事件类型字符串
func (e EventType) String() string {
	switch e {
	case EventToolCall:
		return "tool_call"
	case EventToolResult:
		return "tool_result"
	case EventError:
		return "error"
	case EventCost:
		return "cost"
	case EventCacheHit:
		return "cache_hit"
	case EventMessage:
		return "message"
	case EventGoalCheck:
		return "goal_check"
	default:
		return "unknown"
	}
}

// Event 事件
type Event struct {
	Timestamp time.Time         `json:"timestamp"`
	Type      EventType         `json:"type"`
	Message   string            `json:"message"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
}

// EventLog 事件日志
type EventLog struct {
	mu     sync.Mutex
	events []Event
	maxSize int
}

// NewEventLog 创建事件日志
func NewEventLog(maxSize int) *EventLog {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &EventLog{
		events:  make([]Event, 0),
		maxSize: maxSize,
	}
}

// Log 记录事件
func (l *EventLog) Log(eventType EventType, message string, metadata map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	event := Event{
		Timestamp: time.Now(),
		Type:      eventType,
		Message:   message,
		Metadata:  metadata,
	}

	// 如果超过最大大小，移除最旧的事件
	if len(l.events) >= l.maxSize {
		l.events = l.events[1:]
	}

	l.events = append(l.events, event)
}

// LogToolCall 记录工具调用事件
func (l *EventLog) LogToolCall(toolName string, args map[string]any) {
	l.Log(EventToolCall, fmt.Sprintf("calling %s", toolName), map[string]any{
		"tool": toolName,
		"args": args,
	})
}

// LogToolResult 记录工具结果事件
func (l *EventLog) LogToolResult(toolName string, result string, err error) {
	metadata := map[string]any{
		"tool":   toolName,
		"result": result,
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	l.Log(EventToolResult, fmt.Sprintf("result from %s", toolName), metadata)
}

// LogError 记录错误事件
func (l *EventLog) LogError(err error) {
	if err == nil {
		return
	}
	l.Log(EventError, err.Error(), map[string]any{
		"error": err.Error(),
	})
}

// LogCost 记录成本事件
func (l *EventLog) LogCost(inputTokens, outputTokens int, cost float64) {
	l.Log(EventCost, fmt.Sprintf("cost: $%.4f", cost), map[string]any{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"cost":          cost,
	})
}

// LogCacheHit 记录缓存命中事件
func (l *EventLog) LogCacheHit(tokens int) {
	l.Log(EventCacheHit, fmt.Sprintf("cache hit: %d tokens", tokens), map[string]any{
		"tokens": tokens,
	})
}

// LogMessage 记录消息事件
func (l *EventLog) LogMessage(role, content string) {
	l.Log(EventMessage, fmt.Sprintf("%s: %s", role, truncateStr(content, 100)), map[string]any{
		"role":    role,
		"content": content,
	})
}

// LogGoalCheck 记录 Goal 检查事件
func (l *EventLog) LogGoalCheck(achieved bool, reason string) {
	l.Log(EventGoalCheck, fmt.Sprintf("goal check: %v - %s", achieved, reason), map[string]any{
		"achieved": achieved,
		"reason":   reason,
	})
}

// Events 获取所有事件
func (l *EventLog) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := make([]Event, len(l.events))
	copy(result, l.events)
	return result
}

// EventsByType 按类型获取事件
func (l *EventLog) EventsByType(eventType EventType) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []Event
	for _, event := range l.events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result
}

// Recent 获取最近 N 个事件
func (l *EventLog) Recent(n int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n >= len(l.events) {
		result := make([]Event, len(l.events))
		copy(result, l.events)
		return result
	}

	result := make([]Event, n)
	copy(result, l.events[len(l.events)-n:])
	return result
}

// Clear 清除所有事件
func (l *EventLog) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = make([]Event, 0)
}

// Size 返回事件数量
func (l *EventLog) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.events)
}

// Save 保存事件日志到文件
func (l *EventLog) Save(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.MarshalIndent(l.events, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load 从文件加载事件日志
func (l *EventLog) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return json.Unmarshal(data, &l.events)
}

// truncateStr 截断字符串（按 rune 边界）
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
