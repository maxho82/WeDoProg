package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// CustomTheme пользовательская тема для WeDoProg
type CustomTheme struct{}

var _ fyne.Theme = (*CustomTheme)(nil)

// Цвета темы
var (
	backgroundColor  = color.NRGBA{R: 45, G: 45, B: 48, A: 255}
	foregroundColor  = color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	primaryColor     = color.NRGBA{R: 0, G: 122, B: 204, A: 255}
	secondaryColor   = color.NRGBA{R: 63, G: 63, B: 70, A: 255}
	disabledColor    = color.NRGBA{R: 104, G: 104, B: 104, A: 255}
	hoverColor       = color.NRGBA{R: 28, G: 151, B: 234, A: 255}
	pressedColor     = color.NRGBA{R: 0, G: 97, B: 163, A: 255}
	successColor     = color.NRGBA{R: 76, G: 175, B: 80, A: 255}
	errorColor       = color.NRGBA{R: 244, G: 67, B: 54, A: 255}
	warningColor     = color.NRGBA{R: 255, G: 193, B: 7, A: 255}
	scrollBarColor   = color.NRGBA{R: 90, G: 90, B: 90, A: 255}
	selectionColor   = color.NRGBA{R: 255, G: 255, B: 0, A: 255} // Желтый для выделения
	inputBackground  = color.NRGBA{R: 30, G: 30, B: 30, A: 255}
	inputBorderColor = color.NRGBA{R: 90, G: 90, B: 90, A: 255}
	highlightColor   = color.NRGBA{R: 255, G: 215, B: 0, A: 255} // Золотой для выделенных линий
)

// Color возвращает цвет по имени
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return backgroundColor
	case theme.ColorNameButton:
		return secondaryColor
	case theme.ColorNameDisabled:
		return disabledColor
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 70, G: 70, B: 70, A: 255}
	case theme.ColorNameError:
		return errorColor
	case theme.ColorNameFocus:
		return hoverColor
	case theme.ColorNameForeground:
		return foregroundColor
	case theme.ColorNameHover:
		return hoverColor
	case theme.ColorNameInputBackground:
		return inputBackground
	case theme.ColorNameInputBorder:
		return inputBorderColor
	case theme.ColorNameMenuBackground:
		return backgroundColor
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 30, G: 30, B: 30, A: 230}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 150, G: 150, B: 150, A: 255}
	case theme.ColorNamePressed:
		return pressedColor
	case theme.ColorNamePrimary:
		return primaryColor
	case theme.ColorNameScrollBar:
		return scrollBarColor
	case theme.ColorNameSelection:
		return selectionColor
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 60, G: 60, B: 60, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 50}
	case theme.ColorNameSuccess:
		return successColor
	case theme.ColorNameWarning:
		return warningColor
	default:
		return theme.DarkTheme().Color(name, variant)
	}
}

// Font возвращает шрифт
func (t *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DarkTheme().Font(style)
}

// Icon возвращает иконку
func (t *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(name)
}

// Size возвращает размер элемента
func (t *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 6
	case theme.SizeNameSeparatorThickness:
		return 2 // Увеличиваем толщину разделителя
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameText:
		return 14
	default:
		return theme.DarkTheme().Size(name)
	}
}
