package main

import (
	"context"
	"fmt"
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
	"go.uber.org/zap"

	"interviewai/internal/app"
	"interviewai/internal/asr"
	"interviewai/internal/audio"
	"interviewai/internal/config"
	"interviewai/internal/llm"
	"interviewai/internal/pipeline"
	"interviewai/internal/types"
	apptheme "interviewai/internal/ui/theme"
	"interviewai/internal/ui/widgets"
)

var captureOptions = []struct {
	Label string
	Mode  audio.CaptureMode
}{
	{Label: "扬声器 + 麦克风", Mode: audio.CaptureModeBoth},
	{Label: "仅扬声器", Mode: audio.CaptureModeSpeakerOnly},
	{Label: "仅麦克风", Mode: audio.CaptureModeMicrophoneOnly},
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

// ImprovedGUI 改进版GUI应用
type ImprovedGUI struct {
	app       fyne.App
	window    fyne.Window
	container *app.Container
	cfg       *config.Config
	logger    *zap.Logger

	pipeline    *pipeline.Manager
	llmProvider *llm.Provider

	ctx    context.Context
	cancel context.CancelFunc

	// UI组件
	messageList    *widgets.MessageList
	aiText         binding.String
	micMeter       *widgets.LevelMeter
	speakerMeter   *widgets.LevelMeter
	currentQ       *widget.Label
	statusLabel    *widget.Label
	statsLabel     *widget.Label
	durationLabel  *widget.Label

	captureMode   audio.CaptureMode
	captureSelect *widget.Select

	startButton   *widget.Button
	pauseButton   *widget.Button
	stopButton    *widget.Button
	askButton     *widget.Button
	regenButton   *widget.Button
	copyButton    *widget.Button
	exportButton  *widget.Button
	clearButton   *widget.Button

	// 状态
	session     *state
	isRunning   bool
	isPaused    bool
	startTime   time.Time
	messageCount int

	textMu sync.Mutex
}

func newImprovedGUI(container *app.Container) *ImprovedGUI {
	aiText := binding.NewString()
	_ = aiText.Set("点击「获取 AI 建议」按钮获取回答建议")

	return &ImprovedGUI{
		container:   container,
		cfg:         container.Config,
		logger:      container.Logger.Named("gui"),
		aiText:      aiText,
		session:     &state{},
		captureMode: audio.CaptureModeBoth,
	}
}

func (g *ImprovedGUI) run() error {
	g.app = fyneApp.NewWithID("interview-ai-improved")
	g.app.Settings().SetTheme(apptheme.New())
	g.window = g.app.NewWindow("🎙️ 面试智能助手")

	g.buildUI()

	g.window.Resize(fyne.NewSize(float32(g.cfg.GUI.Width), float32(g.cfg.GUI.Height)))
	g.window.SetCloseIntercept(func() {
		g.shutdown()
		g.window.Close()
	})

	// 启动音频电平更新定时器
	go g.updateAudioLevels()

	g.window.ShowAndRun()
	return nil
}

func (g *ImprovedGUI) buildUI() {
	// 顶部控制面板
	controlPanel := g.buildControlPanel()

	// 主内容区域
	content := g.buildContentArea()

	// 底部状态栏
	statusBar := g.buildStatusBar()

	// 组合布局
	mainLayout := container.NewBorder(
		controlPanel,
		statusBar,
		nil,
		nil,
		content,
	)

	g.window.SetContent(mainLayout)
}

func (g *ImprovedGUI) buildControlPanel() fyne.CanvasObject {
	// 音频捕获选择
	labels := make([]string, len(captureOptions))
	for i, opt := range captureOptions {
		labels[i] = opt.Label
	}
	g.captureSelect = widget.NewSelect(labels, func(value string) {
		g.captureMode = g.modeForLabel(value)
	})
	g.captureSelect.SetSelected(g.labelForMode(g.captureMode))

	captureLabel := widget.NewLabel("🎙️ 音频捕获")
	captureLabel.TextStyle = fyne.TextStyle{Bold: true}

	// 音频电平指示器
	g.micMeter = widgets.NewLevelMeter("麦克风")
	g.speakerMeter = widgets.NewLevelMeter("扬声器")

	audioControl := container.NewVBox(
		container.NewHBox(
			captureLabel,
			layout.NewSpacer(),
			g.captureSelect,
		),
		container.NewGridWithColumns(2,
			g.micMeter,
			g.speakerMeter,
		),
	)

	// 控制按钮
	g.startButton = widget.NewButtonWithIcon("启动识别", theme.MediaPlayIcon(), g.startRecognition)
	g.startButton.Importance = widget.HighImportance

	g.pauseButton = widget.NewButtonWithIcon("暂停", theme.MediaPauseIcon(), g.pauseRecognition)
	g.pauseButton.Importance = widget.MediumImportance
	g.pauseButton.Disable()

	g.stopButton = widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), g.stopRecognition)
	g.stopButton.Importance = widget.DangerImportance
	g.stopButton.Disable()

	g.exportButton = widget.NewButtonWithIcon("导出", theme.DocumentSaveIcon(), g.exportSession)

	buttons := container.NewHBox(
		g.startButton,
		g.pauseButton,
		g.stopButton,
		layout.NewSpacer(),
		g.exportButton,
	)

	// 组合控制面板
	panel := container.NewVBox(
		audioControl,
		widget.NewSeparator(),
		buttons,
	)

	return container.NewPadded(panel)
}

func (g *ImprovedGUI) buildContentArea() fyne.CanvasObject {
	// 左侧:对话历史
	g.messageList = widgets.NewMessageList()

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("🔍 搜索对话...")

	g.clearButton = widget.NewButtonWithIcon("", theme.DeleteIcon(), g.clear)
	g.clearButton.Importance = widget.LowImportance

	conversationHeader := container.NewBorder(
		nil,
		nil,
		nil,
		g.clearButton,
		searchEntry,
	)

	conversationPanel := container.NewBorder(
		conversationHeader,
		nil,
		nil,
		nil,
		g.messageList,
	)

	conversationCard := widget.NewCard("💬 对话记录", "", conversationPanel)

	// 右侧: AI助手
	g.currentQ = widget.NewLabel("等待面试官提问...")
	g.currentQ.Wrapping = fyne.TextWrapWord
	g.currentQ.TextStyle = fyne.TextStyle{Bold: true}

	currentQCard := widget.NewCard("💡 当前问题", "", container.NewPadded(g.currentQ))

	aiLabel := widget.NewLabelWithData(g.aiText)
	aiLabel.Wrapping = fyne.TextWrapWord

	aiScroll := container.NewVScroll(aiLabel)
	aiScroll.SetMinSize(fyne.NewSize(400, 200))

	g.askButton = widget.NewButtonWithIcon("获取 AI 建议", theme.MailSendIcon(), g.askAI)
	g.askButton.Importance = widget.HighImportance
	g.askButton.Disable()

	g.regenButton = widget.NewButtonWithIcon("重新生成", theme.ViewRefreshIcon(), g.regenerateAI)
	g.regenButton.Disable()

	g.copyButton = widget.NewButtonWithIcon("复制", theme.ContentCopyIcon(), g.copyAISuggestion)

	aiButtons := container.NewHBox(
		g.askButton,
		g.regenButton,
		layout.NewSpacer(),
		g.copyButton,
	)

	aiContent := container.NewBorder(
		nil,
		aiButtons,
		nil,
		nil,
		aiScroll,
	)

	assistantPanel := container.NewVBox(
		currentQCard,
		widget.NewCard("🤖 AI 建议", "", aiContent),
	)

	// 左右分栏布局
	content := container.NewHSplit(
		conversationCard,
		assistantPanel,
	)
	content.SetOffset(0.5)

	return content
}

func (g *ImprovedGUI) buildStatusBar() fyne.CanvasObject {
	g.statusLabel = widget.NewLabel("⚪ 就绪")
	g.durationLabel = widget.NewLabel("⏱️ 00:00:00")
	g.statsLabel = widget.NewLabel("💬 0条对话")

	statusBar := container.NewHBox(
		g.statusLabel,
		widget.NewSeparator(),
		g.durationLabel,
		widget.NewSeparator(),
		g.statsLabel,
		layout.NewSpacer(),
	)

	return container.NewPadded(statusBar)
}

func (g *ImprovedGUI) startRecognition() {
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

	g.isRunning = true
	g.isPaused = false
	g.startTime = time.Now()

	g.startButton.Disable()
	g.pauseButton.Enable()
	g.stopButton.Enable()
	g.askButton.Enable()
	g.captureSelect.Disable()

	g.setStatus("🔴 识别中")

	go g.consumeResults()
	go g.consumeErrors()
}

func (g *ImprovedGUI) pauseRecognition() {
	if !g.isRunning || g.pipeline == nil {
		return
	}

	if g.isPaused {
		// 恢复
		g.isPaused = false
		g.pauseButton.SetIcon(theme.MediaPauseIcon())
		g.pauseButton.SetText("暂停")
		g.setStatus("🔴 识别中")
	} else {
		// 暂停
		g.isPaused = true
		g.pauseButton.SetIcon(theme.MediaPlayIcon())
		g.pauseButton.SetText("继续")
		g.setStatus("🟡 已暂停")
	}
}

func (g *ImprovedGUI) stopRecognition() {
	if g.cancel != nil {
		g.cancel()
	}
	if g.pipeline != nil {
		_ = g.pipeline.Stop()
		g.pipeline = nil
	}

	g.isRunning = false
	g.isPaused = false

	g.startButton.Enable()
	g.pauseButton.Disable()
	g.stopButton.Disable()
	g.askButton.Disable()
	g.captureSelect.Enable()

	g.setStatus("⚫ 已停止")
}

func (g *ImprovedGUI) askAI() {
	if g.llmProvider == nil {
		g.setStatus("AI 模块尚未初始化")
		return
	}

	question, ok := g.session.LastSpeaker()
	if !ok {
		g.setStatus("未检测到面试官问题")
		dialog.ShowInformation("提示", "请等待面试官提问后再获取AI建议", g.window)
		return
	}

	g.askButton.Disable()
	g.regenButton.Disable()
	g.setStatus("AI 正在生成回答建议...")
	_ = g.aiText.Set("💭 AI 正在思考...\n")

	go func() {
		ctx, cancel := context.WithTimeout(g.ctxOrBackground(), 2*time.Minute)
		defer cancel()

		_, err := g.llmProvider.Assistant.StreamAndAppend(ctx, question.Text, func(chunk string, done bool) error {
			if chunk != "" {
				g.appendBinding(g.aiText, chunk)
			}
			if done {
				g.appendBinding(g.aiText, "\n")
			}
			return nil
		})

		if err != nil {
			g.appendBinding(g.aiText, fmt.Sprintf("\n❌ AI 生成失败: %v\n", err))
			g.logger.Error("AI 生成失败", zap.Error(err))
			g.setStatus("AI 生成失败")
			dialog.ShowError(fmt.Errorf("AI生成失败: %w", err), g.window)
		} else {
			g.setStatus("AI 建议已生成")
		}

		g.runOnMain(func() {
			g.askButton.Enable()
			g.regenButton.Enable()
		})
	}()
}

func (g *ImprovedGUI) regenerateAI() {
	// 清空当前AI建议并重新生成
	_ = g.aiText.Set("")
	g.askAI()
}

func (g *ImprovedGUI) copyAISuggestion() {
	text, err := g.aiText.Get()
	if err != nil {
		return
	}
	g.window.Clipboard().SetContent(text)
	dialog.ShowInformation("复制成功", "AI建议已复制到剪贴板", g.window)
}

func (g *ImprovedGUI) exportSession() {
	if g.messageCount == 0 {
		dialog.ShowInformation("提示", "当前没有对话记录可导出", g.window)
		return
	}

	content := g.messageList.ExportText()

	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		_, err = writer.Write([]byte(content))
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}

		dialog.ShowInformation("导出成功", "对话记录已保存", g.window)
	}, g.window)

	saveDialog.SetFileName(fmt.Sprintf("面试记录_%s.txt", time.Now().Format("20060102_150405")))
	saveDialog.Show()
}

func (g *ImprovedGUI) clear() {
	if g.messageCount == 0 {
		return
	}

	dialog.ShowConfirm("确认清空", "确定要清空所有对话记录吗?此操作不可恢复。", func(confirmed bool) {
		if confirmed {
			g.messageList.Clear()
			_ = g.aiText.Set("点击「获取 AI 建议」按钮获取回答建议")
			g.currentQ.SetText("等待面试官提问...")
			g.session.Reset()
			g.messageCount = 0
			g.updateStats()
			g.setStatus("已清空历史记录")
		}
	}, g.window)
}

func (g *ImprovedGUI) consumeResults() {
	for {
		select {
		case <-g.ctx.Done():
			return
		case res, ok := <-g.pipeline.Results():
			if !ok {
				return
			}

			if g.isPaused {
				continue
			}

			var msgType widgets.MessageType
			if res.Source == types.SpeechSourceSpeaker {
				msgType = widgets.MessageTypeInterviewer
				g.session.SetSpeaker(res)
				g.runOnMain(func() {
					g.currentQ.SetText(res.Text)
				})
			} else {
				msgType = widgets.MessageTypeUser
			}

			msg := widgets.Message{
				Type:      msgType,
				Text:      res.Text,
				Timestamp: res.Completed,
			}

			g.runOnMain(func() {
				g.messageList.AddMessage(msg)
				g.messageCount++
				g.updateStats()
			})
		}
	}
}

func (g *ImprovedGUI) consumeErrors() {
	for {
		select {
		case <-g.ctx.Done():
			return
		case err, ok := <-g.pipeline.Errors():
			if !ok {
				return
			}
			g.logger.Warn("流水线错误", zap.Error(err))
			g.setStatus(fmt.Sprintf("⚠️ 错误: %v", err))
		}
	}
}

func (g *ImprovedGUI) updateAudioLevels() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if g.pipeline == nil || !g.isRunning || g.isPaused {
			g.runOnMain(func() {
				g.micMeter.SetLevel(0)
				g.speakerMeter.SetLevel(0)
			})
			continue
		}

		// 模拟音频电平 (实际应用应从音频引擎获取)
		// TODO: 集成真实音频电平数据
		micLevel := 0.0
		speakerLevel := 0.0

		g.runOnMain(func() {
			g.micMeter.SetLevel(micLevel)
			g.speakerMeter.SetLevel(speakerLevel)
			g.updateDuration()
		})
	}
}

func (g *ImprovedGUI) updateDuration() {
	if g.isRunning && !g.isPaused {
		duration := time.Since(g.startTime)
		g.durationLabel.SetText(fmt.Sprintf("⏱️ %s", formatDuration(duration)))
	}
}

func (g *ImprovedGUI) updateStats() {
	g.statsLabel.SetText(fmt.Sprintf("💬 %d条对话", g.messageCount))
}

func (g *ImprovedGUI) shutdown() {
	g.stopRecognition()
}

func (g *ImprovedGUI) setStatus(text string) {
	g.runOnMain(func() {
		g.statusLabel.SetText(text)
	})
}

func (g *ImprovedGUI) showError(prefix string, err error) {
	g.logger.Error(prefix, zap.Error(err))
	g.runOnMain(func() {
		dialog.ShowError(fmt.Errorf("%s: %w", prefix, err), g.window)
		g.setStatus(fmt.Sprintf("❌ %s: %v", prefix, err))
	})
}

func (g *ImprovedGUI) ctxOrBackground() context.Context {
	if g.ctx != nil {
		return g.ctx
	}
	return context.Background()
}

func (g *ImprovedGUI) appendBinding(str binding.String, addition string) {
	g.textMu.Lock()
	defer g.textMu.Unlock()

	current, _ := str.Get()
	next := current + addition
	g.runOnMain(func() {
		_ = str.Set(next)
	})
}

func (g *ImprovedGUI) runOnMain(fn func()) {
	if drv := g.app.Driver(); drv != nil {
		if runner, ok := drv.(interface{ RunOnMain(func()) }); ok {
			runner.RunOnMain(fn)
			return
		}
	}
	fn()
}

func (g *ImprovedGUI) modeForLabel(label string) audio.CaptureMode {
	for _, opt := range captureOptions {
		if opt.Label == label {
			return opt.Mode
		}
	}
	return audio.CaptureModeBoth
}

func (g *ImprovedGUI) labelForMode(mode audio.CaptureMode) string {
	for _, opt := range captureOptions {
		if opt.Mode == mode {
			return opt.Label
		}
	}
	return captureOptions[0].Label
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
