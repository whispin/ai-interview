package llm

import (
	"fmt"

	"go.uber.org/zap"

	"interviewai/internal/config"
)

// Provider 聚合 LLM Streamer 与助手封装。
type Provider struct {
	Assistant *Assistant
	Streamer  Streamer
}

// NewProvider 根据配置创建 LLM 提供商。
func NewProvider(cfg config.LLMSection, logger *zap.Logger) (*Provider, error) {
	providerCfg, ok := cfg.Providers[cfg.Active]
	if !ok {
		return nil, fmt.Errorf("未找到激活的 LLM 提供商: %s", cfg.Active)
	}

	var streamer Streamer
	switch providerCfg.Kind {
	case "", "openai", "openai-compatible":
		streamer = newOpenAIClient(providerCfg, logger.Named("llm"))
	default:
		return nil, fmt.Errorf("仅支持 OpenAI 或兼容接口, 类型: %s", providerCfg.Kind)
	}

	assistant := NewAssistant(streamer, cfg.SystemPrompt)

	return &Provider{Assistant: assistant, Streamer: streamer}, nil
}
