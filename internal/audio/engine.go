package audio

import (
	"context"
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
	zap "go.uber.org/zap"

	"interviewai/internal/config"
	"interviewai/internal/types"
)

// CaptureMode 控制需要捕获的音频设备类型。
type CaptureMode int

const (
	CaptureModeBoth CaptureMode = iota
	CaptureModeSpeakerOnly
	CaptureModeMicrophoneOnly
)

// Engine 管理音频上下文和采集线程。
type Engine struct {
	cfg    config.AudioConfig
	logger *zap.Logger
	ctx    *malgo.AllocatedContext
	mode   CaptureMode

	speaker *Device
	micro   *Device
}

// NewEngine 创建基于 miniaudio 的音频引擎。
func NewEngine(cfg config.AudioConfig, mode CaptureMode, logger *zap.Logger) (*Engine, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("初始化音频上下文失败: %w", err)
	}

	speaker, micro, err := discoverDevices(ctx, cfg, mode, logger)
	if err != nil {
		ctx.Free()
		return nil, err
	}

	return &Engine{
		cfg:     cfg,
		logger:  logger,
		ctx:     ctx,
		mode:    mode,
		speaker: speaker,
		micro:   micro,
	}, nil
}

// Start 启动音频采集并返回停止函数。
func (e *Engine) Start(ctx context.Context, out chan<- Chunk) (func() error, error) {
	needSpeaker := e.mode != CaptureModeMicrophoneOnly
	needMicro := e.mode != CaptureModeSpeakerOnly

	captureSpeaker := needSpeaker && e.speaker != nil
	captureMicro := needMicro && e.micro != nil

	if needSpeaker && !captureSpeaker {
		e.logger.Warn("未找到扬声器捕获设备，将跳过扬声器音频")
	}
	if needMicro && !captureMicro {
		e.logger.Warn("未找到麦克风捕获设备，将跳过麦克风音频")
	}

	if !captureSpeaker && !captureMicro {
		if e.ctx != nil {
			e.ctx.Free()
			e.ctx = nil
		}
		return nil, fmt.Errorf("当前捕获设置未检测到可用音频设备")
	}

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var recorders []*Recorder

	if captureSpeaker {
		speakerRecorder, err := NewRecorder(e.ctx, e.speaker, types.SpeechSourceSpeaker, e.cfg, e.logger.Named("speaker"))
		if err != nil {
			cancel()
			if e.ctx != nil {
				e.ctx.Free()
				e.ctx = nil
			}
			return nil, err
		}
		recorders = append(recorders, speakerRecorder)
	}

	if captureMicro {
		micRecorder, err := NewRecorder(e.ctx, e.micro, types.SpeechSourceMicrophone, e.cfg, e.logger.Named("microphone"))
		if err != nil {
			cancel()
			for _, r := range recorders {
				r.Close()
			}
			if e.ctx != nil {
				e.ctx.Free()
				e.ctx = nil
			}
			return nil, err
		}
		recorders = append(recorders, micRecorder)
	}

	for _, rec := range recorders {
		wg.Add(1)
		go func(r *Recorder) {
			defer wg.Done()
			if err := r.Run(ctx, out); err != nil {
				e.logger.Error("音频采集失败", zap.String("source", string(r.source)), zap.Error(err))
				cancel()
			}
		}(rec)
	}

	stop := func() error {
		cancel()
		wg.Wait()
		for _, rec := range recorders {
			rec.Close()
		}
		if e.ctx != nil {
			e.ctx.Free()
			e.ctx = nil
		}
		return nil
	}

	return stop, nil
}

// SpeakerDevice 返回扬声器设备。
func (e *Engine) SpeakerDevice() *Device { return e.speaker }

// MicrophoneDevice 返回麦克风设备。
func (e *Engine) MicrophoneDevice() *Device { return e.micro }
