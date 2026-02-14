package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Toolbar панель инструментов приложения
type Toolbar struct {
	gui          *MainGUI
	container    *fyne.Container
	runButton    *widget.Button
	stopButton   *widget.Button
	saveButton   *widget.Button
	loadButton   *widget.Button
	exportButton *widget.Button
	clearButton  *widget.Button
}

// NewToolbar создаёт новую панель инструментов.
func NewToolbar(gui *MainGUI) *Toolbar {
	toolbar := &Toolbar{
		gui: gui,
	}
	toolbar.container = toolbar.buildUI()
	return toolbar
}

// GetContainer возвращает контейнер панели инструментов.
func (t *Toolbar) GetContainer() fyne.CanvasObject {
	return t.container
}

// UpdateState обновляет состояние кнопок панели инструментов.
func (t *Toolbar) UpdateState(isConnected bool, hasProgram bool, isRunning bool, isInsertMode bool) {
	if t.runButton != nil && t.stopButton != nil {
		if isInsertMode {
			t.runButton.Disable()
			t.stopButton.Disable()
		} else if isConnected && hasProgram && !isRunning {
			t.runButton.Enable()
			t.stopButton.Disable()
		} else if isRunning {
			t.runButton.Disable()
			t.stopButton.Enable()
		} else {
			t.runButton.Disable()
			t.stopButton.Disable()
		}
	}

	if t.saveButton != nil && t.exportButton != nil {
		if hasProgram && !isInsertMode {
			t.saveButton.Enable()
			t.exportButton.Enable()
		} else {
			t.saveButton.Disable()
			t.exportButton.Disable()
		}
	}

	if t.clearButton != nil {
		if isInsertMode || !hasProgram {
			t.clearButton.Disable()
		} else {
			t.clearButton.Enable()
		}
	}
}

// buildUI строит интерфейс панели инструментов.
func (t *Toolbar) buildUI() *fyne.Container {
	connectButton := widget.NewButtonWithIcon("Поиск хаба", theme.SearchIcon(), func() {
		if t.gui != nil {
			t.gui.showHubDiscoveryDialog()
		}
	})
	connectButton.Importance = widget.HighImportance

	disconnectButton := widget.NewButtonWithIcon("Отключиться", theme.CancelIcon(), func() {
		if t.gui != nil && t.gui.hubMgr != nil {
			t.gui.hubMgr.Disconnect()
		}
	})
	disconnectButton.Importance = widget.MediumImportance
	disconnectButton.Disable()

	t.runButton = widget.NewButtonWithIcon("Запуск", theme.MediaPlayIcon(), func() {
		if t.gui != nil && t.gui.programMgr != nil {
			log.Println("Запуск программы...")
			err := t.gui.programMgr.RunProgram()
			if err != nil {
				log.Printf("Ошибка запуска программы: %v", err)
				dialog.ShowError(err, t.gui.window)
			} else {
				log.Println("Программа успешно запущена")
				t.UpdateState(true, true, true, false)
			}
		}
	})
	t.runButton.Importance = widget.HighImportance
	t.runButton.Disable()

	t.stopButton = widget.NewButtonWithIcon("Стоп", theme.MediaStopIcon(), func() {
		if t.gui != nil && t.gui.programMgr != nil {
			t.gui.programMgr.StopProgram()
			log.Println("Программа остановлена")
		}
	})
	t.stopButton.Importance = widget.MediumImportance
	t.stopButton.Disable()

	t.saveButton = widget.NewButtonWithIcon("Сохранить", theme.DocumentSaveIcon(), func() {
		t.saveProgram()
	})
	t.saveButton.Importance = widget.MediumImportance
	t.saveButton.Disable()

	t.loadButton = widget.NewButtonWithIcon("Загрузить", theme.FolderOpenIcon(), func() {
		t.loadProgram()
	})
	t.loadButton.Importance = widget.MediumImportance

	t.exportButton = widget.NewButtonWithIcon("Экспорт", theme.DownloadIcon(), func() {
		t.exportProgram()
	})
	t.exportButton.Importance = widget.MediumImportance
	t.exportButton.Disable()

	t.clearButton = widget.NewButtonWithIcon("Очистить", theme.DeleteIcon(), func() {
		if t.gui.programMgr != nil {
			dialog.ShowConfirm("Очистить программу",
				"Вы уверены, что хотите удалить все блоки программы?",
				func(confirmed bool) {
					if confirmed {
						t.gui.programMgr.ClearProgram()
						t.gui.programPanel.Clear()
						log.Println("Программа очищена")
					}
				}, t.gui.window)
		}
	})
	t.clearButton.Importance = widget.MediumImportance

	helpButton := widget.NewButtonWithIcon("Справка", theme.HelpIcon(), func() {
		t.showHelp()
	})
	helpButton.Importance = widget.LowImportance

	if t.gui != nil {
		t.gui.statusLabel = widget.NewLabel("Не подключено")
		t.gui.statusLabel.Alignment = fyne.TextAlignCenter
		t.gui.statusLabel.TextStyle.Bold = true

		t.gui.connectButton = connectButton
		t.gui.disconnectButton = disconnectButton
	}

	toolbarContainer := container.NewHBox(
		connectButton,
		disconnectButton,
		widget.NewSeparator(),
		t.runButton,
		t.stopButton,
		widget.NewSeparator(),
		t.saveButton,
		t.loadButton,
		t.exportButton,
		widget.NewSeparator(),
		t.clearButton,
		widget.NewSeparator(),
		helpButton,
		layout.NewSpacer(),
	)

	statusContainer := container.NewHBox(
		layout.NewSpacer(),
		t.gui.statusLabel,
		layout.NewSpacer(),
	)

	mainContainer := container.NewVBox(
		toolbarContainer,
		statusContainer,
	)

	return mainContainer
}

// saveProgram сохраняет программу.
func (t *Toolbar) saveProgram() {
	dialog.ShowInformation("Информация", "Функция сохранения программы в разработке", t.gui.window)
}

// loadProgram загружает программу.
func (t *Toolbar) loadProgram() {
	dialog.ShowInformation("Информация", "Функция загрузки программы в разработке", t.gui.window)
}

// exportProgram экспортирует программу.
func (t *Toolbar) exportProgram() {
	dialog.ShowInformation("Информация", "Функция экспорта программы в разработке", t.gui.window)
}

// showHelp показывает справку.
func (t *Toolbar) showHelp() {
	helpText := `WeDoProg - Визуальный программист WeDo 2.0

Основные функции:
1. Подключение к WeDo 2.0 хабу через Bluetooth
2. Визуальное программирование с помощью блоков
3. Управление моторами, светодиодами и датчиками
4. Сохранение и загрузка программ

Использование:
1. Нажмите "Поиск хаба" для подключения
2. Перетаскивайте блоки из палитры на рабочую область
3. Настраивайте параметры блоков в правой панели
4. Используйте "Запуск" и "Стоп" для управления программой

Поддерживаемые устройства:
- Моторы
- RGB светодиод
- Датчик наклона
- Датчик расстояния
- Пищалка (зуммер)`

	dialog.ShowInformation("Справка", helpText, t.gui.window)
}
