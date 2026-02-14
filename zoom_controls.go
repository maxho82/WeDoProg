package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ZoomControls отвечает за управление масштабом панели программирования.
type ZoomControls struct {
	gui        *MainGUI
	container  *fyne.Container
	scaleLabel *widget.Label
}

// NewZoomControls создаёт новые элементы управления масштабом.
func NewZoomControls(gui *MainGUI) *ZoomControls {
	zc := &ZoomControls{
		gui: gui,
	}
	zc.buildUI()
	return zc
}

// GetContainer возвращает корневой контейнер.
func (zc *ZoomControls) GetContainer() fyne.CanvasObject {
	return zc.container
}

// buildUI строит интерфейс.
func (zc *ZoomControls) buildUI() {
	zoomOutButton := widget.NewButtonWithIcon("", theme.ZoomOutIcon(), func() {
		if zc.gui.programPanel != nil {
			zc.gui.programPanel.ZoomOut()
			zc.updateLabel()
		}
	})
	zoomOutButton.Importance = widget.LowImportance

	zoomInButton := widget.NewButtonWithIcon("", theme.ZoomInIcon(), func() {
		if zc.gui.programPanel != nil {
			zc.gui.programPanel.ZoomIn()
			zc.updateLabel()
		}
	})
	zoomInButton.Importance = widget.LowImportance

	resetZoomButton := widget.NewButtonWithIcon("100%", theme.ViewRestoreIcon(), func() {
		if zc.gui.programPanel != nil {
			zc.gui.programPanel.ResetZoom()
			zc.updateLabel()
		}
	})
	resetZoomButton.Importance = widget.LowImportance

	zc.scaleLabel = widget.NewLabel("100%")
	zc.scaleLabel.Alignment = fyne.TextAlignCenter
	zc.scaleLabel.TextStyle.Bold = true

	zc.container = container.NewHBox(
		widget.NewLabel("Масштаб:"),
		zoomOutButton,
		resetZoomButton,
		zoomInButton,
		zc.scaleLabel,
		layout.NewSpacer(),
	)
}

// updateLabel обновляет метку с текущим масштабом.
func (zc *ZoomControls) updateLabel() {
	if zc.gui.programPanel != nil {
		scale := zc.gui.programPanel.GetScale()
		zc.scaleLabel.SetText(fmt.Sprintf("%.0f%%", scale*100))
		zc.scaleLabel.Refresh()
	}
}

// Refresh обновляет отображение метки.
func (zc *ZoomControls) Refresh() {
	zc.updateLabel()
}