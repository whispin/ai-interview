package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"interviewai/internal/app"
	"interviewai/internal/asr"
	"interviewai/internal/audio"
	"interviewai/internal/config"
	"interviewai/internal/llm"
	"interviewai/internal/pipeline"
	"interviewai/internal/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 程序退出: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var cfgFiles []string
	var searchPaths []string
	var envPrefix string
	var verbose bool
	var allowMissing bool

	fs := pflag.NewFlagSet("interview-ai-cli", pflag.ContinueOnError)
	fs.StringArrayVarP(&cfgFiles, "config", "c", nil, "指定配置文件路径 (可重复，后面的覆盖前面的)")
	fs.StringArrayVarP(&searchPaths, "config-path", "p", nil, "配置文件搜索路径 (可重复)")
	fs.StringVar(&envPrefix, "env-prefix", "INTERVIEW_AI_", "环境变量前缀")
	fs.BoolVarP(&verbose, "verbose", "v", false, "显示详细配置加载信息")
	fs.BoolVar(&allowMissing, "allow-missing-config", false, "允许配置文件不存在")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	cfg, err := config.Load(config.Options{
		ConfigFiles:  cfgFiles,
		SearchPaths:  searchPaths,
		FlagSet:      fs,
		EnvPrefix:    envPrefix,
		Verbose:      verbose,
		AllowMissing: allowMissing,
	})
	if err != nil {
		return err
	}

	container, err := app.BuildContainer(cfg)
	if err != nil {
		return err
	}
	defer container.Close()

	logger := container.Logger
	logger.Info("CLI 启动", zap.String("environment", cfg.App.Environment))

	ctx, cancel := signalContext()
	defer cancel()

	transcriber, err := asr.New(cfg.ASR, logger.Named("asr"))
	if err != nil {
		return err
	}

	engine, err := audio.NewEngine(cfg.Audio, audio.CaptureModeBoth, logger.Named("audio"))
	if err != nil {
		return err
	}

	pipe := pipeline.NewManager(container, transcriber, engine)
	if err := pipe.Start(ctx); err != nil {
		return err
	}
	defer pipe.Stop()

	llmProvider, err := llm.NewProvider(cfg.LLM, logger.Named("llm"))
	if err != nil {
		return err
	}

	session := &state{}

	go forwardResults(ctx, pipe.Results(), session, logger)
	go forwardErrors(ctx, pipe.Errors(), logger)

	if cfg.CLI.EnableHotkey {
		if err := startHotkeyListener(ctx, cfg.CLI.Hotkey, logger, func() {
			question, ok := session.LastSpeaker()
			if !ok {
				logger.Warn("没有最新的面试官问题，无法触发 AI")
				return
			}
			logger.Info("触发 AI 建议生成", zap.String("question", question.Text))

			go func() {
				aiCtx, cancelAI := context.WithTimeout(ctx, 2*time.Minute)
				defer cancelAI()

				fmt.Printf("\n====================\n🤖 AI 建议 (基于问题: %s)\n--------------------\n", question.Text)

				_, err := llmProvider.Assistant.StreamAndAppend(aiCtx, question.Text, func(chunk string, done bool) error {
					if chunk != "" {
						fmt.Print(chunk)
					}
					if done {
						fmt.Print("\n====================\n")
					}
					return nil
				})
				if err != nil {
					fmt.Printf("\n❌ AI 生成失败: %v\n", err)
					logger.Error("AI 生成失败", zap.Error(err))
				}
			}()
		}); err != nil {
			logger.Warn("热键监听启动失败", zap.Error(err))
		}
	}

	<-ctx.Done()
	logger.Info("收到退出信号，即将退出")
	return nil
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}

func forwardResults(ctx context.Context, in <-chan types.ASRResult, session *state, logger *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case res, ok := <-in:
			if !ok {
				return
			}
			ts := res.Completed.Format("15:04:05")
			if res.Source == types.SpeechSourceSpeaker {
				session.SetSpeaker(res)
				fmt.Printf("\n[%s] 面试官: %s\n", ts, strings.TrimSpace(res.Text))
			} else {
				fmt.Printf("\n[%s] 我: %s\n", ts, strings.TrimSpace(res.Text))
			}
		}
	}
}

func forwardErrors(ctx context.Context, in <-chan error, logger *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-in:
			if !ok {
				return
			}
			logger.Warn("流水线错误", zap.Error(err))
		}
	}
}

func startHotkeyListener(ctx context.Context, hotkey string, logger *zap.Logger, trigger func()) error {
	if hotkey == "" {
		hotkey = "ctrl+v"
	}
	if err := keyboard.Open(); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		keyboard.Close()
	}()

	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			char, key, err := keyboard.GetKey()
			if err != nil {
				logger.Warn("读取键盘事件失败", zap.Error(err))
				continue
			}
			if matchHotkey(char, key, hotkey) {
				trigger()
			}
		}
	}()

	logger.Info("热键监听已启动", zap.String("hotkey", hotkey))
	return nil
}

func matchHotkey(char rune, key keyboard.Key, hotkey string) bool {
	normalized := strings.ToLower(hotkey)
	if normalized == "ctrl+v" {
		return key == keyboard.KeyCtrlV || char == 0x16
	}
	return false
}

type state struct {
	mu          sync.RWMutex
	lastSpeaker types.ASRResult
	hasSpeaker  bool
}

func (s *state) SetSpeaker(res types.ASRResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSpeaker = res
	s.hasSpeaker = true
}

func (s *state) LastSpeaker() (types.ASRResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSpeaker, s.hasSpeaker
}
