package app

import (
	"fmt"

	"go.uber.org/zap"

	"interviewai/internal/config"
	"interviewai/internal/logging"
)

// Container 聚合可复用的基础设施组件。
type Container struct {
	Config *config.Config
	Logger *zap.Logger
}

// BuildContainer 初始化容器。
func BuildContainer(cfg *config.Config) (*Container, error) {
	logger, err := logging.NewLogger(cfg.Logging)
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	return &Container{
		Config: cfg,
		Logger: logger,
	}, nil
}

// Close 释放资源。
func (c *Container) Close() error {
	if c == nil {
		return nil
	}
	if c.Logger != nil {
		return c.Logger.Sync()
	}
	return nil
}
