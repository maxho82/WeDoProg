package main

import (
	//"image/color"
	"log"

	"fyne.io/fyne/v2"
	//"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PropertiesPanel отображает свойства выбранного блока.
type PropertiesPanel struct {
	gui       *MainGUI
	state     *AppState
	deviceMgr *DeviceManager
	window    fyne.Window
	scroll    *container.Scroll
}

// NewPropertiesPanel создаёт новую панель свойств.
func NewPropertiesPanel(gui *MainGUI, state *AppState, deviceMgr *DeviceManager, window fyne.Window) *PropertiesPanel {
	panel := &PropertiesPanel{
		gui:       gui,
		state:     state,
		deviceMgr: deviceMgr,
		window:    window,
	}
	panel.buildUI()
	return panel
}

// GetContainer возвращает корневой контейнер панели.
func (pp *PropertiesPanel) GetContainer() fyne.CanvasObject {
	return pp.scroll
}

// buildUI строит интерфейс панели.
func (pp *PropertiesPanel) buildUI() {
	content := container.NewVBox(
		widget.NewLabelWithStyle("Свойства", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("Выберите элемент для просмотра свойств"),
	)
	pp.scroll = container.NewVScroll(content)
}

// ShowBlockProperties отображает свойства указанного блока.
func (pp *PropertiesPanel) ShowBlockProperties(block *ProgramBlock) {
	content, ok := pp.scroll.Content.(*fyne.Container)
	if !ok {
		return
	}
	content.Objects = nil

	// Создаём редактор блока
	editor := NewBlockEditor(block, pp.deviceMgr, pp.window, func(updatedBlock *ProgramBlock) {
		pp.gui.programMgr.UpdateBlock(updatedBlock.ID, updatedBlock.Parameters)
		log.Printf("Параметры блока %d обновлены", updatedBlock.ID)
	})

	content.Add(editor.GetContainer())
	content.Refresh()
	pp.scroll.Refresh()
}

// Clear очищает панель свойств.
func (pp *PropertiesPanel) Clear() {
	content, ok := pp.scroll.Content.(*fyne.Container)
	if !ok {
		return
	}
	content.Objects = nil
	content.Add(widget.NewLabel("Выберите элемент для просмотра свойств"))
	content.Refresh()
	pp.scroll.Refresh()
}
