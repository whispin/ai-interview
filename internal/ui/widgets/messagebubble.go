package widgets

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// MessageType 定义消息类型
type MessageType int

const (
	MessageTypeInterviewer MessageType = iota // 面试官
	MessageTypeUser                            // 用户
	MessageTypeSystem                          // 系统消息
)

// Message 消息数据结构
type Message struct {
	Type      MessageType
	Text      string
	Timestamp time.Time
}

// MessageBubble 消息气泡组件
type MessageBubble struct {
	widget.BaseWidget

	message Message

	container *fyne.Container
	icon      *widget.Icon
	nameLabel *widget.Label
	timeLabel *widget.Label
	textLabel *widget.Label
	background *canvas.Rectangle
}

// NewMessageBubble 创建新的消息气泡
func NewMessageBubble(msg Message) *MessageBubble {
	bubble := &MessageBubble{
		message: msg,
	}

	// 设置图标
	var iconResource fyne.Resource
	var name string
	var bgColor color.Color

	switch msg.Type {
	case MessageTypeInterviewer:
		iconResource = theme.AccountIcon()
		name = "面试官"
		// Modern gradient-inspired blue with transparency
		bgColor = color.NRGBA{R: 59, G: 130, B: 246, A: 25}
	case MessageTypeUser:
		iconResource = theme.AccountIcon()
		name = "我"
		// Subtle slate with transparency for glass effect
		bgColor = color.NRGBA{R: 71, G: 85, B: 105, A: 40}
	case MessageTypeSystem:
		iconResource = theme.InfoIcon()
		name = "系统"
		// Amber accent with transparency
		bgColor = color.NRGBA{R: 245, G: 158, B: 11, A: 20}
	}

	bubble.icon = widget.NewIcon(iconResource)
	bubble.icon.Resize(fyne.NewSize(24, 24))

	bubble.nameLabel = widget.NewLabel(name)
	bubble.nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	bubble.timeLabel = widget.NewLabel(msg.Timestamp.Format("15:04:05"))
	bubble.timeLabel.TextStyle = fyne.TextStyle{Italic: true}
	bubble.timeLabel.Importance = widget.LowImportance

	bubble.textLabel = widget.NewLabel(msg.Text)
	bubble.textLabel.Wrapping = fyne.TextWrapWord

	bubble.background = canvas.NewRectangle(bgColor)
	bubble.background.CornerRadius = 12 // Increased corner radius for modern look

	// 创建布局
	header := container.NewHBox(
		bubble.icon,
		bubble.nameLabel,
		widget.NewLabel("·"),
		bubble.timeLabel,
	)

	content := container.NewVBox(
		header,
		bubble.textLabel,
	)

	bubble.container = container.NewMax(
		bubble.background,
		container.NewPadded(content),
	)

	bubble.ExtendBaseWidget(bubble)
	return bubble
}

// SetText 更新消息文本
func (b *MessageBubble) SetText(text string) {
	b.message.Text = text
	b.textLabel.SetText(text)
}

// GetMessage 获取消息数据
func (b *MessageBubble) GetMessage() Message {
	return b.message
}

// CreateRenderer 实现 fyne.Widget 接口
func (b *MessageBubble) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.container)
}

// MessageList 消息列表组件
type MessageList struct {
	widget.BaseWidget

	messages  []Message
	bubbles   []*MessageBubble
	container *fyne.Container
	scroll    *container.Scroll

	autoScroll bool
}

// NewMessageList 创建新的消息列表
func NewMessageList() *MessageList {
	list := &MessageList{
		messages:   make([]Message, 0),
		bubbles:    make([]*MessageBubble, 0),
		autoScroll: true,
	}

	list.container = container.NewVBox()
	list.scroll = container.NewVScroll(list.container)

	list.ExtendBaseWidget(list)
	return list
}

// AddMessage 添加新消息
func (l *MessageList) AddMessage(msg Message) {
	l.messages = append(l.messages, msg)

	bubble := NewMessageBubble(msg)
	l.bubbles = append(l.bubbles, bubble)
	l.container.Add(bubble)

	if l.autoScroll {
		l.ScrollToBottom()
	}
}

// Clear 清空所有消息
func (l *MessageList) Clear() {
	l.messages = make([]Message, 0)
	l.bubbles = make([]*MessageBubble, 0)
	l.container.Objects = nil
	l.container.Refresh()
}

// SetAutoScroll 设置是否自动滚动到底部
func (l *MessageList) SetAutoScroll(enabled bool) {
	l.autoScroll = enabled
}

// ScrollToBottom 滚动到底部
func (l *MessageList) ScrollToBottom() {
	l.scroll.ScrollToBottom()
}

// GetMessages 获取所有消息
func (l *MessageList) GetMessages() []Message {
	return l.messages
}

// ExportText 导出为纯文本
func (l *MessageList) ExportText() string {
	var result string
	for _, msg := range l.messages {
		var speaker string
		switch msg.Type {
		case MessageTypeInterviewer:
			speaker = "面试官"
		case MessageTypeUser:
			speaker = "我"
		case MessageTypeSystem:
			speaker = "系统"
		}
		result += fmt.Sprintf("[%s] %s: %s\n\n",
			msg.Timestamp.Format("15:04:05"),
			speaker,
			msg.Text)
	}
	return result
}

// CreateRenderer 实现 fyne.Widget 接口
func (l *MessageList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.scroll)
}
