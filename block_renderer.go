package main

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// blockRenderer рендерер для кастомных блоков
type blockRenderer struct {
	widget  *DraggableBlock
	objects []fyne.CanvasObject

	// Элементы фигуры (нарисованы линиями)
	shapeLines []*canvas.Line

	// Текстовые элементы
	iconText  *canvas.Text
	titleText *canvas.Text
	descText  *canvas.Text

	// Параметры
	lineColor   color.Color
	lineWidth   float32
	highlighted bool
	executing   bool
	shapeType   BlockShapeType
	scale       float32 // Текущий масштаб
}

// newBlockRenderer создает новый рендерер для блока
func newBlockRenderer(widget *DraggableBlock) *blockRenderer {
	r := &blockRenderer{
		widget:    widget,
		lineColor: parseColor(widget.block.Color),
		lineWidth: 2.0,
		shapeType: getShapeType(widget.block.Type),
		scale:     widget.programPanel.GetScale(), // Получаем текущий масштаб
	}

	// Создаем текстовые элементы с учетом масштаба
	r.iconText = canvas.NewText(getBlockIcon(widget.block.Type), color.White)
	r.iconText.TextSize = 20 * r.scale // Масштабируем размер текста
	r.iconText.TextStyle.Bold = true

	r.titleText = canvas.NewText(widget.block.Title, color.White)
	r.titleText.TextSize = 14 * r.scale // Масштабируем размер текста
	r.titleText.TextStyle.Bold = true

	r.descText = canvas.NewText(widget.block.Description, color.White)
	r.descText.TextSize = 10 * r.scale // Масштабируем размер текста

	// Создаем линии для фигуры
	r.createShapeLines()

	// Собираем все объекты
	r.objects = []fyne.CanvasObject{}
	for _, line := range r.shapeLines {
		r.objects = append(r.objects, line)
	}
	r.objects = append(r.objects, r.iconText, r.titleText, r.descText)

	return r
}

// createShapeLines создает линии для фигуры блока
func (r *blockRenderer) createShapeLines() {
	size := r.widget.Size()
	vertices := calculateShapeVertices(r.shapeType, size)

	// Для эллипса и других замкнутых фигур нужно создать линии между всеми вершинами
	r.shapeLines = make([]*canvas.Line, len(vertices))
	for i := 0; i < len(vertices); i++ {
		next := (i + 1) % len(vertices) // Замыкаем фигуру
		line := canvas.NewLine(r.lineColor)
		line.Position1 = vertices[i]
		line.Position2 = vertices[next]
		line.StrokeWidth = r.lineWidth * r.scale // Масштабируем толщину линии
		r.shapeLines[i] = line
	}
}

// updateShapeLines обновляет позиции линий фигуры
func (r *blockRenderer) updateShapeLines() {
	size := r.widget.Size()
	vertices := calculateShapeVertices(r.shapeType, size)

	// Обновляем существующие линии
	for i := 0; i < len(vertices); i++ {
		if i < len(r.shapeLines) {
			next := (i + 1) % len(vertices)
			r.shapeLines[i].Position1 = vertices[i]
			r.shapeLines[i].Position2 = vertices[next]
		}
	}

	// Если количество вершин изменилось (маловероятно), пересоздаем линии
	if len(vertices) != len(r.shapeLines) {
		log.Printf("Количество вершин изменилось для блока %s", r.widget.block.Title)
		r.createShapeLines()
	}
}

// Layout упорядочивает элементы внутри виджета
func (r *blockRenderer) Layout(size fyne.Size) {
	// Обновляем масштаб
	r.scale = r.widget.programPanel.GetScale()

	// Обновляем линии фигуры
	r.updateShapeLines()

	// Вычисляем позиции для текста
	centerX := size.Width / 2
	centerY := size.Height / 2

	// Для эллипса и шестиугольника позиционируем текст по центру
	r.positionTextElements(centerX, centerY, size)

	// Обновляем стили линий в зависимости от состояния
	r.updateLineStyles()
}

// positionTextElements позиционирует текстовые элементы
func (r *blockRenderer) positionTextElements(centerX, centerY float32, size fyne.Size) {
	// Обновляем размер текста в соответствии с масштабом
	r.iconText.TextSize = 20 * r.scale
	r.titleText.TextSize = 14 * r.scale
	r.descText.TextSize = 10 * r.scale

	// Размеры текстовых элементов (после обновления масштаба)
	iconSize := r.iconText.MinSize()
	titleSize := r.titleText.MinSize()

	// Для блоков начала/конца (эллипсов) и шестиугольников
	if r.shapeType == ShapeEllipse || r.shapeType == ShapeHexagon || r.shapeType == ShapeHexagonInverted {
		// Иконка выше центра
		r.iconText.Move(fyne.NewPos(
			centerX-iconSize.Width/2,
			centerY-iconSize.Height-5*r.scale,
		))

		// Заголовок по центру
		r.titleText.Move(fyne.NewPos(
			centerX-titleSize.Width/2,
			centerY-titleSize.Height/2,
		))

		// Описание ниже центра (если есть)
		if len(r.widget.block.Description) > 0 {
			descSize := r.descText.MinSize()
			r.descText.Move(fyne.NewPos(
				centerX-descSize.Width/2,
				centerY+10*r.scale,
			))
		} else {
			r.descText.Hide()
		}
	} else {
		// Для прямоугольных блоков - стандартное позиционирование
		r.iconText.Move(fyne.NewPos(
			centerX-iconSize.Width/2,
			centerY-iconSize.Height-10*r.scale,
		))

		r.titleText.Move(fyne.NewPos(
			centerX-titleSize.Width/2,
			centerY-titleSize.Height/2,
		))

		descSize := r.descText.MinSize()
		r.descText.Move(fyne.NewPos(
			centerX-descSize.Width/2,
			centerY+10*r.scale,
		))
	}
}

// MinSize возвращает минимальный размер виджета
func (r *blockRenderer) MinSize() fyne.Size {
	// Минимальные размеры для разных типов блоков с учетом масштаба
	var minWidth, minHeight float32

	switch r.shapeType {
	case ShapeEllipse:
		minWidth = 120 * r.scale
		minHeight = 80 * r.scale
	case ShapeHexagon, ShapeHexagonInverted:
		minWidth = 150 * r.scale
		minHeight = 100 * r.scale
	case ShapeDiamond:
		minWidth = 120 * r.scale
		minHeight = 80 * r.scale
	default: // ShapeRectangle
		minWidth = 150 * r.scale
		minHeight = 80 * r.scale
	}

	// Учитываем размер текста
	titleSize := r.titleText.MinSize()
	requiredWidth := maxFloat32(minWidth, titleSize.Width+20*r.scale)
	requiredHeight := minHeight

	return fyne.NewSize(requiredWidth, requiredHeight)
}

// Refresh обновляет внешний вид рендерера
func (r *blockRenderer) Refresh() {
	// Обновляем масштаб
	r.scale = r.widget.programPanel.GetScale()

	// Обновляем цвета линий
	r.lineColor = parseColor(r.widget.block.Color)

	// Обновляем состояние
	r.highlighted = r.widget.isSelected
	r.executing = r.widget.isExecuting

	// Обновляем стили линий
	r.updateLineStyles()

	// Обновляем тексты
	r.titleText.Text = r.widget.block.Title
	r.descText.Text = r.widget.block.Description
	r.iconText.Text = getBlockIcon(r.widget.block.Type)

	// Обновляем размеры текста
	r.iconText.TextSize = 20 * r.scale
	r.titleText.TextSize = 14 * r.scale
	r.descText.TextSize = 10 * r.scale

	// Показываем/скрываем описание
	if len(r.widget.block.Description) > 0 {
		r.descText.Show()
	} else {
		r.descText.Hide()
	}

	// Перерисовываем все
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

// updateLineStyles обновляет стили линий в зависимости от состояния
func (r *blockRenderer) updateLineStyles() {
	var strokeWidth float32 = 2.0 * r.scale
	var strokeColor color.Color = r.lineColor

	if r.executing {
		// Выполняющийся блок - толстые зеленые линии
		strokeWidth = 4.0 * r.scale
		strokeColor = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	} else if r.highlighted {
		// Выделенный блок - толстые желтые линии
		strokeWidth = 3.0 * r.scale
		strokeColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}
	}

	// Применяем стили ко всем линиям фигуры
	for _, line := range r.shapeLines {
		line.StrokeWidth = strokeWidth
		line.StrokeColor = strokeColor
	}
}

// Objects возвращает все объекты для отрисовки
func (r *blockRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Destroy очищает ресурсы рендерера
func (r *blockRenderer) Destroy() {
	// Очищаем ссылки
	r.shapeLines = nil
	r.objects = nil
}

// Вспомогательная функция
func maxFloat32(values ...float32) float32 {
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}
