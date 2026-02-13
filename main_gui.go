package main

import (
	"fmt"
	"image/color"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// MainGUI основной интерфейс приложения
type MainGUI struct {
	window     fyne.Window
	hubMgr     *HubManager
	deviceMgr  *DeviceManager
	programMgr *ProgramManager
	state      *AppState

	statusLabel      *widget.Label
	connectButton    *widget.Button
	disconnectButton *widget.Button
	toolbar          *Toolbar

	insertCancelButton *widget.Button

	devicePanel     *fyne.Container
	propertiesPanel *container.Scroll
	programPanel    *ProgramPanel
	blocksPanel     *container.Scroll

	batteryProgress  *widget.ProgressBar
	hubInfoContainer *fyne.Container
	deviceList       *widget.List

	blockButtons map[BlockType]*widget.Button

	zoomControls *fyne.Container
	scaleLabel   *widget.Label
}

// NewMainGUI создает новый GUI.
func NewMainGUI(window fyne.Window, hubMgr *HubManager) *MainGUI {
	state := NewAppState()
	deviceMgr := NewDeviceManager(hubMgr)
	programMgr := NewProgramManager(hubMgr, deviceMgr, state)

	gui := &MainGUI{
		window:     window,
		hubMgr:     hubMgr,
		deviceMgr:  deviceMgr,
		programMgr: programMgr,
		state:      state,
	}

	hubMgr.SetBatteryUpdateCallback(gui.UpdateBatteryDisplay)
	hubMgr.SetHubInfoUpdateCallback(gui.UpdateHubInfoDisplay)
	hubMgr.SetDeviceUpdateCallback(gui.onDeviceUpdateFromHub)
	hubMgr.SetConnectionStateCallback(gui.updateConnectionStatus)

	programMgr.SetStateChangeCallback(func(stateVal ProgramState) {
		fyne.Do(func() {
			gui.state.SetProgramState(stateVal)
			gui.updateToolbarState()
		})
	})
	programMgr.SetCurrentBlockCallback(func(blockID int) {
		fyne.Do(func() {
			gui.highlightExecutingBlock(blockID)
		})
	})

	return gui
}

// onDeviceUpdateFromHub передаёт обновление от HubManager в DeviceManager и AppState.
func (gui *MainGUI) onDeviceUpdateFromHub(portID byte, device *Device) {
	gui.deviceMgr.AddOrUpdateDevice(device)
	gui.state.UpdateDevice(portID, device)
	fyne.Do(func() {
		gui.updateAvailableBlocks()
		gui.updateDeviceList()
	})
}

// GetSelectedBlock возвращает текущий выбранный блок.
func (gui *MainGUI) GetSelectedBlock() *ProgramBlock {
	return gui.state.GetSelectedBlock()
}

// SetSelectedBlock устанавливает текущий выбранный блок.
func (gui *MainGUI) SetSelectedBlock(block *ProgramBlock) {
	gui.state.SetSelectedBlock(block)
}

// highlightExecutingBlock выделяет выполняемый блок.
func (gui *MainGUI) highlightExecutingBlock(blockID int) {
	if blockID == -1 {
		gui.programPanel.HighlightExecutingBlock(-1)
		gui.programPanel.SetSelectedBlock(nil)
		gui.SetSelectedBlock(nil)
		return
	}
	block := gui.state.FindBlockByID(blockID)
	if block != nil {
		gui.programPanel.HighlightExecutingBlock(blockID)
		gui.programPanel.scrollToBlock(block)
	}
}

// BuildUI строит интерфейс приложения.
func (gui *MainGUI) BuildUI() fyne.CanvasObject {
	gui.createToolbar()
	gui.devicePanel = gui.createDevicePanel()
	gui.propertiesPanel = gui.createPropertiesPanel()
	gui.blocksPanel = gui.createBlocksPanel()
	gui.programPanel = NewProgramPanel(gui, gui.programMgr)

	gui.createZoomControls()

	leftPanel := container.NewVBox(
		gui.devicePanel,
		canvas.NewLine(color.NRGBA{R: 60, G: 60, B: 60, A: 255}),
		gui.blocksPanel,
	)

	leftSplit := container.NewHSplit(leftPanel, gui.programPanel.GetContainer())
	leftSplit.SetOffset(0.25)

	rightSplit := container.NewHSplit(leftSplit, gui.propertiesPanel)
	rightSplit.SetOffset(0.75)

	mainContainer := container.NewBorder(
		container.NewVBox(gui.toolbar.GetContainer(), gui.zoomControls),
		nil,
		nil,
		nil,
		rightSplit,
	)

	gui.setupKeyboardShortcuts()
	// При первом запуске создаём новую программу автоматически
	gui.createNewProgram()

	return mainContainer
}

// createBlocksPanel создает панель блоков программирования.
func (gui *MainGUI) createBlocksPanel() *container.Scroll {
	blocksContainer := container.NewVBox()

	title := canvas.NewText("Палитра блоков", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter
	blocksContainer.Add(container.NewCenter(title))
	blocksContainer.Add(widget.NewSeparator())

	gui.blockButtons = make(map[BlockType]*widget.Button)

	// Кнопка "Новая+" – создаёт новую программу с блоками "Начать" и "Стоп"
	newButton := widget.NewButtonWithIcon("Новая+", theme.DocumentCreateIcon(), func() {
		gui.confirmNewProgram()
	})
	newButton.Importance = widget.HighImportance
	blocksContainer.Add(newButton)
	blocksContainer.Add(widget.NewSeparator())

	categories := []struct {
		name   string
		blocks []BlockType
	}{
		{"Логика", []BlockType{BlockTypeWait, BlockTypeLoopStart, BlockTypeCondition}},
		{"Действия", []BlockType{BlockTypeMotor, BlockTypeLED, BlockTypeSound}},
		{"Датчики", []BlockType{BlockTypeTiltSensor, BlockTypeDistanceSensor, BlockTypeVoltageSensor, BlockTypeCurrentSensor}},
	}

	for _, category := range categories {
		categoryLabel := canvas.NewText(category.name, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
		categoryLabel.TextSize = 14
		categoryLabel.TextStyle.Bold = true
		blocksContainer.Add(categoryLabel)

		for _, blockType := range category.blocks {
			blockName := gui.getBlockName(blockType)

			if blockType == BlockTypeLoopStart {
				// Для цикла создаём специальную кнопку, которая вставляет сразу пару блоков
				blockButton := widget.NewButton(blockName, func() {
					gui.handleLoopBlockSelection()
				})
				blockButton.Importance = widget.LowImportance
				gui.blockButtons[blockType] = blockButton
				blocksContainer.Add(blockButton)
			} else {
				blockButton := widget.NewButton(blockName, func(bt BlockType) func() {
					return func() {
						gui.handleBlockSelection(bt)
					}
				}(blockType))

				blockButton.Importance = widget.LowImportance
				gui.blockButtons[blockType] = blockButton
				blocksContainer.Add(blockButton)
			}
		}
		blocksContainer.Add(widget.NewSeparator())
	}

	cancelButton := widget.NewButton("Отменить вставку", func() {
		gui.CancelInsertMode()
	})
	cancelButton.Importance = widget.WarningImportance
	cancelButton.Hide()
	blocksContainer.Add(cancelButton)
	gui.insertCancelButton = cancelButton

	scroll := container.NewVScroll(container.NewPadded(blocksContainer))
	scroll.SetMinSize(fyne.NewSize(220, 400))
	return scroll
}

// confirmNewProgram показывает диалог подтверждения создания новой программы.
func (gui *MainGUI) confirmNewProgram() {
	dialog.ShowConfirm("Создать новую программу",
		"Вы уверены? Текущая программа будет удалена.",
		func(confirmed bool) {
			if confirmed {
				gui.createNewProgram()
			}
		}, gui.window)
}

// createNewProgram очищает программу и добавляет блоки "Начать" и "Стоп".
func (gui *MainGUI) createNewProgram() {
	gui.programMgr.ClearProgram()

	// Создаём и вставляем блок "Начать" (в начало)
	startBlock := gui.programMgr.CreateBlock(BlockTypeStart, 0, 0)
	gui.programMgr.InsertBlock(startBlock, 0)

	// Создаём и вставляем блок "Стоп" (в конец)
	stopBlock := gui.programMgr.CreateBlock(BlockTypeStop, 0, 0)
	gui.programMgr.InsertBlock(stopBlock, -1)

	// Обновляем отображение
	gui.programPanel.ReloadFromProgram()
	gui.updateToolbarState()
	gui.updateBlockButtonsState()

	log.Println("Создана новая программа с блоками 'Начать' и 'Стоп'")
}

// handleBlockSelection обрабатывает выбор обычного блока для вставки.
func (gui *MainGUI) handleBlockSelection(blockType BlockType) {
	log.Printf("Выбран блок для вставки: %v", blockType)
	gui.programPanel.SetInsertMode(blockType)
	if gui.insertCancelButton != nil {
		gui.insertCancelButton.Show()
	}
	gui.updateBlockButtonsState()
}

// handleLoopBlockSelection обрабатывает выбор блока цикла – создаёт сразу два блока с уникальными ID.
func (gui *MainGUI) handleLoopBlockSelection() {
	log.Println("Создание пары блоков цикла...")

	// Получаем текущий максимальный ID из программы
	prog := gui.programMgr.state.GetProgram()
	maxID := 0
	for _, b := range prog.Blocks {
		if b.ID > maxID {
			maxID = b.ID
		}
	}

	// Создаём два блока с явными уникальными ID
	loopStart := &ProgramBlock{
		ID:         maxID + 1,
		Type:       BlockTypeLoopStart,
		Width:      DefaultBlockWidth,
		Height:     DefaultBlockHeight,
		Parameters: make(map[string]interface{}),
		Color:      getBlockColor(BlockTypeLoopStart),
	}
	loopEnd := &ProgramBlock{
		ID:         maxID + 2,
		Type:       BlockTypeLoopEnd,
		Width:      DefaultBlockWidth,
		Height:     DefaultBlockHeight,
		Parameters: make(map[string]interface{}),
		Color:      getBlockColor(BlockTypeLoopEnd),
	}

	// Настраиваем блоки (заполняем Title, Description, OnExecute и т.д.)
	gui.programMgr.configureBlock(loopStart)
	gui.programMgr.configureBlock(loopEnd)

	// Связываем их параметрами
	loopStart.Parameters["loopEndID"] = loopEnd.ID
	loopEnd.Parameters["loopStartID"] = loopStart.ID

	// Передаём пару в панель для последующей вставки через валентные точки
	gui.programPanel.SetInsertLoopPair(loopStart, loopEnd)

	// Включаем режим вставки
	if gui.insertCancelButton != nil {
		gui.insertCancelButton.Show()
	}
	gui.updateBlockButtonsState()
}

// CancelInsertMode отменяет режим вставки.
func (gui *MainGUI) CancelInsertMode() {
	log.Println("Отмена режима вставки")
	if gui.programPanel != nil {
		gui.programPanel.CancelInsertMode()
	}
	if gui.insertCancelButton != nil {
		gui.insertCancelButton.Hide()
	}
	gui.updateBlockButtonsState()
	gui.updateToolbarState()
}

// setupKeyboardShortcuts настраивает горячие клавиши.
func (gui *MainGUI) setupKeyboardShortcuts() {
	gui.window.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		switch event.Name {
		case fyne.KeyDelete, fyne.KeyBackspace:
			if gui.GetSelectedBlock() != nil {
				gui.deleteSelectedBlock()
			}
		case fyne.KeyEscape:
			if gui.programPanel.IsInsertMode() {
				gui.CancelInsertMode()
			} else if gui.GetSelectedBlock() != nil {
				gui.SetSelectedBlock(nil)
				gui.programPanel.SetSelectedBlock(nil)
				gui.clearPropertiesPanel()
			}
		case fyne.KeySpace:
			gui.handleSpaceKey()
		case fyne.KeyF5:
			if gui.toolbar != nil && gui.toolbar.runButton != nil && !gui.toolbar.runButton.Disabled() {
				gui.handleRunButton()
			}
		case fyne.KeyF6:
			if gui.toolbar != nil && gui.toolbar.stopButton != nil && !gui.toolbar.stopButton.Disabled() {
				gui.handleStopButton()
			}
		case fyne.KeyF1:
			if gui.toolbar != nil {
				gui.toolbar.showHelp()
			}
		case fyne.KeyEqual, fyne.KeyPlus:
			if gui.programPanel != nil {
				gui.programPanel.ZoomIn()
				gui.updateScaleLabel()
			}
		case fyne.KeyMinus:
			if gui.programPanel != nil {
				gui.programPanel.ZoomOut()
				gui.updateScaleLabel()
			}
		case fyne.Key0:
			if gui.programPanel != nil {
				gui.programPanel.ResetZoom()
				gui.updateScaleLabel()
			}
		}
	})
}

// updateBlockButtonsState обновляет состояние кнопок блоков.
func (gui *MainGUI) updateBlockButtonsState() {
	isInsertMode := gui.programPanel != nil && gui.programPanel.IsInsertMode()

	for _, button := range gui.blockButtons {
		if isInsertMode {
			button.Disable()
		} else {
			button.Enable()
		}
	}
}

// createZoomControls создает элементы управления масштабированием.
func (gui *MainGUI) createZoomControls() {
	zoomOutButton := widget.NewButtonWithIcon("", theme.ZoomOutIcon(), func() {
		if gui.programPanel != nil {
			gui.programPanel.ZoomOut()
			gui.updateScaleLabel()
		}
	})
	zoomOutButton.Importance = widget.LowImportance

	zoomInButton := widget.NewButtonWithIcon("", theme.ZoomInIcon(), func() {
		if gui.programPanel != nil {
			gui.programPanel.ZoomIn()
			gui.updateScaleLabel()
		}
	})
	zoomInButton.Importance = widget.LowImportance

	resetZoomButton := widget.NewButtonWithIcon("100%", theme.ViewRestoreIcon(), func() {
		if gui.programPanel != nil {
			gui.programPanel.ResetZoom()
			gui.updateScaleLabel()
		}
	})
	resetZoomButton.Importance = widget.LowImportance

	gui.scaleLabel = widget.NewLabel("100%")
	gui.scaleLabel.Alignment = fyne.TextAlignCenter
	gui.scaleLabel.TextStyle.Bold = true

	gui.zoomControls = container.NewHBox(
		widget.NewLabel("Масштаб:"),
		zoomOutButton,
		resetZoomButton,
		zoomInButton,
		gui.scaleLabel,
		layout.NewSpacer(),
	)
}

// updateScaleLabel обновляет метку масштаба.
func (gui *MainGUI) updateScaleLabel() {
	if gui.programPanel != nil && gui.scaleLabel != nil {
		scale := gui.programPanel.GetScale()
		gui.scaleLabel.SetText(fmt.Sprintf("%.0f%%", scale*100))
		gui.scaleLabel.Refresh()
	}
}

// deleteSelectedBlock удаляет выбранный блок.
func (gui *MainGUI) deleteSelectedBlock() {
	selected := gui.GetSelectedBlock()
	if selected == nil {
		return
	}

	blockID := selected.ID
	blockTitle := selected.Title

	if selected.Type == BlockTypeLoopStart || selected.Type == BlockTypeLoopEnd {
		gui.deleteLoopWithConfirmation(blockID)
		return
	}

	dialog.ShowConfirm("Удалить блок",
		fmt.Sprintf("Удалить блок '%s' (ID: %d)?", blockTitle, blockID),
		func(confirmed bool) {
			if confirmed {
				log.Printf("Начинаем удаление блока %d", blockID)
				gui.programPanel.RemoveBlock(blockID)
				gui.clearPropertiesPanel()
				gui.SetSelectedBlock(nil)
				log.Printf("Блок %d удален", blockID)
				gui.updateToolbarState()
			}
		}, gui.window)
}

// deleteLoopWithConfirmation удаляет весь цикл с подтверждением.
func (gui *MainGUI) deleteLoopWithConfirmation(blockID int) {
	var loopStartID, loopEndID int
	var loopStartBlock, loopEndBlock *ProgramBlock

	if block, exists := gui.programMgr.GetBlock(blockID); exists {
		switch block.Type {
		case BlockTypeLoopStart:
			loopStartID = blockID
			loopStartBlock = block
			if loopEndIDVal, ok := block.Parameters["loopEndID"].(int); ok && loopEndIDVal > 0 {
				loopEndID = loopEndIDVal
				loopEndBlock, _ = gui.programMgr.GetBlock(loopEndID)
			}
		case BlockTypeLoopEnd:
			loopEndID = blockID
			loopEndBlock = block
			if loopStartIDVal, ok := block.Parameters["loopStartID"].(int); ok && loopStartIDVal > 0 {
				loopStartID = loopStartIDVal
				loopStartBlock, _ = gui.programMgr.GetBlock(loopStartID)
			}
		}
	}

	if loopStartBlock == nil || loopEndBlock == nil {
		dialog.ShowError(fmt.Errorf("Не удалось найти связанный блок цикла"), gui.window)
		return
	}

	loopBlocks, found := gui.programMgr.GetLoopBlocks(loopStartID)
	if !found {
		dialog.ShowError(fmt.Errorf("Не удалось найти блоки цикла"), gui.window)
		return
	}

	blockCount := len(loopBlocks)

	dialog.ShowConfirm("Удалить цикл",
		fmt.Sprintf("Вы уверены, что хотите удалить весь цикл?\nУдалено будет %d блоков (включая начало и конец цикла).", blockCount),
		func(confirmed bool) {
			if confirmed {
				log.Printf("Начинаем удаление цикла (начало: %d, конец: %d)", loopStartID, loopEndID)
				gui.programPanel.RemoveLoopBlocks(loopBlocks)
				gui.clearPropertiesPanel()
				gui.SetSelectedBlock(nil)
				gui.programPanel.SetSelectedBlock(nil)
				log.Printf("Цикл удален (начало: %d, конец: %d)", loopStartID, loopEndID)
				gui.updateToolbarState()
			}
		}, gui.window)
}

// clearPropertiesPanel очищает панель свойств.
func (gui *MainGUI) clearPropertiesPanel() {
	if gui.propertiesPanel != nil {
		container, ok := gui.propertiesPanel.Content.(*fyne.Container)
		if ok {
			container.Objects = nil
			container.Add(widget.NewLabel("Выберите элемент для просмотра свойств"))
			container.Refresh()
			gui.propertiesPanel.Refresh()
		}
	}
}

// createToolbar создает панель инструментов.
func (gui *MainGUI) createToolbar() {
	gui.toolbar = NewToolbar(gui)
}

// createPropertiesPanel создает панель свойств.
func (gui *MainGUI) createPropertiesPanel() *container.Scroll {
	content := container.NewVBox(
		widget.NewLabelWithStyle("Свойства", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("Выберите элемент для просмотра свойств"),
	)
	return container.NewVScroll(content)
}

// getBlockName возвращает имя блока по типу.
func (gui *MainGUI) getBlockName(blockType BlockType) string {
	switch blockType {
	case BlockTypeStart:
		return "Начать"
	case BlockTypeMotor:
		return "Мотор"
	case BlockTypeLED:
		return "Светодиод"
	case BlockTypeWait:
		return "Ждать"
	case BlockTypeLoopStart:
		return "ДЛЯ (цикл)"
	case BlockTypeLoopEnd:
		return "КЦ (цикл)"
	case BlockTypeCondition:
		return "Условие"
	case BlockTypeTiltSensor:
		return "Датчик наклона"
	case BlockTypeDistanceSensor:
		return "Датчик расстояния"
	case BlockTypeSound:
		return "Звук"
	case BlockTypeVoltageSensor:
		return "Датчик напряжения"
	case BlockTypeCurrentSensor:
		return "Датчик тока"
	case BlockTypeStop:
		return "Стоп"
	default:
		return "Неизвестный блок"
	}
}

// showBlockProperties показывает свойства выбранного блока.
func (gui *MainGUI) showBlockProperties(block *ProgramBlock) {
	gui.SetSelectedBlock(block)

	if gui.propertiesPanel != nil {
		container, ok := gui.propertiesPanel.Content.(*fyne.Container)
		if ok {
			container.Objects = nil

			editor := NewBlockEditor(block, gui.deviceMgr, gui.window, func(updatedBlock *ProgramBlock) {
				gui.programMgr.UpdateBlock(updatedBlock.ID, updatedBlock.Parameters)
				log.Printf("Параметры блока %d обновлены", updatedBlock.ID)
			})

			container.Add(editor.GetContainer())
			container.Refresh()
			gui.propertiesPanel.Refresh()
		}
	}
}

// showHubDiscoveryDialog показывает диалог поиска хаба.
func (gui *MainGUI) showHubDiscoveryDialog() {
	progress := dialog.NewProgressInfinite("Поиск WeDo 2.0 хаба", "Сканирование...", gui.window)
	progress.Show()

	go func() {
		hubs, err := gui.hubMgr.ScanForHubs(5 * time.Second)

		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(err, gui.window)
				return
			}

			if len(hubs) == 0 {
				dialog.ShowInformation("Хабы не найдены",
					"Убедитесь, что:\n1. Хаб включен (нажата кнопка)\n2. Хаб находится в режиме подключения\n3. Bluetooth адаптер активен",
					gui.window)
				return
			}

			items := make([]string, len(hubs))
			for i, hub := range hubs {
				items[i] = fmt.Sprintf("%s [%s]", hub.Name, hub.Address)
			}

			list := widget.NewSelect(items, func(selected string) {
				if selected != "" {
					parts := strings.Split(selected, " [")
					if len(parts) > 1 {
						address := strings.TrimSuffix(parts[1], "]")
						gui.connectToHub(address)
					}
				}
			})

			content := container.NewVBox(
				widget.NewLabel("Выберите хаб для подключения:"),
				list,
			)

			selectDialog := dialog.NewCustom("Выбор хаба", "Закрыть", content, gui.window)
			selectDialog.Show()
		})
	}()
}

// connectToHub подключается к указанному хабу.
func (gui *MainGUI) connectToHub(address string) {
	progress := dialog.NewProgressInfinite("Подключение", "Подключение к хабу...", gui.window)
	progress.Show()

	go func() {
		err := gui.hubMgr.Connect(address)

		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(err, gui.window)
			} else {
				gui.updateConnectionStatus(true)
				dialog.ShowInformation("Успешно", "Подключение установлено!", gui.window)

				go func() {
					time.Sleep(3 * time.Second)
					log.Println("Запуск обнаружения устройств...")

					if gui.hubMgr != nil && gui.hubMgr.IsConnected() {
						gui.hubMgr.autoDetectDevicesV2()
					}

					time.Sleep(2 * time.Second)
					fyne.Do(func() {
						gui.updateDeviceList()
						gui.updateAvailableBlocks()
					})
				}()
			}
		})
	}()
}

// updateConnectionStatus обновляет статус подключения.
func (gui *MainGUI) updateConnectionStatus(isConnected bool) {
	fyne.Do(func() {
		if isConnected {
			gui.statusLabel.SetText("Подключено ✓")
			gui.connectButton.Disable()
			gui.disconnectButton.Enable()
		} else {
			gui.statusLabel.SetText("Не подключено")
			gui.connectButton.Enable()
			gui.disconnectButton.Disable()
			gui.state.SetConnectedHub(nil)
			gui.state.ClearDevices()
			gui.state.SetAvailableBlocks(make(map[BlockType]bool))
			gui.state.SetSelectedBlock(nil)
			gui.clearDeviceDisplay()
		}
		gui.statusLabel.Refresh()
		gui.connectButton.Refresh()
		gui.disconnectButton.Refresh()
	})
}

// UpdateBatteryDisplay обновляет отображение батареи.
func (gui *MainGUI) UpdateBatteryDisplay(batteryLevel int) {
	gui.state.UpdateHubBattery(batteryLevel)
	fyne.Do(func() {
		if gui.batteryProgress != nil {
			gui.batteryProgress.SetValue(float64(batteryLevel) / 100)
			gui.batteryProgress.Refresh()
		}
	})
}

// UpdateHubInfoDisplay обновляет отображение информации о хабе.
func (gui *MainGUI) UpdateHubInfoDisplay(info *HubInfo) {
	gui.state.SetConnectedHub(info)
	fyne.Do(func() {
		gui.updateHubInfoUI(info)
	})
}

// createDevicePanel создает панель устройств.
func (gui *MainGUI) createDevicePanel() *fyne.Container {
	mainContainer := container.NewVBox()

	title := canvas.NewText("Информация о хабе", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(title))
	mainContainer.Add(widget.NewSeparator())

	batteryContainer := gui.createBatteryWidget()
	mainContainer.Add(batteryContainer)
	mainContainer.Add(widget.NewSeparator())

	hubTitle := canvas.NewText("Хаб", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	hubTitle.TextSize = 14
	hubTitle.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(hubTitle))

	gui.hubInfoContainer = container.NewVBox()
	mainContainer.Add(gui.hubInfoContainer)
	mainContainer.Add(widget.NewSeparator())

	devicesTitle := canvas.NewText("Подключенные устройства", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	devicesTitle.TextSize = 14
	devicesTitle.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(devicesTitle))

	gui.deviceList = gui.createDeviceList()
	mainContainer.Add(gui.deviceList)

	detectButton := widget.NewButton("Обнаружить устройства", func() {
		log.Println("Ручное обнаружение устройств...")
		go func() {
			if gui.deviceMgr != nil {
				gui.deviceMgr.ForceDetectAllDevices()
			}
			time.Sleep(500 * time.Millisecond)
			fyne.Do(func() {
				gui.updateDeviceList()
				gui.updateAvailableBlocks()
			})
		}()
	})
	detectButton.Importance = widget.MediumImportance
	mainContainer.Add(detectButton)

	return mainContainer
}

// createDeviceList создает список устройств.
func (gui *MainGUI) createDeviceList() *widget.List {
	list := widget.NewList(
		func() int {
			devices := gui.state.GetAllDevices()
			count := 0
			for _, dev := range devices {
				if dev.IsConnected {
					count++
				}
			}
			return count
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.ComputerIcon())
			info := widget.NewLabel("Порт X: Устройство")
			info.TextStyle.Bold = true
			status := widget.NewLabel("✓ Подключено")
			status.TextStyle.Italic = true
			return container.NewHBox(icon, info, layout.NewSpacer(), status)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			devices := gui.state.GetAllDevices()
			type pair struct {
				port byte
				dev  *Device
			}
			connected := make([]pair, 0, len(devices))
			for port, dev := range devices {
				if dev.IsConnected {
					connected = append(connected, pair{port, dev})
				}
			}
			// Сортировка по порту
			for i := 0; i < len(connected)-1; i++ {
				for j := i + 1; j < len(connected); j++ {
					if connected[i].port > connected[j].port {
						connected[i], connected[j] = connected[j], connected[i]
					}
				}
			}

			if id < len(connected) {
				port := connected[id].port
				dev := connected[id].dev

				containerObj := obj.(*fyne.Container)
				if len(containerObj.Objects) >= 4 {
					var iconRes fyne.Resource
					switch dev.DeviceType {
					case DEVICE_TYPE_MOTOR:
						iconRes = theme.StorageIcon()
					case DEVICE_TYPE_RGB_LIGHT:
						iconRes = theme.VisibilityIcon()
					case DEVICE_TYPE_TILT_SENSOR:
						iconRes = theme.ViewRefreshIcon()
					case DEVICE_TYPE_MOTION_SENSOR:
						iconRes = theme.MoveDownIcon()
					case DEVICE_TYPE_PIEZO_TONE:
						iconRes = theme.MediaFastForwardIcon()
					default:
						iconRes = theme.ComputerIcon()
					}
					iconWidget := containerObj.Objects[0].(*widget.Icon)
					iconWidget.SetResource(iconRes)

					infoWidget := containerObj.Objects[1].(*widget.Label)
					infoWidget.SetText(fmt.Sprintf("Порт %d: %s", port, dev.Name))
				}
			}
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		list.Unselect(id)
	}

	return list
}

// updateDeviceList обновляет список устройств.
func (gui *MainGUI) updateDeviceList() {
	if gui.deviceList != nil {
		gui.deviceList.Refresh()
	}
}

// createBatteryWidget создает виджет батареи.
func (gui *MainGUI) createBatteryWidget() *fyne.Container {
	title := canvas.NewText("Батарея", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 14
	title.TextStyle.Bold = true

	gui.batteryProgress = widget.NewProgressBar()
	gui.batteryProgress.Min = 0
	gui.batteryProgress.Max = 1
	gui.batteryProgress.SetValue(0)
	gui.batteryProgress.TextFormatter = func() string {
		if gui.batteryProgress.Value <= 0 {
			return "--%"
		}
		return fmt.Sprintf("%.0f%%", gui.batteryProgress.Value*100)
	}

	return container.NewVBox(
		container.NewCenter(title),
		gui.batteryProgress,
	)
}

// updateHubInfoUI обновляет информацию о хабе в UI.
func (gui *MainGUI) updateHubInfoUI(info *HubInfo) {
	if gui.hubInfoContainer == nil {
		return
	}

	gui.hubInfoContainer.Objects = nil

	nameLabel := widget.NewLabel(fmt.Sprintf("Имя: %s", info.Name))
	gui.hubInfoContainer.Add(nameLabel)

	addressLabel := widget.NewLabel(fmt.Sprintf("Адрес: %s", info.Address))
	gui.hubInfoContainer.Add(addressLabel)

	if info.Manufacturer != "" {
		manufacturerLabel := widget.NewLabel(fmt.Sprintf("Производитель: %s", info.Manufacturer))
		gui.hubInfoContainer.Add(manufacturerLabel)
	}

	if info.FirmwareVersion != "" {
		firmwareLabel := widget.NewLabel(fmt.Sprintf("Прошивка: %s", info.FirmwareVersion))
		gui.hubInfoContainer.Add(firmwareLabel)
	}

	if info.SoftwareVersion != "" {
		softwareLabel := widget.NewLabel(fmt.Sprintf("Софт: %s", info.SoftwareVersion))
		gui.hubInfoContainer.Add(softwareLabel)
	}

	gui.hubInfoContainer.Refresh()
}

// clearDeviceDisplay очищает отображение устройств.
func (gui *MainGUI) clearDeviceDisplay() {
	if gui.hubInfoContainer != nil {
		gui.hubInfoContainer.Objects = nil
		gui.hubInfoContainer.Refresh()
	}
	if gui.deviceList != nil {
		gui.deviceList.Refresh()
	}
	if gui.batteryProgress != nil {
		gui.batteryProgress.SetValue(0)
		gui.batteryProgress.Refresh()
	}
}

// updateAvailableBlocks обновляет доступные блоки программирования.
func (gui *MainGUI) updateAvailableBlocks() {
	available := make(map[BlockType]bool)

	// Базовые логические блоки всегда доступны
	available[BlockTypeWait] = true
	available[BlockTypeLoopStart] = true
	available[BlockTypeLoopEnd] = true
	available[BlockTypeCondition] = true

	devices := gui.state.GetAllDevices()
	for _, device := range devices {
		if !device.IsConnected {
			continue
		}
		switch device.DeviceType {
		case DEVICE_TYPE_MOTOR:
			available[BlockTypeMotor] = true
		case DEVICE_TYPE_RGB_LIGHT:
			available[BlockTypeLED] = true
		case DEVICE_TYPE_TILT_SENSOR:
			available[BlockTypeTiltSensor] = true
		case DEVICE_TYPE_MOTION_SENSOR:
			available[BlockTypeDistanceSensor] = true
		case DEVICE_TYPE_PIEZO_TONE:
			available[BlockTypeSound] = true
		case DEVICE_TYPE_VOLTAGE:
			available[BlockTypeVoltageSensor] = true
		case DEVICE_TYPE_CURRENT:
			available[BlockTypeCurrentSensor] = true
		}
	}

	gui.state.SetAvailableBlocks(available)
}

// updateToolbarState обновляет состояние панели инструментов.
func (gui *MainGUI) updateToolbarState() {
	if gui.toolbar == nil {
		return
	}

	isConnected := gui.hubMgr.IsConnected()
	prog := gui.state.GetProgram()
	hasProgram := len(prog.Blocks) > 0
	isRunning := gui.state.GetProgramState() == ProgramStateRunning

	gui.toolbar.UpdateState(isConnected, hasProgram, isRunning)
}

// handleRunButton обрабатывает запуск программы.
func (gui *MainGUI) handleRunButton() {
	if gui.programMgr == nil {
		return
	}

	log.Println("Запуск программы...")

	err := gui.programMgr.RunProgram()
	if err != nil {
		log.Printf("Ошибка запуска программы: %v", err)
		dialog.ShowError(err, gui.window)
	} else {
		log.Println("Программа успешно запущена")
		gui.updateToolbarState()
	}
}

// handleStopButton обрабатывает остановку программы.
func (gui *MainGUI) handleStopButton() {
	if gui.programMgr != nil {
		gui.programMgr.StopProgram()
		log.Println("Программа остановлена")
	}
}

// handleSpaceKey обрабатывает нажатие Space для запуска/остановки.
func (gui *MainGUI) handleSpaceKey() {
	if gui.toolbar == nil || gui.programMgr == nil {
		return
	}
	isRunning := gui.state.GetProgramState() == ProgramStateRunning

	if !isRunning {
		if gui.toolbar.runButton != nil && !gui.toolbar.runButton.Disabled() {
			gui.handleRunButton()
		}
	} else {
		if gui.toolbar.stopButton != nil && !gui.toolbar.stopButton.Disabled() {
			gui.handleStopButton()
		}
	}
}
