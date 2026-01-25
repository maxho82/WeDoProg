package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
}

// NewToolbar создает новую панель инструментов
func NewToolbar(gui *MainGUI) *Toolbar {
	toolbar := &Toolbar{
		gui: gui,
	}

	toolbar.container = toolbar.buildUI()
	return toolbar
}

// GetContainer возвращает контейнер панели инструментов
func (t *Toolbar) GetContainer() fyne.CanvasObject {
	return t.container // Это уже *fyne.Container, который реализует fyne.CanvasObject
}

// buildUI строит интерфейс панели инструментов
func (t *Toolbar) buildUI() *fyne.Container {
	// Кнопка подключения хаба
	connectButton := widget.NewButtonWithIcon("Поиск хаба", theme.SearchIcon(), func() {
		if t.gui != nil {
			t.gui.showHubDiscoveryDialog()
		}
	})
	connectButton.Importance = widget.HighImportance

	// Кнопка отключения
	disconnectButton := widget.NewButtonWithIcon("Отключиться", theme.CancelIcon(), func() {
		if t.gui != nil && t.gui.hubMgr != nil {
			t.gui.hubMgr.Disconnect()
		}
	})
	disconnectButton.Importance = widget.MediumImportance
	disconnectButton.Disable() // По умолчанию выключена

	// Кнопки управления программой
	t.runButton = widget.NewButtonWithIcon("Запуск", theme.MediaPlayIcon(), func() {
		if t.gui != nil && t.gui.programMgr != nil {
			log.Println("Запуск программы...")
			err := t.gui.programMgr.RunProgram()
			if err != nil {
				log.Printf("Ошибка запуска программы: %v", err)
				// Можно показать сообщение об ошибке
			} else {
				log.Println("Программа успешно запущена")
			}
		}
	})
	t.runButton.Importance = widget.HighImportance
	t.runButton.Disable() // По умолчанию выключена

	t.stopButton = widget.NewButtonWithIcon("Стоп", theme.MediaStopIcon(), func() {
		if t.gui != nil && t.gui.programMgr != nil {
			t.gui.programMgr.StopProgram()
			log.Println("Программа остановлена")
		}
	})
	t.stopButton.Importance = widget.MediumImportance
	t.stopButton.Disable() // По умолчанию выключена

	// Кнопки работы с файлами
	t.saveButton = widget.NewButtonWithIcon("Сохранить", theme.DocumentSaveIcon(), func() {
		t.saveProgram()
	})
	t.saveButton.Importance = widget.MediumImportance
	t.saveButton.Disable() // По умолчанию выключена

	t.loadButton = widget.NewButtonWithIcon("Загрузить", theme.FolderOpenIcon(), func() {
		t.loadProgram()
	})
	t.loadButton.Importance = widget.MediumImportance

	t.exportButton = widget.NewButtonWithIcon("Экспорт", theme.DownloadIcon(), func() {
		t.exportProgram()
	})
	t.exportButton.Importance = widget.MediumImportance
	t.exportButton.Disable() // По умолчанию выключена

	// Кнопка очистки
	clearButton := widget.NewButtonWithIcon("Очистить", theme.DeleteIcon(), func() {
		if t.gui.programMgr != nil {
			t.gui.programMgr.ClearProgram()
		}
	})
	clearButton.Importance = widget.MediumImportance

	// Кнопка помощи
	helpButton := widget.NewButtonWithIcon("Справка", theme.HelpIcon(), func() {
		t.showHelp()
	})
	helpButton.Importance = widget.LowImportance

	// Кнопка тестирования протокола
	testProtocolButton := widget.NewButtonWithIcon("Тест протокола", theme.VisibilityIcon(), func() {
		if t.gui != nil {
			t.gui.showProtocolTestDialog()
		}
	})
	testProtocolButton.Importance = widget.LowImportance

	// Статус подключения
	if t.gui != nil {
		t.gui.statusLabel = widget.NewLabel("Не подключено")
		t.gui.statusLabel.Alignment = fyne.TextAlignCenter
		t.gui.statusLabel.TextStyle.Bold = true

		t.gui.connectButton = connectButton
		t.gui.disconnectButton = disconnectButton
		t.gui.testProtocolButton = testProtocolButton
	}

	// Контейнер панели инструментов
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
		clearButton,
		widget.NewSeparator(),
		testProtocolButton,
		helpButton,
		layout.NewSpacer(),
	)

	// Добавляем статус в отдельный контейнер
	statusContainer := container.NewHBox(
		layout.NewSpacer(),
		t.gui.statusLabel,
		layout.NewSpacer(),
	)

	// Основной контейнер с панелью инструментов и статусом
	mainContainer := container.NewVBox(
		toolbarContainer,
		statusContainer,
	)

	return mainContainer
}

// saveProgram сохраняет программу
func (t *Toolbar) saveProgram() {
	// TODO: Реализовать сохранение программы в файл
}

// loadProgram загружает программу
func (t *Toolbar) loadProgram() {
	// TODO: Реализовать загрузку программы из файла
}

// exportProgram экспортирует программу
func (t *Toolbar) exportProgram() {
	// TODO: Реализовать экспорт программы в разные форматы
}

// showHelp показывает справку
func (t *Toolbar) showHelp() {
	// TODO: Реализовать показ справки
}
