package widgets

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MarkdownViewer 是一个支持 Markdown 渲染和流式更新的组件
type MarkdownViewer struct {
	widget.BaseWidget

	content   string
	richText  *widget.RichText
	scroll    *container.Scroll
	mu        sync.RWMutex
	onRefresh func()
}

// NewMarkdownViewer 创建一个新的 Markdown 查看器
func NewMarkdownViewer() *MarkdownViewer {
	m := &MarkdownViewer{
		content:  "",
		richText: widget.NewRichTextFromMarkdown(""),
	}

	m.richText.Wrapping = fyne.TextWrapWord
	m.scroll = container.NewVScroll(m.richText)

	m.ExtendBaseWidget(m)
	return m
}

// CreateRenderer 实现 fyne.Widget 接口
func (m *MarkdownViewer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(m.scroll)
}

// SetContent 设置 Markdown 内容（完整替换）
func (m *MarkdownViewer) SetContent(markdown string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.content = markdown
	m.updateRichText()
}

// AppendContent 追加 Markdown 内容（流式更新）
func (m *MarkdownViewer) AppendContent(markdown string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.content += markdown
	m.updateRichText()
}

// Clear 清空内容
func (m *MarkdownViewer) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.content = ""
	m.updateRichText()
}

// GetContent 获取当前内容
func (m *MarkdownViewer) GetContent() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.content
}

// updateRichText 更新 RichText 组件（内部方法，调用前需要持有锁）
func (m *MarkdownViewer) updateRichText() {
	// 解析 Markdown 内容
	m.richText.ParseMarkdown(m.content)

	// 刷新显示
	m.richText.Refresh()

	// 自动滚动到底部
	if m.scroll != nil {
		// 在下一帧执行滚动，确保内容已更新
		go func() {
			// 小延迟确保布局完成
			// time.Sleep(10 * time.Millisecond)
			m.scroll.ScrollToBottom()
		}()
	}

	// 触发外部刷新回调
	if m.onRefresh != nil {
		m.onRefresh()
	}
}

// SetOnRefresh 设置刷新回调函数
func (m *MarkdownViewer) SetOnRefresh(callback func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRefresh = callback
}

// MinSize 返回最小尺寸
func (m *MarkdownViewer) MinSize() fyne.Size {
	return m.scroll.MinSize()
}

// Resize 调整大小
func (m *MarkdownViewer) Resize(size fyne.Size) {
	m.BaseWidget.Resize(size)
	m.scroll.Resize(size)
}
