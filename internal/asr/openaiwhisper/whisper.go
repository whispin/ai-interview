package openaiwhisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"interviewai/internal/audio"
	"interviewai/internal/config"
)

// Transcriber 使用 OpenAI Whisper 实现语音识别。
type Transcriber struct {
	client *http.Client
	cfg    config.OpenAIWhisperConfig
	logger *zap.Logger
}

// New 创建 Whisper 转写器。
func New(cfg config.OpenAIWhisperConfig, logger *zap.Logger) (*Transcriber, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai whisper 未配置 api_key")
	}

	timeout := cfg.Timeout.Duration()
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Transcriber{
		client: &http.Client{Timeout: timeout},
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Recognize 调用 OpenAI Whisper API 进行识别。
func (t *Transcriber) Recognize(ctx context.Context, chunk audio.Chunk) (string, error) {
	wavData, err := audio.EncodePCM16ToWAV(chunk.Samples, chunk.SampleRate)
	if err != nil {
		return "", fmt.Errorf("编码 WAV 失败: %w", err)
	}

	endpoint := strings.TrimRight(t.cfg.BaseURL, "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = endpoint + "/audio/transcriptions"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fmt.Sprintf("chunk-%d.wav", time.Now().UnixNano()))
	if err != nil {
		return "", err
	}
	if _, err = part.Write(wavData); err != nil {
		return "", err
	}

	model := t.cfg.Model
	if model == "" {
		model = "whisper-1"
	}
	if err = writer.WriteField("model", model); err != nil {
		return "", err
	}

	if t.cfg.Language != "" {
		if err = writer.WriteField("language", t.cfg.Language); err != nil {
			return "", err
		}
	}

	if t.cfg.Temperature != 0 {
		if err = writer.WriteField("temperature", fmt.Sprintf("%f", t.cfg.Temperature)); err != nil {
			return "", err
		}
	}

	if err = writer.WriteField("response_format", "json"); err != nil {
		return "", err
	}

	if err = writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+t.cfg.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		var buf bytes.Buffer
		if _, copyErr := buf.ReadFrom(resp.Body); copyErr == nil {
			return "", fmt.Errorf("whisper 请求失败: %d %s", resp.StatusCode, strings.TrimSpace(buf.String()))
		}
		return "", fmt.Errorf("whisper 请求失败: %d", resp.StatusCode)
	}

	var decoded whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("解析 whisper 响应失败: %w", err)
	}

	text := strings.TrimSpace(decoded.Text)
	return text, nil
}

type whisperResponse struct {
	Text string `json:"text"`
}
