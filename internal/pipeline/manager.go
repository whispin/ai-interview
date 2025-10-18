package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"interviewai/internal/app"
	"interviewai/internal/asr"
	"interviewai/internal/audio"
	"interviewai/internal/config"
	"interviewai/internal/types"
)

// Manager 管理音频 -> ASR 的处理流水线。
type Manager struct {
	cfg         config.Config
	logger      *zap.Logger
	audioEngine *audio.Engine
	transcriber asr.Transcriber
	maxErrors   int

	audioQueue chan audio.Chunk
	results    chan types.ASRResult
	errors     chan error

	stopAudio func() error
	once      sync.Once
}

// NewManager 构建流水线管理器。
func NewManager(container *app.Container, transcriber asr.Transcriber, engine *audio.Engine) *Manager {
	cfg := container.Config
	logger := container.Logger.Named("pipeline")

	queueSize := cfg.Audio.QueueSize
	if queueSize <= 0 {
		queueSize = 4
	}

	return &Manager{
		cfg:         *cfg,
		logger:      logger,
		audioEngine: engine,
		transcriber: transcriber,
		maxErrors:   cfg.Pipeline.MaxConsecutiveErrors,
		audioQueue:  make(chan audio.Chunk, queueSize),
		results:     make(chan types.ASRResult, 16),
		errors:      make(chan error, 8),
	}
}

// Start 启动音频采集和识别。
func (m *Manager) Start(ctx context.Context) error {
	if m.audioEngine == nil {
		return fmt.Errorf("音频引擎未初始化")
	}
	if m.transcriber == nil {
		return fmt.Errorf("ASR 未初始化")
	}

	ctx, cancel := context.WithCancel(ctx)

	stop, err := m.audioEngine.Start(ctx, m.audioQueue)
	if err != nil {
		cancel()
		return err
	}
	m.stopAudio = func() error {
		cancel()
		return stop()
	}

	go m.consume(ctx)
	return nil
}

func (m *Manager) consume(ctx context.Context) {
	consecutiveErrors := 0
	for {
		select {
		case <-ctx.Done():
			return
		case chunk := <-m.audioQueue:
			start := time.Now()
			text, err := m.transcriber.Recognize(ctx, chunk)
			if err != nil {
				consecutiveErrors++
				m.logger.Warn("语音识别失败", zap.Error(err), zap.Int("consecutive", consecutiveErrors))
				select {
				case m.errors <- err:
				default:
				}
				if m.maxErrors > 0 && consecutiveErrors >= m.maxErrors {
					m.logger.Error("连续语音识别失败次数过多，停止流水线")
					select {
					case m.errors <- fmt.Errorf("连续语音识别失败 %d 次", consecutiveErrors):
					default:
					}
					if m.stopAudio != nil {
						_ = m.stopAudio()
					}
					return
				}
				continue
			}

			consecutiveErrors = 0
			if text == "" {
				continue
			}

			result := types.ASRResult{
				Source:    chunk.Source,
				Text:      text,
				Duration:  chunk.Duration,
				Captured:  chunk.CapturedAt,
				Completed: time.Now(),
			}

			m.logger.Debug("识别完成", zap.String("text", result.Text), zap.Duration("latency", time.Since(start)))

			select {
			case m.results <- result:
			default:
				m.logger.Warn("结果队列已满，丢弃识别结果")
			}
		}
	}
}

// Results 返回识别结果通道。
func (m *Manager) Results() <-chan types.ASRResult { return m.results }

// Errors 返回错误通道。
func (m *Manager) Errors() <-chan error { return m.errors }

// Stop 停止流水线。
func (m *Manager) Stop() error {
	var err error
	m.once.Do(func() {
		if m.stopAudio != nil {
			err = m.stopAudio()
		}
		close(m.audioQueue)
		close(m.results)
		close(m.errors)
	})
	return err
}
