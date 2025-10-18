package main

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	appwidgets "interviewai/internal/ui/widgets"
	"interviewai/internal/windowprotect"

	"image/color"
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
	var searchPaths []string
	var envPrefix string
	var verbose bool
	var allowMissing bool

	fs := pflag.NewFlagSet("interview-ai-gui", pflag.ContinueOnError)
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

	// 侧边栏按钮
	sidebarAskButton   *widget.Button
	sidebarStartButton *widget.Button

	// 识别状态
	isRecognizing bool

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

	// 启动窗口保护（防止屏幕共享捕获）
	// Enable window protection (prevent screen capture)
	go g.enableWindowProtection()

	g.window.ShowAndRun()
	return nil
}

func (g *guiApp) buildUI() {
	// ========== 现代化工具栏 ==========
	toolbar := g.buildModernToolbar()

	// ========== 侧边栏控制面板 ==========
	sidebar := g.buildSidebar()

	// ========== 主内容区域 ==========
	mainContent := g.buildMainContent()

	// ========== 采用现代三栏布局 ==========
	// 左侧边栏(固定宽度) + 主内容区(自适应)
	contentArea := container.NewBorder(nil, nil, sidebar, nil, mainContent)

	// 整体布局: 顶部工具栏 + 内容区 (无状态栏,更简洁)
	g.window.SetContent(container.NewBorder(toolbar, nil, nil, nil, contentArea))
}

// buildModernToolbar 构建现代化工具栏
func (g *guiApp) buildModernToolbar() fyne.CanvasObject {
	// 应用标题
	appTitle := widget.NewLabelWithStyle("面试助手", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	appTitle.Importance = widget.HighImportance

	// 主要操作按钮 - 图标为主，使用切换回调
	// Main action buttons - icon-based, using toggle callback
	g.startButton = widget.NewButtonWithIcon("", theme.MediaPlayIcon(), g.toggleRecognition)
	g.startButton.Importance = widget.HighImportance

	g.askButton = widget.NewButtonWithIcon("", theme.MailSendIcon(), g.askAI)
	g.askButton.Importance = widget.HighImportance
	g.askButton.Disable()

	// 工具按钮
	g.clearButton = widget.NewButtonWithIcon("", theme.DeleteIcon(), g.clear)

	settingsButton := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		dialog := appwidgets.NewSettingsDialog(g.window, g.cfg, g.handleConfigUpdate)
		dialog.Show()
	})

	// 分组:左侧标题,中间主操作,右侧工具
	leftGroup := container.NewHBox(appTitle)
	centerGroup := container.NewHBox(
		layout.NewSpacer(),
		g.startButton,
		g.askButton,
		layout.NewSpacer(),
	)
	rightGroup := container.NewHBox(g.clearButton, settingsButton)

	toolbar := container.NewBorder(nil, nil, leftGroup, rightGroup, centerGroup)

	return container.NewVBox(
		container.NewPadded(toolbar),
		widget.NewSeparator(),
	)
}

// buildSidebar 构建侧边栏控制面板
func (g *guiApp) buildSidebar() fyne.CanvasObject {
	// 音频源配置
	sourceLabel := widget.NewLabelWithStyle("音频源", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	labels := make([]string, len(captureOptions))
	for i, opt := range captureOptions {
		labels[i] = opt.Label
	}
	g.captureSelect = widget.NewSelect(labels, func(value string) {
		newMode := g.modeForLabel(value)

		// 检查是否正在识别中（通过检查开始按钮状态）
		// Check if recognition is running by checking start button state
		if g.startButton != nil && g.startButton.Disabled() {
			// 识别运行中，需要切换音频源
			// Recognition is running, need to switch audio source
			g.handleAudioSourceChange(newMode)
		} else {
			// 未运行，仅更新模式
			// Not running, just update the mode
			g.captureMode = newMode
		}
	})
	g.captureSelect.SetSelected(g.labelForMode(g.captureMode))

	sourceCard := widget.NewCard("", "", container.NewVBox(
		sourceLabel,
		g.captureSelect,
	))

	// 快捷操作卡片
	quickActionsLabel := widget.NewLabelWithStyle("快捷操作", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// 保存侧边栏启动按钮引用，使用切换回调
	// Save sidebar start button reference, using toggle callback
	g.sidebarStartButton = widget.NewButton("启动识别", g.toggleRecognition)
	g.sidebarStartButton.Importance = widget.HighImportance

	g.sidebarAskButton = widget.NewButton("获取建议", g.askAI)
	g.sidebarAskButton.Disable()

	clearBtn := widget.NewButton("清空记录", g.clear)

	quickActionsCard := widget.NewCard("", "", container.NewVBox(
		quickActionsLabel,
		g.sidebarStartButton,
		g.sidebarAskButton,
		clearBtn,
	))

	// 使用提示卡片
	infoLabel := widget.NewLabel("💡 提示")
	infoLabel.TextStyle = fyne.TextStyle{Bold: true}

	infoText := widget.NewLabel("1. 点击「启动识别」开始\n2. 等待面试官提问\n3. 点击「获取建议」生成回答")
	infoText.Wrapping = fyne.TextWrapWord
	infoText.Importance = widget.LowImportance

	infoCard := widget.NewCard("", "", container.NewVBox(
		infoLabel,
		infoText,
	))

	// 状态信息卡片
	statusLabel := widget.NewLabel("📊 状态")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	statusText := widget.NewLabelWithData(g.status)
	statusText.Wrapping = fyne.TextWrapWord
	statusText.Importance = widget.MediumImportance

	statusCard := widget.NewCard("", "", container.NewVBox(
		statusLabel,
		statusText,
	))

	sidebar := container.NewVBox(
		sourceCard,
		quickActionsCard,
		infoCard,
		statusCard,
	)

	// 固定宽度的侧边栏容器
	sidebarBox := container.NewPadded(sidebar)

	// 使用固定大小容器
	sidebarWithWidth := container.NewMax(sidebarBox)
	sidebarWithWidth.Resize(fyne.NewSize(280, 0))

	return sidebarWithWidth
}

// buildMainContent 构建主内容区域
func (g *guiApp) buildMainContent() fyne.CanvasObject {
	// 面试官问题区域 - 左侧
	interviewerLabel := widget.NewLabelWithData(g.interviewer)
	interviewerLabel.Wrapping = fyne.TextWrapWord
	interviewerContent := container.NewVScroll(interviewerLabel)

	interviewerCard := appwidgets.NewModernCard(
		"👔 面试官问题",
		"实时语音识别",
		interviewerContent,
		color.NRGBA{R: 59, G: 130, B: 246, A: 255},
	)

	// AI 建议区域 - 右侧
	aiLabel := widget.NewLabelWithData(g.ai)
	aiLabel.Wrapping = fyne.TextWrapWord
	aiContent := container.NewVScroll(aiLabel)

	aiCard := appwidgets.NewGradientCard(
		"🤖 AI 建议",
		aiContent,
		color.NRGBA{R: 88, G: 101, B: 242, A: 255},
		color.NRGBA{R: 129, G: 140, B: 248, A: 255},
	)

	// 采用左右对照布局,方便同时查看问题和建议
	// 使用Split容器实现可调整大小的分栏
	split := container.NewHSplit(
		interviewerCard,
		aiCard,
	)

	// 设置初始分割比例为 50:50
	split.Offset = 0.5

	return container.NewPadded(split)
}

// buildModernStatusBar 已移除 - 状态信息集成到侧边栏中

// toggleRecognition 切换识别状态（启动或停止）
// Toggle recognition state (start or stop)
func (g *guiApp) toggleRecognition() {
	if g.isRecognizing {
		g.stopRecognition()
	} else {
		g.startRecognition()
	}
}

// updateRecognitionButtons 更新识别按钮的文本和图标
// Update recognition button text and icons
func (g *guiApp) updateRecognitionButtons() {
	g.runOnMain(func() {
		if g.isRecognizing {
			// 识别运行中：显示停止按钮
			// Recognition running: show stop button
			g.startButton.SetIcon(theme.MediaStopIcon())
			if g.sidebarStartButton != nil {
				g.sidebarStartButton.SetText("停止识别")
			}
		} else {
			// 识别未运行：显示启动按钮
			// Recognition not running: show start button
			g.startButton.SetIcon(theme.MediaPlayIcon())
			if g.sidebarStartButton != nil {
				g.sidebarStartButton.SetText("启动识别")
			}
		}
		g.startButton.Refresh()
		if g.sidebarStartButton != nil {
			g.sidebarStartButton.Refresh()
		}
	})
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

	// 设置识别状态并更新按钮
	// Set recognition state and update buttons
	g.isRecognizing = true
	g.updateRecognitionButtons()

	// 启用 AI 建议按钮
	// Enable AI suggestion buttons
	g.askButton.Enable()
	if g.sidebarAskButton != nil {
		g.sidebarAskButton.Enable()
	}
	// 保持音频源选择器启用状态，允许运行时切换
	// Keep audio source selector enabled to allow runtime switching

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
}

// stopRecognition 停止语音识别
// Stop speech recognition
func (g *guiApp) stopRecognition() {
	if !g.isRecognizing {
		return // 已经停止，避免重复操作
	}

	g.logger.Info("停止语音识别")
	g.setStatus("正在停止识别...")

	// 取消上下文
	// Cancel context
	if g.cancel != nil {
		g.cancel()
	}

	// 停止流水线
	// Stop pipeline
	if g.pipeline != nil {
		if err := g.pipeline.Stop(); err != nil {
			g.logger.Error("停止流水线失败", zap.Error(err))
			g.showError("停止流水线失败", err)
		}
		g.pipeline = nil
	}

	// 更新状态
	// Update state
	g.isRecognizing = false
	g.updateRecognitionButtons()

	// 禁用 AI 建议按钮
	// Disable AI suggestion buttons
	g.runOnMain(func() {
		if g.askButton != nil {
			g.askButton.Disable()
			g.askButton.Refresh()
		}
		if g.sidebarAskButton != nil {
			g.sidebarAskButton.Disable()
			g.sidebarAskButton.Refresh()
		}
	})

	g.setStatus("识别已停止")
}

// handleAudioSourceChange 处理运行时音频源切换
// Handle audio source switching during runtime
func (g *guiApp) handleAudioSourceChange(newMode audio.CaptureMode) {
	g.logger.Info("切换音频源", zap.String("new_mode", string(newMode)))
	g.setStatus(fmt.Sprintf("正在切换音频源至: %s", g.labelForMode(newMode)))

	// 停止当前流水线
	// Stop current pipeline
	if g.pipeline != nil {
		if err := g.pipeline.Stop(); err != nil {
			g.logger.Error("停止流水线失败", zap.Error(err))
			g.showError("停止流水线失败", err)
			return
		}
		g.pipeline = nil
	}

	// 取消当前上下文
	// Cancel current context
	if g.cancel != nil {
		g.cancel()
	}

	// 更新音频模式
	// Update audio mode
	g.captureMode = newMode

	// 短暂延迟确保清理完成
	// Brief delay to ensure cleanup completes
	time.Sleep(100 * time.Millisecond)

	// 使用新的音频源重启识别
	// Restart recognition with new audio source
	g.startRecognition()
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
	if g.sidebarAskButton != nil {
		g.sidebarAskButton.Disable()
	}
	g.setStatus("AI 正在生成回答建议")
	_ = g.ai.Set("💭 AI 正在思考…\n")

	go func() {
		ctx, cancel := context.WithTimeout(g.ctxOrBackground(), 2*time.Minute)
		defer cancel()

		// 构建包含历史对话的完整提示词
		// Build complete prompt with conversation history
		prompt := g.buildPromptWithHistory(question)

		_, err := g.llmProvider.Assistant.StreamAndAppend(ctx, prompt, func(chunk string, done bool) error {
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
			if g.sidebarAskButton != nil {
				g.sidebarAskButton.Enable()
				g.sidebarAskButton.Refresh()
			}
		})
	}()
}

// buildPromptWithHistory 构建包含历史对话的完整提示词
// Build complete prompt with conversation history
func (g *guiApp) buildPromptWithHistory(currentQuestion types.ASRResult) string {
	contextCount := g.cfg.LLM.ContextMessages
	if contextCount <= 0 {
		// 如果配置为0或负数，只发送当前问题
		return currentQuestion.Text
	}

	// 获取历史问题（不包括当前问题，因为它已经在最后）
	history := g.session.GetRecentQuestions(contextCount)
	if len(history) <= 1 {
		// 只有当前问题或没有历史，直接返回当前问题
		return currentQuestion.Text
	}

	// 构建提示词
	var prompt strings.Builder
	prompt.WriteString("以下是面试对话历史（按时间顺序）：\n\n")

	// 添加历史问题（除了最后一条，因为最后一条是当前问题）
	for i := 0; i < len(history)-1; i++ {
		timestamp := history[i].Completed.Format("15:04:05")
		prompt.WriteString(fmt.Sprintf("[%s] 面试官: %s\n", timestamp, history[i].Text))
	}

	prompt.WriteString(fmt.Sprintf("\n当前问题:\n面试官: %s\n\n", currentQuestion.Text))
	prompt.WriteString("请基于以上对话历史和当前问题，为候选人生成回答建议。")

	return prompt.String()
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

// enableWindowProtection 启用窗口保护，防止屏幕共享时被捕获
// Enable window protection to prevent capture during screen sharing
func (g *guiApp) enableWindowProtection() {
	// 延迟确保窗口已完全创建
	// Delay to ensure window is fully created
	time.Sleep(500 * time.Millisecond)

	if !windowprotect.IsSupported() {
		g.logger.Info("当前平台不支持窗口保护功能")
		return
	}

	windowTitle := "面试助手 - Go 版"
	if err := windowprotect.ProtectWindow(windowTitle, g.logger); err != nil {
		g.logger.Warn("窗口保护启用失败", zap.Error(err))
		// 不中断程序运行，仅记录警告
		return
	}

	g.logger.Info("✅ 窗口保护已启用", zap.String("info", "屏幕共享时窗口将不可见"))
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
	history     []types.ASRResult // 面试官问题历史
}

func (s *state) SetSpeaker(res types.ASRResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSpeaker = res
	s.hasSpeaker = true
	// 添加到历史记录
	s.history = append(s.history, res)
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
	s.history = nil // 清空历史记录
}

// GetRecentQuestions 获取最近 n 条面试官问题
// Get recent n interviewer questions
func (s *state) GetRecentQuestions(n int) []types.ASRResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 || len(s.history) == 0 {
		return nil
	}

	// 获取最近n条，如果历史记录不足n条，返回全部
	start := len(s.history) - n
	if start < 0 {
		start = 0
	}

	// 返回副本，避免并发问题
	result := make([]types.ASRResult, len(s.history[start:]))
	copy(result, s.history[start:])
	return result
}

// handleConfigUpdate 处理配置更新
func (g *guiApp) handleConfigUpdate(newConfig *config.Config) error {
	g.logger.Info("配置更新请求")

	// 验证新配置
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 判断是否需要重启服务
	needsRestart := g.configNeedsRestart(newConfig)

	// 更新配置
	g.cfg = newConfig
	g.container.Config = newConfig

	if needsRestart && g.pipeline != nil {
		// 如果语音识别正在运行,需要重启
		g.runOnMain(func() {
			dialog.ShowConfirm(
				"需要重启服务",
				"配置已更新,但需要重启语音识别服务才能生效。是否立即重启?",
				func(restart bool) {
					if restart {
						g.restartPipeline()
					}
				},
				g.window,
			)
		})
		g.setStatus("配置已更新,请重启语音识别服务使其生效")
	} else {
		// 立即生效的配置更新
		if g.llmProvider != nil {
			// 重新初始化 LLM 提供商
			provider, err := llm.NewProvider(g.cfg.LLM, g.logger.Named("llm"))
			if err != nil {
				g.logger.Error("重新初始化 LLM 失败", zap.Error(err))
				return fmt.Errorf("LLM 初始化失败: %w", err)
			}
			g.llmProvider = provider
			g.logger.Info("LLM 提供商已更新")
		}

		g.setStatus("配置已更新并生效")
	}

	return nil
}

// configNeedsRestart 判断配置更新是否需要重启服务
func (g *guiApp) configNeedsRestart(newConfig *config.Config) bool {
	if g.cfg.ASR.Active != newConfig.ASR.Active {
		return true
	}
	if g.cfg.Audio.SilenceThreshold != newConfig.Audio.SilenceThreshold {
		return true
	}
	if g.cfg.Audio.SilenceDuration != newConfig.Audio.SilenceDuration {
		return true
	}
	return false
}

// restartPipeline 重启语音识别流水线
func (g *guiApp) restartPipeline() {
	g.setStatus("正在重启语音识别服务...")

	// 停止当前服务
	if g.cancel != nil {
		g.cancel()
	}
	if g.pipeline != nil {
		_ = g.pipeline.Stop()
		g.pipeline = nil
	}

	// 等待服务完全停止
	time.Sleep(500 * time.Millisecond)

	// 重新启动
	g.startRecognition()
}
