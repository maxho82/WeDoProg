package main

import (
	"fmt"
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// DraggableBlock блок программирования без перетаскивания
type DraggableBlock struct {
	widget.BaseWidget
	block           *ProgramBlock
	programMgr      *ProgramManager
	gui             *MainGUI
	content         fyne.CanvasObject
	isSelected      bool
	connectorTop    *canvas.Circle
	connectorBottom *canvas.Circle
	selectionBorder *canvas.Rectangle
	highlightColor  color.Color // Цвет выделения
}

// draggableBlockRenderer рендерер для DraggableBlock
type draggableBlockRenderer struct {
	widget  *DraggableBlock
	objects []fyne.CanvasObject
}

// NewDraggableBlock создает блок
func NewDraggableBlock(block *ProgramBlock, programMgr *ProgramManager, gui *MainGUI) *DraggableBlock {
	d := &DraggableBlock{
		block:          block,
		programMgr:     programMgr,
		gui:            gui,
		isSelected:     false,
		highlightColor: color.NRGBA{R: 255, G: 215, B: 0, A: 255}, // Желтый цвет выделения
	}

	d.ExtendBaseWidget(d)
	d.createContent()

	return d
}

// createContent создает содержимое блока
func (d *DraggableBlock) createContent() {
	// Цвет блока
	blockColor := parseColor(d.block.Color)
	if blockColor == nil {
		blockColor = color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	}

	// Фон блока
	bg := canvas.NewRectangle(blockColor)
	bg.SetMinSize(fyne.NewSize(float32(d.block.Width), float32(d.block.Height)))
	bg.CornerRadius = 8

	// Добавляем выделение при выборе (желтая рамка 5 пикселей)
	d.selectionBorder = canvas.NewRectangle(color.Transparent)
	d.selectionBorder.SetMinSize(fyne.NewSize(float32(d.block.Width)+10, float32(d.block.Height)+10))
	d.selectionBorder.CornerRadius = 10
	d.selectionBorder.StrokeColor = color.Transparent
	d.selectionBorder.StrokeWidth = 5 // Толщина рамки выделения

	// Иконка
	icon := canvas.NewText("◼", color.White)
	icon.TextSize = 20

	// Заголовок
	title := canvas.NewText(d.block.Title, color.White)
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter
	title.TextSize = 14

	// Описание
	desc := canvas.NewText(d.block.Description, color.White)
	desc.Alignment = fyne.TextAlignCenter
	desc.TextSize = 10

	// Контейнер содержимого
	content := container.NewVBox(
		container.NewCenter(icon),
		container.NewCenter(title),
		container.NewCenter(desc),
	)

	// Создаем коннекторы (точки соединения)
	d.connectorTop = canvas.NewCircle(color.Transparent)
	d.connectorTop.StrokeWidth = 0
	d.connectorTop.Resize(fyne.NewSize(1, 1))

	d.connectorBottom = canvas.NewCircle(color.Transparent)
	d.connectorBottom.StrokeWidth = 0
	d.connectorBottom.Resize(fyne.NewSize(1, 1))

	// Контейнер для коннекторов
	connectors := container.NewWithoutLayout(
		d.connectorTop,
		d.connectorBottom,
	)

	// ВАЖНО: Порядок элементов в Stack имеет значение!
	// 1. Фон (bg)
	// 2. Содержимое (content)
	// 3. Рамка выделения (selectionBorder) - должна быть над фоном, но под коннекторами
	// 4. Коннекторы (connectors) - должны быть сверху всего
	d.content = container.NewStack(
		bg,
		container.NewPadded(content),
		d.selectionBorder,
		connectors,
	)
}

// CreateRenderer создает рендерер виджета
func (d *DraggableBlock) CreateRenderer() fyne.WidgetRenderer {
	return &draggableBlockRenderer{
		widget:  d,
		objects: []fyne.CanvasObject{d.content},
	}
}

// Tapped обработка клика по блоку
func (d *DraggableBlock) Tapped(e *fyne.PointEvent) {
	log.Printf("Клик по блоку: %s (ID: %d)", d.block.Title, d.block.ID)

	// Устанавливаем выбранный блок в GUI
	d.gui.selectedBlock = d.block
	d.gui.programPanel.SetSelectedBlock(d.block)

	// Показываем свойства блока
	d.gui.showBlockProperties(d.block)

	// Если это не стартовый блок, предлагаем соединить с предыдущим
	if d.block.Type != BlockTypeStart && d.block.NextBlockID == 0 {
		// Автоматически соединяем с предыдущим блоком, если он есть
		d.autoConnectToPrevious()
	}
}

// TappedSecondary обработка правого клика по блоку
func (d *DraggableBlock) TappedSecondary(e *fyne.PointEvent) {
	// Создаем контекстное меню
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
	d.isSelected = selected
	d.updateSelection()
}

// updateSelection обновляет внешний вид блока в зависимости от выделения
func (d *DraggableBlock) updateSelection() {
	if d.selectionBorder != nil {
		if d.isSelected {
			d.selectionBorder.StrokeColor = d.highlightColor // Желтая рамка
			d.selectionBorder.StrokeWidth = 5                // 5 пикселей
		} else {
			d.selectionBorder.StrokeColor = color.Transparent
			d.selectionBorder.StrokeWidth = 0
		}
		d.selectionBorder.Refresh()
	}
	d.Refresh()
}

// autoConnectToPrevious автоматически соединяет с предыдущим блоком
func (d *DraggableBlock) autoConnectToPrevious() {
	// Находим последний блок в программе (кроме текущего)
	var lastBlock *ProgramBlock
	for _, block := range d.programMgr.program.Blocks {
		if block.ID != d.block.ID {
			lastBlock = block
		}
	}

	if lastBlock != nil && lastBlock.NextBlockID == 0 {
		// Соединяем последний блок с текущим
		d.programMgr.AddConnection(lastBlock.ID, d.block.ID)

		// Обновляем визуальное соединение - УДАЛЕН ВЫЗОВ
		// d.gui.programPanel.updateConnections() // Этот метод больше не существует

		log.Printf("Автоматически соединен блок %d -> блок %d", lastBlock.ID, d.block.ID)
	}
}

// GetTopConnectorPosition возвращает позицию верхнего коннектора
func (d *DraggableBlock) GetTopConnectorPosition() fyne.Position {
	blockPos := d.Position()
	blockSize := d.Size()
	return fyne.NewPos(blockPos.X+blockSize.Width/2, blockPos.Y)
}

// GetBottomConnectorPosition возвращает позицию нижнего коннектора
func (d *DraggableBlock) GetBottomConnectorPosition() fyne.Position {
	blockPos := d.Position()
	blockSize := d.Size()
	return fyne.NewPos(blockPos.X+blockSize.Width/2, blockPos.Y+blockSize.Height)
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
	return nil
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

func (r *draggableBlockRenderer) Layout(size fyne.Size) {
	// Обновляем размеры всех объектов
	for _, obj := range r.objects {
		obj.Resize(size)
	}
}

func (r *draggableBlockRenderer) MinSize() fyne.Size {
	return fyne.NewSize(float32(r.widget.block.Width), float32(r.widget.block.Height))
}

func (r *draggableBlockRenderer) Refresh() {
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *draggableBlockRenderer) Destroy() {}

func (r *draggableBlockRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}
