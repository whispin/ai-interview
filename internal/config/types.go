package config

import (
	"fmt"
	"strings"
	"time"
)

// Duration 包装 time.Duration 以支持 YAML/JSON 文本配置
// 示例值："800ms"、"5s"。
type Duration time.Duration

// UnmarshalText 解析持续时间字符串。
func (d *Duration) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}
	dur, err := time.ParseDuration(strings.TrimSpace(string(text)))
	if err != nil {
		return fmt.Errorf("解析持续时间失败: %w", err)
	}
	*d = Duration(dur)
	return nil
}

// MarshalText 将持续时间转换为字符串。
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Duration 返回原生 time.Duration。
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// PtrDuration 辅助函数，便于设置指针默认值。
func PtrDuration(d time.Duration) *Duration {
	val := Duration(d)
	return &val
}
