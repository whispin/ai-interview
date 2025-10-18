//go:build windows

package windowprotect

import (
	"fmt"
	"syscall"
	"unsafe"

	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

const (
	// WDA_EXCLUDEFROMCAPTURE 防止窗口被屏幕捕获
	// Exclude window from screen capture
	WDA_EXCLUDEFROMCAPTURE = 0x00000011
)

var (
	user32                      = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW             = user32.NewProc("FindWindowW")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
)

// protectWindowImpl Windows平台实现
func protectWindowImpl(windowTitle string, logger *zap.Logger) error {
	logger.Info("正在设置窗口保护", zap.String("title", windowTitle))

	// 查找窗口句柄
	hwnd, err := findWindowByTitle(windowTitle)
	if err != nil {
		logger.Error("查找窗口失败", zap.Error(err))
		return fmt.Errorf("查找窗口失败: %w", err)
	}

	logger.Info("找到窗口", zap.Uintptr("hwnd", hwnd))

	// 设置窗口保护
	if err := setWindowDisplayAffinity(hwnd); err != nil {
		logger.Error("设置窗口保护失败", zap.Error(err))
		return fmt.Errorf("设置窗口保护失败: %w", err)
	}

	logger.Info("窗口保护已启用", zap.String("message", "屏幕共享时将不可见"))
	return nil
}

// isSupportedImpl Windows 10 2000+ 支持
func isSupportedImpl() bool {
	return true
}

// findWindowByTitle 通过窗口标题查找窗口句柄
// Find window handle by title
func findWindowByTitle(title string) (uintptr, error) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0, fmt.Errorf("转换窗口标题失败: %w", err)
	}

	ret, _, err := procFindWindowW.Call(
		0, // lpClassName (NULL = any class)
		uintptr(unsafe.Pointer(titlePtr)),
	)

	if ret == 0 {
		return 0, fmt.Errorf("未找到窗口 '%s': %w", title, err)
	}

	return ret, nil
}

// setWindowDisplayAffinity 设置窗口显示亲和性
// Set window display affinity to exclude from capture
func setWindowDisplayAffinity(hwnd uintptr) error {
	ret, _, err := procSetWindowDisplayAffinity.Call(
		hwnd,
		WDA_EXCLUDEFROMCAPTURE,
	)

	if ret == 0 {
		return fmt.Errorf("SetWindowDisplayAffinity 失败: %w", err)
	}

	return nil
}
