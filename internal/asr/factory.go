package asr

import (
	"fmt"

	"go.uber.org/zap"

	"interviewai/internal/asr/http"
	"interviewai/internal/asr/openaiwhisper"
	"interviewai/internal/asr/tencent"
	"interviewai/internal/config"
)

// New 根据配置创建合适的 Transcriber。
func New(cfg config.ASRSection, logger *zap.Logger) (Transcriber, error) {
	switch cfg.Active {
	case "tencent":
		return tencent.New(cfg.Tencent, logger)
	case "http":
		return http.New(cfg.HTTP, logger)
	case "openai_whisper":
		return openaiwhisper.New(cfg.OpenAIWhisper, logger)
	default:
		return nil, fmt.Errorf("未知的 ASR 提供商: %s", cfg.Active)
	}
}
