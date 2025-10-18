package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneApp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	binding "fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"interviewai/internal/app"
	"interviewai/internal/asr"
	"interviewai/internal/audio"
	"interviewai/internal/config"
	"interviewai/internal/llm"
	"interviewai/internal/pipeline"
	"interviewai/internal/types"
	apptheme "interviewai/internal/ui/theme"
)

var captureOptions = []struct {
	Label string
	Mode  audio.CaptureMode
}{
	{Label: "扬声器 + 麦克风", Mode: audio.CaptureModeBoth},
	{Label: "仅扬声器", Mode: audio.CaptureModeSpeakerOnly},
	{Label: "仅麦克风", Mode: audio.CaptureModeMicrophoneOnly},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ GUI 启动失败: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var cfgFiles []string
	var envPrefix string

	fs := pflag.NewFlagSet("interview-ai-gui", pflag.ContinueOnError)
	fs.StringArrayVarP(&cfgFiles, "config", "c", nil, "指定额外的配置文件")
	fs.StringVar(&envPrefix, "env-prefix", "INTERVIEW_AI_", "覆盖环境变量前缀")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	cfg, err := config.Load(config.Options{
		ConfigFiles: cfgFiles,
		FlagSet:     fs,
		EnvPrefix:   envPrefix,
	})
	if err != nil {
		return err
	}

	container, err := app.BuildContainer(cfg)
	if err != nil {
		return err
	}
	defer container.Close()

	gui := newGUI(container)
	return gui.run()
}

type guiApp struct {
	app       fyne.App
	window    fyne.Window
	container *app.Container
	cfg       *config.Config
	logger    *zap.Logger

	pipeline    *pipeline.Manager
	llmProvider *llm.Provider

	ctx    context.Context
	cancel context.CancelFunc

	interviewer binding.String
	ai          binding.String
	status      binding.String

	captureMode   audio.CaptureMode
	captureSelect *widget.Select

	startButton *widget.Button
	askButton   *widget.Button
	clearButton *widget.Button

	textMu  sync.Mutex
	session *state
}

func newGUI(container *app.Container) *guiApp {
	interviewer := binding.NewString()
	_ = interviewer.Set("等待面试官提问…\n")

	ai := binding.NewString()
	_ = ai.Set("点击「获取 AI 建议」按钮获取回答建议…\n")

	status := binding.NewString()
	_ = status.Set("点击「启动语音识别」准备系统")

	return &guiApp{
		container:   container,
		cfg:         container.Config,
		logger:      container.Logger.Named("gui"),
		interviewer: interviewer,
		ai:          ai,
		status:      status,
		captureMode: audio.CaptureModeBoth,
		session:     &state{},
	}
}

func (g *guiApp) run() error {
	g.app = fyneApp.NewWithID("interview-ai")
	g.app.Settings().SetTheme(apptheme.New())
	g.window = g.app.NewWindow("面试助手 - Go 版")

	g.buildUI()

	g.window.Resize(fyne.NewSize(float32(g.cfg.GUI.Width), float32(g.cfg.GUI.Height)))
	g.window.SetCloseIntercept(func() {
		g.shutdown()
		g.window.Close()
	})

	g.window.ShowAndRun()
	return nil
}

func (g *guiApp) buildUI() {
	title := widget.NewLabelWithStyle("面试助手", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("实时捕获面试官问题并生成 AI 建议")
	headerBox := container.NewVBox(title, subtitle)

	g.startButton = widget.NewButtonWithIcon("启动语音识别", theme.MediaPlayIcon(), g.startRecognition)
	g.startButton.Importance = widget.HighImportance

	g.askButton = widget.NewButtonWithIcon("获取 AI 建议", theme.MailSendIcon(), g.askAI)
	g.askButton.Importance = widget.MediumImportance
	g.askButton.Disable()

	g.clearButton = widget.NewButtonWithIcon("清空", theme.DeleteIcon(), g.clear)
	g.clearButton.Importance = widget.LowImportance

	buttons := container.New(layout.NewGridLayoutWithColumns(3), g.startButton, g.askButton, g.clearButton)

	labels := make([]string, len(captureOptions))
	for i, opt := range captureOptions {
		labels[i] = opt.Label
	}
	g.captureSelect = widget.NewSelect(labels, func(value string) {
		g.captureMode = g.modeForLabel(value)
	})
	g.captureSelect.SetSelected(g.labelForMode(g.captureMode))
	modeBox := container.NewVBox(widget.NewLabel("音频捕获来源"), g.captureSelect)
	controls := container.NewGridWithColumns(2, buttons, modeBox)

	interviewerLabel := widget.NewLabelWithData(g.interviewer)
	interviewerLabel.Wrapping = fyne.TextWrapWord
	interviewerCard := widget.NewCard("👔 面试官问题", "", container.NewVScroll(interviewerLabel))

	aiLabel := widget.NewLabelWithData(g.ai)
	aiLabel.Wrapping = fyne.TextWrapWord
	aiCard := widget.NewCard("🤖 AI 建议", "", container.NewVScroll(aiLabel))

	cards := container.NewAdaptiveGrid(2, interviewerCard, aiCard)

	statusIcon := widget.NewIcon(theme.InfoIcon())
	statusLabel := widget.NewLabelWithData(g.status)
	statusLabel.Wrapping = fyne.TextWrapWord
	statusBar := container.NewBorder(nil, nil, statusIcon, nil, statusLabel)

	header := container.NewVBox(
		container.NewPadded(headerBox),
		widget.NewSeparator(),
		container.NewPadded(controls),
		widget.NewSeparator(),
	)

	footer := container.NewPadded(statusBar)

	content := container.NewPadded(cards)

	g.window.SetContent(container.NewBorder(header, footer, nil, nil, content))
}

func (g *guiApp) startRecognition() {
	if g.pipeline != nil {
		g.setStatus("语音识别已在运行")
		return
	}

	transcriber, err := asr.New(g.cfg.ASR, g.logger.Named("asr"))
	if err != nil {
		g.showError("ASR 初始化失败", err)
		return
	}

	engine, err := audio.NewEngine(g.cfg.Audio, g.captureMode, g.logger.Named("audio"))
	if err != nil {
		g.showError("音频引擎初始化失败", err)
		return
	}

	provider, err := llm.NewProvider(g.cfg.LLM, g.logger.Named("llm"))
	if err != nil {
		g.showError("LLM 初始化失败", err)
		return
	}
	g.llmProvider = provider

	ctx, cancel := context.WithCancel(context.Background())
	g.ctx = ctx
	g.cancel = cancel

	pipe := pipeline.NewManager(g.container, transcriber, engine)
	if err := pipe.Start(ctx); err != nil {
		g.showError("流水线启动失败", err)
		return
	}
	g.pipeline = pipe

	g.startButton.Disable()
	g.askButton.Enable()
	if g.captureSelect != nil {
		g.captureSelect.Disable()
	}

	actualMode := g.captureMode
	speakerAvailable := engine.SpeakerDevice() != nil
	micAvailable := engine.MicrophoneDevice() != nil

	switch {
	case speakerAvailable && micAvailable:
		actualMode = audio.CaptureModeBoth
	case speakerAvailable:
		actualMode = audio.CaptureModeSpeakerOnly
	case micAvailable:
		actualMode = audio.CaptureModeMicrophoneOnly
	}

	if actualMode != g.captureMode {
		g.captureMode = actualMode
		g.runOnMain(func() {
			if g.captureSelect != nil {
				g.captureSelect.SetSelected(g.labelForMode(actualMode))
			}
		})
	}

	switch actualMode {
	case audio.CaptureModeSpeakerOnly:
		g.setStatus("未检测到麦克风音频，仅捕获扬声器。")
	case audio.CaptureModeMicrophoneOnly:
		g.setStatus("未检测到扬声器音频，仅捕获麦克风。")
	default:
		g.setStatus("语音识别运行中，等待面试官提问")
	}

	go g.consumeResults()
	go g.consumeErrors()

	go func() {
		<-ctx.Done()
		g.pipeline = nil
		g.runOnMain(func() {
			if g.captureSelect != nil {
				g.captureSelect.Enable()
				g.captureSelect.Refresh()
			}
			if g.startButton != nil {
				g.startButton.Enable()
				g.startButton.Refresh()
			}
			if g.askButton != nil {
				g.askButton.Disable()
				g.askButton.Refresh()
			}
		})
	}()
}

func (g *guiApp) askAI() {
	if g.llmProvider == nil {
		g.setStatus("AI 模块尚未初始化")
		return
	}

	question, ok := g.session.LastSpeaker()
	if !ok {
		g.setStatus("未检测到面试官问题")
		return
	}

	g.askButton.Disable()
	g.setStatus("AI 正在生成回答建议")
	_ = g.ai.Set("💭 AI 正在思考…\n")

	go func() {
		ctx, cancel := context.WithTimeout(g.ctxOrBackground(), 2*time.Minute)
		defer cancel()

		_, err := g.llmProvider.Assistant.StreamAndAppend(ctx, question.Text, func(chunk string, done bool) error {
			if chunk != "" {
				g.appendBinding(g.ai, chunk)
			}
			if done {
				g.appendBinding(g.ai, "\n")
			}
			return nil
		})

		if err != nil {
			g.appendBinding(g.ai, fmt.Sprintf("\n❌ AI 生成失败: %v\n", err))
			g.logger.Error("AI 生成失败", zap.Error(err))
			g.setStatus("AI 生成失败，请稍后重试")
		} else {
			g.setStatus("AI 建议已生成")
		}

		g.runOnMain(func() {
			g.askButton.Enable()
			g.askButton.Refresh()
		})
	}()
}

func (g *guiApp) clear() {
	g.runOnMain(func() {
		_ = g.interviewer.Set("等待面试官提问…\n")
		_ = g.ai.Set("点击「获取 AI 建议」按钮获取回答建议…\n")
	})
	g.session.Reset()
	g.setStatus("已清空历史记录")
}

func (g *guiApp) consumeResults() {
	for {
		select {
		case <-g.ctx.Done():
			return
		case res, ok := <-g.pipeline.Results():
			if !ok {
				return
			}
			timestamp := res.Completed.Format("15:04:05")
			line := fmt.Sprintf("[%s] %s: %s\n", timestamp, g.labelForSource(res.Source), res.Text)
			if res.Source == types.SpeechSourceSpeaker {
				g.session.SetSpeaker(res)
				g.appendBinding(g.interviewer, line)
			} else {
				g.appendBinding(g.interviewer, line)
			}
		}
	}
}

func (g *guiApp) consumeErrors() {
	for {
		select {
		case <-g.ctx.Done():
			return
		case err, ok := <-g.pipeline.Errors():
			if !ok {
				return
			}
			g.logger.Warn("流水线错误", zap.Error(err))
			g.setStatus(fmt.Sprintf("流水线错误: %v", err))
		}
	}
}

func (g *guiApp) shutdown() {
	if g.cancel != nil {
		g.cancel()
	}
	if g.pipeline != nil {
		_ = g.pipeline.Stop()
		g.pipeline = nil
	}

	g.runOnMain(func() {
		if g.captureSelect != nil {
			g.captureSelect.Enable()
			g.captureSelect.Refresh()
		}
		if g.startButton != nil {
			g.startButton.Enable()
			g.startButton.Refresh()
		}
	})
}

func (g *guiApp) ctxOrBackground() context.Context {
	if g.ctx != nil {
		return g.ctx
	}
	return context.Background()
}

func (g *guiApp) appendBinding(str binding.String, addition string) {
	g.textMu.Lock()
	defer g.textMu.Unlock()

	current, _ := str.Get()
	next := current + addition
	g.runOnMain(func() {
		_ = str.Set(next)
	})
}

func (g *guiApp) setStatus(text string) {
	g.runOnMain(func() {
		_ = g.status.Set(text)
	})
}

func (g *guiApp) showError(prefix string, err error) {
	g.logger.Error(prefix, zap.Error(err))
	g.runOnMain(func() {
		dialog.NewError(fmt.Errorf("%s: %w", prefix, err), g.window).Show()
		_ = g.status.Set(fmt.Sprintf("%s: %v", prefix, err))
	})
}

func (g *guiApp) runOnMain(fn func()) {
	if drv := g.app.Driver(); drv != nil {
		if runner, ok := drv.(interface{ RunOnMain(func()) }); ok {
			runner.RunOnMain(fn)
			return
		}
	}
	fn()
}

func (g *guiApp) modeForLabel(label string) audio.CaptureMode {
	for _, opt := range captureOptions {
		if opt.Label == label {
			return opt.Mode
		}
	}
	return audio.CaptureModeBoth
}

func (g *guiApp) labelForMode(mode audio.CaptureMode) string {
	for _, opt := range captureOptions {
		if opt.Mode == mode {
			return opt.Label
		}
	}
	return captureOptions[0].Label
}

func (g *guiApp) labelForSource(source types.SpeechSource) string {
	if source == types.SpeechSourceSpeaker {
		return "面试官"
	}
	return "我"
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

func (s *state) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasSpeaker = false
	s.lastSpeaker = types.ASRResult{}
}
