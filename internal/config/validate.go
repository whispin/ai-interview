package config

import (
	"errors"
	"fmt"
)

// Validate 检查配置合法性。
func (c *Config) Validate() error {
	if c.ASR.Active == "" {
		return errors.New("asr.active 未配置")
	}
	if c.LLM.Active == "" {
		return errors.New("llm.active 未配置")
	}

	if _, ok := c.LLM.Providers[c.LLM.Active]; !ok {
		return fmt.Errorf("llm.active 指向未知提供商: %s", c.LLM.Active)
	}

	provider := c.LLM.Providers[c.LLM.Active]
	switch provider.Kind {
	case "", "openai", "openai-compatible":
		if provider.APIKey == "" {
			return errors.New("LLM 提供商需配置 api_key")
		}
	default:
		return fmt.Errorf("仅支持 OpenAI 或兼容接口, provider.kind=%s", provider.Kind)
	}

	switch c.ASR.Active {
	case "tencent":
		if c.ASR.Tencent.SecretID == "" || c.ASR.Tencent.SecretKey == "" {
			return errors.New("使用腾讯云 ASR 时必须配置 secret_id/secret_key")
		}
	case "http":
		if c.ASR.HTTP.Endpoint == "" {
			return errors.New("使用 HTTP ASR 时必须提供 endpoint")
		}
	case "openai_whisper":
		if c.ASR.OpenAIWhisper.APIKey == "" {
			return errors.New("使用 OpenAI Whisper 时必须配置 api_key")
		}
	default:
		return fmt.Errorf("不支持的 ASR 类型: %s", c.ASR.Active)
	}

	if c.Audio.SampleRate <= 0 {
		return errors.New("audio.sample_rate 必须为正数")
	}
	if c.Audio.QueueSize <= 0 {
		return errors.New("audio.queue_size 必须为正数")
	}
	if c.Audio.SilenceThreshold < 0 {
		return errors.New("audio.silence_threshold 不能为负数")
	}
	if c.Pipeline.MaxConsecutiveErrors <= 0 {
		return errors.New("pipeline.max_consecutive_errors 必须大于0")
	}

	return nil
}
