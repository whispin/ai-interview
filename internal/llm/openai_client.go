package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"interviewai/internal/config"
)

// openAIClient 实现 OpenAI 兼容协议。
type openAIClient struct {
	httpClient *http.Client
	cfg        config.LLMProviderConfig
	logger     *zap.Logger
}

func newOpenAIClient(cfg config.LLMProviderConfig, logger *zap.Logger) *openAIClient {
	timeout := 60 * time.Second
	if cfg.MaxTokens > 0 && timeout < 90*time.Second {
		timeout = 90 * time.Second
	}

	return &openAIClient{
		httpClient: &http.Client{Timeout: timeout},
		cfg:        cfg,
		logger:     logger,
	}
}

func (c *openAIClient) Stream(ctx context.Context, messages []Message, handler StreamHandler) error {
	if c.cfg.APIKey == "" {
		err := errors.New("未配置 LLM API Key")
		c.logger.Error("LLM 配置错误", zap.Error(err))
		return err
	}

	endpoint := strings.TrimRight(c.cfg.BaseURL, "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint += "/chat/completions"

	payload := map[string]any{
		"model":       c.cfg.Model,
		"messages":    messages,
		"stream":      true,
		"temperature": c.cfg.Temperature,
	}
	if c.cfg.MaxTokens > 0 {
		payload["max_tokens"] = c.cfg.MaxTokens
	}

	body, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error("LLM 请求构建失败", zap.Error(err))
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("创建 LLM 请求失败", zap.Error(err))
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.cfg.APIKey))
	for _, header := range c.cfg.Headers {
		if header.Key != "" {
			req.Header.Set(header.Key, header.Value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("LLM 请求失败", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("LLM 请求失败，状态码 %d", resp.StatusCode)
		c.logger.Error("LLM 请求失败", zap.Error(err))
		return err
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				c.logger.Warn("LLM 流式被取消", zap.Error(err))
				return err
			}
			c.logger.Error("读取 LLM 流式数据失败", zap.Error(err))
			break
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			if handler != nil {
				if err := handler("", true); err != nil {
					c.logger.Error("LLM 响应处理失败", zap.Error(err))
					return err
				}
			}
			return nil
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			c.logger.Warn("解析流式响应失败", zap.Error(err))
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		if handler != nil {
			if err := handler(delta, false); err != nil {
				c.logger.Error("LLM 响应处理失败", zap.Error(err))
				return err
			}
		}
	}

	if handler != nil {
		if err := handler("", true); err != nil {
			c.logger.Error("LLM 响应处理失败", zap.Error(err))
			return err
		}
	}

	return nil
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}
