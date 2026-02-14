package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// BlocksPalette отвечает за палитру блоков программирования.
type BlocksPalette struct {
	gui                *MainGUI
	state              *AppState
	container          fyne.CanvasObject // изменено с *fyne.Container на fyne.CanvasObject
	blockButtons       map[BlockType]*widget.Button
	insertCancelButton *widget.Button
}

// NewBlocksPalette создаёт новую палитру блоков.
func NewBlocksPalette(gui *MainGUI, state *AppState) *BlocksPalette {
	palette := &BlocksPalette{
		gui:          gui,
		state:        state,
		blockButtons: make(map[BlockType]*widget.Button),
	}
	palette.buildUI()
	return palette
}

// GetContainer возвращает корневой контейнер палитры.
func (bp *BlocksPalette) GetContainer() fyne.CanvasObject {
	return bp.container
}

// buildUI строит интерфейс палитры.
func (bp *BlocksPalette) buildUI() {
	blocksContainer := container.NewVBox()

	title := canvas.NewText("Палитра блоков", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter
	blocksContainer.Add(container.NewCenter(title))
	blocksContainer.Add(widget.NewSeparator())

	// Кнопка "Новая+" – создаёт новую программу с блоками "Начать" и "Стоп"
	newButton := widget.NewButtonWithIcon("Новая+", theme.DocumentCreateIcon(), func() {
		bp.gui.confirmNewProgram()
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
			blockName := bp.getBlockName(blockType)

			if blockType == BlockTypeLoopStart {
				// Для цикла создаём специальную кнопку, которая вставляет сразу пару блоков
				blockButton := widget.NewButton(blockName, func() {
					bp.gui.handleLoopBlockSelection()
				})
				blockButton.Importance = widget.LowImportance
				bp.blockButtons[blockType] = blockButton
				blocksContainer.Add(blockButton)
			} else {
				blockButton := widget.NewButton(blockName, func(bt BlockType) func() {
					return func() {
						bp.gui.handleBlockSelection(bt)
					}
				}(blockType))

				blockButton.Importance = widget.LowImportance
				bp.blockButtons[blockType] = blockButton
				blocksContainer.Add(blockButton)
			}
		}
		blocksContainer.Add(widget.NewSeparator())
	}

	cancelButton := widget.NewButton("Отменить вставку", func() {
		bp.gui.CancelInsertMode()
	})
	cancelButton.Importance = widget.WarningImportance
	cancelButton.Hide()
	blocksContainer.Add(cancelButton)
	bp.insertCancelButton = cancelButton

	scroll := container.NewVScroll(container.NewPadded(blocksContainer))
	scroll.SetMinSize(fyne.NewSize(220, 400))
	bp.container = scroll // теперь присваивание корректно, т.к. поле имеет тип fyne.CanvasObject
}

// getBlockName возвращает имя блока по типу.
func (bp *BlocksPalette) getBlockName(blockType BlockType) string {
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

// UpdateButtonsState обновляет состояние кнопок в зависимости от режима вставки.
func (bp *BlocksPalette) UpdateButtonsState(isInsertMode bool) {
	for _, button := range bp.blockButtons {
		if isInsertMode {
			button.Disable()
		} else {
			button.Enable()
		}
	}
}

// ShowCancelButton показывает или скрывает кнопку отмены вставки.
func (bp *BlocksPalette) ShowCancelButton(show bool) {
	if bp.insertCancelButton != nil {
		if show {
			bp.insertCancelButton.Show()
		} else {
			bp.insertCancelButton.Hide()
		}
	}
}
