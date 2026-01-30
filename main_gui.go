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

	// Виджеты
	statusLabel      *widget.Label
	connectButton    *widget.Button
	disconnectButton *widget.Button
	toolbar          *Toolbar

	// Панели
	devicePanel     *fyne.Container
	propertiesPanel *container.Scroll
	programPanel    *ProgramPanel
	blocksPanel     *container.Scroll

	// Динамические элементы
	batteryProgress  *widget.ProgressBar
	hubInfoContainer *fyne.Container
	devicesContainer *fyne.Container

	// Данные
	connectedHub     *HubInfo
	connectedDevices map[byte]*Device
	availableBlocks  map[BlockType]bool
	selectedBlock    *ProgramBlock
}

// NewMainGUI создает новый GUI
func NewMainGUI(window fyne.Window, hubMgr *HubManager) *MainGUI {
	deviceMgr := NewDeviceManager(hubMgr)
	programMgr := NewProgramManager(hubMgr, deviceMgr)

	gui := &MainGUI{
		window:           window,
		hubMgr:           hubMgr,
		deviceMgr:        deviceMgr,
		programMgr:       programMgr,
		connectedDevices: make(map[byte]*Device),
		availableBlocks:  make(map[BlockType]bool),
	}

	hubMgr.SetBatteryUpdateCallback(gui.UpdateBatteryDisplay)
	hubMgr.SetHubInfoUpdateCallback(gui.UpdateHubInfoDisplay)
	hubMgr.SetDeviceUpdateCallback(gui.UpdateDeviceDisplay)
	hubMgr.SetConnectionStateCallback(gui.updateConnectionStatus)

	return gui
}

// BuildUI строит интерфейс приложения
func (gui *MainGUI) BuildUI() fyne.CanvasObject {
	// Создаем панели
	toolbar := gui.createToolbar()
	gui.devicePanel = gui.createDevicePanel()
	gui.propertiesPanel = gui.createPropertiesPanel()
	gui.blocksPanel = gui.createBlocksPanel()
	gui.programPanel = NewProgramPanel(gui, gui.programMgr)

	// Левая панель: устройства + разделитель + блоки
	leftPanel := container.NewVBox(
		gui.devicePanel,
		canvas.NewLine(color.NRGBA{R: 60, G: 60, B: 60, A: 255}),
		gui.blocksPanel,
	)

	// Используем Split для правильного ресайза
	leftSplit := container.NewHSplit(leftPanel, gui.programPanel.GetContainer())
	leftSplit.SetOffset(0.25)

	rightSplit := container.NewHSplit(leftSplit, gui.propertiesPanel)
	rightSplit.SetOffset(0.75)

	// Основной макет
	mainContainer := container.NewBorder(
		toolbar,
		nil,
		nil,
		nil,
		rightSplit,
	)

	// Настраиваем горячие клавиши
	gui.setupKeyboardShortcuts()

	return mainContainer
}

// deleteSelectedBlock удаляет выбранный блок
func (gui *MainGUI) deleteSelectedBlock() {
	if gui.selectedBlock == nil {
		return
	}

	blockID := gui.selectedBlock.ID
	blockTitle := gui.selectedBlock.Title

	dialog.ShowConfirm("Удалить блок",
		fmt.Sprintf("Удалить блок '%s' (ID: %d)?", blockTitle, blockID),
		func(confirmed bool) {
			if confirmed {
				log.Printf("Начинаем удаление блока %d", blockID)

				// Удаляем блок из менеджера программ
				success := gui.programMgr.RemoveBlock(blockID)
				if !success {
					log.Printf("Не удалось удалить блок %d из менеджера программ", blockID)
				}

				// Удаляем блок с панели программирования
				gui.programPanel.RemoveBlock(blockID)

				// Очищаем панель свойств
				gui.clearPropertiesPanel()

				// Сбрасываем выделение
				gui.selectedBlock = nil

				log.Printf("Блок %d удален", blockID)

				// Обновляем состояние кнопок
				hasProgram := len(gui.programMgr.program.Blocks) > 0
				isConnected := gui.hubMgr != nil && gui.hubMgr.IsConnected()
				gui.updateToolbarState(isConnected, hasProgram)
			}
		}, gui.window)
}

// clearPropertiesPanel очищает панель свойств
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

// createToolbar создает панель инструментов
func (gui *MainGUI) createToolbar() *fyne.Container {
	gui.toolbar = NewToolbar(gui)
	if container, ok := gui.toolbar.GetContainer().(*fyne.Container); ok {
		return container
	}
	return container.NewWithoutLayout()
}

// createPropertiesPanel создает панель свойств
func (gui *MainGUI) createPropertiesPanel() *container.Scroll {
	content := container.NewVBox(
		widget.NewLabelWithStyle("Свойства", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("Выберите элемент для просмотра свойств"),
	)
	return container.NewVScroll(content)
}

// createBlocksPanel создает панель блоков программирования
func (gui *MainGUI) createBlocksPanel() *container.Scroll {
	blocksContainer := container.NewVBox()

	// Заголовок
	title := canvas.NewText("Палитра блоков", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter
	blocksContainer.Add(container.NewCenter(title))
	blocksContainer.Add(widget.NewSeparator())

	// Категории блоков
	categories := []struct {
		name   string
		blocks []BlockType
	}{
		{"Управление", []BlockType{BlockTypeStart, BlockTypeWait, BlockTypeLoop, BlockTypeStop}},
		{"Действия", []BlockType{BlockTypeMotor, BlockTypeLED, BlockTypeSound}},
		{"Датчики", []BlockType{BlockTypeTiltSensor, BlockTypeDistanceSensor, BlockTypeVoltageSensor, BlockTypeCurrentSensor}},
		{"Логика", []BlockType{BlockTypeCondition}},
	}

	for _, category := range categories {
		// Заголовок категории
		categoryLabel := canvas.NewText(category.name, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
		categoryLabel.TextSize = 14
		categoryLabel.TextStyle.Bold = true
		blocksContainer.Add(categoryLabel)

		// Блоки в категории
		for _, blockType := range category.blocks {
			blockName := gui.getBlockName(blockType)
			blockButton := widget.NewButton(blockName, func(bt BlockType) func() {
				return func() {
					block := gui.programMgr.CreateBlock(bt, 100, 100)
					gui.programPanel.AddBlock(block)
					hasProgram := len(gui.programMgr.program.Blocks) > 0
					gui.updateToolbarState(gui.hubMgr.IsConnected(), hasProgram)
					log.Printf("Добавлен новый блок: %s (ID: %d)", block.Title, block.ID)
				}
			}(blockType))

			blockButton.Importance = widget.LowImportance
			blocksContainer.Add(blockButton)
		}

		blocksContainer.Add(widget.NewSeparator())
	}

	scroll := container.NewVScroll(container.NewPadded(blocksContainer))
	scroll.SetMinSize(fyne.NewSize(220, 400))
	return scroll
}

// getBlockName возвращает имя блока по типу
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
	case BlockTypeLoop:
		return "Повторять"
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

// showBlockProperties показывает свойства выбранного блока
func (gui *MainGUI) showBlockProperties(block *ProgramBlock) {
	gui.selectedBlock = block
	gui.programPanel.SetSelectedBlock(block)

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

// showHubDiscoveryDialog показывает диалог поиска хаба
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

// connectToHub подключается к указанному хабу
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
						gui.ForceUpdateUI()
					})
				}()
			}
		})
	}()
}

// updateConnectionStatus обновляет статус подключения
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
			gui.connectedHub = nil
			gui.connectedDevices = make(map[byte]*Device)
			gui.clearDeviceDisplay()
		}

		gui.statusLabel.Refresh()
		gui.connectButton.Refresh()
		gui.disconnectButton.Refresh()
	})
}

// UpdateBatteryDisplay обновляет отображение батареи
func (gui *MainGUI) UpdateBatteryDisplay(batteryLevel int) {
	fyne.Do(func() {
		if gui.batteryProgress != nil {
			gui.batteryProgress.SetValue(float64(batteryLevel) / 100)
			gui.batteryProgress.Refresh()
		}
	})
}

// UpdateHubInfoDisplay обновляет отображение информации о хабе
func (gui *MainGUI) UpdateHubInfoDisplay(info *HubInfo) {
	fyne.Do(func() {
		gui.connectedHub = info
		gui.updateHubInfoUI(info)
	})
}

// UpdateDeviceDisplay обновляет отображение устройств
func (gui *MainGUI) UpdateDeviceDisplay(portID byte, device *Device) {
	log.Printf("UpdateDeviceDisplay: порт %d, устройство: %s, подключено: %v",
		portID, device.Name, device.IsConnected)

	fyne.Do(func() {
		gui.connectedDevices[portID] = device
		gui.updateAvailableBlocks()
		gui.updateDeviceList()
	})
}

// createDevicePanel создает панель устройств
func (gui *MainGUI) createDevicePanel() *fyne.Container {
	mainContainer := container.NewVBox()

	// Заголовок
	title := canvas.NewText("Информация о хабе", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(title))
	mainContainer.Add(widget.NewSeparator())

	// Батарея
	batteryContainer := gui.createBatteryWidget()
	mainContainer.Add(batteryContainer)
	mainContainer.Add(widget.NewSeparator())

	// Информация о хабе
	hubTitle := canvas.NewText("Хаб", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	hubTitle.TextSize = 14
	hubTitle.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(hubTitle))

	gui.hubInfoContainer = container.NewVBox()
	mainContainer.Add(gui.hubInfoContainer)
	mainContainer.Add(widget.NewSeparator())

	// Подключенные устройства
	devicesTitle := canvas.NewText("Подключенные устройства", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	devicesTitle.TextSize = 14
	devicesTitle.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(devicesTitle))

	gui.devicesContainer = container.NewVBox()
	mainContainer.Add(gui.devicesContainer)

	// Кнопка синхронизации
	syncButton := widget.NewButton("Синхронизировать устройства", func() {
		log.Println("Ручная синхронизация устройств...")
		go func() {
			if gui.deviceMgr != nil {
				gui.deviceMgr.SyncDevices()
			}
			time.Sleep(500 * time.Millisecond)
			fyne.Do(func() {
				gui.updateDeviceList()
				gui.updateAvailableBlocks()
			})
		}()
	})
	syncButton.Importance = widget.MediumImportance
	mainContainer.Add(syncButton)

	return mainContainer
}

// createBatteryWidget создает виджет батареи
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

// updateHubInfoUI обновляет информацию о хабе в UI
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

// updateDeviceList обновляет список устройств
func (gui *MainGUI) updateDeviceList() {
	if gui.devicesContainer == nil {
		return
	}

	log.Printf("Обновление списка устройств. Всего: %d", len(gui.connectedDevices))

	gui.devicesContainer.Objects = nil

	if len(gui.connectedDevices) == 0 {
		noDevicesLabel := widget.NewLabel("Нет подключенных устройств")
		noDevicesLabel.Alignment = fyne.TextAlignCenter
		noDevicesLabel.TextStyle.Italic = true
		gui.devicesContainer.Add(noDevicesLabel)
	} else {
		connectedCount := 0
		for portID, device := range gui.connectedDevices {
			if device.IsConnected {
				connectedCount++
				deviceCard := gui.createDeviceCard(portID, device)
				gui.devicesContainer.Add(deviceCard)
			}
		}

		if connectedCount == 0 {
			noDevicesLabel := widget.NewLabel("Все устройства отключены")
			noDevicesLabel.Alignment = fyne.TextAlignCenter
			noDevicesLabel.TextStyle.Italic = true
			gui.devicesContainer.Add(noDevicesLabel)
		}
	}

	gui.devicesContainer.Refresh()
}

// createDeviceCard создает карточку устройства
func (gui *MainGUI) createDeviceCard(portID byte, device *Device) *fyne.Container {
	var iconRes fyne.Resource
	switch device.DeviceType {
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

	icon := widget.NewIcon(iconRes)
	info := widget.NewLabel(fmt.Sprintf("Порт %d: %s", portID, device.Name))
	info.TextStyle.Bold = true

	status := widget.NewLabel("✓ Подключено")
	status.TextStyle.Italic = true

	return container.NewVBox(
		container.NewHBox(
			icon,
			info,
			layout.NewSpacer(),
			status,
		),
		widget.NewSeparator(),
	)
}

// clearDeviceDisplay очищает отображение устройств
func (gui *MainGUI) clearDeviceDisplay() {
	if gui.hubInfoContainer != nil {
		gui.hubInfoContainer.Objects = nil
		gui.hubInfoContainer.Refresh()
	}

	if gui.devicesContainer != nil {
		gui.devicesContainer.Objects = nil
		gui.devicesContainer.Refresh()
	}

	if gui.batteryProgress != nil {
		gui.batteryProgress.SetValue(0)
		gui.batteryProgress.Refresh()
	}
}

// updateAvailableBlocks обновляет доступные блоки программирования
func (gui *MainGUI) updateAvailableBlocks() {
	// Сбрасываем все блоки
	for blockType := BlockTypeStart; blockType <= BlockTypeStop; blockType++ {
		gui.availableBlocks[blockType] = false
	}

	// Всегда доступны базовые блоки
	gui.availableBlocks[BlockTypeStart] = true
	gui.availableBlocks[BlockTypeWait] = true
	gui.availableBlocks[BlockTypeLoop] = true
	gui.availableBlocks[BlockTypeStop] = true
	gui.availableBlocks[BlockTypeCondition] = true

	// Активируем блоки в зависимости от подключенных устройств
	for _, device := range gui.connectedDevices {
		if !device.IsConnected {
			continue
		}

		switch device.DeviceType {
		case DEVICE_TYPE_MOTOR:
			gui.availableBlocks[BlockTypeMotor] = true
		case DEVICE_TYPE_RGB_LIGHT:
			gui.availableBlocks[BlockTypeLED] = true
		case DEVICE_TYPE_TILT_SENSOR:
			gui.availableBlocks[BlockTypeTiltSensor] = true
		case DEVICE_TYPE_MOTION_SENSOR:
			gui.availableBlocks[BlockTypeDistanceSensor] = true
		case DEVICE_TYPE_PIEZO_TONE:
			gui.availableBlocks[BlockTypeSound] = true
		case DEVICE_TYPE_VOLTAGE:
			gui.availableBlocks[BlockTypeVoltageSensor] = true
		case DEVICE_TYPE_CURRENT:
			gui.availableBlocks[BlockTypeCurrentSensor] = true
		}
	}
}

// ForceUpdateUI принудительно обновляет весь интерфейс
func (gui *MainGUI) ForceUpdateUI() {
	fyne.Do(func() {
		isConnected := gui.hubMgr.IsConnected()
		gui.updateConnectionStatus(isConnected)

		if isConnected {
			hubInfo := gui.hubMgr.GetHubInfo()
			if hubInfo != nil {
				gui.UpdateHubInfoDisplay(hubInfo)
			}

			if hubInfo != nil && hubInfo.Battery > 0 {
				gui.UpdateBatteryDisplay(hubInfo.Battery)
			}

			gui.updateDeviceList()
			gui.updateAvailableBlocks()
		} else {
			gui.clearDeviceDisplay()
			gui.connectedDevices = make(map[byte]*Device)
			gui.availableBlocks = make(map[BlockType]bool)
		}

		hasProgram := len(gui.programMgr.program.Blocks) > 0
		if gui.toolbar != nil {
			gui.toolbar.UpdateState(isConnected, hasProgram)
		}
	})
}

func (gui *MainGUI) updateToolbarState(isConnected bool, hasProgram bool) {
	if gui.toolbar != nil {
		gui.toolbar.UpdateState(isConnected, hasProgram)
	}
}
