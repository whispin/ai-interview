package widgets

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"interviewai/internal/config"
)

// SettingsDialog 设置对话框
type SettingsDialog struct {
	window fyne.Window
	config *config.Config

	// 回调函数
	onSave func(*config.Config) error

	// 绑定数据
	asrProvider       binding.String
	llmProvider       binding.String
	llmSystemPrompt   binding.String
	llmTemperature    binding.String
	llmContextMessages binding.String  // 历史对话条数
	silenceThreshold  binding.String
	silenceDuration   binding.String
	logLevel          binding.String

	// UI 组件
	dialog dialog.Dialog

	// Tencent ASR 配置组件
	tencentSecretIDEntry  *widget.Entry
	tencentSecretKeyEntry *widget.Entry
	tencentAppIDEntry     *widget.Entry
	tencentRegionSelect   *widget.Select

	// OpenAI Whisper 配置组件
	whisperAPIKeyEntry  *widget.Entry
	whisperBaseURLEntry *widget.Entry
	whisperModelSelect  *widget.Select
}

// NewSettingsDialog 创建设置对话框
func NewSettingsDialog(w fyne.Window, cfg *config.Config, onSave func(*config.Config) error) *SettingsDialog {
	s := &SettingsDialog{
		window:             w,
		config:             cfg,
		onSave:             onSave,
		asrProvider:        binding.NewString(),
		llmProvider:        binding.NewString(),
		llmSystemPrompt:    binding.NewString(),
		llmTemperature:     binding.NewString(),
		llmContextMessages: binding.NewString(),
		silenceThreshold:   binding.NewString(),
		silenceDuration:    binding.NewString(),
		logLevel:           binding.NewString(),
	}

	s.loadCurrentSettings()
	return s
}

// loadCurrentSettings 从配置加载当前设置
func (s *SettingsDialog) loadCurrentSettings() {
	s.asrProvider.Set(s.config.ASR.Active)
	s.llmProvider.Set(s.config.LLM.Active)
	s.llmSystemPrompt.Set(s.config.LLM.SystemPrompt)

	// 获取当前 LLM 提供商的配置
	if llmCfg, ok := s.config.LLM.Providers[s.config.LLM.Active]; ok {
		s.llmTemperature.Set(fmt.Sprintf("%.2f", llmCfg.Temperature))
	} else {
		s.llmTemperature.Set("0.70")
	}

	// 历史对话条数（默认3条）
	contextCount := s.config.LLM.ContextMessages
	if contextCount <= 0 {
		contextCount = 3
	}
	s.llmContextMessages.Set(fmt.Sprintf("%d", contextCount))

	s.silenceThreshold.Set(fmt.Sprintf("%.2f", s.config.Audio.SilenceThreshold))
	s.silenceDuration.Set(fmt.Sprintf("%.0f", float64(time.Duration(s.config.Audio.SilenceDuration))/float64(time.Millisecond)))
	s.logLevel.Set(s.config.Logging.Level)
}

// Show 显示设置对话框
func (s *SettingsDialog) Show() {
	content := s.buildContent()

	s.dialog = dialog.NewCustomConfirm(
		"⚙️ 设置",
		"保存",
		"取消",
		content,
		func(save bool) {
			if save {
				if err := s.saveSettings(); err != nil {
					dialog.ShowError(fmt.Errorf("保存设置失败: %w", err), s.window)
				}
			}
		},
		s.window,
	)

	// 更大的对话框尺寸,更好的内容展示
	s.dialog.Resize(fyne.NewSize(800, 650))
	s.dialog.Show()
}

// buildContent 构建设置界面内容
func (s *SettingsDialog) buildContent() fyne.CanvasObject {
	// 创建选项卡
	tabs := container.NewAppTabs(
		container.NewTabItem("🎤 语音识别", s.buildASRSettings()),
		container.NewTabItem("🤖 AI 模型", s.buildLLMSettings()),
		container.NewTabItem("🔊 音频设置", s.buildAudioSettings()),
		container.NewTabItem("📝 日志设置", s.buildLoggingSettings()),
	)

	tabs.SetTabLocation(container.TabLocationLeading)

	return container.NewBorder(
		s.buildHeader(),
		nil, nil, nil,
		tabs,
	)
}

// buildHeader 构建头部说明
func (s *SettingsDialog) buildHeader() fyne.CanvasObject {
	headerLabel := widget.NewLabel("应用设置")
	headerLabel.TextStyle = fyne.TextStyle{Bold: true}

	infoLabel := widget.NewLabel("修改设置后点击「保存」按钮使配置生效")
	infoLabel.Importance = widget.LowImportance

	return container.NewPadded(
		container.NewVBox(
			headerLabel,
			infoLabel,
			widget.NewSeparator(),
		),
	)
}

// buildASRSettings 构建语音识别设置
func (s *SettingsDialog) buildASRSettings() fyne.CanvasObject {
	// ASR 提供商选择
	asrProviders := []string{"tencent", "openai_whisper", "http"}
	asrSelect := widget.NewSelect(asrProviders, func(value string) {
		s.asrProvider.Set(value)
	})
	asrSelect.SetSelected(s.config.ASR.Active)

	// 腾讯云配置
	tencentForm := s.buildTencentASRForm()

	// OpenAI Whisper 配置
	whisperForm := s.buildWhisperASRForm()

	// 动态显示对应的配置表单
	providerConfig := container.NewVBox()

	updateProviderConfig := func(provider string) {
		providerConfig.Objects = nil
		switch provider {
		case "tencent":
			card := NewModernCard(
				"腾讯云 ASR 配置",
				"",
				tencentForm,
				color.NRGBA{R: 59, G: 130, B: 246, A: 255},
			)
			providerConfig.Add(card)
		case "openai_whisper":
			card := NewModernCard(
				"OpenAI Whisper 配置",
				"",
				whisperForm,
				color.NRGBA{R: 88, G: 101, B: 242, A: 255},
			)
			providerConfig.Add(card)
		case "http":
			providerConfig.Add(widget.NewLabel("HTTP ASR 配置请在配置文件中编辑"))
		}
		providerConfig.Refresh()
	}

	// 初始显示
	updateProviderConfig(s.config.ASR.Active)

	// 更新选择时刷新配置表单
	asrSelect.OnChanged = func(value string) {
		s.asrProvider.Set(value)
		updateProviderConfig(value)
	}

	return container.NewBorder(
		container.NewVBox(
			widget.NewForm(
				widget.NewFormItem("语音识别提供商", asrSelect),
			),
			widget.NewSeparator(),
		),
		nil, nil, nil,
		container.NewVScroll(providerConfig),
	)
}

// buildTencentASRForm 构建腾讯云 ASR 配置表单
func (s *SettingsDialog) buildTencentASRForm() fyne.CanvasObject {
	s.tencentSecretIDEntry = widget.NewEntry()
	s.tencentSecretIDEntry.SetPlaceHolder("输入腾讯云 Secret ID")
	s.tencentSecretIDEntry.SetText(s.config.ASR.Tencent.SecretID)

	s.tencentSecretKeyEntry = widget.NewPasswordEntry()
	s.tencentSecretKeyEntry.SetPlaceHolder("输入腾讯云 Secret Key")
	s.tencentSecretKeyEntry.SetText(s.config.ASR.Tencent.SecretKey)

	s.tencentAppIDEntry = widget.NewEntry()
	s.tencentAppIDEntry.SetPlaceHolder("输入腾讯云 App ID")
	s.tencentAppIDEntry.SetText(s.config.ASR.Tencent.AppID)

	s.tencentRegionSelect = widget.NewSelect(
		[]string{"ap-shanghai", "ap-guangzhou", "ap-beijing"},
		nil,
	)
	s.tencentRegionSelect.SetSelected(s.config.ASR.Tencent.Region)

	return widget.NewForm(
		widget.NewFormItem("Secret ID", s.tencentSecretIDEntry),
		widget.NewFormItem("Secret Key", s.tencentSecretKeyEntry),
		widget.NewFormItem("App ID", s.tencentAppIDEntry),
		widget.NewFormItem("区域", s.tencentRegionSelect),
	)
}

// buildWhisperASRForm 构建 OpenAI Whisper 配置表单
func (s *SettingsDialog) buildWhisperASRForm() fyne.CanvasObject {
	s.whisperAPIKeyEntry = widget.NewPasswordEntry()
	s.whisperAPIKeyEntry.SetPlaceHolder("输入 OpenAI API Key")
	s.whisperAPIKeyEntry.SetText(s.config.ASR.OpenAIWhisper.APIKey)

	s.whisperBaseURLEntry = widget.NewEntry()
	s.whisperBaseURLEntry.SetPlaceHolder("API Base URL")
	s.whisperBaseURLEntry.SetText(s.config.ASR.OpenAIWhisper.BaseURL)

	s.whisperModelSelect = widget.NewSelect(
		[]string{"whisper-1"},
		nil,
	)
	s.whisperModelSelect.SetSelected(s.config.ASR.OpenAIWhisper.Model)

	return widget.NewForm(
		widget.NewFormItem("API Key", s.whisperAPIKeyEntry),
		widget.NewFormItem("Base URL", s.whisperBaseURLEntry),
		widget.NewFormItem("模型", s.whisperModelSelect),
	)
}

// buildLLMSettings 构建 LLM 设置
func (s *SettingsDialog) buildLLMSettings() fyne.CanvasObject {
	// LLM 提供商选择
	llmProviders := []string{}
	for name := range s.config.LLM.Providers {
		llmProviders = append(llmProviders, name)
	}

	llmSelect := widget.NewSelect(llmProviders, func(value string) {
		s.llmProvider.Set(value)
		// 更新温度值
		if cfg, ok := s.config.LLM.Providers[value]; ok {
			s.llmTemperature.Set(fmt.Sprintf("%.2f", cfg.Temperature))
		}
	})
	llmSelect.SetSelected(s.config.LLM.Active)

	// 系统提示词
	systemPromptEntry := widget.NewMultiLineEntry()
	systemPromptEntry.SetPlaceHolder("输入系统提示词...")
	systemPromptEntry.Wrapping = fyne.TextWrapWord
	systemPromptEntry.SetMinRowsVisible(4)
	systemPromptEntry.Bind(s.llmSystemPrompt)

	// 温度参数
	tempEntry := widget.NewEntry()
	tempEntry.SetPlaceHolder("0.00 - 2.00")
	tempEntry.Bind(s.llmTemperature)

	tempSlider := widget.NewSlider(0, 2)
	tempStr, _ := s.llmTemperature.Get()
	if temp, err := strconv.ParseFloat(tempStr, 64); err == nil {
		tempSlider.SetValue(temp)
	}
	tempSlider.Step = 0.1
	tempSlider.OnChanged = func(value float64) {
		s.llmTemperature.Set(fmt.Sprintf("%.2f", value))
	}

	tempControl := container.NewBorder(nil, nil, nil, tempEntry, tempSlider)

	// 历史对话条数
	contextEntry := widget.NewEntry()
	contextEntry.SetPlaceHolder("1 - 10")
	contextEntry.Bind(s.llmContextMessages)

	contextHelp := widget.NewLabel("发送给AI的历史对话轮数，帮助AI理解完整上下文（建议3-5条）")
	contextHelp.Wrapping = fyne.TextWrapWord
	contextHelp.Importance = widget.LowImportance

	contextControl := container.NewVBox(
		contextEntry,
		contextHelp,
	)

	// 提供商详细配置
	currentProvider := s.config.LLM.Active
	var apiKeyEntry *widget.Entry
	var baseURLEntry *widget.Entry
	var modelEntry *widget.Entry

	if cfg, ok := s.config.LLM.Providers[currentProvider]; ok {
		apiKeyEntry = widget.NewPasswordEntry()
		apiKeyEntry.SetText(cfg.APIKey)
		apiKeyEntry.SetPlaceHolder("输入 API Key")

		baseURLEntry = widget.NewEntry()
		baseURLEntry.SetText(cfg.BaseURL)
		baseURLEntry.SetPlaceHolder("输入 Base URL")

		modelEntry = widget.NewEntry()
		modelEntry.SetText(cfg.Model)
		modelEntry.SetPlaceHolder("输入模型名称")
	}

	providerForm := widget.NewForm(
		widget.NewFormItem("API Key", apiKeyEntry),
		widget.NewFormItem("Base URL", baseURLEntry),
		widget.NewFormItem("模型", modelEntry),
	)

	form := widget.NewForm(
		widget.NewFormItem("LLM 提供商", llmSelect),
		widget.NewFormItem("系统提示词", systemPromptEntry),
		widget.NewFormItem("温度参数", tempControl),
		widget.NewFormItem("历史对话条数", contextControl),
	)

	// 提供商配置卡片 - 使用ModernCard
	providerCard := NewModernCard(
		"提供商配置",
		"当前LLM提供商的详细配置",
		providerForm,
		color.NRGBA{R: 88, G: 101, B: 242, A: 255},
	)

	return container.NewVScroll(
		container.NewVBox(
			form,
			widget.NewSeparator(),
			providerCard,
		),
	)
}

// buildAudioSettings 构建音频设置
func (s *SettingsDialog) buildAudioSettings() fyne.CanvasObject {
	// ========== 静音检测阈值 ==========
	thresholdLabel := widget.NewLabel("🔇 静音检测阈值")
	thresholdLabel.TextStyle = fyne.TextStyle{Bold: true}

	thresholdHelp := widget.NewLabel("音量低于此阈值将被视为静音 (0=完全静音, 1=最大音量)")
	thresholdHelp.Wrapping = fyne.TextWrapWord
	thresholdHelp.Importance = widget.LowImportance

	thresholdEntry := widget.NewEntry()
	thresholdEntry.SetPlaceHolder("0.00 - 1.00")
	thresholdEntry.Bind(s.silenceThreshold)

	thresholdSlider := widget.NewSlider(0, 1)
	thresholdSlider.SetValue(s.config.Audio.SilenceThreshold)
	thresholdSlider.Step = 0.01
	thresholdSlider.OnChanged = func(value float64) {
		s.silenceThreshold.Set(fmt.Sprintf("%.2f", value))
	}

	thresholdControl := container.NewBorder(nil, nil, nil,
		container.NewHBox(thresholdEntry, widget.NewLabel("")),
		thresholdSlider,
	)

	// ========== 静音持续时间 ==========
	durationLabel := widget.NewLabel("⏱️ 静音持续时间")
	durationLabel.TextStyle = fyne.TextStyle{Bold: true}

	durationHelp := widget.NewLabel("检测到静音后等待多久才结束当前语音片段")
	durationHelp.Wrapping = fyne.TextWrapWord
	durationHelp.Importance = widget.LowImportance

	durationEntry := widget.NewEntry()
	durationEntry.SetPlaceHolder("毫秒 (ms)")
	durationEntry.Bind(s.silenceDuration)

	durationSlider := widget.NewSlider(100, 3000)
	durationSlider.SetValue(float64(time.Duration(s.config.Audio.SilenceDuration)) / float64(time.Millisecond))
	durationSlider.Step = 100
	durationSlider.OnChanged = func(value float64) {
		s.silenceDuration.Set(fmt.Sprintf("%.0f", value))
	}

	durationControl := container.NewBorder(nil, nil, nil,
		container.NewHBox(durationEntry, widget.NewLabel("ms")),
		durationSlider,
	)

	// ========== 组合布局 ==========
	form := container.NewVBox(
		thresholdLabel,
		thresholdHelp,
		thresholdControl,
		widget.NewSeparator(),
		durationLabel,
		durationHelp,
		durationControl,
	)

	return container.NewPadded(form)
}

// buildLoggingSettings 构建日志设置
func (s *SettingsDialog) buildLoggingSettings() fyne.CanvasObject {
	logLevels := []string{"debug", "info", "warn", "error"}

	levelSelect := widget.NewSelect(logLevels, func(value string) {
		s.logLevel.Set(value)
	})
	levelSelect.SetSelected(s.config.Logging.Level)

	levelHelp := widget.NewLabel("日志级别越低,输出的日志信息越详细")
	levelHelp.Wrapping = fyne.TextWrapWord
	levelHelp.Importance = widget.LowImportance

	form := widget.NewForm(
		widget.NewFormItem("日志级别", levelSelect),
	)

	return container.NewPadded(
		container.NewVBox(
			form,
			levelHelp,
		),
	)
}

// saveSettings 保存设置
func (s *SettingsDialog) saveSettings() error {
	// 更新配置结构
	newConfig := *s.config

	// ASR 设置
	asrProv, _ := s.asrProvider.Get()
	newConfig.ASR.Active = asrProv

	// 保存 Tencent ASR 配置
	if s.tencentSecretIDEntry != nil {
		newConfig.ASR.Tencent.SecretID = s.tencentSecretIDEntry.Text
	}
	if s.tencentSecretKeyEntry != nil {
		newConfig.ASR.Tencent.SecretKey = s.tencentSecretKeyEntry.Text
	}
	if s.tencentAppIDEntry != nil {
		newConfig.ASR.Tencent.AppID = s.tencentAppIDEntry.Text
	}
	if s.tencentRegionSelect != nil {
		newConfig.ASR.Tencent.Region = s.tencentRegionSelect.Selected
	}

	// 保存 OpenAI Whisper 配置
	if s.whisperAPIKeyEntry != nil {
		newConfig.ASR.OpenAIWhisper.APIKey = s.whisperAPIKeyEntry.Text
	}
	if s.whisperBaseURLEntry != nil {
		newConfig.ASR.OpenAIWhisper.BaseURL = s.whisperBaseURLEntry.Text
	}
	if s.whisperModelSelect != nil {
		newConfig.ASR.OpenAIWhisper.Model = s.whisperModelSelect.Selected
	}

	// LLM 设置
	llmProv, _ := s.llmProvider.Get()
	newConfig.LLM.Active = llmProv

	sysPrompt, _ := s.llmSystemPrompt.Get()
	newConfig.LLM.SystemPrompt = sysPrompt

	tempStr, _ := s.llmTemperature.Get()
	if temp, err := strconv.ParseFloat(tempStr, 32); err == nil {
		if provCfg, ok := newConfig.LLM.Providers[llmProv]; ok {
			provCfg.Temperature = float32(temp)
			newConfig.LLM.Providers[llmProv] = provCfg
		}
	}

	// 历史对话条数
	contextStr, _ := s.llmContextMessages.Get()
	if contextCount, err := strconv.Atoi(contextStr); err == nil {
		if contextCount < 0 {
			contextCount = 0
		} else if contextCount > 10 {
			contextCount = 10
		}
		newConfig.LLM.ContextMessages = contextCount
	}

	// 音频设置
	threshStr, _ := s.silenceThreshold.Get()
	if thresh, err := strconv.ParseFloat(threshStr, 64); err == nil {
		newConfig.Audio.SilenceThreshold = thresh
	}

	durStr, _ := s.silenceDuration.Get()
	if dur, err := strconv.ParseFloat(durStr, 64); err == nil {
		newConfig.Audio.SilenceDuration = config.Duration(int64(dur) * 1000000)
	}

	// 日志设置
	level, _ := s.logLevel.Get()
	newConfig.Logging.Level = level

	// 验证配置
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 调用保存回调
	if s.onSave != nil {
		if err := s.onSave(&newConfig); err != nil {
			return err
		}
	}

	// 更新本地配置引用
	*s.config = newConfig

	return nil
}

// SettingsButton 创建设置按钮
func NewSettingsButton(w fyne.Window, cfg *config.Config, onSave func(*config.Config) error) *widget.Button {
	return widget.NewButtonWithIcon("设置", theme.SettingsIcon(), func() {
		dialog := NewSettingsDialog(w, cfg, onSave)
		dialog.Show()
	})
}
