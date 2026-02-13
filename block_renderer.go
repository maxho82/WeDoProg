package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// blockRenderer рендерер для кастомных блоков
type blockRenderer struct {
	widget  *DraggableBlock
	objects []fyne.CanvasObject

	shapeLines []*canvas.Line
	iconText   *canvas.Text
	titleText  *canvas.Text
	descText   *canvas.Text

	lineColor   color.Color
	lineWidth   float32
	highlighted bool
	executing   bool
	shapeType   BlockShapeType
	scale       float32

	// Кэшированные вершины фигуры (для оптимизации)
	cachedVertices []fyne.Position
	lastSize       fyne.Size // последний размер, для которого вычислены вершины
}

func newBlockRenderer(widget *DraggableBlock) *blockRenderer {
	scale := widget.programPanel.GetScale()

	r := &blockRenderer{
		widget:    widget,
		lineColor: parseColor(widget.block.Color),
		lineWidth: LineStrokeWidthNormal,
		shapeType: getShapeType(widget.block.Type),
		scale:     scale,
	}

	r.iconText = canvas.NewText(getBlockIcon(widget.block.Type), color.White)
	r.titleText = canvas.NewText(widget.block.Title, color.White)
	r.descText = canvas.NewText(widget.block.Description, color.White)

	// Создаём линии для фигуры с кэшированием
	r.updateShapeLines(true)

	r.objects = []fyne.CanvasObject{}
	for _, line := range r.shapeLines {
		r.objects = append(r.objects, line)
	}
	r.objects = append(r.objects, r.iconText, r.titleText, r.descText)

	return r
}

// updateShapeLines обновляет линии фигуры, используя кэш, если размер не изменился.
// forceRecalc принудительно пересчитывает вершины.
func (r *blockRenderer) updateShapeLines(forceRecalc bool) {
	size := r.widget.Size()

	// Если размер не изменился и есть кэш, используем его
	if !forceRecalc && size == r.lastSize && r.cachedVertices != nil {
		r.updateLinesFromVertices(r.cachedVertices)
		return
	}

	// Пересчитываем вершины
	vertices := calculateShapeVertices(r.shapeType, size)
	r.cachedVertices = vertices
	r.lastSize = size

	// Если количество линий не совпадает с количеством вершин, пересоздаём линии
	if len(r.shapeLines) != len(vertices) {
		r.createShapeLines(vertices)
	} else {
		r.updateLinesFromVertices(vertices)
	}
}

// createShapeLines создаёт новые линии по заданным вершинам.
func (r *blockRenderer) createShapeLines(vertices []fyne.Position) {
	r.shapeLines = make([]*canvas.Line, len(vertices))
	for i := 0; i < len(vertices); i++ {
		next := (i + 1) % len(vertices)
		line := canvas.NewLine(r.lineColor)
		line.Position1 = vertices[i]
		line.Position2 = vertices[next]
		line.StrokeWidth = r.lineWidth * r.scale
		r.shapeLines[i] = line
	}
}

// updateLinesFromVertices обновляет позиции существующих линий.
func (r *blockRenderer) updateLinesFromVertices(vertices []fyne.Position) {
	for i := 0; i < len(vertices); i++ {
		if i >= len(r.shapeLines) {
			break
		}
		next := (i + 1) % len(vertices)
		r.shapeLines[i].Position1 = vertices[i]
		r.shapeLines[i].Position2 = vertices[next]
	}
}

// Layout упорядочивает элементы внутри виджета.
func (r *blockRenderer) Layout(size fyne.Size) {
	r.scale = r.widget.programPanel.GetScale()

	// Обновляем линии с проверкой кэша
	r.updateShapeLines(false)

	centerX := size.Width / 2
	centerY := size.Height / 2

	r.positionTextElements(centerX, centerY)
	r.updateLineStyles()
}

func (r *blockRenderer) positionTextElements(centerX, centerY float32) {
	r.iconText.TextSize = 20 * r.scale
	r.titleText.TextSize = 14 * r.scale
	r.descText.TextSize = 10 * r.scale

	iconSize := r.iconText.MinSize()
	titleSize := r.titleText.MinSize()

	// Позиционирование в зависимости от формы блока (без изменений)
	if r.shapeType == ShapeEllipse || r.shapeType == ShapeHexagon || r.shapeType == ShapeHexagonInverted {
		r.iconText.Move(fyne.NewPos(centerX-iconSize.Width/2, centerY-iconSize.Height-5*r.scale))
		r.titleText.Move(fyne.NewPos(centerX-titleSize.Width/2, centerY-titleSize.Height/2))
		if len(r.widget.block.Description) > 0 {
			descSize := r.descText.MinSize()
			r.descText.Move(fyne.NewPos(centerX-descSize.Width/2, centerY+10*r.scale))
		} else {
			r.descText.Hide()
		}
	} else {
		r.iconText.Move(fyne.NewPos(centerX-iconSize.Width/2, centerY-iconSize.Height-10*r.scale))
		r.titleText.Move(fyne.NewPos(centerX-titleSize.Width/2, centerY-titleSize.Height/2))
		descSize := r.descText.MinSize()
		r.descText.Move(fyne.NewPos(centerX-descSize.Width/2, centerY+10*r.scale))
	}
}

func (r *blockRenderer) MinSize() fyne.Size {
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
	default:
		minWidth = 150 * r.scale
		minHeight = 80 * r.scale
	}

	titleSize := r.titleText.MinSize()
	requiredWidth := maxFloat32(minWidth, titleSize.Width+20*r.scale)
	return fyne.NewSize(requiredWidth, minHeight)
}

func (r *blockRenderer) Refresh() {
	r.scale = r.widget.programPanel.GetScale()
	r.lineColor = parseColor(r.widget.block.Color)
	r.highlighted = r.widget.isSelected
	r.executing = r.widget.isExecuting

	r.updateLineStyles()

	r.titleText.Text = r.widget.block.Title
	r.descText.Text = r.widget.block.Description
	r.iconText.Text = getBlockIcon(r.widget.block.Type)

	r.iconText.TextSize = 20 * r.scale
	r.titleText.TextSize = 14 * r.scale
	r.descText.TextSize = 10 * r.scale

	if len(r.widget.block.Description) > 0 {
		r.descText.Show()
	} else {
		r.descText.Hide()
	}

	// Принудительно пересчитать линии, т.к. мог измениться масштаб
	r.updateShapeLines(true)

	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *blockRenderer) updateLineStyles() {
	strokeWidth := LineStrokeWidthNormal * r.scale
	strokeColor := r.lineColor

	if r.executing {
		strokeWidth = LineStrokeWidthExecuting * r.scale
		strokeColor = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	} else if r.highlighted {
		strokeWidth = LineStrokeWidthHighlighted * r.scale
		strokeColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}
	}

	for _, line := range r.shapeLines {
		line.StrokeWidth = strokeWidth
		line.StrokeColor = strokeColor
	}
}

func (r *blockRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *blockRenderer) Destroy() {
	r.shapeLines = nil
	r.objects = nil
}

func maxFloat32(values ...float32) float32 {
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}
