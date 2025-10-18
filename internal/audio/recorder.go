package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
	zap "go.uber.org/zap"

	"interviewai/internal/config"
	"interviewai/internal/types"
)

// Recorder 封装单个输入设备的捕获与 VAD 逻辑。
type Recorder struct {
	ctx        *malgo.AllocatedContext
	device     *malgo.Device
	deviceInfo *Device

	source types.SpeechSource
	cfg    config.AudioConfig
	logger *zap.Logger

	sampleRateIn int
	targetRate   int
	channels     int
	chunkQueue   chan []int16
	dataCallback malgo.DataProc
	stopOnce     sync.Once

	speaking       bool
	buffer         []int16
	bufferDuration float64
	silenceSeconds float64
	startTimestamp time.Time
}

// NewRecorder 创建 Recorder。
func NewRecorder(ctx *malgo.AllocatedContext, device *Device, source types.SpeechSource, cfg config.AudioConfig, logger *zap.Logger) (*Recorder, error) {
	if ctx == nil {
		return nil, fmt.Errorf("音频上下文未初始化")
	}

	channels := cfg.DefaultChannels
	if channels <= 0 {
		channels = 1
	}

	targetRate := cfg.SampleRate
	if targetRate <= 0 {
		targetRate = 16000
	}

	chunkQueueSize := cfg.QueueSize
	if chunkQueueSize <= 0 {
		chunkQueueSize = 4
	}

	rec := &Recorder{
		ctx:        ctx,
		deviceInfo: device,
		source:     source,
		cfg:        cfg,
		logger:     logger,
		targetRate: targetRate,
		channels:   channels,
		chunkQueue: make(chan []int16, chunkQueueSize),
		buffer:     make([]int16, 0, targetRate),
	}

	rec.dataCallback = func(_, input []byte, frameCount uint32) {
		rec.onInput(input, int(frameCount))
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(channels)
	if device != nil {
		id := device.ID
		deviceConfig.Capture.DeviceID = id.Pointer()
	}

	sampleRate := targetRate
	if device != nil && device.SampleRate != 0 {
		sampleRate = int(device.SampleRate)
	}
	if sampleRate <= 0 {
		sampleRate = targetRate
	}
	deviceConfig.SampleRate = uint32(sampleRate)

	samplesPerChunk := int(float64(sampleRate) * rec.cfg.ChunkDuration.Duration().Seconds())
	if samplesPerChunk <= 0 {
		samplesPerChunk = 1024
	}
	deviceConfig.PeriodSizeInFrames = uint32(samplesPerChunk)

	dev, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{Data: rec.dataCallback})
	if err != nil {
		if device != nil && device.SampleRate != 0 && device.SampleRate != uint32(sampleRate) {
			fallbackCfg := deviceConfig
			fallbackCfg.SampleRate = device.SampleRate
			dev, err = malgo.InitDevice(ctx.Context, fallbackCfg, malgo.DeviceCallbacks{Data: rec.dataCallback})
			if err != nil {
				return nil, fmt.Errorf("初始化音频设备失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("初始化音频设备失败: %w", err)
		}
	}

	rec.device = dev
	rec.sampleRateIn = int(dev.SampleRate())
	if rec.sampleRateIn <= 0 {
		rec.sampleRateIn = sampleRate
	}

	rec.channels = int(dev.CaptureChannels())
	if rec.channels <= 0 {
		rec.channels = channels
	}

	return rec, nil
}

// Run 启动设备并处理采集数据。
func (r *Recorder) Run(ctx context.Context, out chan<- Chunk) error {
	if r.device == nil {
		return fmt.Errorf("音频设备未初始化")
	}

	if err := r.device.Start(); err != nil {
		return fmt.Errorf("启动音频设备失败: %w", err)
	}
	defer r.Close()

	silenceLimit := r.cfg.SilenceDuration.Duration().Seconds()
	if silenceLimit <= 0 {
		silenceLimit = 0.8
	}
	maxBuffer := r.cfg.MaxBuffer.Duration().Seconds()
	if maxBuffer <= 0 {
		maxBuffer = 10
	}
	threshold := r.cfg.SilenceThreshold
	if threshold <= 0 {
		threshold = 0.02
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case samples, ok := <-r.chunkQueue:
			if !ok {
				return nil
			}
			r.processSamples(samples, out, threshold, silenceLimit, maxBuffer)
		}
	}
}

// Close 停止并释放设备。
func (r *Recorder) Close() {
	r.stopOnce.Do(func() {
		if r.device != nil {
			_ = r.device.Stop()
			r.device.Uninit()
			r.device = nil
		}
	})
}

func (r *Recorder) onInput(input []byte, frameCount int) {
	if frameCount <= 0 || len(input) == 0 {
		return
	}

	samplesPerFrame := r.channels
	if samplesPerFrame <= 0 {
		samplesPerFrame = 1
	}

	sampleCount := frameCount * samplesPerFrame
	neededBytes := sampleCount * 2
	if neededBytes > len(input) {
		sampleCount = len(input) / 2
	}

	data := make([]int16, sampleCount)
	for i := 0; i < sampleCount; i++ {
		data[i] = int16(binary.LittleEndian.Uint16(input[i*2:]))
	}

	select {
	case r.chunkQueue <- data:
	default:
		// 丢弃数据以避免阻塞回调
	}
}

func (r *Recorder) processSamples(samples []int16, out chan<- Chunk, threshold, silenceLimit, maxBuffer float64) {
	mono := toMonoInt16(samples, r.channels)
	peak := peakAmplitudeInt16(mono)
	chunkDuration := float64(len(mono)) / float64(r.sampleRateIn)
	if chunkDuration <= 0 {
		return
	}

	if peak >= threshold {
		if !r.speaking {
			r.speaking = true
			r.buffer = r.buffer[:0]
			r.bufferDuration = 0
			r.silenceSeconds = 0
			r.startTimestamp = time.Now()
		}
		r.silenceSeconds = 0
		r.buffer = append(r.buffer, mono...)
		r.bufferDuration += chunkDuration
	} else if r.speaking {
		r.silenceSeconds += chunkDuration
		r.buffer = append(r.buffer, mono...)
		r.bufferDuration += chunkDuration
	}

	if r.speaking {
		if r.silenceSeconds >= silenceLimit || r.bufferDuration >= maxBuffer {
			r.flush(out)
		}
	}
}

func (r *Recorder) flush(out chan<- Chunk) {
	if len(r.buffer) == 0 {
		r.resetState()
		return
	}

	samples := make([]int16, len(r.buffer))
	copy(samples, r.buffer)

	if r.sampleRateIn != r.targetRate && r.targetRate > 0 {
		samples = linearResample(samples, r.sampleRateIn, r.targetRate)
	}

	chunk := Chunk{
		Source:     r.source,
		Samples:    samples,
		SampleRate: r.targetRate,
		CapturedAt: r.startTimestamp,
		Duration:   time.Duration(r.bufferDuration * float64(time.Second)),
	}

	select {
	case out <- chunk:
	default:
		r.logger.Warn("音频块队列已满，丢弃识别片段", zap.String("source", string(r.source)))
	}

	r.resetState()
}

func (r *Recorder) resetState() {
	r.speaking = false
	r.buffer = r.buffer[:0]
	r.bufferDuration = 0
	r.silenceSeconds = 0
}

func toMonoInt16(samples []int16, channels int) []int16 {
	if channels <= 1 {
		dup := make([]int16, len(samples))
		copy(dup, samples)
		return dup
	}
	frames := len(samples) / channels
	mono := make([]int16, frames)
	for i := 0; i < frames; i++ {
		mono[i] = samples[i*channels]
	}
	return mono
}

func peakAmplitudeInt16(samples []int16) float64 {
	var peak int16
	for _, v := range samples {
		if v < 0 {
			if -v > peak {
				peak = -v
			}
		} else if v > peak {
			peak = v
		}
	}
	return float64(peak) / 32768.0
}
