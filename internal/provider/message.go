package provider

// Message 对话消息
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // DeepSeek thinking 模式
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest 对话请求
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

// ToolDef 工具定义（OpenAI tool 格式）
type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	Content          string
	ReasoningContent string // DeepSeek thinking 模式
	ToolCalls        []ToolCall
	Usage            Usage
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function ToolCallFunc   `json:"function"`
	Args     map[string]any `json:"-"` // 内部使用，不序列化
}

// ToolCallFunc 工具调用函数
type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type             StreamEventType
	Content          string
	ReasoningContent string // DeepSeek thinking 模式
	ToolCall         *ToolCallDelta
	Usage            *Usage
}

// StreamEventType 流式事件类型
type StreamEventType int

const (
	EventText StreamEventType = iota
	EventToolCall
	EventDone
	EventError
)

// ToolCallDelta 流式工具调用增量
type ToolCallDelta struct {
	Index     int // 同一轮中第几个 tool call（OpenAI/DeepSeek 流式格式使用）
	ID        string
	Name      string
	Arguments string // JSON 片段，需要累积
}
