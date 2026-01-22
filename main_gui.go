package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	//"fyne.io/fyne/v2/driver/desktop"
)

// MainGUI основной интерфейс приложения
type MainGUI struct {
	window     fyne.Window
	hubMgr     *HubManager
	deviceMgr  *DeviceManager
	programMgr *ProgramManager

	// Виджеты
	statusLabel        *widget.Label
	connectButton      *widget.Button
	disconnectButton   *widget.Button
	testProtocolButton *widget.Button
	toolbar            *Toolbar

	// Панели
	devicePanel     *container.Scroll
	propertiesPanel *container.Scroll
	programPanel    *ProgramPanel
	blocksPanel     *container.Scroll

	// Динамические элементы
	batteryProgress  *widget.ProgressBar
	batteryLabel     *widget.Label
	hubInfoContainer *fyne.Container
	devicesContainer *fyne.Container

	// Данные
	connectedHub     *HubInfo
	connectedDevices map[byte]*Device
	availableBlocks  map[BlockType]bool
}

// NewMainGUI создает новый GUI
func NewMainGUI(window fyne.Window, hubMgr *HubManager) *MainGUI {
	// Создаем менеджер устройств
	deviceMgr := NewDeviceManager(hubMgr)

	// Создаем менеджер программ
	programMgr := NewProgramManager(hubMgr, deviceMgr)

	gui := &MainGUI{
		window:           window,
		hubMgr:           hubMgr,
		deviceMgr:        deviceMgr,
		programMgr:       programMgr,
		connectedDevices: make(map[byte]*Device),
		availableBlocks:  make(map[BlockType]bool),
	}
	// Устанавливаем callback-функции
	hubMgr.SetBatteryUpdateCallback(gui.UpdateBatteryDisplay)
	hubMgr.SetHubInfoUpdateCallback(gui.UpdateHubInfoDisplay)
	hubMgr.SetDeviceUpdateCallback(gui.UpdateDeviceDisplay)
	hubMgr.SetConnectionStateCallback(gui.updateConnectionStatus)

	return gui
}

// BuildUI строит интерфейс приложения
func (gui *MainGUI) BuildUI() fyne.CanvasObject {
	// Создаем панели
	toolbar := gui.createToolbar() // Теперь это fyne.CanvasObject
	gui.devicePanel = gui.createDevicePanel()
	gui.propertiesPanel = gui.createPropertiesPanel()
	gui.blocksPanel = gui.createBlocksPanel()
	gui.programPanel = NewProgramPanel(gui, gui.programMgr)

	// Разделители
	leftSplit := container.NewHSplit(gui.devicePanel, gui.programPanel.GetContainer())
	leftSplit.SetOffset(0.25)

	rightSplit := container.NewHSplit(leftSplit, gui.propertiesPanel)
	rightSplit.SetOffset(0.75)

	// Основной макет
	mainContent := container.NewBorder(
		toolbar, // Верх - теперь это fyne.CanvasObject
		//gui.createStatusBar(), // Низ
		nil,        // Лево
		nil,        // Право
		rightSplit, // Центр
	)

	// Добавляем панель блоков слева
	fullLayout := container.NewBorder(
		nil, nil, gui.blocksPanel, nil, mainContent,
	)

	return fullLayout
}

// createToolbar создает панель инструментов
func (gui *MainGUI) createToolbar() *fyne.Container {
	/* gui.statusLabel = widget.NewLabel("Не подключено")
	gui.statusLabel.Alignment = fyne.TextAlignCenter
	gui.statusLabel.TextStyle.Bold = true

	gui.connectButton = widget.NewButtonWithIcon("Поиск хаба", theme.SearchIcon(), func() {
		gui.showHubDiscoveryDialog()
	})
	gui.connectButton.Importance = widget.MediumImportance

	gui.disconnectButton = widget.NewButtonWithIcon("Отключиться", theme.CancelIcon(), func() {
		gui.hubMgr.Disconnect()
		gui.updateConnectionStatus(false)
	})
	gui.disconnectButton.Importance = widget.MediumImportance
	gui.disconnectButton.Disable()

	gui.testProtocolButton = widget.NewButtonWithIcon("Тест протокола", theme.VisibilityIcon(), func() {
		gui.showProtocolTestDialog()
	})
	gui.testProtocolButton.Importance = widget.LowImportance

	toolbar := container.NewHBox(
		gui.connectButton,
		gui.disconnectButton,
		widget.NewSeparator(),
		gui.testProtocolButton,
		layout.NewSpacer(),
		gui.statusLabel,
		layout.NewSpacer(),
	)

	//return toolbar */
	// Создаем Toolbar объект
	gui.toolbar = NewToolbar(gui)
	// Приведение типа с проверкой
	if container, ok := gui.toolbar.GetContainer().(*fyne.Container); ok {
		return container
	}
	// Если не удалось, создаем пустой контейнер
	return container.NewWithoutLayout()
}

// createPropertiesPanel создает панель свойств
func (gui *MainGUI) createPropertiesPanel() *container.Scroll {
	content := container.NewVBox(
		widget.NewLabel("Свойства"),
		widget.NewSeparator(),
		widget.NewLabel("Выберите элемент для просмотра свойств"),
	)
	return container.NewVScroll(content)
}

// createBlocksPanel создает панель блоков программирования
func (gui *MainGUI) createBlocksPanel() *container.Scroll {
	// Основные блоки
	blocksContainer := container.NewVBox()

	// Заголовок
	title := canvas.NewText("Блоки программирования", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter
	blocksContainer.Add(title)
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
			blockButton := widget.NewButton(gui.getBlockName(blockType), func(bt BlockType) func() {
				return func() {
					// Добавляем блок в программу
					block := gui.programMgr.CreateBlock(bt, 100, 100)
					gui.programPanel.AddBlock(block)
					gui.updateBlocksPanel()
				}
			}(blockType))

			blockButton.Importance = widget.LowImportance
			blocksContainer.Add(blockButton)
		}

		blocksContainer.Add(widget.NewSeparator())
	}

	return container.NewVScroll(container.NewPadded(blocksContainer))
}

// createProgramPanel создает панель программирования
func (gui *MainGUI) createProgramPanel() *container.Scroll {
	// Эта функция больше не используется напрямую
	// ProgramPanel создается через NewProgramPanel в BuildUI
	return container.NewVScroll(widget.NewLabel("Панель программирования"))
}

// createStatusBar создает строку состояния
/* func (gui *MainGUI) createStatusBar() fyne.CanvasObject { // Измените возвращаемый тип
	statusText := widget.NewLabel("Готов")
	statusText.Alignment = fyne.TextAlignCenter

	return container.NewHBox( // Теперь возвращаем fyne.CanvasObject
		layout.NewSpacer(),
		statusText,
		layout.NewSpacer(),
	)
} */

// showProtocolTestDialog показывает диалог тестирования протокола
func (gui *MainGUI) showProtocolTestDialog() {
	dialog := NewProtocolTestDialog(gui, gui.window)
	dialog.Show()
}

// updateBlocksPanel обновляет панель блоков
func (gui *MainGUI) updateBlocksPanel() {
	// В реальном приложении здесь должна быть логика
	// обновления доступности блоков в зависимости от
	// подключенных устройств и состояния программы
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
	// Очищаем панель свойств
	if gui.propertiesPanel != nil {
		container, ok := gui.propertiesPanel.Content.(*fyne.Container)
		if ok {
			container.Objects = nil

			// Добавляем заголовок
			title := widget.NewLabelWithStyle(
				fmt.Sprintf("Свойства: %s", block.Title),
				fyne.TextAlignCenter,
				fyne.TextStyle{Bold: true},
			)
			container.Add(title)
			container.Add(widget.NewSeparator())

			// Добавляем информацию о блоке
			container.Add(widget.NewLabel(fmt.Sprintf("ID: %d", block.ID)))
			container.Add(widget.NewLabel(fmt.Sprintf("Тип: %s", gui.getBlockName(block.Type))))
			container.Add(widget.NewLabel(fmt.Sprintf("Позиция: (%.0f, %.0f)", block.X, block.Y)))

			// Добавляем параметры
			if len(block.Parameters) > 0 {
				container.Add(widget.NewSeparator())
				paramsLabel := widget.NewLabel("Параметры:")
				paramsLabel.TextStyle.Bold = true
				container.Add(paramsLabel)

				for key, value := range block.Parameters {
					container.Add(widget.NewLabel(fmt.Sprintf("  %s: %v", key, value)))
				}
			}

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

			// Создаем список хаба
			items := make([]string, len(hubs))
			for i, hub := range hubs {
				items[i] = fmt.Sprintf("%s [%s]", hub.Name, hub.Address)
			}

			list := widget.NewSelect(items, func(selected string) {
				if selected != "" {
					// Извлекаем адрес из выбранного элемента
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

			// Очищаем информацию
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
		if gui.batteryProgress != nil && gui.batteryLabel != nil {
			gui.batteryProgress.SetValue(float64(batteryLevel) / 100)
			gui.batteryLabel.SetText(fmt.Sprintf("%d%%", batteryLevel))
			gui.batteryProgress.Refresh()
			gui.batteryLabel.Refresh()
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
	fyne.Do(func() {
		// Сохраняем устройство
		gui.connectedDevices[portID] = device

		// Обновляем доступные блоки
		gui.updateAvailableBlocks()

		// Обновляем отображение
		gui.updateDeviceList()
	})
}

// createDevicePanel создает панель устройств
func (gui *MainGUI) createDevicePanel() *container.Scroll {
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

	return container.NewVScroll(container.NewPadded(mainContainer))
}

// createBatteryWidget создает виджет батареи
func (gui *MainGUI) createBatteryWidget() *fyne.Container {
	// Заголовок
	title := canvas.NewText("Батарея", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 14
	title.TextStyle.Bold = true

	// Прогресс-бар
	gui.batteryProgress = widget.NewProgressBar()
	gui.batteryProgress.Min = 0
	gui.batteryProgress.Max = 1
	gui.batteryProgress.SetValue(0)

	// Метка
	gui.batteryLabel = widget.NewLabel("--%")
	gui.batteryLabel.Alignment = fyne.TextAlignCenter

	return container.NewVBox(
		container.NewCenter(title),
		gui.batteryProgress,
		gui.batteryLabel,
	)
}

// updateHubInfoUI обновляет информацию о хабе в UI
func (gui *MainGUI) updateHubInfoUI(info *HubInfo) {
	if gui.hubInfoContainer == nil {
		return
	}

	gui.hubInfoContainer.Objects = nil

	// Имя хаба
	nameLabel := widget.NewLabel(fmt.Sprintf("Имя: %s", info.Name))
	gui.hubInfoContainer.Add(nameLabel)

	// Адрес
	addressLabel := widget.NewLabel(fmt.Sprintf("Адрес: %s", info.Address))
	gui.hubInfoContainer.Add(addressLabel)

	// Производитель
	if info.Manufacturer != "" {
		manufacturerLabel := widget.NewLabel(fmt.Sprintf("Производитель: %s", info.Manufacturer))
		gui.hubInfoContainer.Add(manufacturerLabel)
	}

	// Версия прошивки
	if info.FirmwareVersion != "" {
		firmwareLabel := widget.NewLabel(fmt.Sprintf("Прошивка: %s", info.FirmwareVersion))
		gui.hubInfoContainer.Add(firmwareLabel)
	}

	// Версия софта
	if info.SoftwareVersion != "" {
		softwareLabel := widget.NewLabel(fmt.Sprintf("Софт: %s", info.SoftwareVersion))
		gui.hubInfoContainer.Add(softwareLabel)
	}

	// System ID
	if info.SystemID != "" {
		systemIDLabel := widget.NewLabel(fmt.Sprintf("System ID: %s", info.SystemID))
		gui.hubInfoContainer.Add(systemIDLabel)
	}

	gui.hubInfoContainer.Refresh()
}

// updateDeviceList обновляет список устройств
func (gui *MainGUI) updateDeviceList() {
	if gui.devicesContainer == nil {
		return
	}

	gui.devicesContainer.Objects = nil

	if len(gui.connectedDevices) == 0 {
		noDevicesLabel := widget.NewLabel("Нет подключенных устройств")
		noDevicesLabel.Alignment = fyne.TextAlignCenter
		noDevicesLabel.TextStyle.Italic = true
		gui.devicesContainer.Add(noDevicesLabel)
	} else {
		for portID, device := range gui.connectedDevices {
			if device.IsConnected {
				deviceCard := gui.createDeviceCard(portID, device)
				gui.devicesContainer.Add(deviceCard)
			}
		}
	}

	gui.devicesContainer.Refresh()
}

// createDeviceCard создает карточку устройства
func (gui *MainGUI) createDeviceCard(portID byte, device *Device) *fyne.Container {
	// Иконка устройства
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

	// Информация об устройстве
	info := widget.NewLabel(fmt.Sprintf("Порт %d: %s", portID, device.Name))
	info.TextStyle.Bold = true

	status := widget.NewLabel("✓ Подключено")
	status.TextStyle.Italic = true

	// Контейнер
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

	if gui.batteryLabel != nil {
		gui.batteryLabel.SetText("--%")
		gui.batteryLabel.Refresh()
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

	// Обновляем панель блоков
	gui.updateBlocksPanelUI()
}

func (gui *MainGUI) updateBlocksPanelUI() {
	if gui.blocksPanel == nil {
		return
	}

	container, ok := gui.blocksPanel.Content.(*fyne.Container)
	if !ok {
		return
	}

	// Проходим по всем кнопкам блоков и обновляем их состояние
	for _, obj := range container.Objects {
		if button, ok := obj.(*widget.Button); ok {
			// Получаем тип блока из текста кнопки
			text := button.Text
			var blockType BlockType

			// Сопоставляем текст с типом блока
			switch text {
			case "Мотор":
				blockType = BlockTypeMotor
			case "Светодиод":
				blockType = BlockTypeLED
			case "Датчик наклона":
				blockType = BlockTypeTiltSensor
			case "Датчик расстояния":
				blockType = BlockTypeDistanceSensor
			case "Звук":
				blockType = BlockTypeSound
			case "Датчик напряжения":
				blockType = BlockTypeVoltageSensor
			case "Датчик тока":
				blockType = BlockTypeCurrentSensor
			default:
				continue
			}

			// Включаем/выключаем кнопку
			if enabled, exists := gui.availableBlocks[blockType]; exists && enabled {
				button.Enable()
			} else {
				button.Disable()
			}
		}
	}

	container.Refresh()
}

// ForceUpdateUI принудительно обновляет весь интерфейс
func (gui *MainGUI) ForceUpdateUI() {
	fyne.Do(func() {
		// Обновляем статус подключения
		isConnected := gui.hubMgr.IsConnected()
		gui.updateConnectionStatus(isConnected)

		if isConnected {
			// Обновляем информацию о хабе
			hubInfo := gui.hubMgr.GetHubInfo()
			if hubInfo != nil {
				gui.UpdateHubInfoDisplay(hubInfo)
			}

			// Обновляем батарею
			if hubInfo != nil && hubInfo.Battery > 0 {
				gui.UpdateBatteryDisplay(hubInfo.Battery)
			}

			// Обновляем устройства
			gui.updateDeviceList()

			// Обновляем доступные блоки
			gui.updateAvailableBlocks()
		} else {
			// Очищаем все при отключении
			gui.clearDeviceDisplay()
			gui.connectedDevices = make(map[byte]*Device)
			gui.availableBlocks = make(map[BlockType]bool)
		}

		// Обновляем панель инструментов
		hasProgram := len(gui.programMgr.program.Blocks) > 0
		if gui.toolbar != nil {
			gui.toolbar.UpdateState(isConnected, hasProgram)
		}
	})
}

func (gui *MainGUI) updateToolbarState(isConnected bool, hasProgram bool) {
	// Эта функция должна вызываться из Toolbar.UpdateState
	// или напрямую обновлять кнопки
	if gui.toolbar != nil {
		gui.toolbar.UpdateState(isConnected, hasProgram)
	}
}

// UpdateState обновляет состояние кнопок
func (t *Toolbar) UpdateState(isConnected bool, hasProgram bool) {
	if t.runButton != nil && t.stopButton != nil {
		if isConnected {
			t.runButton.Enable()
			t.stopButton.Enable()
		} else {
			t.runButton.Disable()
			t.stopButton.Disable()
		}
	}

	if t.saveButton != nil && t.exportButton != nil {
		if hasProgram {
			t.saveButton.Enable()
			t.exportButton.Enable()
		} else {
			t.saveButton.Disable()
			t.exportButton.Disable()
		}
	}
}
