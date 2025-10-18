//go:build linux

package windowprotect

import (
	"fmt"

	"go.uber.org/zap"
)

// protectWindowImpl Linux平台实现（不支持）
func protectWindowImpl(windowTitle string, logger *zap.Logger) error {
	logger.Warn("Linux平台不支持窗口保护功能")
	return fmt.Errorf("Linux平台暂不支持窗口保护功能")
}

// isSupportedImpl Linux支持检查
func isSupportedImpl() bool {
	return false
}
