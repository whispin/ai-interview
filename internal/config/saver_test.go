package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSave(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", "test.yaml")

	// 创建测试配置
	cfg := DefaultConfig()
	cfg.App.Name = "test-app"
	cfg.ASR.Active = "openai_whisper"
	cfg.ASR.OpenAIWhisper.APIKey = "test-api-key-12345"
	cfg.LLM.Active = "openai"
	cfg.LLM.SystemPrompt = "Test system prompt"

	// 设置有效的 API Key 以通过验证
	if provider, ok := cfg.LLM.Providers["openai"]; ok {
		provider.APIKey = "test-openai-api-key"
		cfg.LLM.Providers["openai"] = provider
	}

	// 测试保存
	err := Save(&cfg, SaveOptions{
		FilePath:     configPath,
		CreateBackup: false,
		CreateDirs:   true,
	})

	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created: %s", configPath)
	}

	// 重新加载配置（跳过验证以测试序列化）
	loadedCfg, err := Load(Options{
		ConfigFiles:     []string{configPath},
		AllowMissing:    false,
		Verbose:         false,
		DisableDefaults: true, // 不加载默认值，只测试保存的内容
	})

	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// 验证值
	if loadedCfg.App.Name != "test-app" {
		t.Errorf("App.Name mismatch: got %s, want test-app", loadedCfg.App.Name)
	}

	if loadedCfg.ASR.Active != "openai_whisper" {
		t.Errorf("ASR.Active mismatch: got %s, want openai_whisper", loadedCfg.ASR.Active)
	}

	if loadedCfg.ASR.OpenAIWhisper.APIKey != "test-api-key-12345" {
		t.Errorf("API Key mismatch: got %s, want test-api-key-12345", loadedCfg.ASR.OpenAIWhisper.APIKey)
	}

	if loadedCfg.LLM.SystemPrompt != "Test system prompt" {
		t.Errorf("SystemPrompt mismatch: got %s, want Test system prompt", loadedCfg.LLM.SystemPrompt)
	}
}

func TestSaveWithBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// 创建初始配置
	cfg1 := DefaultConfig()
	cfg1.App.Name = "version-1"
	if provider, ok := cfg1.LLM.Providers["openai"]; ok {
		provider.APIKey = "test-api-key-v1"
		cfg1.LLM.Providers["openai"] = provider
	}

	err := Save(&cfg1, SaveOptions{
		FilePath:     configPath,
		CreateBackup: false,
		CreateDirs:   true,
	})
	if err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// 等待一秒确保时间戳不同
	time.Sleep(1 * time.Second)

	// 保存新配置（应创建备份）
	cfg2 := DefaultConfig()
	cfg2.App.Name = "version-2"
	if provider, ok := cfg2.LLM.Providers["openai"]; ok {
		provider.APIKey = "test-api-key-v2"
		cfg2.LLM.Providers["openai"] = provider
	}

	err = Save(&cfg2, SaveOptions{
		FilePath:     configPath,
		CreateBackup: true,
		CreateDirs:   true,
		BackupSuffix: ".backup",
	})
	if err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// 验证主文件已更新
	loadedCfg, err := Load(Options{
		ConfigFiles:  []string{configPath},
		AllowMissing: false,
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loadedCfg.App.Name != "version-2" {
		t.Errorf("Main config not updated: got %s, want version-2", loadedCfg.App.Name)
	}

	// 验证备份文件存在
	backupFiles, err := filepath.Glob(configPath + ".*.backup")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}

	if len(backupFiles) == 0 {
		t.Error("Backup file was not created")
	}
}

func TestGetConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		expected string
	}{
		{
			name: "explicit config file",
			opts: Options{
				ConfigFiles: []string{"/path/to/config.yaml"},
			},
			expected: "/path/to/config.yaml",
		},
		{
			name:     "default path",
			opts:     Options{},
			expected: "config/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetConfigPath(tt.opts)
			if result != tt.expected {
				t.Errorf("GetConfigPath() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestSaveToDefault(t *testing.T) {
	// 保存当前目录
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	// 创建临时目录并切换到该目录
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	// 创建测试配置
	cfg := DefaultConfig()
	cfg.App.Name = "default-test"
	if provider, ok := cfg.LLM.Providers["openai"]; ok {
		provider.APIKey = "test-default-api-key"
		cfg.LLM.Providers["openai"] = provider
	}

	// 测试保存到默认位置
	err := SaveToDefault(&cfg)
	if err != nil {
		t.Fatalf("SaveToDefault failed: %v", err)
	}

	// 验证文件存在于默认位置
	defaultPath := "config/config.yaml"
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		t.Fatalf("Config file not created at default path: %s", defaultPath)
	}

	// 重新加载验证
	loadedCfg, err := Load(Options{
		ConfigFiles: []string{defaultPath},
	})
	if err != nil {
		t.Fatalf("Load from default failed: %v", err)
	}

	if loadedCfg.App.Name != "default-test" {
		t.Errorf("Config mismatch: got %s, want default-test", loadedCfg.App.Name)
	}
}
