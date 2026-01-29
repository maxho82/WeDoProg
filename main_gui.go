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
	hubInfoContainer *fyne.Container
	devicesContainer *fyne.Container

	// Данные
	connectedHub     *HubInfo
	connectedDevices map[byte]*Device
	availableBlocks  map[BlockType]bool
	selectedBlock    *ProgramBlock // Выбранный блок для удаления
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
	toolbar := gui.createToolbar()
	gui.devicePanel = gui.createDevicePanel()
	gui.propertiesPanel = gui.createPropertiesPanel()
	gui.blocksPanel = gui.createBlocksPanel()
	gui.programPanel = NewProgramPanel(gui, gui.programMgr)

	// Устанавливаем минимальные размеры для лучшего отображения
	gui.blocksPanel.SetMinSize(fyne.NewSize(200, 400))
	gui.devicePanel.SetMinSize(fyne.NewSize(250, 400))
	gui.propertiesPanel.SetMinSize(fyne.NewSize(250, 400))

	// Создаем разделители с правильными пропорциями

	// 1. Слева панель устройств и блоков
	leftPanel := container.NewBorder(
		nil,             // верх
		nil,             // низ
		gui.devicePanel, // лево
		nil,             // право
		gui.blocksPanel, // центр
	)

	// 2. В центре панель программирования
	// 3. Справа панель свойств

	// Используем HBox с пропорциями
	/* 	mainContent := container.NewHBox(
		leftPanel,
		gui.programPanel.GetContainer(),
		gui.propertiesPanel,
	) */

	// Устанавливаем пропорции через layout.Spacer
	// Переделываем на использование Split для правильного ресайза
	leftSplit := container.NewHSplit(leftPanel, gui.programPanel.GetContainer())
	leftSplit.SetOffset(0.3) // Левая часть (устройства + блоки) 30%

	rightSplit := container.NewHSplit(leftSplit, gui.propertiesPanel)
	rightSplit.SetOffset(0.7) // Программирование + левая часть 70%, свойства 30%

	// Основной макет
	mainContainer := container.NewBorder(
		toolbar,    // Верх - панель инструментов
		nil,        // Низ
		nil,        // Лево
		nil,        // Право
		rightSplit, // Центр - основное содержимое
	)
	// Настраиваем обработку клавиатуры
	gui.setupKeyboardShortcuts()

	return mainContainer
}

// deleteSelectedBlock удаляет выбранный блок
func (gui *MainGUI) deleteSelectedBlock() {
	if gui.selectedBlock == nil {
		return
	}

	// Сохраняем ID для лога перед удалением
	blockID := gui.selectedBlock.ID
	blockTitle := gui.selectedBlock.Title

	// Спрашиваем подтверждение
	dialog.ShowConfirm("Удалить блок",
		fmt.Sprintf("Удалить блок '%s' (ID: %d)?", blockTitle, blockID),
		func(confirmed bool) {
			if confirmed {
				log.Printf("Начинаем удаление блока %d", blockID)

				// 1. Удаляем блок из менеджера программ
				success := gui.programMgr.RemoveBlock(blockID)
				if !success {
					log.Printf("Не удалось удалить блок %d из менеджера программ", blockID)
				}

				// 2. Удаляем блок с панели программирования
				gui.programPanel.RemoveBlock(blockID)

				// 3. Репозиционируем все оставшиеся блоки
				gui.programPanel.RepositionAllBlocks()

				// 4. Очищаем панель свойств
				gui.clearPropertiesPanel()

				// 5. Сбрасываем выделение
				gui.selectedBlock = nil

				log.Printf("Блок %d удален", blockID)

				// 6. Обновляем состояние кнопок
				hasProgram := len(gui.programMgr.program.Blocks) > 0
				isConnected := gui.hubMgr != nil && gui.hubMgr.IsConnected()
				gui.updateToolbarState(isConnected, hasProgram)
			}
		}, gui.window)
}

// removeBlockFromProgram удаляет блок из программы
func (gui *MainGUI) removeBlockFromProgram(blockID int) bool {
	log.Printf("Удаление блока %d из программы", blockID)

	// Удаляем блок из ProgramManager
	blockFound := false
	var newBlocks []*ProgramBlock
	for _, block := range gui.programMgr.program.Blocks {
		if block.ID != blockID {
			newBlocks = append(newBlocks, block)
		} else {
			blockFound = true
		}
	}

	if !blockFound {
		log.Printf("Блок %d не найден в программе", blockID)
		return false
	}

	gui.programMgr.program.Blocks = newBlocks

	// Удаляем все соединения, связанные с этим блоком
	var newConnections []*Connection
	for _, conn := range gui.programMgr.program.Connections {
		if conn.FromBlockID != blockID && conn.ToBlockID != blockID {
			newConnections = append(newConnections, conn)
		} else {
			// Если это соединение ИЗ удаляемого блока, сбрасываем NextBlockID у всех блоков, которые ссылались на него
			if conn.FromBlockID == blockID {
				// Находим блок, который ссылался на удаляемый блок и сбрасываем его NextBlockID
				for _, block := range newBlocks {
					if block.NextBlockID == blockID {
						block.NextBlockID = 0
					}
				}
			}
		}
	}
	gui.programMgr.program.Connections = newConnections

	// Обновляем последний блок Y-координаты в панели программирования
	gui.updateLastBlockY()

	gui.programMgr.program.Modified = time.Now()
	log.Printf("Блок %d удален из программы. Осталось блоков: %d, соединений: %d",
		blockID, len(newBlocks), len(newConnections))

	return true
}

// updateLastBlockY обновляет последнюю Y-координату для добавления новых блоков
func (gui *MainGUI) updateLastBlockY() {
	if len(gui.programMgr.program.Blocks) == 0 {
		gui.programPanel.lastBlockY = 50
		return
	}

	// Находим максимальную Y-координату среди всех блоков
	maxY := float64(50)
	for _, block := range gui.programMgr.program.Blocks {
		if block.Y+block.Height > maxY {
			maxY = block.Y + block.Height
		}
	}

	gui.programPanel.lastBlockY = maxY + 40 // Добавляем отступ
}

// removeConnectionsForBlock удаляет соединения для блока
func (gui *MainGUI) removeConnectionsForBlock(blockID int) {
	var newConnections []*Connection
	for _, conn := range gui.programMgr.program.Connections {
		if conn.FromBlockID != blockID && conn.ToBlockID != blockID {
			newConnections = append(newConnections, conn)
		} else {
			// Сбрасываем NextBlockID у блока, который ссылался на удаляемый
			if conn.FromBlockID != blockID {
				if block, exists := gui.programMgr.GetBlock(conn.FromBlockID); exists {
					block.NextBlockID = 0
				}
			}
		}
	}
	gui.programMgr.program.Connections = newConnections
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
	title := canvas.NewText("Палитра блоков", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
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
			// Проверяем, доступен ли блок
			blockName := gui.getBlockName(blockType)

			blockButton := widget.NewButton(blockName, func(bt BlockType) func() {
				return func() {
					// Добавляем блок в программу
					block := gui.programMgr.CreateBlock(bt, 100, 100)
					gui.programPanel.AddBlock(block)

					// Обновляем состояние кнопок панели инструментов
					hasProgram := len(gui.programMgr.program.Blocks) > 0
					gui.updateToolbarState(gui.hubMgr.IsConnected(), hasProgram)

					log.Printf("Добавлен новый блок: %s (ID: %d)", block.Title, block.ID)
				}
			}(blockType))

			blockButton.Importance = widget.LowImportance

			// Блокируем кнопку, если блок недоступен
			if enabled, exists := gui.availableBlocks[blockType]; exists && !enabled && blockType != BlockTypeStart && blockType != BlockTypeWait && blockType != BlockTypeLoop && blockType != BlockTypeStop && blockType != BlockTypeCondition {
				blockButton.Disable()
			}

			blocksContainer.Add(blockButton)
		}

		blocksContainer.Add(widget.NewSeparator())
	}

	scroll := container.NewVScroll(container.NewPadded(blocksContainer))
	scroll.SetMinSize(fyne.NewSize(220, 600))
	return scroll
}

// createProgramPanel создает панель программирования
func (gui *MainGUI) createProgramPanel() *container.Scroll {
	// Эта функция больше не используется напрямую
	// ProgramPanel создается через NewProgramPanel в BuildUI
	return container.NewVScroll(widget.NewLabel("Панель программирования"))
}

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
	// Сохраняем выбранный блок
	gui.selectedBlock = block

	// Очищаем панель свойств
	if gui.propertiesPanel != nil {
		container, ok := gui.propertiesPanel.Content.(*fyne.Container)
		if ok {
			container.Objects = nil

			// Создаем редактор свойств блока
			editor := NewBlockEditor(block, gui.deviceMgr, gui.window, func(updatedBlock *ProgramBlock) {
				// Сохраняем изменения в менеджере программ
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

				// Запускаем обнаружение портов через 3 секунды
				// После успешного подключения
				go func() {
					time.Sleep(3 * time.Second)
					log.Println("Запуск улучшенного обнаружения устройств...")

					if gui.hubMgr != nil && gui.hubMgr.IsConnected() {
						gui.hubMgr.autoDetectDevicesV2()
					}

					// Обновляем GUI
					time.Sleep(2 * time.Second)
					fyne.Do(func() {
						gui.updateDeviceList()
						gui.updateAvailableBlocks()
						gui.ForceUpdateUI() // Принудительное обновление UI
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
	log.Printf("UpdateDeviceDisplay вызван: порт %d, устройство: %s, подключено: %v",
		portID, device.Name, device.IsConnected)

	fyne.Do(func() {
		// Сохраняем устройство
		gui.connectedDevices[portID] = device

		log.Printf("Устройство сохранено. Всего устройств: %d", len(gui.connectedDevices))

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

	// Кнопка для ручного обнаружения устройств
	discoverButton := widget.NewButton("Обнаружить устройства", func() {
		log.Println("Запуск ручного обнаружения устройств...")

		go func() {
			// Даем время на обновление GUI
			time.Sleep(100 * time.Millisecond)

			// Запускаем автоматическое определение
			if gui.hubMgr != nil {
				gui.hubMgr.autoDetectDevicesV2()
			}

			// Обновляем список устройств
			time.Sleep(1 * time.Second)
			fyne.Do(func() {
				gui.updateDeviceList()
			})
		}()
	})

	syncButton := widget.NewButton("Синхронизировать устройства", func() {
		log.Println("Ручная синхронизация устройств...")

		go func() {
			if gui.deviceMgr != nil {
				gui.deviceMgr.SyncDevices()
			}

			// Обновляем список устройств
			time.Sleep(500 * time.Millisecond)
			fyne.Do(func() {
				gui.updateDeviceList()
				gui.updateAvailableBlocks()
			})
		}()
	})

	syncButton.Importance = widget.MediumImportance
	mainContainer.Add(syncButton)
	mainContainer.Add(widget.NewSeparator())

	discoverButton.Importance = widget.MediumImportance
	mainContainer.Add(discoverButton)
	mainContainer.Add(widget.NewSeparator())

	scroll := container.NewVScroll(container.NewPadded(mainContainer))
	scroll.SetMinSize(fyne.NewSize(280, 600)) // Увеличиваем ширину
	return scroll
}

// createBatteryWidget создает виджет батареи (только прогресс-бар)
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

	// Настраиваем отображение текста внутри прогресс-бара
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
		log.Println("ERROR: devicesContainer равен nil!")
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
				log.Printf("Добавлена карточка для устройства: порт %d, %s", portID, device.Name)
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
	log.Println("Список устройств обновлен")
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

// BatchUpdate выполняет несколько обновлений UI за один раз
func (gui *MainGUI) BatchUpdate(updates ...func()) {
	fyne.Do(func() {
		for _, update := range updates {
			update()
		}
	})
}
