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
	"fyne.io/fyne/v2/widget"
)

// MainGUI основной интерфейс приложения (теперь координирует компоненты)
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

	devicePanel     *DevicePanel
	propertiesPanel *PropertiesPanel
	programPanel    *ProgramPanel
	blocksPalette   *BlocksPalette
	zoomControls    *ZoomControls
}

// NewMainGUI создаёт новый GUI.
func NewMainGUI(window fyne.Window, hubMgr *HubManager) *MainGUI {
	state := NewAppState()
	deviceMgr := NewDeviceManager(hubMgr, state) // теперь передаём state
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
	// Обновляем состояние
	gui.state.UpdateDevice(portID, device)
	// DeviceManager не хранит кэш, но мы можем обновить значение, если это датчик (но здесь полная замена)
	// Если нужно обновить только значение, используем UpdateDeviceValue отдельно.
	fyne.Do(func() {
		gui.updateAvailableBlocks()
		if gui.devicePanel != nil {
			gui.devicePanel.Refresh()
		}
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
	gui.devicePanel = NewDevicePanel(gui, gui.state)
	gui.blocksPalette = NewBlocksPalette(gui, gui.state)
	gui.propertiesPanel = NewPropertiesPanel(gui, gui.state, gui.deviceMgr, gui.window)
	gui.programPanel = NewProgramPanel(gui, gui.programMgr)
	gui.zoomControls = NewZoomControls(gui)

	leftPanel := container.NewVBox(
		gui.devicePanel.GetContainer(),
		canvas.NewLine(color.NRGBA{R: 60, G: 60, B: 60, A: 255}),
		gui.blocksPalette.GetContainer(),
	)

	leftSplit := container.NewHSplit(leftPanel, gui.programPanel.GetContainer())
	leftSplit.SetOffset(0.25)

	rightSplit := container.NewHSplit(leftSplit, gui.propertiesPanel.GetContainer())
	rightSplit.SetOffset(0.75)

	mainContainer := container.NewBorder(
		container.NewVBox(gui.toolbar.GetContainer(), gui.zoomControls.GetContainer()),
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

	startBlock := gui.programMgr.CreateBlock(BlockTypeStart, 0, 0)
	gui.programMgr.InsertBlock(startBlock, 0)

	stopBlock := gui.programMgr.CreateBlock(BlockTypeStop, 0, 0)
	gui.programMgr.InsertBlock(stopBlock, -1)

	gui.programPanel.ReloadFromProgram()
	gui.updateToolbarState()
	gui.blocksPalette.UpdateButtonsState(false)

	log.Println("Создана новая программа с блоками 'Начать' и 'Стоп'")
}

// handleBlockSelection обрабатывает выбор обычного блока для вставки.
func (gui *MainGUI) handleBlockSelection(blockType BlockType) {
	log.Printf("Выбран блок для вставки: %v", blockType)
	gui.programPanel.SetInsertMode(blockType)
	gui.blocksPalette.ShowCancelButton(true)
	gui.blocksPalette.UpdateButtonsState(true)
}

// handleLoopBlockSelection обрабатывает выбор блока цикла – создаёт сразу два блока.
func (gui *MainGUI) handleLoopBlockSelection() {
	log.Println("Создание пары блоков цикла...")

	prog := gui.programMgr.state.GetProgram()
	maxID := 0
	for _, b := range prog.Blocks {
		if b.ID > maxID {
			maxID = b.ID
		}
	}

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

	gui.programMgr.configureBlock(loopStart)
	gui.programMgr.configureBlock(loopEnd)

	loopStart.Parameters["loopEndID"] = loopEnd.ID
	loopEnd.Parameters["loopStartID"] = loopStart.ID

	gui.programPanel.SetInsertLoopPair(loopStart, loopEnd)
	gui.blocksPalette.ShowCancelButton(true)
	gui.blocksPalette.UpdateButtonsState(true)
}

// CancelInsertMode отменяет режим вставки.
func (gui *MainGUI) CancelInsertMode() {
	log.Println("Отмена режима вставки")
	if gui.programPanel != nil {
		gui.programPanel.CancelInsertMode()
	}
	gui.blocksPalette.ShowCancelButton(false)
	gui.blocksPalette.UpdateButtonsState(false)
	gui.updateToolbarState()
}

// setupKeyboardShortcuts настраивает горячие клавиши (без изменений, но обновлены ссылки).
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
				gui.propertiesPanel.Clear()
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
				gui.zoomControls.Refresh()
			}
		case fyne.KeyMinus:
			if gui.programPanel != nil {
				gui.programPanel.ZoomOut()
				gui.zoomControls.Refresh()
			}
		case fyne.Key0:
			if gui.programPanel != nil {
				gui.programPanel.ResetZoom()
				gui.zoomControls.Refresh()
			}
		}
	})
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
	isInsertMode := gui.programPanel != nil && gui.programPanel.IsInsertMode()

	gui.toolbar.UpdateState(isConnected, hasProgram, isRunning, isInsertMode)
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
				gui.propertiesPanel.Clear()
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
				gui.propertiesPanel.Clear()
				gui.SetSelectedBlock(nil)
				gui.programPanel.SetSelectedBlock(nil)
				log.Printf("Цикл удален (начало: %d, конец: %d)", loopStartID, loopEndID)
				gui.updateToolbarState()
			}
		}, gui.window)
}

// showBlockProperties показывает свойства выбранного блока.
func (gui *MainGUI) showBlockProperties(block *ProgramBlock) {
	gui.SetSelectedBlock(block)
	gui.propertiesPanel.ShowBlockProperties(block)
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
						gui.devicePanel.Refresh()
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
			gui.devicePanel.Clear()
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
		gui.devicePanel.UpdateBattery(batteryLevel)
	})
}

// UpdateHubInfoDisplay обновляет отображение информации о хабе.
func (gui *MainGUI) UpdateHubInfoDisplay(info *HubInfo) {
	gui.state.SetConnectedHub(info)
	fyne.Do(func() {
		gui.devicePanel.UpdateHubInfo(info)
	})
}

// updateAvailableBlocks обновляет доступные блоки программирования.
func (gui *MainGUI) updateAvailableBlocks() {
	available := make(map[BlockType]bool)

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

// createToolbar создаёт панель инструментов (оставлена без изменений, но метод toolbar.UpdateState теперь принимает isInsertMode).
func (gui *MainGUI) createToolbar() {
	gui.toolbar = NewToolbar(gui)
}

/* // startMergeMode вызывается после вставки блока условия для выбора точки слияния.
func (gui *MainGUI) startMergeMode(conditionID int) {
	gui.programPanel.StartMergePointSelection(conditionID)
	// Можно также показать подсказку в статусной строке
	gui.statusLabel.SetText("Выберите точку слияния для ветвей условия")
	gui.statusLabel.Refresh()
}
*/
