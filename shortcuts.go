package main

import "fyne.io/fyne/v2"

// setupKeyboardShortcuts настраивает горячие клавиши
func (gui *MainGUI) setupKeyboardShortcuts() {
	// Обработка клавиши Delete для удаления выделенного блока
	gui.window.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		if event.Name == fyne.KeyDelete || event.Name == fyne.KeyBackspace {
			if gui.selectedBlock != nil {
				gui.deleteSelectedBlock()
			}
		}
	})
}
