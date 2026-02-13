package main

import (
	"fmt"
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// DraggableBlock кастомный виджет блока программирования
type DraggableBlock struct {
	widget.BaseWidget
	block        *ProgramBlock
	programMgr   *ProgramManager
	gui          *MainGUI
	programPanel *ProgramPanel
	isSelected   bool
	isExecuting  bool
}

// NewDraggableBlock создает новый кастомный блок
func NewDraggableBlock(block *ProgramBlock, programMgr *ProgramManager, gui *MainGUI, programPanel *ProgramPanel) *DraggableBlock {
	d := &DraggableBlock{
		block:        block,
		programMgr:   programMgr,
		gui:          gui,
		programPanel: programPanel,
	}
	block.Width = DefaultBlockWidth
	block.Height = DefaultBlockHeight
	d.ExtendBaseWidget(d)
	return d
}

// CreateRenderer создает кастомный рендерер для блока
func (d *DraggableBlock) CreateRenderer() fyne.WidgetRenderer {
	return newBlockRenderer(d)
}

// Tapped обработка клика по блоку
func (d *DraggableBlock) Tapped(e *fyne.PointEvent) {
	// Проверяем, можно ли выбирать блок
	if d.block.Type == BlockTypeStop {
		// Блок "Стоп" нельзя выбирать - он всегда в конце
		return
	}

	log.Printf("Клик по блоку: %s (ID: %d)", d.block.Title, d.block.ID)

	// Устанавливаем выбранный блок в GUI через метод (было прямое присваивание)
	d.gui.SetSelectedBlock(d.block)
	d.programPanel.SetSelectedBlock(d.block)

	// Показываем свойства блока
	d.gui.showBlockProperties(d.block)

	// Обновляем выделение
	d.SetSelected(true)
}

// TappedSecondary обработка правого клика по блоку
func (d *DraggableBlock) TappedSecondary(e *fyne.PointEvent) {
	// Блоки "Начать" и "Стоп" нельзя удалять
	if d.block.Type == BlockTypeStart || d.block.Type == BlockTypeStop {
		return
	}

	// Для блоков цикла используем специальное меню
	if d.block.Type == BlockTypeLoopStart || d.block.Type == BlockTypeLoopEnd {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("Удалить цикл", func() {
				d.gui.deleteLoopWithConfirmation(d.block.ID)
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Свойства", func() {
				d.gui.showBlockProperties(d.block)
			}),
		)
		widget.ShowPopUpMenuAtPosition(menu, d.gui.window.Canvas(), e.AbsolutePosition)
		return
	}

	// Создаем контекстное меню для обычных блоков
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Удалить", func() {
			d.gui.deleteSelectedBlock()
		}),
		fyne.NewMenuItem("Копировать", func() {
			// TODO: реализовать копирование
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Свойства", func() {
			d.gui.showBlockProperties(d.block)
		}),
	)

	// Показываем контекстное меню
	widget.ShowPopUpMenuAtPosition(menu, d.gui.window.Canvas(), e.AbsolutePosition)
}

// SetSelected устанавливает состояние выделения блока
func (d *DraggableBlock) SetSelected(selected bool) {
	// Блок "Стоп" нельзя выделять
	if d.block.Type == BlockTypeStop {
		return
	}

	d.isSelected = selected
	if selected {
		d.isExecuting = false // Сбрасываем выделение выполнения при обычном выделении
	}
	d.Refresh()
}

// SetExecuting устанавливает состояние выполнения блока
func (d *DraggableBlock) SetExecuting(executing bool) {
	// Блок "Стоп" не выделяем как выполняющийся
	if d.block.Type == BlockTypeStop {
		return
	}

	d.isExecuting = executing
	d.Refresh()
}

// GetTopConnectorPosition возвращает позицию верхнего коннектора
func (d *DraggableBlock) GetTopConnectorPosition() fyne.Position {
	pos := d.Position()
	size := d.Size()
	top, _ := calculateConnectorPositions(pos, size)
	return top
}

// GetBottomConnectorPosition возвращает позицию нижнего коннектора
func (d *DraggableBlock) GetBottomConnectorPosition() fyne.Position {
	pos := d.Position()
	size := d.Size()
	_, bottom := calculateConnectorPositions(pos, size)
	return bottom
}

// parseColor преобразует строку цвета в color.Color
func parseColor(colorStr string) color.Color {
	if len(colorStr) == 7 && colorStr[0] == '#' {
		// Формат #RRGGBB
		r, _ := hexToByte(colorStr[1:3])
		g, _ := hexToByte(colorStr[3:5])
		b, _ := hexToByte(colorStr[5:7])
		return color.NRGBA{R: r, G: g, B: b, A: 255}
	}

	// Цвета по умолчанию для типов блоков
	return color.NRGBA{R: 100, G: 100, B: 100, A: 255}
}

// hexToByte преобразует hex строку в байт
func hexToByte(hexStr string) (byte, error) {
	var value byte
	for i := 0; i < 2; i++ {
		char := hexStr[i]
		var digit byte
		if char >= '0' && char <= '9' {
			digit = char - '0'
		} else if char >= 'a' && char <= 'f' {
			digit = char - 'a' + 10
		} else if char >= 'A' && char <= 'F' {
			digit = char - 'A' + 10
		} else {
			return 0, fmt.Errorf("неверный символ hex")
		}
		value = value*16 + digit
	}
	return value, nil
}
