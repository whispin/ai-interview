package logging

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"interviewai/internal/config"
)

// NewLogger 根据配置创建 zap.Logger。
func NewLogger(cfg config.LoggingConfig) (*zap.Logger, error) {
	encoding := strings.ToLower(cfg.Encoding)
	if encoding == "" {
		encoding = "console"
	}

	zapCfg := zap.Config{
		Level:       parseLevel(cfg.Level),
		Development: cfg.Development,
		Encoding:    encoding,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stack",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if encoding == "json" {
		zapCfg.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	}

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

func parseLevel(level string) zap.AtomicLevel {
	if level == "" {
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	switch strings.ToLower(level) {
	case "debug":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn", "warning":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "dpanic":
		return zap.NewAtomicLevelAt(zap.DPanicLevel)
	case "panic":
		return zap.NewAtomicLevelAt(zap.PanicLevel)
	case "fatal":
		return zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}
}
