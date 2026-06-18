package voice

import (
	"context"
	"sync"
)

// State 语音输入状态
type State int

const (
	StateIdle       State = iota
	StateListening
	StateProcessing
	StateError
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateListening:
		return "listening"
	case StateProcessing:
		return "processing"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ASRResult 语音识别结果
type ASRResult struct {
	Text       string
	Confidence float64
	IsFinal    bool
}

// ASRProvider 语音识别提供者接口
type ASRProvider interface {
	// Recognize 将音频数据转为文本
	Recognize(ctx context.Context, audioData []byte, format AudioFormat) (*ASRResult, error)

	// StreamRecognize 流式识别
	StreamRecognize(ctx context.Context, audioCh <-chan []byte, format AudioFormat) (<-chan *ASRResult, error)
}

// AudioFormat 音频格式
type AudioFormat struct {
	SampleRate int    // 采样率 (e.g. 16000)
	Channels   int    // 声道数
	Encoding   string // 编码格式 (e.g. "pcm_s16le", "wav")
}

// MiMoASR MiMo 语音识别适配器
type MiMoASR struct {
	apiKey  string
	baseURL string
}

// NewMiMoASR 创建 MiMo ASR 适配器
func NewMiMoASR(baseURL, apiKey string) *MiMoASR {
	return &MiMoASR{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Recognize 调用 MiMo ASR 进行识别
func (m *MiMoASR) Recognize(ctx context.Context, audioData []byte, format AudioFormat) (*ASRResult, error) {
	// TODO: 实现 MiMo ASR HTTP API 调用
	return &ASRResult{
		Text:    "[MiMo ASR integration pending]",
		IsFinal: true,
	}, nil
}

// StreamRecognize 流式识别
func (m *MiMoASR) StreamRecognize(ctx context.Context, audioCh <-chan []byte, format AudioFormat) (<-chan *ASRResult, error) {
	resultCh := make(chan *ASRResult, 10)

	go func() {
		defer close(resultCh)
		for audio := range audioCh {
			select {
			case <-ctx.Done():
				return
			default:
				result, _ := m.Recognize(ctx, audio, format)
				if result != nil {
					resultCh <- result
				}
			}
		}
	}()

	return resultCh, nil
}

// Input 语音输入控制器
type Input struct {
	mu     sync.Mutex
	state  State
	asr    ASRProvider
	format AudioFormat

	// 结果缓冲
	buffer     string
	finalText  string
	resultCh   chan *ASRResult
}

// NewInput 创建语音输入控制器
func NewInput(asr ASRProvider) *Input {
	return &Input{
		state:    StateIdle,
		asr:      asr,
		format:   DefaultFormat(),
		resultCh: make(chan *ASRResult, 100),
	}
}

// DefaultFormat 默认音频格式
func DefaultFormat() AudioFormat {
	return AudioFormat{
		SampleRate: 16000,
		Channels:   1,
		Encoding:   "pcm_s16le",
	}
}

// State 返回当前状态
func (in *Input) State() State {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.state
}

// SetState 设置状态
func (in *Input) setState(s State) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.state = s
}

// Start 开始语音输入
func (in *Input) Start() error {
	in.setState(StateListening)
	return nil
}

// Stop 停止语音输入
func (in *Input) Stop() string {
	in.setState(StateProcessing)

	// 返回已缓冲的文本
	in.mu.Lock()
	result := in.finalText
	if result == "" {
		result = in.buffer
	}
	in.buffer = ""
	in.finalText = ""
	in.mu.Unlock()

	in.setState(StateIdle)
	return result
}

// Feed 喂入音频数据（由音频采集器调用）
func (in *Input) Feed(audioData []byte) {
	in.mu.Lock()
	state := in.state
	in.mu.Unlock()

	if state != StateListening {
		return
	}

	go func() {
		result, err := in.asr.Recognize(context.Background(), audioData, in.format)
		if err != nil {
			in.setState(StateError)
			return
		}

		in.mu.Lock()
		defer in.mu.Unlock()

		if result.IsFinal {
			in.finalText = result.Text
		} else {
			in.buffer = result.Text
		}

		select {
		case in.resultCh <- result:
		default:
		}
	}()
}

// ResultChan 返回识别结果通道
func (in *Input) ResultChan() <-chan *ASRResult {
	return in.resultCh
}

// Buffer 返回当前缓冲文本
func (in *Input) Buffer() string {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.buffer
}

// MockASR 模拟 ASR 提供者（测试用）
type MockASR struct {
	results []string
	index   int
}

// NewMockASR 创建模拟 ASR
func NewMockASR(results ...string) *MockASR {
	return &MockASR{results: results}
}

// Recognize 返回预设结果
func (m *MockASR) Recognize(ctx context.Context, audioData []byte, format AudioFormat) (*ASRResult, error) {
	if m.index >= len(m.results) {
		return &ASRResult{Text: "", IsFinal: true}, nil
	}
	text := m.results[m.index]
	m.index++
	return &ASRResult{Text: text, IsFinal: true, Confidence: 1.0}, nil
}

// StreamRecognize 流式返回预设结果
func (m *MockASR) StreamRecognize(ctx context.Context, audioCh <-chan []byte, format AudioFormat) (<-chan *ASRResult, error) {
	resultCh := make(chan *ASRResult, len(m.results))

	go func() {
		defer close(resultCh)
		for range audioCh {
			if m.index >= len(m.results) {
				return
			}
			resultCh <- &ASRResult{
				Text:    m.results[m.index],
				IsFinal: m.index == len(m.results)-1,
			}
			m.index++
		}
	}()

	return resultCh, nil
}
