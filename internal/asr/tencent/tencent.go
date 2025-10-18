package tencent

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	tencentasr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/asr/v20190614"
	common "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	profile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"go.uber.org/zap"

	"interviewai/internal/audio"
	"interviewai/internal/config"
)

// Transcriber 腾讯云实现。
type Transcriber struct {
	client  *tencentasr.Client
	cfg     config.TencentConfig
	logger  *zap.Logger
	mu      sync.Mutex
	timeout time.Duration
}

// New 创建腾讯云 ASR。
func New(cfg config.TencentConfig, logger *zap.Logger) (*Transcriber, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("缺少腾讯云凭证配置")
	}

	cred := common.NewCredential(cfg.SecretID, cfg.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "asr.tencentcloudapi.com"

	client, err := tencentasr.NewClient(cred, cfg.Region, cpf)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云客户端失败: %w", err)
	}

	timeout := 15 * time.Second

	return &Transcriber{
		client:  client,
		cfg:     cfg,
		logger:  logger,
		timeout: timeout,
	}, nil
}

// Recognize 调用腾讯云一句话识别。
func (t *Transcriber) Recognize(ctx context.Context, chunk audio.Chunk) (string, error) {
	wavData, err := audio.EncodePCM16ToWAV(chunk.Samples, chunk.SampleRate)
	if err != nil {
		return "", fmt.Errorf("编码 WAV 失败: %w", err)
	}

	req := tencentasr.NewSentenceRecognitionRequest()
	req.ProjectId = common.Uint64Ptr(0)
	req.SubServiceType = common.Uint64Ptr(2)
	req.EngSerViceType = common.StringPtr(t.cfg.EngineModelType)
	req.SourceType = common.Uint64Ptr(1)
	req.VoiceFormat = common.StringPtr("wav")
	req.UsrAudioKey = common.StringPtr(fmt.Sprintf("session_%d", time.Now().UnixNano()))
	req.Data = common.StringPtr(base64.StdEncoding.EncodeToString(wavData))

	ctxWithTimeout, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	t.mu.Lock()
	resp, err := t.client.SentenceRecognitionWithContext(ctxWithTimeout, req)
	t.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("腾讯云识别失败: %w", err)
	}

	if resp == nil || resp.Response == nil || resp.Response.Result == nil {
		return "", fmt.Errorf("腾讯云返回结果为空")
	}

	text := resp.Response.Result
	if text == nil {
		return "", nil
	}

	return *text, nil
}
