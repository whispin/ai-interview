package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// Options 控制配置装载。
type Options struct {
	ConfigFiles []string
	FlagSet     *pflag.FlagSet
	EnvPrefix   string
}

// Load 读取配置并返回结构体。
func Load(opts Options) (*Config, error) {
	k := koanf.New(".")

	defaults := DefaultConfig()
	if err := k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("加载默认配置失败: %w", err)
	}

	paths := opts.ConfigFiles
	if len(paths) == 0 {
		paths = []string{"config/config.yaml"}
	}

	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("访问配置文件 %s 失败: %w", path, err)
		}
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
		}
	}

	prefix := opts.EnvPrefix
	if prefix == "" {
		prefix = "INTERVIEW_AI_"
	}

	envMapper := func(key string) string {
		if !strings.HasPrefix(strings.ToUpper(key), strings.ToUpper(prefix)) {
			return ""
		}
		trimmed := strings.TrimPrefix(strings.ToUpper(key), strings.ToUpper(prefix))
		trimmed = strings.ReplaceAll(trimmed, "__", ".")
		trimmed = strings.ReplaceAll(trimmed, "_", "-")
		trimmed = strings.ReplaceAll(trimmed, "-", "_")
		return strings.ToLower(trimmed)
	}

	if err := k.Load(env.Provider(prefix, ".", envMapper), nil); err != nil {
		return nil, fmt.Errorf("读取环境变量失败: %w", err)
	}

	if opts.FlagSet != nil {
		if err := k.Load(posflag.Provider(opts.FlagSet, ".", k), nil); err != nil {
			return nil, fmt.Errorf("解析命令行参数失败: %w", err)
		}
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
