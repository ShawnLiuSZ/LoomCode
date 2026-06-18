package voice

import (
	"testing"
	"time"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateIdle, "idle"},
		{StateListening, "listening"},
		{StateProcessing, "processing"},
		{StateError, "error"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, tt.state.String(), tt.want)
		}
	}
}

func TestDefaultFormat(t *testing.T) {
	f := DefaultFormat()
	if f.SampleRate != 16000 {
		t.Errorf("SampleRate = %d, want 16000", f.SampleRate)
	}
	if f.Channels != 1 {
		t.Errorf("Channels = %d, want 1", f.Channels)
	}
}

func TestInput_StateTransitions(t *testing.T) {
	asr := NewMockASR("hello")
	in := NewInput(asr)

	if in.State() != StateIdle {
		t.Errorf("initial State() = %v, want idle", in.State())
	}

	in.Start()
	if in.State() != StateListening {
		t.Errorf("State() after Start = %v, want listening", in.State())
	}

	result := in.Stop()
	if result != "" {
		t.Errorf("Stop() result = %q, want empty (no feed yet)", result)
	}
	if in.State() != StateIdle {
		t.Errorf("State() after Stop = %v, want idle", in.State())
	}
}

func TestInput_Feed(t *testing.T) {
	asr := NewMockASR("hello world")
	in := NewInput(asr)

	in.Start()
	in.Feed([]byte("audio data"))
	time.Sleep(50 * time.Millisecond) // 等待异步处理

	// 检查结果
	select {
	case result := <-in.ResultChan():
		if result.Text != "hello world" {
			t.Errorf("result.Text = %q", result.Text)
		}
		if !result.IsFinal {
			t.Error("result should be final")
		}
	default:
		t.Error("expected result on channel")
	}
}

func TestInput_StopWithBuffer(t *testing.T) {
	asr := NewMockASR("recognized text")
	in := NewInput(asr)

	in.Start()
	in.Feed([]byte("audio"))
	time.Sleep(50 * time.Millisecond)

	// 消耗结果通道
	<-in.ResultChan()

	result := in.Stop()
	if result != "recognized text" {
		t.Errorf("Stop() result = %q", result)
	}
}

func TestInput_FeedWhenIdle(t *testing.T) {
	asr := NewMockASR("should not appear")
	in := NewInput(asr)

	// 不调用 Start，直接 Feed
	in.Feed([]byte("audio"))
	time.Sleep(30 * time.Millisecond)

	select {
	case <-in.ResultChan():
		t.Error("should not receive result when idle")
	default:
		// 预期行为
	}
}

func TestMockASR_Recognize(t *testing.T) {
	asr := NewMockASR("first", "second", "third")

	r1, _ := asr.Recognize(t.Context(), nil, DefaultFormat())
	if r1.Text != "first" {
		t.Errorf("r1 = %q", r1.Text)
	}

	r2, _ := asr.Recognize(t.Context(), nil, DefaultFormat())
	if r2.Text != "second" {
		t.Errorf("r2 = %q", r2.Text)
	}

	r3, _ := asr.Recognize(t.Context(), nil, DefaultFormat())
	if r3.Text != "third" {
		t.Errorf("r3 = %q", r3.Text)
	}

	// 超过预设数量
	r4, _ := asr.Recognize(t.Context(), nil, DefaultFormat())
	if r4.Text != "" {
		t.Errorf("r4 should be empty, got %q", r4.Text)
	}
}

func TestMockASR_StreamRecognize(t *testing.T) {
	asr := NewMockASR("chunk1", "chunk2")
	audioCh := make(chan []byte, 2)
	audioCh <- []byte("a")
	audioCh <- []byte("b")
	close(audioCh)

	ch, err := asr.StreamRecognize(t.Context(), audioCh, DefaultFormat())
	if err != nil {
		t.Fatalf("StreamRecognize error: %v", err)
	}

	var results []string
	for r := range ch {
		results = append(results, r.Text)
	}

	if len(results) != 2 {
		t.Errorf("results count = %d, want 2", len(results))
	}
}

func TestMiMoASR_Recognize(t *testing.T) {
	asr := NewMiMoASR("https://api.mimo.xiaomi.com", "test-key")

	result, err := asr.Recognize(t.Context(), []byte("audio"), DefaultFormat())
	if err != nil {
		t.Fatalf("Recognize error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.IsFinal {
		t.Error("result should be final")
	}
}

func TestInput_Buffer(t *testing.T) {
	asr := NewMockASR("partial result")
	in := NewInput(asr)

	if in.Buffer() != "" {
		t.Error("buffer should be empty initially")
	}
}
