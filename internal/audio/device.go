package audio

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gen2brain/malgo"
	zap "go.uber.org/zap"

	"interviewai/internal/config"
)

// Device 描述可用的音频捕获设备。
type Device struct {
	ID         malgo.DeviceID
	Name       string
	Channels   uint32
	SampleRate uint32
	Priority   int
	IsDefault  bool
}

// discoverDevices 按优先级挑选扬声器与麦克风设备。
func discoverDevices(ctx *malgo.AllocatedContext, cfg config.AudioConfig, mode CaptureMode, logger *zap.Logger) (*Device, *Device, error) {
	devices, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return nil, nil, fmt.Errorf("枚举音频捕获设备失败: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil, fmt.Errorf("未检测到任何音频捕获设备")
	}

	wantSpeaker := mode != CaptureModeMicrophoneOnly
	wantMicro := mode != CaptureModeSpeakerOnly

	var speakerCandidates []*Device
	var micCandidates []*Device

	for _, info := range devices {
		name := strings.TrimSpace(info.Name())
		if name == "" {
			name = "Unknown Capture Device"
		}

		channels := uint32(cfg.DefaultChannels)
		if channels == 0 {
			channels = 1
		}
		sampleRate := chooseSampleRate(info, uint32(cfg.SampleRate))

		dev := &Device{
			ID:         info.ID,
			Name:       name,
			Channels:   channels,
			SampleRate: sampleRate,
			Priority:   0,
			IsDefault:  info.IsDefault != 0,
		}

		if wantSpeaker {
			speakerScore := scoreDevice(name, cfg.DeviceHints.SpeakerKeywords)
			if speakerScore > 0 {
				dev.Priority = speakerScore
				speakerCandidates = append(speakerCandidates, dev)
			}
		}

		if wantMicro {
			micScore := scoreDevice(name, cfg.DeviceHints.MicrophoneKeywords)
			if micScore > 0 {
				copyDev := *dev
				copyDev.Priority = micScore
				micCandidates = append(micCandidates, &copyDev)
			}
		}

		logger.Debug("检测到音频设备", zap.String("name", name), zap.Uint32("sample_rate", sampleRate), zap.Uint32("channels", channels))
	}

	if wantSpeaker && len(speakerCandidates) == 0 {
		speakerCandidates = duplicateCandidates(devices, cfg, logger)
	}
	if wantMicro && len(micCandidates) == 0 {
		micCandidates = duplicateCandidates(devices, cfg, logger)
	}

	sortDevices(speakerCandidates)
	sortDevices(micCandidates)

	var speaker *Device
	var mic *Device
	if wantSpeaker && len(speakerCandidates) > 0 {
		speaker = speakerCandidates[0]
		logger.Info("选择扬声器捕获设备", zap.String("name", speaker.Name), zap.Uint32("rate", speaker.SampleRate))
	}
	if wantMicro && len(micCandidates) > 0 {
		candidate := micCandidates[0]
		if speaker != nil && strings.EqualFold(speaker.Name, candidate.Name) {
			mic = &Device{
				ID:         candidate.ID,
				Name:       candidate.Name,
				Channels:   candidate.Channels,
				SampleRate: candidate.SampleRate,
				Priority:   candidate.Priority,
				IsDefault:  candidate.IsDefault,
			}
		} else {
			mic = candidate
		}
		logger.Info("选择麦克风捕获设备", zap.String("name", mic.Name), zap.Uint32("rate", mic.SampleRate))
	}

	if wantSpeaker && speaker == nil {
		return nil, nil, fmt.Errorf("未找到扬声器捕获设备")
	}
	if wantMicro && mic == nil {
		return nil, nil, fmt.Errorf("未找到麦克风捕获设备")
	}

	if !wantSpeaker {
		speaker = nil
	}
	if !wantMicro {
		mic = nil
	}

	return speaker, mic, nil
}

func chooseSampleRate(info malgo.DeviceInfo, preferred uint32) uint32 {
	if preferred != 0 {
		for _, fmt := range info.Formats {
			if fmt.SampleRate == preferred && fmt.Format == malgo.FormatS16 {
				return preferred
			}
		}
	}
	for _, fmt := range info.Formats {
		if fmt.Format == malgo.FormatS16 && fmt.SampleRate > 0 {
			return fmt.SampleRate
		}
	}
	if preferred != 0 {
		return preferred
	}
	return 16000
}

func duplicateCandidates(devices []malgo.DeviceInfo, cfg config.AudioConfig, logger *zap.Logger) []*Device {
	var result []*Device
	for _, info := range devices {
		name := strings.TrimSpace(info.Name())
		if name == "" {
			name = "Unknown Capture Device"
		}
		channels := uint32(cfg.DefaultChannels)
		if channels == 0 {
			channels = 1
		}
		sampleRate := chooseSampleRate(info, uint32(cfg.SampleRate))
		result = append(result, &Device{
			ID:         info.ID,
			Name:       name,
			Channels:   channels,
			SampleRate: sampleRate,
			Priority:   1,
			IsDefault:  info.IsDefault != 0,
		})
		logger.Debug("追加默认候选设备", zap.String("name", name))
	}
	return result
}

func scoreDevice(name string, keywords []string) int {
	nameLower := strings.ToLower(name)
	best := 0
	for _, kw := range keywords {
		kwLower := strings.ToLower(strings.TrimSpace(kw))
		if kwLower == "" {
			continue
		}
		if strings.Contains(nameLower, kwLower) {
			score := 100 - len(kwLower)
			if score > best {
				best = score
			}
		}
	}
	return best
}

func sortDevices(devices []*Device) {
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Priority == devices[j].Priority {
			if devices[i].IsDefault && !devices[j].IsDefault {
				return true
			}
			if devices[j].IsDefault && !devices[i].IsDefault {
				return false
			}
			return strings.Compare(devices[i].Name, devices[j].Name) < 0
		}
		return devices[i].Priority > devices[j].Priority
	})
}
