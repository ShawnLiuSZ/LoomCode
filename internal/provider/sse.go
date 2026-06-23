package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxSSEReaderBufSize SSE 读缓冲区最大 10MB
const maxSSEReaderBufSize = 10 * 1024 * 1024

// SSEReader SSE 事件读取器
type SSEReader struct {
	reader io.Reader
	buf    []byte
	tmp    []byte // 复用的临时缓冲
}

// NewSSEReader 创建 SSE 事件读取器
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: r,
		buf:    make([]byte, 0, 4096),
		tmp:    make([]byte, 4096),
	}
}

// Read 读取下一个 SSE 事件的数据部分
func (s *SSEReader) Read() ([]byte, error) {
	for {
		n, err := s.reader.Read(s.tmp)
		if n > 0 {
			s.buf = append(s.buf, s.tmp[:n]...)
		}

		// 检查缓冲区大小，防止 OOM
		if len(s.buf) > maxSSEReaderBufSize {
			s.buf = s.buf[:0]
			return nil, fmt.Errorf("SSE buffer exceeded maximum size (%d bytes)", maxSSEReaderBufSize)
		}

		for {
			idx := bytes.Index(s.buf, []byte("\n\n"))
			if idx == -1 {
				break
			}

			line := s.buf[:idx]
			s.buf = s.buf[idx+2:]

			if len(line) == 0 {
				continue
			}

			data, ok := ExtractSSEData(line)
			if ok {
				return data, nil
			}
		}

		if err != nil {
			if err == io.EOF && len(s.buf) == 0 {
				return nil, err
			}
			if err != io.EOF {
				return nil, err
			}
			if len(s.buf) > 0 {
				remaining := make([]byte, len(s.buf))
				copy(remaining, s.buf)
				s.buf = s.buf[:0]
				return remaining, nil
			}
			return nil, io.EOF
		}
	}
}

// ExtractSSEData 从 SSE 行中提取 data 部分
func ExtractSSEData(line []byte) ([]byte, bool) {
	if data, ok := bytes.CutPrefix(line, []byte("data: ")); ok {
		return data, true
	}
	if data, ok := bytes.CutPrefix(line, []byte("data:")); ok {
		return data, true
	}
	return nil, false
}

// IsSSEDone 检查是否为 [DONE] 信号
func IsSSEDone(data []byte) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]"))
}

// IsSSECommentOrEmpty 检查是否为 SSE 注释或空行
func IsSSECommentOrEmpty(line []byte) bool {
	return len(line) == 0 || line[0] == ':'
}

// SSELineReader 基于行的 SSE 读取器（用于需要逐行处理的场景）
type SSELineReader struct {
	scanner *SSEReader
}

// NewSSELineReader 创建基于行的 SSE 读取器
func NewSSELineReader(r io.Reader) *SSELineReader {
	return &SSELineReader{scanner: NewSSEReader(r)}
}

// ReadLine 读取下一个 data 行
func (r *SSELineReader) ReadLine() ([]byte, error) {
	return r.scanner.Read()
}

// ChunkHandler 解析 SSE chunk 的回调函数
// 返回 content, toolCalls, usage, extraData, error
type ChunkHandler func(data []byte) (content string, toolCalls []ToolCallDelta, usage *Usage, extra map[string]any, done bool, err error)

// ReadSSEStream 通用 SSE 流读取器
// ctx: 上下文（用于取消）
// resp: HTTP 响应
// ch: 事件输出通道
// handler: chunk 解析回调
func ReadSSEStream(ctx context.Context, resp *http.Response, ch chan<- StreamEvent, handler ChunkHandler) {
	defer close(ch)
	defer resp.Body.Close()

	reader := NewSSEReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			ch <- StreamEvent{Type: EventError, Content: ctx.Err().Error()}
			return
		default:
		}

		data, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				ch <- StreamEvent{Type: EventError, Content: err.Error()}
			}
			return
		}

		if IsSSEDone(data) {
			ch <- StreamEvent{Type: EventDone}
			return
		}

		content, toolCalls, usage, extra, done, err := handler(data)
		if err != nil {
			ch <- StreamEvent{Type: EventError, Content: err.Error()}
			return
		}

		if done {
			return
		}

		if content != "" {
			ch <- StreamEvent{Type: EventText, Content: content}
		}

		// 处理 extra 字段（如 DeepSeek reasoning_content）
		if extra != nil {
			if reasoningContent, ok := extra["reasoning_content"].(string); ok && reasoningContent != "" {
				ch <- StreamEvent{Type: EventText, ReasoningContent: reasoningContent}
			}
		}

		for _, tc := range toolCalls {
			ch <- StreamEvent{Type: EventToolCall, ToolCall: &tc}
		}

		if usage != nil {
			ch <- StreamEvent{Type: EventDone, Usage: usage}
		}
	}
}

// ParseStandardChunk 解析标准 OpenAI 格式的 SSE chunk
// 支持 content, tool_calls, usage
func ParseStandardChunk(data []byte) (content string, toolCalls []ToolCallDelta, usage *Usage, err error) {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *Usage `json:"usage"`
	}

	if err := json.Unmarshal(data, &chunk); err != nil {
		return "", nil, nil, fmt.Errorf("parse SSE chunk: %w", err)
	}

	if len(chunk.Choices) > 0 {
		delta := chunk.Choices[0].Delta
		content = delta.Content
		for _, tc := range delta.ToolCalls {
			toolCalls = append(toolCalls, ToolCallDelta{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	return content, toolCalls, chunk.Usage, nil
}

// TrimSSELine 去除 SSE 行的空格和换行
func TrimSSELine(line string) string {
	return strings.TrimSpace(line)
}
