package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// SaveOptions 控制配置保存行为
type SaveOptions struct {
	FilePath      string // 保存的目标文件路径
	CreateBackup  bool   // 是否创建备份
	CreateDirs    bool   // 是否自动创建目录
	BackupSuffix  string // 备份文件后缀（默认：.backup）
}

// Save 将配置保存到 YAML 文件
func Save(cfg *Config, opts SaveOptions) error {
	// 1. 确定保存路径
	savePath := opts.FilePath
	if savePath == "" {
		// 默认保存到 config/config.yaml
		savePath = "config/config.yaml"
	}

	// 2. 确保目录存在
	dir := filepath.Dir(savePath)
	if opts.CreateDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建配置目录失败 %s: %w", dir, err)
		}
	}

	// 3. 创建备份（如果文件存在且启用备份）
	if opts.CreateBackup {
		if _, err := os.Stat(savePath); err == nil {
			backupSuffix := opts.BackupSuffix
			if backupSuffix == "" {
				backupSuffix = ".backup"
			}

			// 使用时间戳的备份文件名
			timestamp := time.Now().Format("20060102_150405")
			backupPath := fmt.Sprintf("%s.%s%s", savePath, timestamp, backupSuffix)

			// 读取原文件
			originalData, err := os.ReadFile(savePath)
			if err != nil {
				return fmt.Errorf("读取原配置文件失败: %w", err)
			}

			// 写入备份
			if err := os.WriteFile(backupPath, originalData, 0644); err != nil {
				return fmt.Errorf("创建备份文件失败: %w", err)
			}
		}
	}

	// 4. 序列化配置为 YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 5. 写入文件
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// SaveToDefault 保存配置到默认位置（config/config.yaml）
func SaveToDefault(cfg *Config) error {
	return Save(cfg, SaveOptions{
		FilePath:     "config/config.yaml",
		CreateBackup: true,
		CreateDirs:   true,
		BackupSuffix: ".backup",
	})
}

// GetConfigPath 获取当前使用的配置文件路径
// 如果没有找到配置文件，返回默认路径
func GetConfigPath(opts Options) string {
	// 1. 如果明确指定了配置文件，使用第一个
	if len(opts.ConfigFiles) > 0 {
		return opts.ConfigFiles[0]
	}

	// 2. 使用搜索路径查找第一个存在的文件
	paths := opts.SearchPaths
	if len(paths) == 0 {
		paths = []string{
			"config/config.yaml",
			"config.yaml",
			"./config/config.yaml",
		}
	}

	for _, path := range paths {
		expanded := os.ExpandEnv(path)
		if _, err := os.Stat(expanded); err == nil {
			return expanded
		}
	}

	// 3. 没找到，返回默认路径
	return "config/config.yaml"
}
