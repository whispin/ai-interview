package config

import "time"

// Config 表示全局配置。
type Config struct {
	App      AppConfig      `koanf:"app"`
	Audio    AudioConfig    `koanf:"audio"`
	Pipeline PipelineConfig `koanf:"pipeline"`
	ASR      ASRSection     `koanf:"asr"`
	LLM      LLMSection     `koanf:"llm"`
	CLI      CLIConfig      `koanf:"cli"`
	GUI      GUIConfig      `koanf:"gui"`
	Logging  LoggingConfig  `koanf:"logging"`
}

// AppConfig 描述通用应用信息。
type AppConfig struct {
	Name        string `koanf:"name"`
	Environment string `koanf:"environment"`
}

// AudioConfig 控制音频录制与 VAD 行为。
type AudioConfig struct {
	SampleRate       int         `koanf:"sample_rate"`
	ChunkDuration    Duration    `koanf:"chunk_duration"`
	MaxBuffer        Duration    `koanf:"max_buffer"`
	SilenceDuration  Duration    `koanf:"silence_duration"`
	SilenceThreshold float64     `koanf:"silence_threshold"`
	QueueSize        int         `koanf:"queue_size"`
	DefaultChannels  int         `koanf:"default_channels"`
	DeviceHints      DeviceHints `koanf:"device_hints"`
}

// DeviceHints 用于筛选音频输入设备。
type DeviceHints struct {
	SpeakerKeywords    []string `koanf:"speaker_keywords"`
	MicrophoneKeywords []string `koanf:"microphone_keywords"`
}

// PipelineConfig 描述队列和容错策略。
type PipelineConfig struct {
	MaxConsecutiveErrors int      `koanf:"max_consecutive_errors"`
	StatsInterval        Duration `koanf:"stats_interval"`
}

// ASRSection 定义多种语音识别后端。
type ASRSection struct {
	Active        string              `koanf:"active"`
	Tencent       TencentConfig       `koanf:"tencent"`
	HTTP          HTTPASRConfig       `koanf:"http"`
	OpenAIWhisper OpenAIWhisperConfig `koanf:"openai_whisper"`
}

// TencentConfig 包装腾讯云配置。
type TencentConfig struct {
	SecretID        string `koanf:"secret_id"`
	SecretKey       string `koanf:"secret_key"`
	AppID           string `koanf:"app_id"`
	Region          string `koanf:"region"`
	EngineModelType string `koanf:"engine_model_type"`
}

// HTTPASRConfig 描述自定义 HTTP ASR 服务。
type HTTPASRConfig struct {
	Endpoint       string   `koanf:"endpoint"`
	AuthHeader     string   `koanf:"auth_header"`
	AuthValue      string   `koanf:"auth_value"`
	ResultJSONPath []string `koanf:"result_json_path"`
	Timeout        Duration `koanf:"timeout"`
}

// OpenAIWhisperConfig 描述 OpenAI Whisper 接口。
type OpenAIWhisperConfig struct {
	APIKey      string   `koanf:"api_key"`
	Model       string   `koanf:"model"`
	BaseURL     string   `koanf:"base_url"`
	Language    string   `koanf:"language"`
	Temperature float32  `koanf:"temperature"`
	Timeout     Duration `koanf:"timeout"`
}

// LLMSection 描述多种 LLM 提供商。
type LLMSection struct {
	Active          string                       `koanf:"active"`
	SystemPrompt    string                       `koanf:"system_prompt"`
	ContextMessages int                          `koanf:"context_messages"` // 发送的历史对话条数
	Providers       map[string]LLMProviderConfig `koanf:"providers"`
}

// LLMProviderConfig 统一描述 OpenAI 兼容与其它厂商的配置。
type LLMProviderConfig struct {
	Kind        string   `koanf:"kind"`
	APIKey      string   `koanf:"api_key"`
	Model       string   `koanf:"model"`
	BaseURL     string   `koanf:"base_url"`
	Temperature float32  `koanf:"temperature"`
	MaxTokens   int      `koanf:"max_tokens"`
	Headers     []Header `koanf:"headers"`
}

// Header 用于配置自定义 HTTP 头。
type Header struct {
	Key   string `koanf:"key"`
	Value string `koanf:"value"`
}

// CLIConfig 控制命令行模式行为。
type CLIConfig struct {
	EnableHotkey bool   `koanf:"enable_hotkey"`
	Hotkey       string `koanf:"hotkey"`
}

// GUIConfig 控制 Fyne 界面默认参数。
type GUIConfig struct {
	AutoStartASR bool `koanf:"auto_start_asr"`
	Width        int  `koanf:"width"`
	Height       int  `koanf:"height"`
}

// LoggingConfig 控制日志。
type LoggingConfig struct {
	Level       string `koanf:"level"`
	Encoding    string `koanf:"encoding"`
	Development bool   `koanf:"development"`
}

// DefaultConfig 返回一份默认配置。
func DefaultConfig() Config {
	return Config{
		App: AppConfig{
			Name:        "interview-ai",
			Environment: "development",
		},
		Audio: AudioConfig{
			SampleRate:       16000,
			ChunkDuration:    Duration(100 * time.Millisecond),
			MaxBuffer:        Duration(10 * time.Second),
			SilenceDuration:  Duration(800 * time.Millisecond),
			SilenceThreshold: 0.2,
			QueueSize:        20,
			DefaultChannels:  1,
			DeviceHints: DeviceHints{
				SpeakerKeywords:    []string{"blackhole", "loopback", "virtual", "aggregate"},
				MicrophoneKeywords: []string{"microphone", "built-in", "usb"},
			},
		},
		Pipeline: PipelineConfig{
			MaxConsecutiveErrors: 5,
			StatsInterval:        Duration(30 * time.Second),
		},
		ASR: ASRSection{
			Active: "tencent",
			Tencent: TencentConfig{
				Region:          "ap-shanghai",
				EngineModelType: "16k_zh",
			},
			HTTP: HTTPASRConfig{
				Timeout: Duration(12 * time.Second),
			},
			OpenAIWhisper: OpenAIWhisperConfig{
				Model:       "whisper-1",
				BaseURL:     "https://api.openai.com/v1",
				Temperature: 0,
				Timeout:     Duration(30 * time.Second),
			},
		},
		LLM: LLMSection{
			Active:       "openai",
			SystemPrompt: "你是一个专业的面试助手，需要帮助候选人生成结构化回答。",
			Providers: map[string]LLMProviderConfig{
				"openai": {
					Kind:        "openai",
					Model:       "gpt-4",
					BaseURL:     "https://api.openai.com/v1",
					Temperature: 0.7,
				},
				"custom": {
					Kind:        "openai-compatible",
					Model:       "gpt-4",
					BaseURL:     "https://your-openai-compatible-endpoint/v1",
					Temperature: 0.7,
				},
			},
		},
		CLI: CLIConfig{
			EnableHotkey: true,
			Hotkey:       "ctrl+v",
		},
		GUI: GUIConfig{
			AutoStartASR: false,
			Width:        1400,
			Height:       800,
		},
		Logging: LoggingConfig{
			Level:       "info",
			Encoding:    "console",
			Development: false,
		},
	}
}
