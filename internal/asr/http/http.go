package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"interviewai/internal/audio"
	"interviewai/internal/config"
)

// Transcriber 泛化的 HTTP ASR 接口。
type Transcriber struct {
	client *http.Client
	cfg    config.HTTPASRConfig
	logger *zap.Logger
}

// New 创建 HTTP ASR。
func New(cfg config.HTTPASRConfig, logger *zap.Logger) (*Transcriber, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("http.asr endpoint 不能为空")
	}

	timeout := cfg.Timeout.Duration()
	if timeout == 0 {
		timeout = 12 * time.Second
	}

	return &Transcriber{
		client: &http.Client{Timeout: timeout},
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Recognize 将音频发送至自定义 HTTP 服务。
func (t *Transcriber) Recognize(ctx context.Context, chunk audio.Chunk) (string, error) {
	wavData, err := audio.EncodePCM16ToWAV(chunk.Samples, chunk.SampleRate)
	if err != nil {
		return "", fmt.Errorf("编码 WAV 失败: %w", err)
	}

	payload := map[string]any{
		"audio_base64": base64.StdEncoding.EncodeToString(wavData),
		"sample_rate":  chunk.SampleRate,
		"format":       "wav",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if t.cfg.AuthHeader != "" && t.cfg.AuthValue != "" {
		req.Header.Set(t.cfg.AuthHeader, t.cfg.AuthValue)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("HTTP ASR 返回状态码 %d", resp.StatusCode)
	}

	var decoded any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}

	text, err := extractText(decoded, t.cfg.ResultJSONPath)
	if err != nil {
		return "", err
	}

	return text, nil
}

func extractText(data any, path []string) (string, error) {
	if len(path) == 0 {
		switch v := data.(type) {
		case string:
			return v, nil
		default:
			return "", fmt.Errorf("未提供 JSON 路径且响应不是字符串")
		}
	}

	current := data
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("JSON 路径 %v 无法解析", path)
		}
		next, ok := m[key]
		if !ok {
			return "", fmt.Errorf("JSON 路径 %v 不存在", path)
		}
		current = next
	}

	switch v := current.(type) {
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("JSON 路径 %v 对应值不是字符串", path)
	}
}
