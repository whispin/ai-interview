package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ModernCard 现代化卡片组件，具有玻璃态效果和渐变
type ModernCard struct {
	widget.BaseWidget

	title       string
	subtitle    string
	content     fyne.CanvasObject
	accentColor color.Color

	titleLabel    *widget.Label
	subtitleLabel *widget.Label
	background    *canvas.Rectangle
	border        *canvas.Rectangle
	accentBar     *canvas.Rectangle
	container     *fyne.Container
}

// NewModernCard 创建新的现代化卡片
func NewModernCard(title, subtitle string, content fyne.CanvasObject, accentColor color.Color) *ModernCard {
	card := &ModernCard{
		title:       title,
		subtitle:    subtitle,
		content:     content,
		accentColor: accentColor,
	}

	// Title with modern styling
	card.titleLabel = widget.NewLabel(title)
	card.titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	card.titleLabel.Importance = widget.HighImportance

	// Subtitle with secondary styling
	card.subtitleLabel = widget.NewLabel(subtitle)
	card.subtitleLabel.Importance = widget.LowImportance

	// Glassmorphism background with subtle transparency
	card.background = canvas.NewRectangle(color.NRGBA{R: 51, G: 65, B: 85, A: 230})
	card.background.CornerRadius = 16

	// Border for depth
	card.border = canvas.NewRectangle(color.NRGBA{R: 71, G: 85, B: 105, A: 80})
	card.border.CornerRadius = 16
	card.border.StrokeWidth = 1
	card.border.StrokeColor = color.NRGBA{R: 148, G: 163, B: 184, A: 40}

	// Accent bar at top for visual hierarchy
	card.accentBar = canvas.NewRectangle(accentColor)
	card.accentBar.CornerRadius = 16

	// Header section
	header := container.NewVBox(
		card.titleLabel,
	)
	if subtitle != "" {
		header.Add(card.subtitleLabel)
	}

	// Main content area
	contentArea := container.NewPadded(content)

	// Compose layout with layers
	layers := container.NewStack(
		card.background,
		card.border,
	)

	innerContent := container.NewBorder(
		container.NewVBox(
			card.accentBar,
			container.NewPadded(header),
		),
		nil, nil, nil,
		contentArea,
	)

	card.container = container.NewStack(layers, innerContent)

	card.ExtendBaseWidget(card)
	return card
}

// SetTitle 更新卡片标题
func (c *ModernCard) SetTitle(title string) {
	c.title = title
	c.titleLabel.SetText(title)
}

// SetSubtitle 更新卡片副标题
func (c *ModernCard) SetSubtitle(subtitle string) {
	c.subtitle = subtitle
	c.subtitleLabel.SetText(subtitle)
}

// SetContent 更新卡片内容
func (c *ModernCard) SetContent(content fyne.CanvasObject) {
	c.content = content
	c.Refresh()
}

// SetAccentColor 更新强调色
func (c *ModernCard) SetAccentColor(color color.Color) {
	c.accentColor = color
	c.accentBar.FillColor = color
	c.accentBar.Refresh()
}

// CreateRenderer 实现 fyne.Widget 接口
func (c *ModernCard) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.container)
}

// MinSize 计算最小尺寸
func (c *ModernCard) MinSize() fyne.Size {
	return c.container.MinSize()
}

// GradientCard 渐变卡片组件
type GradientCard struct {
	widget.BaseWidget

	title       string
	content     fyne.CanvasObject
	gradient    *canvas.LinearGradient
	container   *fyne.Container
	titleLabel  *widget.Label
}

// NewGradientCard 创建渐变背景卡片
func NewGradientCard(title string, content fyne.CanvasObject, startColor, endColor color.Color) *GradientCard {
	card := &GradientCard{
		title:   title,
		content: content,
	}

	card.titleLabel = widget.NewLabel(title)
	card.titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	card.titleLabel.Importance = widget.HighImportance

	// Create gradient background
	card.gradient = canvas.NewLinearGradient(startColor, endColor, 135) // Diagonal gradient

	// Compose layout
	header := container.NewPadded(card.titleLabel)
	contentArea := container.NewPadded(content)

	innerContent := container.NewBorder(
		header,
		nil, nil, nil,
		contentArea,
	)

	card.container = container.NewStack(card.gradient, innerContent)

	card.ExtendBaseWidget(card)
	return card
}

// SetTitle 更新渐变卡片标题
func (c *GradientCard) SetTitle(title string) {
	c.title = title
	c.titleLabel.SetText(title)
}

// SetGradient 更新渐变色
func (c *GradientCard) SetGradient(startColor, endColor color.Color) {
	c.gradient.StartColor = startColor
	c.gradient.EndColor = endColor
	c.gradient.Refresh()
}

// CreateRenderer 实现 fyne.Widget 接口
func (c *GradientCard) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.container)
}

// MinSize 计算最小尺寸
func (c *GradientCard) MinSize() fyne.Size {
	return c.container.MinSize()
}
