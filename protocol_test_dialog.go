package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ProtocolTestDialog диалог для тестирования протокола LPF2
type ProtocolTestDialog struct {
	gui       *MainGUI
	window    fyne.Window
	container *fyne.Container
}

// NewProtocolTestDialog создает диалог тестирования протокола
func NewProtocolTestDialog(gui *MainGUI, window fyne.Window) *ProtocolTestDialog {
	return &ProtocolTestDialog{
		gui:    gui,
		window: window,
	}
}

// Show показывает диалог тестирования протокола
func (d *ProtocolTestDialog) Show() {
	d.buildUI()

	content := container.NewVScroll(container.NewPadded(d.container))

	testDialog := dialog.NewCustom("Тест протокола LPF2", "Закрыть", content, d.window)
	testDialog.Resize(fyne.NewSize(600, 500))
	testDialog.Show()
}

// buildUI строит интерфейс диалога
func (d *ProtocolTestDialog) buildUI() {
	d.container = container.NewVBox()

	// Заголовок
	title := widget.NewLabel("Тест отправки команд по протоколу LPF2")
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter

	d.container.Add(title)
	d.container.Add(widget.NewSeparator())

	// Выбор режима
	modeLabel := widget.NewLabel("Режим тестирования:")
	d.container.Add(modeLabel)

	testModes := []string{
		"Ручная отправка команд",
		"Тест светодиода",
		"Тест мотора",
		"Тест пищалки",
		"Тест датчиков",
	}

	modeSelect := widget.NewSelect(testModes, func(selected string) {
		d.showModeContent(selected)
	})
	modeSelect.SetSelected("Ручная отправка команд")
	d.container.Add(modeSelect)

	// Контейнер для содержимого режима
	d.container.Add(widget.NewSeparator())
	d.showModeContent("Ручная отправка команд")
}

// showModeContent показывает содержимое выбранного режима
func (d *ProtocolTestDialog) showModeContent(mode string) {
	// Удаляем предыдущее содержимое
	if len(d.container.Objects) > 5 {
		d.container.Objects = d.container.Objects[:5]
	}

	switch mode {
	case "Ручная отправка команд":
		d.showManualSendContent()
	case "Тест светодиода":
		d.showLEDTestContent()
	case "Тест мотора":
		d.showMotorTestContent()
	case "Тест пищалки":
		d.showPiezoTestContent()
	case "Тест датчиков":
		d.showSensorTestContent()
	}
}

// showManualSendContent показывает содержимое для ручной отправки
func (d *ProtocolTestDialog) showManualSendContent() {
	// UUID характеристики
	uuidLabel := widget.NewLabel("UUID характеристики:")
	uuidEntry := widget.NewEntry()
	uuidEntry.SetPlaceHolder("Например: 00001565-1212-efde-1523-785feabcd123")

	// Предустановленные UUID
	presetUUIDs := []string{
		"00001565-1212-efde-1523-785feabcd123 - Команды (Output)",
		"00001563-1212-efde-1523-785feabcd123 - Настройка (Input)",
		"00001524-1212-efde-1523-785feabcd123 - Уведомления портов",
		"00001560-1212-efde-1523-785feabcd123 - Значения сенсоров",
	}

	uuidSelect := widget.NewSelect(presetUUIDs, func(selected string) {
		if selected != "" {
			parts := strings.Split(selected, " - ")
			if len(parts) > 0 {
				uuidEntry.SetText(parts[0])
			}
		}
	})

	// Данные в HEX
	dataLabel := widget.NewLabel("Данные (HEX):")
	dataEntry := widget.NewEntry()
	dataEntry.SetPlaceHolder("Например: 0102061701010000000201")
	dataEntry.SetText("0102061701010000000201") // Пример по умолчанию

	// Предустановленные команды
	presetCommands := []string{
		"0102061701010000000201 - Режим RGB светодиода",
		"0102061700010000000201 - Режим индексных цветов",
		"06040603FF000000 - Красный светодиод",
		"0604060300FF0000 - Зеленый светодиод",
		"060406030000FF00 - Синий светодиод",
		"0102010001010000000201 - Настройка мотора",
		"01010101 - Мотор 50% вперед",
	}

	commandSelect := widget.NewSelect(presetCommands, func(selected string) {
		if selected != "" {
			parts := strings.Split(selected, " - ")
			if len(parts) > 0 {
				dataEntry.SetText(parts[0])
			}
		}
	})

	// Результат
	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	// Кнопка отправки
	sendButton := widget.NewButton("Отправить команду", func() {
		uuid := uuidEntry.Text
		hexData := dataEntry.Text

		if uuid == "" || hexData == "" {
			resultLabel.SetText("Ошибка: заполните оба поля")
			resultLabel.TextStyle.Bold = true
			resultLabel.Refresh()
			return
		}

		// Преобразуем HEX в байты
		data, err := hexStringToBytes(hexData)
		if err != nil {
			resultLabel.SetText(fmt.Sprintf("Ошибка преобразования данных: %v", err))
			resultLabel.TextStyle.Bold = true
			resultLabel.Refresh()
			return
		}

		// Отправляем команду
		err = d.gui.hubMgr.WriteCharacteristic(uuid, data)
		if err != nil {
			resultLabel.SetText(fmt.Sprintf("Ошибка отправки: %v", err))
			resultLabel.TextStyle.Bold = true
			resultLabel.Refresh()
		} else {
			resultLabel.SetText(fmt.Sprintf("✅ Успешно отправлено!\nUUID: %s\nДанные (%d байт): %x",
				uuid, len(data), data))
			resultLabel.TextStyle.Bold = false
			resultLabel.Refresh()
		}
	})

	// Чтение характеристики
	readButton := widget.NewButton("Прочитать характеристику", func() {
		uuid := uuidEntry.Text

		if uuid == "" {
			resultLabel.SetText("Ошибка: укажите UUID характеристики")
			resultLabel.TextStyle.Bold = true
			resultLabel.Refresh()
			return
		}

		data, err := d.gui.hubMgr.ReadCharacteristic(uuid)
		if err != nil {
			resultLabel.SetText(fmt.Sprintf("Ошибка чтения: %v", err))
			resultLabel.TextStyle.Bold = true
			resultLabel.Refresh()
		} else {
			resultLabel.SetText(fmt.Sprintf("✅ Прочитано успешно!\nUUID: %s\nДанные (%d байт): %x\nТекст: %s",
				uuid, len(data), data, string(data)))
			resultLabel.TextStyle.Bold = false
			resultLabel.Refresh()
		}
	})

	d.container.Add(uuidLabel)
	d.container.Add(uuidEntry)
	d.container.Add(uuidSelect)
	d.container.Add(dataLabel)
	d.container.Add(dataEntry)
	d.container.Add(commandSelect)
	d.container.Add(container.NewHBox(sendButton, readButton))
	d.container.Add(widget.NewSeparator())
	d.container.Add(resultLabel)
}

// showLEDTestContent показывает тест светодиода
func (d *ProtocolTestDialog) showLEDTestContent() {
	infoLabel := widget.NewLabel("Тестирование RGB светодиода хаба")
	infoLabel.Alignment = fyne.TextAlignCenter
	d.container.Add(infoLabel)

	// Выбор порта
	portLabel := widget.NewLabel("Порт светодиода:")
	portSelect := widget.NewSelect([]string{"Порт 6 (встроенный)"}, func(selected string) {
		// По умолчанию порт 6
	})
	portSelect.SetSelected("Порт 6 (встроенный)")

	// Выбор цвета
	colorLabel := widget.NewLabel("Выберите цвет:")
	colorButtons := container.NewGridWithColumns(3)

	colors := []struct {
		name    string
		r, g, b byte
	}{
		{"Красный", 255, 0, 0},
		{"Зеленый", 0, 255, 0},
		{"Синий", 0, 0, 255},
		{"Желтый", 255, 255, 0},
		{"Фиолетовый", 255, 0, 255},
		{"Голубой", 0, 255, 255},
		{"Белый", 255, 255, 255},
		{"Выкл", 0, 0, 0},
	}

	for _, color := range colors {
		btn := widget.NewButton(color.name, func(r, g, b byte, name string) func() {
			return func() {
				// Устанавливаем режим RGB
				modeCmd := []byte{0x01, 0x02, 6, 0x17, 1, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
				d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", modeCmd)

				// Устанавливаем цвет
				colorCmd := []byte{0x06, 0x04, 0x03, r, g, b}
				err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", colorCmd)

				if err != nil {
					d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
				} else {
					d.showResult(fmt.Sprintf("✅ Светодиод установлен в %s", name), false)
				}
			}
		}(color.r, color.g, color.b, color.name))

		colorButtons.Add(btn)
	}

	// Индексные цвета (Lego colors)
	legoLabel := widget.NewLabel("Индексные цвета LEGO:")
	legoButtons := container.NewGridWithColumns(3)

	legoColors := []struct {
		name  string
		index byte
	}{
		{"Розовый", 0x01},
		{"Фиолетовый", 0x02},
		{"Синий", 0x03},
		{"Бирюзовый", 0x04},
		{"Зеленый", 0x05},
		{"Желтый", 0x06},
		{"Оранжевый", 0x07},
		{"Красный", 0x09},
		{"Белый", 0x0A},
	}

	for _, color := range legoColors {
		btn := widget.NewButton(color.name, func(index byte, name string) func() {
			return func() {
				// Устанавливаем режим индексных цветов
				modeCmd := []byte{0x01, 0x02, 6, 0x17, 0, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
				d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", modeCmd)

				// Устанавливаем индексный цвет
				colorCmd := []byte{0x06, 0x04, 0x01, index}
				err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", colorCmd)

				if err != nil {
					d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
				} else {
					d.showResult(fmt.Sprintf("✅ Установлен LEGO цвет: %s", name), false)
				}
			}
		}(color.index, color.name))

		legoButtons.Add(btn)
	}

	// Результат
	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	d.container.Add(portLabel)
	d.container.Add(portSelect)
	d.container.Add(colorLabel)
	d.container.Add(colorButtons)
	d.container.Add(legoLabel)
	d.container.Add(legoButtons)
	d.container.Add(widget.NewSeparator())
	d.container.Add(resultLabel)

	// Сохраняем ссылку на resultLabel для использования в замыканиях
	d.container.Add(widget.NewLabel("")) // placeholder
}

// showMotorTestContent показывает тест мотора
func (d *ProtocolTestDialog) showMotorTestContent() {
	infoLabel := widget.NewLabel("Тестирование моторов WeDo 2.0")
	infoLabel.Alignment = fyne.TextAlignCenter
	d.container.Add(infoLabel)

	// Выбор порта
	portLabel := widget.NewLabel("Порт мотора:")
	portSelect := widget.NewSelect([]string{"Порт 1 (Motor A)", "Порт 2 (Motor B)"}, func(selected string) {
		// Обработка выбора порта
	})
	portSelect.SetSelected("Порт 1 (Motor A)")

	// Мощность
	powerLabel := widget.NewLabel("Мощность (-100% до 100%):")
	powerSlider := widget.NewSlider(-100, 100)
	powerSlider.Value = 50
	powerValueLabel := widget.NewLabel("50%")

	powerSlider.OnChanged = func(value float64) {
		powerValueLabel.SetText(fmt.Sprintf("%.0f%%", value))
	}

	// Длительность
	durationLabel := widget.NewLabel("Длительность (мс):")
	durationEntry := widget.NewEntry()
	durationEntry.SetText("1000")

	// Кнопки управления
	controlButtons := container.NewGridWithColumns(3)

	// Кнопка вперед
	forwardBtn := widget.NewButton("▶ Вперед", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2 (Motor B)" {
			port = 2
		}

		// Настраиваем мотор
		setupCmd := []byte{0x01, 0x02, port, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", setupCmd)

		// Устанавливаем скорость
		power := powerSlider.Value
		var speedByte byte
		if power < 0 {
			speedByte = byte((0x54 * power / 100) + 0xF0)
		} else if power > 0 {
			speedByte = byte((0x54 * power / 100) + 0x10)
		} else {
			speedByte = 0x00
		}

		motorCmd := []byte{port, 0x01, 0x01, speedByte}
		err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", motorCmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Мотор %d запущен: %.0f%%", port, power), false)
		}
	})

	// Кнопка стоп
	stopBtn := widget.NewButton("⏹ Стоп", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2 (Motor B)" {
			port = 2
		}

		stopCmd := []byte{port, 0x01, 0x01, 0x00}
		err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", stopCmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Мотор %d остановлен", port), false)
		}
	})

	// Кнопка назад
	backwardBtn := widget.NewButton("◀ Назад", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2 (Motor B)" {
			port = 2
		}

		// Настраиваем мотор
		setupCmd := []byte{0x01, 0x02, port, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", setupCmd)

		// Устанавливаем скорость (отрицательную)
		power := -powerSlider.Value
		var speedByte byte
		if power < 0 {
			speedByte = byte((0x54 * power / 100) + 0xF0)
		} else if power > 0 {
			speedByte = byte((0x54 * power / 100) + 0x10)
		} else {
			speedByte = 0x00
		}

		motorCmd := []byte{port, 0x01, 0x01, speedByte}
		err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", motorCmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Мотор %d назад: %.0f%%", port, -power), false)
		}
	})

	controlButtons.Add(forwardBtn)
	controlButtons.Add(stopBtn)
	controlButtons.Add(backwardBtn)

	// Результат
	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	d.container.Add(portLabel)
	d.container.Add(portSelect)
	d.container.Add(powerLabel)
	d.container.Add(container.NewHBox(powerSlider, powerValueLabel))
	d.container.Add(durationLabel)
	d.container.Add(durationEntry)
	d.container.Add(controlButtons)
	d.container.Add(widget.NewSeparator())
	d.container.Add(resultLabel)
}

// showPiezoTestContent показывает тест пищалки
func (d *ProtocolTestDialog) showPiezoTestContent() {
	infoLabel := widget.NewLabel("Тестирование пищалки (зуммера) WeDo 2.0")
	infoLabel.Alignment = fyne.TextAlignCenter
	d.container.Add(infoLabel)

	// Выбор порта
	portLabel := widget.NewLabel("Порт пищалки:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, func(selected string) {
		// Обработка выбора порта
	})
	portSelect.SetSelected("Порт 1")

	// Частота
	freqLabel := widget.NewLabel("Частота (Гц):")
	freqEntry := widget.NewEntry()
	freqEntry.SetText("440")

	// Длительность
	durationLabel := widget.NewLabel("Длительность (мс):")
	durationEntry := widget.NewEntry()
	durationEntry.SetText("1000")

	// Предустановленные ноты
	notesLabel := widget.NewLabel("Предустановленные ноты:")
	notesButtons := container.NewGridWithColumns(4)

	musicNotes := []struct {
		name      string
		frequency uint16
	}{
		{"До (C)", 262},
		{"Ре (D)", 294},
		{"Ми (E)", 330},
		{"Фа (F)", 349},
		{"Соль (G)", 392},
		{"Ля (A)", 440},
		{"Си (B)", 494},
		{"До² (C²)", 523},
	}

	for _, note := range musicNotes {
		btn := widget.NewButton(note.name, func(freq uint16, name string) func() {
			return func() {
				port := byte(1)
				if portSelect.Selected == "Порт 2" {
					port = 2
				}

				duration, _ := strconv.ParseUint(durationEntry.Text, 10, 16)

				// Настраиваем пищалку
				setupCmd := []byte{0x01, 0x02, port, 0x16, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
				d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", setupCmd)

				// Формируем команду тона
				freqLow := byte(freq & 0xFF)
				freqHigh := byte((freq >> 8) & 0xFF)
				durLow := byte(uint16(duration) & 0xFF)
				durHigh := byte((uint16(duration) >> 8) & 0xFF)

				toneCmd := []byte{
					port,     // connectId
					0x02,     // commandId
					0x04,     // dataLength
					freqLow,  // frequency low byte
					freqHigh, // frequency high byte
					durLow,   // duration low byte
					durHigh,  // duration high byte
				}

				err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", toneCmd)

				if err != nil {
					d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
				} else {
					d.showResult(fmt.Sprintf("✅ Воспроизводится нота %s (%d Гц)", name, freq), false)
				}
			}
		}(note.frequency, note.name))

		notesButtons.Add(btn)
	}

	// Кнопки управления
	controlButtons := container.NewHBox()

	playButton := widget.NewButton("▶ Воспроизвести", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2" {
			port = 2
		}

		freq, _ := strconv.ParseUint(freqEntry.Text, 10, 16)
		duration, _ := strconv.ParseUint(durationEntry.Text, 10, 16)

		// Настраиваем пищалку
		setupCmd := []byte{0x01, 0x02, port, 0x16, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", setupCmd)

		// Формируем команду тона
		freqLow := byte(uint16(freq) & 0xFF)
		freqHigh := byte((uint16(freq) >> 8) & 0xFF)
		durLow := byte(uint16(duration) & 0xFF)
		durHigh := byte((uint16(duration) >> 8) & 0xFF)

		toneCmd := []byte{
			port,     // connectId
			0x02,     // commandId
			0x04,     // dataLength
			freqLow,  // frequency low byte
			freqHigh, // frequency high byte
			durLow,   // duration low byte
			durHigh,  // duration high byte
		}

		err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", toneCmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Воспроизводится тон: %d Гц, %d мс", freq, duration), false)
		}
	})

	stopButton := widget.NewButton("⏹ Остановить", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2" {
			port = 2
		}

		stopCmd := []byte{
			port, // connectId
			0x03, // commandId
			0x00, // dataLength
		}

		err := d.gui.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", stopCmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка: %v", err), true)
		} else {
			d.showResult("✅ Пищалка остановлена", false)
		}
	})

	controlButtons.Add(playButton)
	controlButtons.Add(stopButton)

	// Результат
	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord

	d.container.Add(portLabel)
	d.container.Add(portSelect)
	d.container.Add(freqLabel)
	d.container.Add(freqEntry)
	d.container.Add(durationLabel)
	d.container.Add(durationEntry)
	d.container.Add(notesLabel)
	d.container.Add(notesButtons)
	d.container.Add(controlButtons)
	d.container.Add(widget.NewSeparator())
	d.container.Add(resultLabel)
}

// showSensorTestContent показывает тест датчиков
func (d *ProtocolTestDialog) showSensorTestContent() {
	infoLabel := widget.NewLabel("Тестирование датчиков WeDo 2.0")
	infoLabel.Alignment = fyne.TextAlignCenter
	d.container.Add(infoLabel)

	// Выбор типа датчика
	sensorTypeLabel := widget.NewLabel("Тип датчика:")
	sensorTypeSelect := widget.NewSelect([]string{
		"Датчик наклона (Tilt Sensor)",
		"Датчик расстояния (Motion Sensor)",
		"Датчик напряжения (Voltage Sensor)",
		"Датчик тока (Current Sensor)",
	}, func(selected string) {
		d.showSensorConfig(selected)
	})
	sensorTypeSelect.SetSelected("Датчик наклона (Tilt Sensor)")

	d.container.Add(sensorTypeLabel)
	d.container.Add(sensorTypeSelect)

	// Контейнер для конфигурации датчика
	configContainer := container.NewVBox()
	d.container.Add(configContainer)

	// Контейнер для результатов
	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord
	d.container.Add(widget.NewSeparator())
	d.container.Add(resultLabel)

	// Показываем начальную конфигурацию
	d.showSensorConfig("Датчик наклона (Tilt Sensor)")
}

// showSensorConfig показывает конфигурацию выбранного датчика
func (d *ProtocolTestDialog) showSensorConfig(sensorType string) {
	// Находим контейнер конфигурации (предполагаем, что он 7-й элемент)
	if len(d.container.Objects) > 7 {
		configContainer := d.container.Objects[7].(*fyne.Container)
		configContainer.Objects = nil

		switch sensorType {
		case "Датчик наклона (Tilt Sensor)":
			d.addTiltSensorConfig(configContainer)
		case "Датчик расстояния (Motion Sensor)":
			d.addDistanceSensorConfig(configContainer)
		case "Датчик напряжения (Voltage Sensor)":
			d.addVoltageSensorConfig(configContainer)
		case "Датчик тока (Current Sensor)":
			d.addCurrentSensorConfig(configContainer)
		}

		configContainer.Refresh()
	}
}

// addTiltSensorConfig добавляет конфигурацию датчика наклона
func (d *ProtocolTestDialog) addTiltSensorConfig(container *fyne.Container) {
	portLabel := widget.NewLabel("Порт датчика:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, nil)
	portSelect.SetSelected("Порт 1")

	modeLabel := widget.NewLabel("Режим работы:")
	modeSelect := widget.NewSelect([]string{
		"Режим угла наклона (0)",
		"Режим определения наклона (1)",
		"Режим определения удара (2)",
	}, nil)
	modeSelect.SetSelected("Режим определения наклона (1)")

	setupButton := widget.NewButton("Настроить датчик", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2" {
			port = 2
		}

		mode := byte(1)
		switch modeSelect.Selected {
		case "Режим угла наклона (0)":
			mode = 0
		case "Режим определения наклона (1)":
			mode = 1
		case "Режим определения удара (2)":
			mode = 2
		}

		cmd := []byte{0x01, 0x02, port, 0x22, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		err := d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка настройки: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Датчик наклона настроен (порт %d, режим %d)", port, mode), false)
		}
	})

	container.Add(portLabel)
	container.Add(portSelect)
	container.Add(modeLabel)
	container.Add(modeSelect)
	container.Add(setupButton)
}

// addDistanceSensorConfig добавляет конфигурацию датчика расстояния
func (d *ProtocolTestDialog) addDistanceSensorConfig(container *fyne.Container) {
	portLabel := widget.NewLabel("Порт датчика:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, nil)
	portSelect.SetSelected("Порт 1")

	modeLabel := widget.NewLabel("Режим работы:")
	modeSelect := widget.NewSelect([]string{
		"Измерение расстояния (0)",
		"Подсчет объектов (1)",
	}, nil)
	modeSelect.SetSelected("Измерение расстояния (0)")

	setupButton := widget.NewButton("Настроить датчик", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2" {
			port = 2
		}

		mode := byte(0)
		if modeSelect.Selected == "Подсчет объектов (1)" {
			mode = 1
		}

		cmd := []byte{0x01, 0x02, port, 0x23, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		err := d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка настройки: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Датчик расстояния настроен (порт %d, режим %d)", port, mode), false)
		}
	})

	container.Add(portLabel)
	container.Add(portSelect)
	container.Add(modeLabel)
	container.Add(modeSelect)
	container.Add(setupButton)
}

// addVoltageSensorConfig добавляет конфигурацию датчика напряжения
func (d *ProtocolTestDialog) addVoltageSensorConfig(container *fyne.Container) {
	portLabel := widget.NewLabel("Порт датчика:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, nil)
	portSelect.SetSelected("Порт 1")

	infoLabel := widget.NewLabel("Датчик напряжения измеряет напряжение батареи хаба")
	infoLabel.Wrapping = fyne.TextWrapWord

	setupButton := widget.NewButton("Настроить датчик", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2" {
			port = 2
		}

		cmd := []byte{0x01, 0x02, port, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		err := d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка настройки: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Датчик напряжения настроен (порт %d)", port), false)
		}
	})

	container.Add(portLabel)
	container.Add(portSelect)
	container.Add(infoLabel)
	container.Add(setupButton)
}

// addCurrentSensorConfig добавляет конфигурацию датчика тока
func (d *ProtocolTestDialog) addCurrentSensorConfig(container *fyne.Container) {
	portLabel := widget.NewLabel("Порт датчика:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, nil)
	portSelect.SetSelected("Порт 1")

	infoLabel := widget.NewLabel("Датчик тока измеряет потребление тока устройствами")
	infoLabel.Wrapping = fyne.TextWrapWord

	setupButton := widget.NewButton("Настроить датчик", func() {
		port := byte(1)
		if portSelect.Selected == "Порт 2" {
			port = 2
		}

		cmd := []byte{0x01, 0x02, port, 0x15, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		err := d.gui.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)

		if err != nil {
			d.showResult(fmt.Sprintf("❌ Ошибка настройки: %v", err), true)
		} else {
			d.showResult(fmt.Sprintf("✅ Датчик тока настроен (порт %d)", port), false)
		}
	})

	container.Add(portLabel)
	container.Add(portSelect)
	container.Add(infoLabel)
	container.Add(setupButton)
}

// showResult показывает результат операции
func (d *ProtocolTestDialog) showResult(message string, isError bool) {
	// Находим resultLabel (предполагаем, что это 9-й элемент для режима теста датчиков,
	// но в других режимах индекс может быть другим)

	// Простой подход: обновляем все текстовые виджеты
	for _, obj := range d.container.Objects {
		if label, ok := obj.(*widget.Label); ok && label.Text != "" {
			// Проверяем, не является ли это статической меткой
			if !strings.Contains(label.Text, ":") && len(label.Text) > 0 {
				runes := []rune(label.Text)
				if len(runes) > 0 && runes[0] != '✅' && runes[0] != '❌' {
					// Это может быть наш resultLabel
					label.SetText(message)
					if isError {
						label.Refresh()
					} else {
						label.Refresh()
					}
					break
				}
			}
		}
	}
}
