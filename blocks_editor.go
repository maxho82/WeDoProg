package main

import (
	"fmt"
	"log"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// BlockEditor редактор свойств блока
type BlockEditor struct {
	block     *ProgramBlock
	deviceMgr *DeviceManager
	container *fyne.Container
	onChange  func(block *ProgramBlock)
	window    fyne.Window // Добавляем поле для окна
}

// NewBlockEditor создает редактор свойств блока
func NewBlockEditor(block *ProgramBlock, deviceMgr *DeviceManager, window fyne.Window, onChange func(block *ProgramBlock)) *BlockEditor {
	editor := &BlockEditor{
		block:     block,
		deviceMgr: deviceMgr,
		window:    window,
		onChange:  onChange,
	}

	editor.container = editor.buildUI()
	return editor
}

// GetContainer возвращает контейнер редактора
func (e *BlockEditor) GetContainer() *fyne.Container {
	return e.container
}

// buildUI строит интерфейс редактора
func (e *BlockEditor) buildUI() *fyne.Container {
	mainContainer := container.NewVBox()

	// Заголовок
	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Настройки: %s", e.block.Title),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	mainContainer.Add(title)
	mainContainer.Add(widget.NewSeparator())

	// В зависимости от типа блока показываем разные настройки
	switch e.block.Type {
	case BlockTypeMotor:
		e.addMotorControls(mainContainer)
	case BlockTypeLED:
		e.addLEDControls(mainContainer)
	case BlockTypeWait:
		e.addWaitControls(mainContainer)
	case BlockTypeLoop:
		e.addLoopControls(mainContainer)
	case BlockTypeTiltSensor:
		e.addTiltSensorControls(mainContainer)
	case BlockTypeDistanceSensor:
		e.addDistanceSensorControls(mainContainer)
	case BlockTypeSound:
		e.addSoundControls(mainContainer)
	case BlockTypeVoltageSensor, BlockTypeCurrentSensor:
		e.addSimpleSensorControls(mainContainer, e.block.Type)
	default:
		// Для остальных блоков показываем базовую информацию
		mainContainer.Add(widget.NewLabel(fmt.Sprintf("Тип: %s", e.block.Title)))
		mainContainer.Add(widget.NewLabel(fmt.Sprintf("ID: %d", e.block.ID)))
		mainContainer.Add(widget.NewLabel(fmt.Sprintf("Позиция: (%.0f, %.0f)", e.block.X, e.block.Y)))
	}

	return mainContainer
}

// addMotorControls добавляет элементы управления для мотора
func (e *BlockEditor) addMotorControls(cont *fyne.Container) {
	// Выбор порта
	portLabel := widget.NewLabel("Порт мотора:")
	portSelect := widget.NewSelect([]string{"Порт 1 (Motor A)", "Порт 2 (Motor B)"}, func(selected string) {
		if selected == "Порт 1 (Motor A)" {
			e.block.Parameters["port"] = byte(1)
		} else {
			e.block.Parameters["port"] = byte(2)
		}
		e.notifyChange()
	})

	// Устанавливаем текущее значение
	if port, ok := e.block.Parameters["port"].(byte); ok {
		if port == 2 {
			portSelect.SetSelected("Порт 2 (Motor B)")
		} else {
			portSelect.SetSelected("Порт 1 (Motor A)")
			e.block.Parameters["port"] = byte(1)
		}
	} else {
		portSelect.SetSelected("Порт 1 (Motor A)")
		e.block.Parameters["port"] = byte(1)
	}

	// Мощность
	powerLabelWidget := widget.NewLabel("Мощность (-100% до 100%):")
	powerSlider := widget.NewSlider(-100, 100)
	powerValueLabel := widget.NewLabel("")

	// Устанавливаем текущее значение
	if power, ok := e.block.Parameters["power"].(int8); ok {
		powerSlider.Value = float64(power)
		powerValueLabel.SetText(fmt.Sprintf("%d%%", power))
	} else {
		powerSlider.Value = 50
		e.block.Parameters["power"] = int8(50)
		powerValueLabel.SetText("50%")
	}

	powerSlider.OnChanged = func(value float64) {
		e.block.Parameters["power"] = int8(value)
		powerValueLabel.SetText(fmt.Sprintf("%.0f%%", value))
		e.notifyChange()
	}

	// Длительность
	durationLabelWidget := widget.NewLabel("Длительность (мс, 0 = бесконечно):")
	durationEntry := widget.NewEntry()

	// Устанавливаем текущее значение
	if duration, ok := e.block.Parameters["duration"].(uint16); ok {
		durationEntry.SetText(fmt.Sprintf("%d", duration))
	} else {
		durationEntry.SetText("1000")
		e.block.Parameters["duration"] = uint16(1000)
	}

	durationEntry.OnChanged = func(text string) {
		if text == "" {
			e.block.Parameters["duration"] = uint16(0)
		} else if dur, err := strconv.ParseUint(text, 10, 16); err == nil {
			e.block.Parameters["duration"] = uint16(dur)
		}
		e.notifyChange()
	}

	// Кнопка теста
	testButton := widget.NewButton("Тест", func() {
		if e.deviceMgr != nil && e.deviceMgr.hubMgr != nil && e.deviceMgr.hubMgr.IsConnected() {
			port := e.block.Parameters["port"].(byte)
			power := e.block.Parameters["power"].(int8)
			duration := e.block.Parameters["duration"].(uint16)

			// Сначала синхронизируем устройства
			e.deviceMgr.SyncDevices()

			// Тестируем
			err := e.deviceMgr.SetMotorPower(port, power, duration)
			if err != nil {
				log.Printf("Ошибка теста мотора: %v", err)
				dialog.ShowError(fmt.Errorf("Ошибка теста мотора: %v\nПроверьте подключение устройства", err), e.window)
			} else {
				message := fmt.Sprintf("Мотор на порту %d запущен на мощности %d%%", port, power)
				if duration > 0 {
					message += fmt.Sprintf("\nАвтоматически остановится через %d мс", duration)
				}
				dialog.ShowInformation("Тест мотора", message, e.window)
			}
		} else {
			dialog.ShowError(fmt.Errorf("Нет подключения к хабу"), e.window)
		}
	})

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(powerLabelWidget)
	cont.Add(container.NewHBox(powerSlider, powerValueLabel))
	cont.Add(durationLabelWidget)
	cont.Add(durationEntry)
	cont.Add(testButton)
}

// addLEDControls добавляет элементы управления для светодиода
func (e *BlockEditor) addLEDControls(cont *fyne.Container) {
	// Выбор порта
	portLabel := widget.NewLabel("Порт светодиода:")
	portSelect := widget.NewSelect([]string{"Порт 6 (встроенный)"}, func(selected string) {
		e.block.Parameters["port"] = byte(6)
		e.notifyChange()
	})
	portSelect.SetSelected("Порт 6 (встроенный)")
	e.block.Parameters["port"] = byte(6)

	// Цвет
	colorLabelWidget := widget.NewLabel("Цвет (RGB):")

	// Красный
	redLabelWidget := widget.NewLabel("Красный:")
	redSlider := widget.NewSlider(0, 255)
	redValueLabel := widget.NewLabel("")

	if red, ok := e.block.Parameters["red"].(byte); ok {
		redSlider.Value = float64(red)
		redValueLabel.SetText(fmt.Sprintf("%d", red))
	} else {
		redSlider.Value = 255
		e.block.Parameters["red"] = byte(255)
		redValueLabel.SetText("255")
	}

	redSlider.OnChanged = func(value float64) {
		e.block.Parameters["red"] = byte(value)
		redValueLabel.SetText(fmt.Sprintf("%.0f", value))
		e.notifyChange()
	}

	// Зеленый
	greenLabelWidget := widget.NewLabel("Зеленый:")
	greenSlider := widget.NewSlider(0, 255)
	greenValueLabel := widget.NewLabel("")

	if green, ok := e.block.Parameters["green"].(byte); ok {
		greenSlider.Value = float64(green)
		greenValueLabel.SetText(fmt.Sprintf("%d", green))
	} else {
		greenSlider.Value = 0
		e.block.Parameters["green"] = byte(0)
		greenValueLabel.SetText("0")
	}

	greenSlider.OnChanged = func(value float64) {
		e.block.Parameters["green"] = byte(value)
		greenValueLabel.SetText(fmt.Sprintf("%.0f", value))
		e.notifyChange()
	}

	// Синий
	blueLabelWidget := widget.NewLabel("Синий:")
	blueSlider := widget.NewSlider(0, 255)
	blueValueLabel := widget.NewLabel("")

	if blue, ok := e.block.Parameters["blue"].(byte); ok {
		blueSlider.Value = float64(blue)
		blueValueLabel.SetText(fmt.Sprintf("%d", blue))
	} else {
		blueSlider.Value = 0
		e.block.Parameters["blue"] = byte(0)
		blueValueLabel.SetText("0")
	}

	blueSlider.OnChanged = func(value float64) {
		e.block.Parameters["blue"] = byte(value)
		blueValueLabel.SetText(fmt.Sprintf("%.0f", value))
		e.notifyChange()
	}

	// Быстрые цвета
	quickColorsLabelWidget := widget.NewLabel("Быстрые цвета:")
	quickColorsContainer := container.NewGridWithColumns(4)

	colors := []struct {
		name    string
		r, g, b byte
	}{
		{"Красный", 255, 0, 0},
		{"Зеленый", 0, 255, 0},
		{"Синий", 0, 0, 255},
		{"Белый", 255, 255, 255},
		{"Желтый", 255, 255, 0},
		{"Фиолетовый", 255, 0, 255},
		{"Выкл", 0, 0, 0},
	}

	for _, color := range colors {
		btn := widget.NewButton(color.name, func(r, g, b byte) func() {
			return func() {
				e.block.Parameters["red"] = r
				e.block.Parameters["green"] = g
				e.block.Parameters["blue"] = b

				redSlider.Value = float64(r)
				greenSlider.Value = float64(g)
				blueSlider.Value = float64(b)

				redValueLabel.SetText(fmt.Sprintf("%d", r))
				greenValueLabel.SetText(fmt.Sprintf("%d", g))
				blueValueLabel.SetText(fmt.Sprintf("%d", b))

				e.notifyChange()
			}
		}(color.r, color.g, color.b))

		btn.Importance = widget.LowImportance
		quickColorsContainer.Add(btn)
	}

	// Кнопка теста
	testButton := widget.NewButton("Тест", func() {
		if e.deviceMgr != nil && e.deviceMgr.hubMgr != nil && e.deviceMgr.hubMgr.IsConnected() {
			port := e.block.Parameters["port"].(byte)
			red := e.block.Parameters["red"].(byte)
			green := e.block.Parameters["green"].(byte)
			blue := e.block.Parameters["blue"].(byte)

			err := e.deviceMgr.SetLEDColor(port, red, green, blue)
			if err != nil {
				log.Printf("Ошибка теста светодиода: %v", err)
				dialog.ShowError(fmt.Errorf("Ошибка теста светодиода: %v", err), e.window)
			} else {
				dialog.ShowInformation("Тест светодиода",
					fmt.Sprintf("Светодиод на порту %d установлен в RGB(%d,%d,%d)", port, red, green, blue),
					e.window)
			}
		} else {
			dialog.ShowError(fmt.Errorf("Нет подключения к хабу"), e.window)
		}
	})

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(colorLabelWidget)
	cont.Add(redLabelWidget)
	cont.Add(container.NewHBox(redSlider, redValueLabel))
	cont.Add(greenLabelWidget)
	cont.Add(container.NewHBox(greenSlider, greenValueLabel))
	cont.Add(blueLabelWidget)
	cont.Add(container.NewHBox(blueSlider, blueValueLabel))
	cont.Add(quickColorsLabelWidget)
	cont.Add(quickColorsContainer)
	cont.Add(testButton)
}

// addWaitControls добавляет элементы управления для блока ожидания
func (e *BlockEditor) addWaitControls(cont *fyne.Container) {
	durationLabel := widget.NewLabel("Длительность ожидания (секунды):")
	durationEntry := widget.NewEntry()

	if duration, ok := e.block.Parameters["duration"].(float64); ok {
		durationEntry.SetText(fmt.Sprintf("%.1f", duration))
	} else {
		durationEntry.SetText("1.0")
		e.block.Parameters["duration"] = 1.0
	}

	durationEntry.OnChanged = func(text string) {
		if dur, err := strconv.ParseFloat(text, 64); err == nil {
			e.block.Parameters["duration"] = dur
			e.notifyChange()
		}
	}

	cont.Add(durationLabel)
	cont.Add(durationEntry)
}

// addLoopControls добавляет элементы управления для цикла
func (e *BlockEditor) addLoopControls(cont *fyne.Container) {
	loopTypeLabel := widget.NewLabel("Тип цикла:")
	loopTypeSelect := widget.NewSelect([]string{"Определенное число раз", "Бесконечно"}, func(selected string) {
		e.block.Parameters["forever"] = (selected == "Бесконечно")
		e.notifyChange()
	})

	if forever, ok := e.block.Parameters["forever"].(bool); ok && forever {
		loopTypeSelect.SetSelected("Бесконечно")
	} else {
		loopTypeSelect.SetSelected("Определенное число раз")
		e.block.Parameters["forever"] = false
	}

	countLabel := widget.NewLabel("Количество повторений:")
	countEntry := widget.NewEntry()

	if count, ok := e.block.Parameters["count"].(int); ok {
		countEntry.SetText(fmt.Sprintf("%d", count))
	} else {
		countEntry.SetText("5")
		e.block.Parameters["count"] = 5
	}

	countEntry.OnChanged = func(text string) {
		if count, err := strconv.Atoi(text); err == nil {
			e.block.Parameters["count"] = count
			e.notifyChange()
		}
	}

	cont.Add(loopTypeLabel)
	cont.Add(loopTypeSelect)
	cont.Add(countLabel)
	cont.Add(countEntry)
}

// addTiltSensorControls добавляет элементы управления для датчика наклона
func (e *BlockEditor) addTiltSensorControls(cont *fyne.Container) {
	portLabel := widget.NewLabel("Порт датчика:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, func(selected string) {
		if selected == "Порт 1" {
			e.block.Parameters["port"] = byte(1)
		} else {
			e.block.Parameters["port"] = byte(2)
		}
		e.notifyChange()
	})

	if port, ok := e.block.Parameters["port"].(byte); ok && port == 2 {
		portSelect.SetSelected("Порт 2")
	} else {
		portSelect.SetSelected("Порт 1")
		e.block.Parameters["port"] = byte(1)
	}

	cont.Add(portLabel)
	cont.Add(portSelect)
}

// addDistanceSensorControls добавляет элементы управления для датчика расстояния
func (e *BlockEditor) addDistanceSensorControls(cont *fyne.Container) {
	portLabel := widget.NewLabel("Порт датчика:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, func(selected string) {
		if selected == "Порт 1" {
			e.block.Parameters["port"] = byte(1)
		} else {
			e.block.Parameters["port"] = byte(2)
		}
		e.notifyChange()
	})

	if port, ok := e.block.Parameters["port"].(byte); ok && port == 2 {
		portSelect.SetSelected("Порт 2")
	} else {
		portSelect.SetSelected("Порт 1")
		e.block.Parameters["port"] = byte(1)
	}

	cont.Add(portLabel)
	cont.Add(portSelect)
}

// addSoundControls добавляет элементы управления для звука
func (e *BlockEditor) addSoundControls(cont *fyne.Container) {
	portLabel := widget.NewLabel("Порт пищалки:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, func(selected string) {
		if selected == "Порт 1" {
			e.block.Parameters["port"] = byte(1)
		} else {
			e.block.Parameters["port"] = byte(2)
		}
		e.notifyChange()
	})

	if port, ok := e.block.Parameters["port"].(byte); ok && port == 2 {
		portSelect.SetSelected("Порт 2")
	} else {
		portSelect.SetSelected("Порт 1")
		e.block.Parameters["port"] = byte(1)
	}

	freqLabel := widget.NewLabel("Частота (Гц):")
	freqEntry := widget.NewEntry()

	if freq, ok := e.block.Parameters["frequency"].(uint16); ok {
		freqEntry.SetText(fmt.Sprintf("%d", freq))
	} else {
		freqEntry.SetText("440")
		e.block.Parameters["frequency"] = uint16(440)
	}

	freqEntry.OnChanged = func(text string) {
		if freq, err := strconv.ParseUint(text, 10, 16); err == nil {
			e.block.Parameters["frequency"] = uint16(freq)
			e.notifyChange()
		}
	}

	durationLabel := widget.NewLabel("Длительность (мс):")
	durationEntry := widget.NewEntry()

	if duration, ok := e.block.Parameters["duration"].(uint16); ok {
		durationEntry.SetText(fmt.Sprintf("%d", duration))
	} else {
		durationEntry.SetText("1000")
		e.block.Parameters["duration"] = uint16(1000)
	}

	durationEntry.OnChanged = func(text string) {
		if dur, err := strconv.ParseUint(text, 10, 16); err == nil {
			e.block.Parameters["duration"] = uint16(dur)
			e.notifyChange()
		}
	}

	// Кнопка теста
	testButton := widget.NewButton("Тест", func() {
		if e.deviceMgr != nil && e.deviceMgr.hubMgr != nil && e.deviceMgr.hubMgr.IsConnected() {
			port := e.block.Parameters["port"].(byte)
			frequency := e.block.Parameters["frequency"].(uint16)
			duration := e.block.Parameters["duration"].(uint16)

			err := e.deviceMgr.PlayTone(port, frequency, duration)
			if err != nil {
				log.Printf("Ошибка теста звука: %v", err)
				dialog.ShowError(fmt.Errorf("Ошибка теста звука: %v", err), e.window)
			} else {
				dialog.ShowInformation("Тест звука",
					fmt.Sprintf("Звук на порту %d: частота %d Гц, длительность %d мс", port, frequency, duration),
					e.window)
			}
		} else {
			dialog.ShowError(fmt.Errorf("Нет подключения к хабу"), e.window)
		}
	})

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(freqLabel)
	cont.Add(freqEntry)
	cont.Add(durationLabel)
	cont.Add(durationEntry)
	cont.Add(testButton)
}

// addSimpleSensorControls добавляет элементы управления для простых датчиков
func (e *BlockEditor) addSimpleSensorControls(cont *fyne.Container, sensorType BlockType) {
	portLabel := widget.NewLabel("Порт датчика:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, func(selected string) {
		if selected == "Порт 1" {
			e.block.Parameters["port"] = byte(1)
		} else {
			e.block.Parameters["port"] = byte(2)
		}
		e.notifyChange()
	})

	if port, ok := e.block.Parameters["port"].(byte); ok && port == 2 {
		portSelect.SetSelected("Порт 2")
	} else {
		portSelect.SetSelected("Порт 1")
		e.block.Parameters["port"] = byte(1)
	}

	// Информация о типе датчика
	var sensorName string
	switch sensorType {
	case BlockTypeVoltageSensor:
		sensorName = "Датчик напряжения"
	case BlockTypeCurrentSensor:
		sensorName = "Датчик тока"
	}

	infoLabel := widget.NewLabel(fmt.Sprintf("%s измеряет значение на указанном порту", sensorName))
	infoLabel.Wrapping = fyne.TextWrapWord

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(infoLabel)
}

// notifyChange уведомляет об изменении блока
func (e *BlockEditor) notifyChange() {
	if e.onChange != nil {
		e.onChange(e.block)
	}
}
