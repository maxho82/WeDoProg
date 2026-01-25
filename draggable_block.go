package main

import (
	"fmt"
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// DraggableBlock перетаскиваемый блок программирования
type DraggableBlock struct {
	widget.BaseWidget
	block           *ProgramBlock
	programMgr      *ProgramManager
	gui             *MainGUI
	content         fyne.CanvasObject
	isDragging      bool
	dragStart       fyne.Position
	isSelected      bool
	connectorTop    *canvas.Circle
	connectorBottom *canvas.Circle
	selectionBorder *canvas.Rectangle
}

// NewDraggableBlock создает перетаскиваемый блок
func NewDraggableBlock(block *ProgramBlock, programMgr *ProgramManager, gui *MainGUI) *DraggableBlock {
	d := &DraggableBlock{
		block:      block,
		programMgr: programMgr,
		gui:        gui,
		isSelected: false,
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
	bg.CornerRadius = 5

	// Добавляем выделение при выборе
	d.selectionBorder = canvas.NewRectangle(color.Transparent)
	d.selectionBorder.SetMinSize(fyne.NewSize(float32(d.block.Width)+4, float32(d.block.Height)+4))
	d.selectionBorder.CornerRadius = 7
	d.selectionBorder.StrokeColor = color.Transparent
	d.selectionBorder.StrokeWidth = 2

	// Иконка (заглушка)
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

	// Создаем коннекторы (точки соединения) - делаем их невидимыми
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

	// Объединяем все элементы
	d.content = container.NewStack(
		d.selectionBorder,
		bg,
		container.NewPadded(content),
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

	// Выделяем этот блок и показываем его свойства
	d.selectBlock()

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
			d.selectBlock()
		}),
	)

	// Показываем контекстное меню
	widget.ShowPopUpMenuAtPosition(menu, d.gui.window.Canvas(), e.AbsolutePosition)
}

// selectBlock выделяет этот блок и показывает его свойства
func (d *DraggableBlock) selectBlock() {
	// Снимаем выделение со всех блоков
	for _, obj := range d.gui.programPanel.content.Objects {
		if block, ok := obj.(*DraggableBlock); ok {
			block.deselect()
		}
	}

	// Выделяем этот блок
	d.isSelected = true
	d.updateSelection()

	// Показываем свойства блока
	d.gui.showBlockProperties(d.block)
}

// deselect снимает выделение с блока
func (d *DraggableBlock) deselect() {
	d.isSelected = false
	d.updateSelection()
}

// updateSelection обновляет внешний вид блока в зависимости от выделения
func (d *DraggableBlock) updateSelection() {
	if d.selectionBorder != nil {
		if d.isSelected {
			d.selectionBorder.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
		} else {
			d.selectionBorder.StrokeColor = color.Transparent
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

		// Обновляем визуальное соединение
		d.gui.programPanel.updateConnections()

		log.Printf("Автоматически соединен блок %d -> блок %d", lastBlock.ID, d.block.ID)
	}
}

// Dragged обработка перетаскивания
func (d *DraggableBlock) Dragged(e *fyne.DragEvent) {
	if !d.isDragging {
		d.isDragging = true
		d.dragStart = d.Position()
	}

	// Вычисляем новую позицию
	newPos := fyne.NewPos(
		d.dragStart.X+e.Dragged.DX,
		d.dragStart.Y+e.Dragged.DY,
	)

	// Ограничиваем движение в пределах положительных координат
	if newPos.X < 0 {
		newPos.X = 0
	}
	if newPos.Y < 0 {
		newPos.Y = 0
	}

	// Перемещаем блок
	d.Move(newPos)

	// Обновляем позицию в данных блока
	d.block.X = float64(newPos.X)
	d.block.Y = float64(newPos.Y)

	// Обновляем позиции коннекторов
	d.updateConnectorPositions()

	// Обновляем соединения
	d.gui.programPanel.updateConnections()
}

// updateConnectorPositions обновляет позиции коннекторов
func (d *DraggableBlock) updateConnectorPositions() {
	blockPos := d.Position()
	blockSize := d.Size()

	// Верхний коннектор (центр верхней границы)
	topX := blockPos.X + blockSize.Width/2
	topY := blockPos.Y
	d.connectorTop.Move(fyne.NewPos(topX-0.5, topY-0.5))

	// Нижний коннектор (центр нижней границы)
	bottomX := blockPos.X + blockSize.Width/2
	bottomY := blockPos.Y + blockSize.Height
	d.connectorBottom.Move(fyne.NewPos(bottomX-0.5, bottomY-0.5))

	d.connectorTop.Refresh()
	d.connectorBottom.Refresh()
}

// DragEnd завершение перетаскивания
func (d *DraggableBlock) DragEnd() {
	if d.isDragging {
		d.isDragging = false
		log.Printf("Блок перемещен: %s -> (%.0f, %.0f)",
			d.block.Title, d.block.X, d.block.Y)

		// Обновляем позицию в менеджере программ
		d.programMgr.UpdateBlockPosition(d.block.ID, d.block.X, d.block.Y)
	}
}

// MouseDown обработка нажатия мыши
func (d *DraggableBlock) MouseDown(e *desktop.MouseEvent) {
	d.isDragging = true
	d.dragStart = e.Position
}

// MouseUp обработка отпускания мыши
func (d *DraggableBlock) MouseUp(e *desktop.MouseEvent) {
	if d.isDragging {
		d.isDragging = false
		d.DragEnd()
	}
}

// MouseMoved обработка движения мыши
func (d *DraggableBlock) MouseMoved(e *desktop.MouseEvent) {
	if d.isDragging {
		// Вычисляем смещение
		deltaX := e.Position.X - d.dragStart.X
		deltaY := e.Position.Y - d.dragStart.Y

		// Новая позиция
		newPos := fyne.NewPos(
			d.block.DragStartPos.X+deltaX,
			d.block.DragStartPos.Y+deltaY,
		)

		// Ограничиваем минимальные координаты
		if newPos.X < 0 {
			newPos.X = 0
		}
		if newPos.Y < 0 {
			newPos.Y = 0
		}

		// Обновляем позицию
		d.Move(newPos)

		// Обновляем данные блока
		d.block.X = float64(newPos.X)
		d.block.Y = float64(newPos.Y)

		// Обновляем позиции коннекторов
		d.updateConnectorPositions()

		// Обновляем соединения
		d.gui.programPanel.updateConnections()
	}
}

// Cursor возвращает курсор для блока
func (d *DraggableBlock) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

// GetBlockPosition возвращает позицию блока для соединений
func (d *DraggableBlock) GetBlockPosition() fyne.Position {
	return d.Position()
}

// GetBlockSize возвращает размер блока
func (d *DraggableBlock) GetBlockSize() fyne.Size {
	return d.Size()
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

// draggableBlockRenderer рендерер для DraggableBlock
type draggableBlockRenderer struct {
	widget  *DraggableBlock
	objects []fyne.CanvasObject
}

func (r *draggableBlockRenderer) Layout(size fyne.Size) {
	// Обновляем размеры всех объектов
	for _, obj := range r.objects {
		obj.Resize(size)
	}
	// Обновляем позиции коннекторов
	r.widget.updateConnectorPositions()
}

func (r *draggableBlockRenderer) MinSize() fyne.Size {
	return fyne.NewSize(float32(r.widget.block.Width), float32(r.widget.block.Height))
}

func (r *draggableBlockRenderer) Refresh() {
	r.widget.updateConnectorPositions()
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *draggableBlockRenderer) Destroy() {}

func (r *draggableBlockRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}
