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
	colorPrimary    = color.NRGBA{R: 51, G: 105, B: 255, A: 255}
	colorBackground = color.NRGBA{R: 244, G: 246, B: 250, A: 255}
	colorSurface    = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colorText       = color.NRGBA{R: 35, G: 39, B: 47, A: 255}
	colorSubtle     = color.NRGBA{R: 120, G: 133, B: 150, A: 255}
)

func (t *interviewTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
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
	case theme.ColorNameHover, theme.ColorNamePlaceHolder:
		return colorSubtle
	default:
		return theme.DefaultTheme().Color(name, variant)
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
		return theme.DefaultTheme().Size(name) * 1.1
	default:
		return theme.DefaultTheme().Size(name)
	}
}
