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
	ConfigFiles     []string // 明确指定的配置文件列表
	FlagSet         *pflag.FlagSet
	EnvPrefix       string
	SearchPaths     []string // 配置文件搜索路径
	AllowMissing    bool     // 是否允许配置文件不存在
	Verbose         bool     // 是否输出详细加载信息
	DisableDefaults bool     // 是否禁用默认配置
}

// Load 读取配置并返回结构体。
func Load(opts Options) (*Config, error) {
	k := koanf.New(".")
	loadedFiles := []string{}

	// 1. 加载默认配置
	if !opts.DisableDefaults {
		defaults := DefaultConfig()
		if err := k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
			return nil, fmt.Errorf("加载默认配置失败: %w", err)
		}
		if opts.Verbose {
			fmt.Println("✓ 已加载默认配置")
		}
	}

	// 2. 确定要加载的配置文件列表
	paths := opts.ConfigFiles
	if len(paths) == 0 {
		// 如果没有指定配置文件，使用搜索路径
		paths = resolveConfigPaths(opts.SearchPaths)
		if opts.Verbose && len(paths) > 0 {
			fmt.Printf("→ 搜索到配置文件: %v\n", paths)
		}
	}

	// 3. 加载配置文件（按顺序，后面的覆盖前面的）
	foundAny := false
	for _, path := range paths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if opts.Verbose {
					fmt.Printf("✗ 配置文件不存在，跳过: %s\n", path)
				}
				continue
			}
			return nil, fmt.Errorf("访问配置文件 %s 失败: %w", path, err)
		}

		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
		}

		loadedFiles = append(loadedFiles, path)
		foundAny = true
		if opts.Verbose {
			fmt.Printf("✓ 已加载配置文件: %s\n", path)
		}
	}

	// 4. 检查是否至少加载了一个配置文件
	if !foundAny && !opts.AllowMissing && !opts.DisableDefaults {
		if opts.Verbose {
			fmt.Println("⚠ 警告: 未找到任何配置文件，使用默认配置")
		}
	}

	// 5. 加载环境变量配置
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
	if opts.Verbose {
		fmt.Printf("✓ 已加载环境变量 (前缀: %s)\n", prefix)
	}

	// 6. 加载命令行参数配置
	if opts.FlagSet != nil {
		if err := k.Load(posflag.Provider(opts.FlagSet, ".", k), nil); err != nil {
			return nil, fmt.Errorf("解析命令行参数失败: %w", err)
		}
		if opts.Verbose {
			fmt.Println("✓ 已加载命令行参数")
		}
	}

	// 7. 反序列化配置
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	// 8. 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// 9. 输出加载摘要
	if opts.Verbose {
		fmt.Printf("\n配置加载完成:\n")
		fmt.Printf("  - 已加载 %d 个配置文件\n", len(loadedFiles))
		for i, f := range loadedFiles {
			fmt.Printf("    %d. %s\n", i+1, f)
		}
		fmt.Printf("  - ASR 提供商: %s\n", cfg.ASR.Active)
		fmt.Printf("  - LLM 提供商: %s\n", cfg.LLM.Active)
		fmt.Printf("  - 日志级别: %s\n\n", cfg.Logging.Level)
	}

	return &cfg, nil
}

// resolveConfigPaths 解析配置文件搜索路径
func resolveConfigPaths(searchPaths []string) []string {
	if len(searchPaths) == 0 {
		// 默认搜索路径
		searchPaths = []string{
			"config/config.yaml",
			"config.yaml",
			"./config/config.yaml",
			"$HOME/.config/interview-ai/config.yaml",
			"/etc/interview-ai/config.yaml",
		}
	}

	var result []string
	for _, path := range searchPaths {
		// 展开环境变量
		expanded := os.ExpandEnv(path)
		if expanded != "" {
			result = append(result, expanded)
		}
	}

	return result
}
