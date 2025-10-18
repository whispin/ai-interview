package audio

import (
	"time"

	"interviewai/internal/types"
)

// Chunk 表示经过 VAD 切分后的音频段。
type Chunk struct {
	Source     types.SpeechSource
	Samples    []int16
	SampleRate int
	CapturedAt time.Time
	Duration   time.Duration
}

// Copy 创建深拷贝，避免共享底层数组。
func (c Chunk) Copy() Chunk {
	dup := make([]int16, len(c.Samples))
	copy(dup, c.Samples)
	c.Samples = dup
	return c
}
