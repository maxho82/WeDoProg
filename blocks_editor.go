package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// BlockEditor редактор свойств блока
type BlockEditor struct {
	block     *ProgramBlock
	deviceMgr *DeviceManager
	container *fyne.Container
	onChange  func(block *ProgramBlock)
	window    fyne.Window
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
	case BlockTypeLoopStart:
		e.addLoopStartControls(mainContainer)
	case BlockTypeLoopEnd:
		e.addLoopEndControls(mainContainer)
	case BlockTypeCondition:
		e.addConditionControls(mainContainer)
	case BlockTypeTiltSensor:
		e.addTiltSensorControls(mainContainer)
	case BlockTypeDistanceSensor:
		e.addDistanceSensorControls(mainContainer)
	case BlockTypeSound:
		e.addSoundControls(mainContainer)
	case BlockTypeVoltageSensor, BlockTypeCurrentSensor:
		e.addSimpleSensorControls(mainContainer, e.block.Type)
	case BlockTypeVariable:
		e.addVariableControls(mainContainer)
	default:
		// Для остальных блоков показываем базовую информацию
		mainContainer.Add(widget.NewLabel(fmt.Sprintf("Тип: %s", e.block.Title)))
		mainContainer.Add(widget.NewLabel(fmt.Sprintf("ID: %d", e.block.ID)))
		mainContainer.Add(widget.NewLabel(fmt.Sprintf("Позиция: (%.0f, %.0f)", e.block.X, e.block.Y)))
	}

	return mainContainer
}

func (e *BlockEditor) addVariableControls(cont *fyne.Container) {
	// Имя переменной
	nameLabel := widget.NewLabel("Имя переменной:")
	nameEntry := widget.NewEntry()
	if name, ok := e.block.Parameters["name"].(string); ok {
		nameEntry.SetText(name)
	} else {
		nameEntry.SetText("var")
		e.block.Parameters["name"] = "var"
	}
	nameEntry.OnChanged = func(text string) {
		e.block.Parameters["name"] = text
		e.notifyChange()
	}

	// Тип переменной
	typeLabel := widget.NewLabel("Тип:")
	typeSelect := widget.NewSelect([]string{"bool", "int", "string", "color"}, func(selected string) {
		e.block.Parameters["varType"] = selected
		e.updateVariableValueUI(cont, selected)
		e.notifyChange()
	})
	currentType, _ := e.block.Parameters["varType"].(string)
	if currentType == "" {
		currentType = "int"
	}
	typeSelect.SetSelected(currentType)

	// Контейнер для поля ввода/выбора значения
	valueContainer := container.NewVBox()
	e.updateVariableValueUI(valueContainer, currentType)

	infoLabel := widget.NewLabel("Если выражение пусто, переменная получит значение по умолчанию для типа.")
	infoLabel.Wrapping = fyne.TextWrapWord

	cont.Add(nameLabel)
	cont.Add(nameEntry)
	cont.Add(typeLabel)
	cont.Add(typeSelect)
	cont.Add(valueContainer)
	cont.Add(infoLabel)
}

func (e *BlockEditor) updateVariableValueUI(cont *fyne.Container, varType string) {
	cont.Objects = nil
	switch varType {
	case "bool":
		boolSelect := widget.NewSelect([]string{"Да", "Нет"}, func(selected string) {
			if selected == "Да" {
				e.block.Parameters["expression"] = "true"
			} else {
				e.block.Parameters["expression"] = "false"
			}
			e.notifyChange()
		})
		// Устанавливаем текущее значение
		if expr, ok := e.block.Parameters["expression"].(string); ok {
			switch expr {
			case "true":
				boolSelect.SetSelected("Да")
			case "false":
				boolSelect.SetSelected("Нет")
			default:
				boolSelect.SetSelected("Нет")
			}
		} else {
			boolSelect.SetSelected("Нет")
			e.block.Parameters["expression"] = "false"
		}
		cont.Add(boolSelect)
	case "int", "string", "color":
		exprEntry := widget.NewEntry()
		if expr, ok := e.block.Parameters["expression"].(string); ok {
			exprEntry.SetText(expr)
		} else {
			exprEntry.SetText("")
			e.block.Parameters["expression"] = ""
		}
		exprEntry.SetPlaceHolder("Например: 5 + x * 2")
		exprEntry.OnChanged = func(text string) {
			e.block.Parameters["expression"] = text
			e.notifyChange()
		}
		cont.Add(exprEntry)
	}
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

	// Контейнер для ползунка мощности
	powerContainer := container.NewBorder(nil, nil, nil, powerValueLabel, powerSlider)

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
	testButton := widget.NewButton("Тест мотор", func() {
		if e.deviceMgr != nil && e.deviceMgr.hubMgr != nil && e.deviceMgr.hubMgr.IsConnected() {
			port := e.block.Parameters["port"].(byte)
			power := e.block.Parameters["power"].(int8)
			duration := e.block.Parameters["duration"].(uint16)

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
	testButton.Importance = widget.HighImportance

	// Добавляем все элементы в контейнер
	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(powerLabelWidget)
	cont.Add(powerContainer)
	cont.Add(durationLabelWidget)
	cont.Add(durationEntry)
	cont.Add(layout.NewSpacer())
	cont.Add(container.NewCenter(testButton))
}

func (e *BlockEditor) addLEDControls(cont *fyne.Container) {
	// Режим работы
	modeLabel := widget.NewLabel("Режим:")
	modeSelect := widget.NewSelect([]string{"RGB", "Индексный"}, func(selected string) {
		mode := byte(0)
		if selected == "Индексный" {
			mode = 1
		}
		e.block.Parameters["mode"] = mode
		e.updateLEDModeUI(cont, mode)
		e.notifyChange()
	})
	currentMode, _ := e.block.Parameters["mode"].(byte)
	if currentMode == 1 {
		modeSelect.SetSelected("Индексный")
	} else {
		modeSelect.SetSelected("RGB")
		e.block.Parameters["mode"] = byte(0)
	}

	// Контейнеры для разных режимов
	rgbContainer := container.NewVBox()
	indexContainer := container.NewVBox()

	e.buildLEDRGBUI(rgbContainer)
	e.buildLEDIndexUI(indexContainer)

	modeContainer := container.NewMax()
	if currentMode == 1 {
		modeContainer.Objects = []fyne.CanvasObject{indexContainer}
	} else {
		modeContainer.Objects = []fyne.CanvasObject{rgbContainer}
	}

	cont.Add(modeLabel)
	cont.Add(modeSelect)
	cont.Add(modeContainer)
}

// buildLEDRGBUI добавляет элементы управления для светодиода RGB
func (e *BlockEditor) buildLEDRGBUI(cont *fyne.Container) {
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

	// Контейнер для ползунка красного
	redContainer := container.NewBorder(nil, nil, nil, redValueLabel, redSlider)

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

	// Контейнер для ползунка зеленого
	greenContainer := container.NewBorder(nil, nil, nil, greenValueLabel, greenSlider)

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

	// Контейнер для ползунка синего
	blueContainer := container.NewBorder(nil, nil, nil, blueValueLabel, blueSlider)

	// Быстрые цвета
	quickColorsLabelWidget := widget.NewLabel("Быстрые цвета:")
	quickColorsContainer := container.NewGridWithColumns(3)

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
	testButton := widget.NewButton("Тест светодиод", func() {
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
	testButton.Importance = widget.HighImportance

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(colorLabelWidget)
	cont.Add(redLabelWidget)
	cont.Add(redContainer)
	cont.Add(greenLabelWidget)
	cont.Add(greenContainer)
	cont.Add(blueLabelWidget)
	cont.Add(blueContainer)
	cont.Add(quickColorsLabelWidget)
	cont.Add(quickColorsContainer)
	cont.Add(layout.NewSpacer())
	cont.Add(container.NewCenter(testButton))
}

func (e *BlockEditor) buildLEDIndexUI(cont *fyne.Container) {
	portLabel := widget.NewLabel("Порт светодиода:")
	portSelect := widget.NewSelect([]string{"Порт 6 (встроенный)"}, func(selected string) {
		e.block.Parameters["port"] = byte(6)
		e.notifyChange()
	})
	portSelect.SetSelected("Порт 6 (встроенный)")
	e.block.Parameters["port"] = byte(6)

	colorExprLabel := widget.NewLabel("Цвет (индекс или выражение):")
	colorExprEntry := widget.NewEntry()
	if expr, ok := e.block.Parameters["colorExpr"].(string); ok {
		colorExprEntry.SetText(expr)
	} else {
		colorExprEntry.SetText("")
		e.block.Parameters["colorExpr"] = ""
	}
	colorExprEntry.SetPlaceHolder("Например: красный, 5, myColor")
	colorExprEntry.OnChanged = func(text string) {
		e.block.Parameters["colorExpr"] = text
		e.notifyChange()
	}

	// Кнопки быстрых цветов
	quickColorsLabel := widget.NewLabel("Быстрые цвета:")
	quickColorsContainer := container.NewGridWithColumns(3)

	colors := []struct {
		name  string
		index byte
	}{
		{"Розовый", LED_INDEX_PINK},
		{"Фиолетовый", LED_INDEX_PURPLE},
		{"Синий", LED_INDEX_BLUE},
		{"Зелёный", LED_INDEX_GREEN},
		{"Красный", LED_INDEX_RED},
		{"Белый", LED_INDEX_WHITE},
		{"Выкл", 0},
	}

	for _, col := range colors {
		btn := widget.NewButton(col.name, func(idx byte, name string) func() {
			return func() {
				colorExprEntry.SetText(name)
				e.block.Parameters["colorExpr"] = name
				e.notifyChange()
			}
		}(col.index, col.name))
		btn.Importance = widget.LowImportance
		quickColorsContainer.Add(btn)
	}

	// Кнопка теста
	testButton := widget.NewButton("Тест светодиод", func() {
		if e.deviceMgr != nil && e.deviceMgr.hubMgr != nil && e.deviceMgr.hubMgr.IsConnected() {
			port := e.block.Parameters["port"].(byte)
			expr := e.block.Parameters["colorExpr"].(string)
			// Упрощённое вычисление для теста
			var colorIdx byte = 1
			if expr != "" {
				// Пробуем как число
				if num, err := strconv.Atoi(expr); err == nil {
					colorIdx = byte(num)
				} else {
					// Пробуем как название цвета
					if idx, ok := NameToIndexColor[strings.ToLower(expr)]; ok {
						colorIdx = idx
					}
				}
			}
			err := e.deviceMgr.SetLEDIndexColor(port, colorIdx)
			if err != nil {
				log.Printf("Ошибка теста светодиода: %v", err)
				dialog.ShowError(fmt.Errorf("Ошибка теста светодиода: %v", err), e.window)
			} else {
				dialog.ShowInformation("Тест светодиода",
					fmt.Sprintf("Светодиод на порту %d установлен в индекс %d", port, colorIdx),
					e.window)
			}
		} else {
			dialog.ShowError(fmt.Errorf("Нет подключения к хабу"), e.window)
		}
	})
	testButton.Importance = widget.HighImportance

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(colorExprLabel)
	cont.Add(colorExprEntry)
	cont.Add(quickColorsLabel)
	cont.Add(quickColorsContainer)
	cont.Add(layout.NewSpacer())
	cont.Add(container.NewCenter(testButton))
}

// Вспомогательный метод для обновления UI при смене режима
func (e *BlockEditor) updateLEDModeUI(cont *fyne.Container, mode byte) {
	// Находим контейнер modeContainer (последний добавленный)
	if len(cont.Objects) < 3 {
		return
	}
	modeContainer, ok := cont.Objects[2].(*fyne.Container)
	if !ok {
		return
	}
	rgbContainer := container.NewVBox()
	indexContainer := container.NewVBox()
	e.buildLEDRGBUI(rgbContainer)
	e.buildLEDIndexUI(indexContainer)

	if mode == 1 {
		modeContainer.Objects = []fyne.CanvasObject{indexContainer}
	} else {
		modeContainer.Objects = []fyne.CanvasObject{rgbContainer}
	}
	modeContainer.Refresh()
}

// addWaitControls добавляет элементы управления для блока ожидания
func (e *BlockEditor) addWaitControls(cont *fyne.Container) {
	durationLabel := widget.NewLabel("Длительность ожидания (секунды):")
	durationSlider := widget.NewSlider(0.1, 10.0)
	durationSlider.Step = 0.1
	durationValueLabel := widget.NewLabel("")

	if duration, ok := e.block.Parameters["duration"].(float64); ok {
		durationSlider.Value = duration
		durationValueLabel.SetText(fmt.Sprintf("%.1f с", duration))
	} else {
		durationSlider.Value = 1.0
		e.block.Parameters["duration"] = 1.0
		durationValueLabel.SetText("1.0 с")
	}

	durationSlider.OnChanged = func(value float64) {
		e.block.Parameters["duration"] = value
		durationValueLabel.SetText(fmt.Sprintf("%.1f с", value))
		e.notifyChange()
	}

	// Контейнер для ползунка
	durationContainer := container.NewBorder(nil, nil, nil, durationValueLabel, durationSlider)

	cont.Add(durationLabel)
	cont.Add(durationContainer)
}

// addLoopStartControls добавляет элементы управления для начала цикла
func (e *BlockEditor) addLoopStartControls(cont *fyne.Container) {
	// Тип цикла
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

	// Количество повторений (только если не бесконечный цикл)
	countLabel := widget.NewLabel("Количество повторений:")
	countSlider := widget.NewSlider(1, 100)
	countSlider.Step = 1
	countValueLabel := widget.NewLabel("")

	// Обновляем видимость элементов в зависимости от типа цикла
	updateVisibility := func() {
		if forever, ok := e.block.Parameters["forever"].(bool); ok && forever {
			countLabel.Hide()
			countSlider.Hide()
			countValueLabel.Hide()
		} else {
			countLabel.Show()
			countSlider.Show()
			countValueLabel.Show()
		}
	}

	// Инициализируем значения
	if count, ok := e.block.Parameters["count"].(int); ok {
		countSlider.Value = float64(count)
		countValueLabel.SetText(fmt.Sprintf("%d раз", count))
	} else {
		countSlider.Value = 5
		e.block.Parameters["count"] = 5
		countValueLabel.SetText("5 раз")
	}

	countSlider.OnChanged = func(value float64) {
		e.block.Parameters["count"] = int(value)
		countValueLabel.SetText(fmt.Sprintf("%.0f раз", value))
		e.notifyChange()
	}

	// Обработчик изменения типа цикла
	loopTypeSelect.OnChanged = func(selected string) {
		e.block.Parameters["forever"] = (selected == "Бесконечно")
		updateVisibility()
		e.notifyChange()
	}

	// Контейнер для ползунка
	countContainer := container.NewBorder(nil, nil, nil, countValueLabel, countSlider)

	// Информация о цикле
	infoLabel := widget.NewLabel("Это блок начала цикла 'ДЛЯ'.\nБлок конца цикла 'КЦ' создается автоматически.\nМежду ними можно добавлять другие блоки.")
	infoLabel.Wrapping = fyne.TextWrapWord

	// Находим ID конца цикла, если есть
	var loopEndID int
	if loopEndIDVal, ok := e.block.Parameters["loopEndID"].(int); ok {
		loopEndID = loopEndIDVal
	}

	// Информация о связанном блоке
	if loopEndID > 0 {
		connectionLabel := widget.NewLabel(fmt.Sprintf("Связан с блоком 'КЦ' (ID: %d)", loopEndID))
		cont.Add(connectionLabel)
	}

	cont.Add(loopTypeLabel)
	cont.Add(loopTypeSelect)
	cont.Add(countLabel)
	cont.Add(countContainer)
	cont.Add(infoLabel)

	// Инициализируем видимость
	updateVisibility()
}

// addLoopEndControls добавляет элементы управления для конца цикла
func (e *BlockEditor) addLoopEndControls(cont *fyne.Container) {
	// Информация о блоке
	infoLabel := widget.NewLabel("Это блок конца цикла 'КЦ'.\nОн автоматически создается вместе с блоком 'ДЛЯ'.\nУдаление этого блока удалит весь цикл.")
	infoLabel.Wrapping = fyne.TextWrapWord

	// Находим ID начала цикла, если есть
	var loopStartID int
	if loopStartIDVal, ok := e.block.Parameters["loopStartID"].(int); ok {
		loopStartID = loopStartIDVal
	}

	// Информация о связанном блоке
	if loopStartID > 0 {
		connectionLabel := widget.NewLabel(fmt.Sprintf("Связан с блоком 'ДЛЯ' (ID: %d)", loopStartID))
		cont.Add(connectionLabel)
	}

	cont.Add(infoLabel)
}

// addConditionControls добавляет элементы управления для условного оператора
func (e *BlockEditor) addConditionControls(cont *fyne.Container) {
	infoLabel := widget.NewLabel("Блок условия.\nВ будущих версиях здесь будут настройки условий.")
	infoLabel.Wrapping = fyne.TextWrapWord
	cont.Add(infoLabel)
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

	modeLabel := widget.NewLabel("Режим работы:")
	modeSelect := widget.NewSelect([]string{
		"Режим угла наклона (0)",
		"Режим определения наклона (1)",
		"Режим определения удара (2)",
	}, func(selected string) {
		var mode byte
		switch selected {
		case "Режим угла наклона (0)":
			mode = 0
		case "Режим определения наклона (1)":
			mode = 1
		case "Режим определения удара (2)":
			mode = 2
		}
		e.block.Parameters["mode"] = mode
		e.notifyChange()
	})

	if mode, ok := e.block.Parameters["mode"].(byte); ok {
		switch mode {
		case 0:
			modeSelect.SetSelected("Режим угла наклона (0)")
		case 1:
			modeSelect.SetSelected("Режим определения наклона (1)")
		case 2:
			modeSelect.SetSelected("Режим определения удара (2)")
		}
	} else {
		modeSelect.SetSelected("Режим определения наклона (1)")
		e.block.Parameters["mode"] = byte(1)
	}

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(modeLabel)
	cont.Add(modeSelect)
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

	modeLabel := widget.NewLabel("Режим работы:")
	modeSelect := widget.NewSelect([]string{
		"Измерение расстояния (0)",
		"Подсчет объектов (1)",
	}, func(selected string) {
		var mode byte
		if selected == "Подсчет объектов (1)" {
			mode = 1
		} else {
			mode = 0
		}
		e.block.Parameters["mode"] = mode
		e.notifyChange()
	})

	if mode, ok := e.block.Parameters["mode"].(byte); ok {
		if mode == 1 {
			modeSelect.SetSelected("Подсчет объектов (1)")
		} else {
			modeSelect.SetSelected("Измерение расстояния (0)")
		}
	} else {
		modeSelect.SetSelected("Измерение расстояния (0)")
		e.block.Parameters["mode"] = byte(0)
	}

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(modeLabel)
	cont.Add(modeSelect)
}

// addSoundControls добавляет элементы управления для звука
func (e *BlockEditor) addSoundControls(cont *fyne.Container) {
	portLabel := widget.NewLabel("Порт пищалки:")
	portSelect := widget.NewSelect([]string{"Порт 1", "Порт 2"}, func(selected string) {
		if selected == "Порт 1" {
			e.block.Parameters["port"] = byte(1)
		} else {
			e.block.Parameters["port"] = byte(2)
			e.notifyChange()
		}
	})

	if port, ok := e.block.Parameters["port"].(byte); ok && port == 2 {
		portSelect.SetSelected("Порт 2")
	} else {
		portSelect.SetSelected("Порт 1")
		e.block.Parameters["port"] = byte(1)
	}

	// Частота
	freqLabel := widget.NewLabel("Частота (Гц, 100-2000):")
	freqSlider := widget.NewSlider(100, 2000)
	freqSlider.Step = 10
	freqValueLabel := widget.NewLabel("")

	if freq, ok := e.block.Parameters["frequency"].(uint16); ok {
		freqSlider.Value = float64(freq)
		freqValueLabel.SetText(fmt.Sprintf("%d Гц", freq))
	} else {
		freqSlider.Value = 440
		e.block.Parameters["frequency"] = uint16(440)
		freqValueLabel.SetText("440 Гц")
	}

	freqSlider.OnChanged = func(value float64) {
		e.block.Parameters["frequency"] = uint16(value)
		freqValueLabel.SetText(fmt.Sprintf("%.0f Гц", value))
		e.notifyChange()
	}

	// Контейнер для ползунка частоты
	freqContainer := container.NewBorder(nil, nil, nil, freqValueLabel, freqSlider)

	// Длительность
	durationLabel := widget.NewLabel("Длительность (мс, 100-5000):")
	durationSlider := widget.NewSlider(100, 5000)
	durationSlider.Step = 100
	durationValueLabel := widget.NewLabel("")

	if duration, ok := e.block.Parameters["duration"].(uint16); ok {
		durationSlider.Value = float64(duration)
		durationValueLabel.SetText(fmt.Sprintf("%d мс", duration))
	} else {
		durationSlider.Value = 1000
		e.block.Parameters["duration"] = uint16(1000)
		durationValueLabel.SetText("1000 мс")
	}

	durationSlider.OnChanged = func(value float64) {
		e.block.Parameters["duration"] = uint16(value)
		durationValueLabel.SetText(fmt.Sprintf("%.0f мс", value))
		e.notifyChange()
	}

	// Контейнер для ползунка длительности
	durationContainer := container.NewBorder(nil, nil, nil, durationValueLabel, durationSlider)

	// Предустановленные ноты
	notesLabel := widget.NewLabel("Предустановленные ноты:")
	notesContainer := container.NewGridWithColumns(3)

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
				e.block.Parameters["frequency"] = freq
				freqSlider.Value = float64(freq)
				freqValueLabel.SetText(fmt.Sprintf("%d Гц", freq))
				e.notifyChange()
			}
		}(note.frequency, note.name))

		btn.Importance = widget.LowImportance
		notesContainer.Add(btn)
	}

	// Кнопка теста
	testButton := widget.NewButton("Тест звук", func() {
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
	testButton.Importance = widget.HighImportance

	cont.Add(portLabel)
	cont.Add(portSelect)
	cont.Add(freqLabel)
	cont.Add(freqContainer)
	cont.Add(durationLabel)
	cont.Add(durationContainer)
	cont.Add(notesLabel)
	cont.Add(notesContainer)
	cont.Add(layout.NewSpacer())
	cont.Add(container.NewCenter(testButton))
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
