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
	block      *ProgramBlock
	programMgr *ProgramManager
	content    fyne.CanvasObject
	isDragging bool
	dragStart  fyne.Position
}

// NewDraggableBlock создает перетаскиваемый блок
func NewDraggableBlock(block *ProgramBlock, programMgr *ProgramManager) *DraggableBlock {
	d := &DraggableBlock{
		block:      block,
		programMgr: programMgr,
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

	// Заголовок
	title := canvas.NewText(d.block.Title, color.White)
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter
	title.TextSize = 14

	// Описание
	desc := canvas.NewText(d.block.Description, color.White)
	desc.Alignment = fyne.TextAlignCenter
	desc.TextSize = 10

	// Иконка (заглушка)
	icon := canvas.NewText("◼", color.White)
	icon.TextSize = 20

	// Контейнер содержимого
	content := container.NewVBox(
		container.NewCenter(icon),
		container.NewCenter(title),
		container.NewCenter(desc),
	)

	// Объединяем фон и содержимое
	d.content = container.NewStack(
		bg,
		container.NewPadded(content),
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

	// Здесь можно добавить логику выделения блока
	// или открытия его свойств
}

// DoubleTapped обработка двойного клика
func (d *DraggableBlock) DoubleTapped(e *fyne.PointEvent) {
	log.Printf("Двойной клик по блоку: %s", d.block.Title)

	// Здесь можно добавить логику редактирования блока
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
}

// DragEnd завершение перетаскивания
func (d *DraggableBlock) DragEnd() {
	if d.isDragging {
		d.isDragging = false
		log.Printf("Блок перемещен: %s -> (%.0f, %.0f)",
			d.block.Title, d.block.X, d.block.Y)
	}
}

// MouseIn обработка наведения мыши
func (d *DraggableBlock) MouseIn(e *desktop.MouseEvent) {
	// Можно добавить визуальный эффект при наведении
}

// MouseOut обработка ухода мыши
func (d *DraggableBlock) MouseOut() {
	// Сброс визуальных эффектов
}

// MouseDown обработка нажатия мыши
func (d *DraggableBlock) MouseDown(e *desktop.MouseEvent) {
	d.isDragging = true
	d.dragStart = e.Position
	d.dragStart = fyne.NewPos(float32(d.block.X), float32(d.block.Y))
}

// MouseUp обработка отпускания мыши
func (d *DraggableBlock) MouseUp(e *desktop.MouseEvent) {
	if d.isDragging {
		d.isDragging = false
		log.Printf("Блок перемещен: %s -> (%.0f, %.0f)",
			d.block.Title, d.block.X, d.block.Y)
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
	}
}

// Cursor возвращает курсор для блока
func (d *DraggableBlock) Cursor() desktop.Cursor {
	return desktop.PointerCursor
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
	r.objects[0].Resize(size)
}

func (r *draggableBlockRenderer) MinSize() fyne.Size {
	return r.objects[0].MinSize()
}

func (r *draggableBlockRenderer) Refresh() {
	r.objects[0].Refresh()
}

func (r *draggableBlockRenderer) Destroy() {}

func (r *draggableBlockRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}
