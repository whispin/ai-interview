package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// interviewTheme 定义应用的统一外观配色。
type interviewTheme struct{}

// New 返回自定义主题实例。
func New() fyne.Theme {
	return &interviewTheme{}
}

var (
	// Modern tech-inspired color palette with depth
	colorPrimary       = color.NRGBA{R: 88, G: 101, B: 242, A: 255}   // Vibrant indigo
	colorPrimaryLight  = color.NRGBA{R: 129, G: 140, B: 248, A: 255}  // Light indigo
	colorAccent        = color.NRGBA{R: 59, G: 130, B: 246, A: 255}   // Electric blue
	colorSuccess       = color.NRGBA{R: 16, G: 185, B: 129, A: 255}   // Emerald
	colorWarning       = color.NRGBA{R: 245, G: 158, B: 11, A: 255}   // Amber

	// Background layers with subtle gradients
	colorBackground    = color.NRGBA{R: 15, G: 23, B: 42, A: 255}     // Deep slate
	colorBackgroundAlt = color.NRGBA{R: 30, G: 41, B: 59, A: 255}     // Medium slate
	colorSurface       = color.NRGBA{R: 51, G: 65, B: 85, A: 255}     // Light slate
	colorSurfaceLight  = color.NRGBA{R: 71, G: 85, B: 105, A: 255}    // Lighter slate

	// Text hierarchy
	colorText          = color.NRGBA{R: 241, G: 245, B: 249, A: 255}  // Near white
	colorTextSecondary = color.NRGBA{R: 203, G: 213, B: 225, A: 255}  // Light gray
	colorTextSubtle    = color.NRGBA{R: 148, G: 163, B: 184, A: 255}  // Medium gray

	// Card backgrounds with glassmorphism
	colorCardBg        = color.NRGBA{R: 51, G: 65, B: 85, A: 230}     // Semi-transparent surface
	colorCardHover     = color.NRGBA{R: 71, G: 85, B: 105, A: 240}    // Hover state
)

func (t *interviewTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// 强制使用深色主题,忽略variant参数
	switch name {
	case theme.ColorNamePrimary:
		return colorPrimary
	case theme.ColorNameButton:
		return colorPrimary
	case theme.ColorNameBackground:
		return colorBackground
	case theme.ColorNameInputBackground:
		return colorSurface
	case theme.ColorNameForeground:
		return colorText
	case theme.ColorNameHover:
		return colorSurfaceLight
	case theme.ColorNamePlaceHolder:
		return colorTextSubtle
	case theme.ColorNameFocus:
		return colorPrimaryLight
	case theme.ColorNameScrollBar:
		return colorSurfaceLight
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 40}
	case theme.ColorNameDisabled:
		return colorTextSubtle
	case theme.ColorNameSuccess:
		return colorSuccess
	case theme.ColorNameWarning:
		return colorWarning

	// 对话框和覆盖层颜色
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 180} // 半透明黑色遮罩

	// 明确设置深色主题的关键颜色
	case theme.ColorNameHeaderBackground:
		return colorBackgroundAlt
	case theme.ColorNameMenuBackground:
		return colorSurface
	case theme.ColorNameInputBorder:
		return colorSurfaceLight

	default:
		// 对于未明确指定的颜色,使用深色变体
		return theme.DarkTheme().Color(name, theme.VariantDark)
	}
}

func (t *interviewTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *interviewTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *interviewTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 4
	default:
		return theme.DefaultTheme().Size(name)
	}
}
