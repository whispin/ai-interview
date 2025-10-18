package windowprotect

import "go.uber.org/zap"

// ProtectWindow 保护窗口免受屏幕捕获
// Protect window from screen capture
func ProtectWindow(windowTitle string, logger *zap.Logger) error {
	return protectWindowImpl(windowTitle, logger)
}

// IsSupported 检查当前平台是否支持窗口保护
// Check if window protection is supported on current platform
func IsSupported() bool {
	return isSupportedImpl()
}
