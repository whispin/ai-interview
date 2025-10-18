package asr

import (
	"context"

	"interviewai/internal/audio"
	"interviewai/internal/config"
)

// Transcriber 抽象语音识别实现。
type Transcriber interface {
	Recognize(ctx context.Context, chunk audio.Chunk) (string, error)
}

// Factory 根据配置创建 Transcriber。
type Factory interface {
	NewTranscriber(cfg config.Config) (Transcriber, error)
}
