//go:build darwin

package windowprotect

import (
	"fmt"

	"go.uber.org/zap"
)

// protectWindowImpl macOS平台实现（待实现）
func protectWindowImpl(windowTitle string, logger *zap.Logger) error {
	logger.Warn("macOS窗口保护功能待实现")
	return fmt.Errorf("macOS暂不支持窗口保护功能")
}

// isSupportedImpl macOS支持检查
func isSupportedImpl() bool {
	return false // 待实现
}
