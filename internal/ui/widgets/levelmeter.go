package widgets

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// LevelMeter 显示音频电平的可视化组件
type LevelMeter struct {
	widget.BaseWidget

	label    string
	level    float64
	maxLevel float64

	labelText    *canvas.Text
	progressBar  *canvas.Rectangle
	background   *canvas.Rectangle
	peakIndicator *canvas.Rectangle

	mu sync.RWMutex
}

// NewLevelMeter 创建新的音频电平指示器
func NewLevelMeter(label string) *LevelMeter {
	meter := &LevelMeter{
		label:    label,
		maxLevel: 1.0,
	}

	meter.labelText = canvas.NewText(label, color.NRGBA{R: 120, G: 133, B: 150, A: 255})
	meter.labelText.TextSize = 12
	meter.labelText.TextStyle = fyne.TextStyle{Bold: true}

	meter.background = canvas.NewRectangle(color.NRGBA{R: 230, G: 230, B: 230, A: 255})
	meter.background.CornerRadius = 3

	meter.progressBar = canvas.NewRectangle(color.NRGBA{R: 82, G: 196, B: 26, A: 255})
	meter.progressBar.CornerRadius = 3

	meter.peakIndicator = canvas.NewRectangle(color.NRGBA{R: 245, G: 34, B: 45, A: 255})
	meter.peakIndicator.CornerRadius = 2
	meter.peakIndicator.Hidden = true

	meter.ExtendBaseWidget(meter)
	return meter
}

// SetLevel 设置当前音频电平 (0.0 - 1.0)
func (m *LevelMeter) SetLevel(level float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if level < 0 {
		level = 0
	} else if level > 1 {
		level = 1
	}

	m.level = level

	if level > m.maxLevel {
		m.maxLevel = level
		m.peakIndicator.Hidden = false
	}

	// 根据电平设置颜色
	var barColor color.NRGBA
	if level > 0.8 {
		barColor = color.NRGBA{R: 245, G: 34, B: 45, A: 255} // 红色 - 过高
	} else if level > 0.6 {
		barColor = color.NRGBA{R: 250, G: 173, B: 20, A: 255} // 橙色 - 较高
	} else {
		barColor = color.NRGBA{R: 82, G: 196, B: 26, A: 255} // 绿色 - 正常
	}
	m.progressBar.FillColor = barColor

	m.Refresh()
}

// GetLevel 获取当前电平
func (m *LevelMeter) GetLevel() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.level
}

// ResetPeak 重置峰值指示器
func (m *LevelMeter) ResetPeak() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxLevel = m.level
	m.peakIndicator.Hidden = true
	m.Refresh()
}

// CreateRenderer 实现 fyne.Widget 接口
func (m *LevelMeter) CreateRenderer() fyne.WidgetRenderer {
	return &levelMeterRenderer{
		meter:    m,
		objects: []fyne.CanvasObject{
			m.labelText,
			m.background,
			m.progressBar,
			m.peakIndicator,
		},
	}
}

type levelMeterRenderer struct {
	meter   *LevelMeter
	objects []fyne.CanvasObject
}

func (r *levelMeterRenderer) Layout(size fyne.Size) {
	labelHeight := float32(16)
	barHeight := float32(8)
	spacing := float32(4)

	// 标签位置
	r.meter.labelText.Move(fyne.NewPos(0, 0))
	r.meter.labelText.Resize(fyne.NewSize(size.Width, labelHeight))

	// 背景条位置
	barTop := labelHeight + spacing
	r.meter.background.Move(fyne.NewPos(0, barTop))
	r.meter.background.Resize(fyne.NewSize(size.Width, barHeight))

	// 进度条位置 (根据电平计算宽度)
	r.meter.mu.RLock()
	level := r.meter.level
	maxLevel := r.meter.maxLevel
	r.meter.mu.RUnlock()

	progressWidth := float32(level) * size.Width
	r.meter.progressBar.Move(fyne.NewPos(0, barTop))
	r.meter.progressBar.Resize(fyne.NewSize(progressWidth, barHeight))

	// 峰值指示器位置
	if maxLevel > 0 {
		peakPos := float32(maxLevel) * size.Width
		if peakPos > size.Width-2 {
			peakPos = size.Width - 2
		}
		r.meter.peakIndicator.Move(fyne.NewPos(peakPos, barTop))
		r.meter.peakIndicator.Resize(fyne.NewSize(2, barHeight))
	}
}

func (r *levelMeterRenderer) MinSize() fyne.Size {
	return fyne.NewSize(150, 28)
}

func (r *levelMeterRenderer) Refresh() {
	r.Layout(r.meter.Size())
	canvas.Refresh(r.meter)
}

func (r *levelMeterRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *levelMeterRenderer) Destroy() {}
