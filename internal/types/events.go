package types

import "time"

// SpeechSource 枚举音频来源。
type SpeechSource string

const (
	SpeechSourceSpeaker    SpeechSource = "speaker"
	SpeechSourceMicrophone SpeechSource = "microphone"
)

// ASRResult 表示一次语音识别结果。
type ASRResult struct {
	Source    SpeechSource
	Text      string
	Duration  time.Duration
	Captured  time.Time
	Completed time.Time
}

// LLMChunk 表示 LLM 的流式输出片段。
type LLMChunk struct {
	Text      string
	Done      bool
	Completed bool
}

// ErrorEvent 表示处理过程中的错误。
type ErrorEvent struct {
	Component string
	Err       error
	Time      time.Time
}
