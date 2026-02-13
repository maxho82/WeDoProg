package main

import (
	"math"

	"fyne.io/fyne/v2"
)

// BlockShapeType определяет тип фигуры блока
type BlockShapeType int

const (
	ShapeRectangle       BlockShapeType = iota
	ShapeHexagon
	ShapeHexagonInverted
	ShapeEllipse
	ShapeDiamond
)

// calculateShapeVertices вычисляет вершины для фигуры
func calculateShapeVertices(shapeType BlockShapeType, size fyne.Size) []fyne.Position {
	width := size.Width
	height := size.Height

	switch shapeType {
	case ShapeHexagon:
		topHeight := height / 3
		topWidthOffset := width / 4
		return []fyne.Position{
			{X: topWidthOffset, Y: 0},
			{X: width - topWidthOffset, Y: 0},
			{X: width, Y: topHeight},
			{X: width, Y: height},
			{X: 0, Y: height},
			{X: 0, Y: topHeight},
		}
	case ShapeHexagonInverted:
		topHeight := height * 2 / 3
		topWidthOffset := width / 4
		return []fyne.Position{
			{X: 0, Y: 0},
			{X: width, Y: 0},
			{X: width, Y: topHeight},
			{X: width - topWidthOffset, Y: height},
			{X: topWidthOffset, Y: height},
			{X: 0, Y: topHeight},
		}
	case ShapeDiamond:
		centerX := width / 2
		centerY := height / 2
		return []fyne.Position{
			{X: centerX, Y: 0},
			{X: width, Y: centerY},
			{X: centerX, Y: height},
			{X: 0, Y: centerY},
		}
	case ShapeEllipse:
		centerX := width / 2
		centerY := height / 2
		radiusX := width / 2
		radiusY := height / 2
		segments := 16
		vertices := make([]fyne.Position, segments)
		for i := 0; i < segments; i++ {
			angle := 2 * math.Pi * float64(i) / float64(segments)
			cos := math.Cos(angle)
			sin := math.Sin(angle)
			vertices[i] = fyne.Position{
				X: centerX + float32(cos)*radiusX,
				Y: centerY + float32(sin)*radiusY,
			}
		}
		return vertices
	default:
		return []fyne.Position{
			{X: 0, Y: 0},
			{X: width, Y: 0},
			{X: width, Y: height},
			{X: 0, Y: height},
		}
	}
}

// calculateConnectorPositions вычисляет позиции коннекторов.
// Удалён неиспользуемый параметр shapeType.
func calculateConnectorPositions(pos fyne.Position, size fyne.Size) (topConnector, bottomConnector fyne.Position) {
	width := size.Width
	height := size.Height
	topConnector = fyne.Position{
		X: pos.X + width/2,
		Y: pos.Y,
	}
	bottomConnector = fyne.Position{
		X: pos.X + width/2,
		Y: pos.Y + height,
	}
	return topConnector, bottomConnector
}

// getShapeType возвращает тип фигуры для блока (без изменений)
func getShapeType(blockType BlockType) BlockShapeType {
	switch blockType {
	case BlockTypeLoopStart:
		return ShapeHexagon
	case BlockTypeLoopEnd:
		return ShapeHexagonInverted
	case BlockTypeCondition:
		return ShapeDiamond
	case BlockTypeStart, BlockTypeStop:
		return ShapeEllipse
	default:
		return ShapeRectangle
	}
}

// getBlockIcon возвращает иконку для блока (без изменений)
func getBlockIcon(blockType BlockType) string {
	switch blockType {
	case BlockTypeMotor:
		return "↺"
	case BlockTypeLED:
		return "💡"
	case BlockTypeWait:
		return "⏱"
	case BlockTypeLoopStart, BlockTypeLoopEnd:
		return "♻"
	case BlockTypeCondition:
		return "?"
	case BlockTypeStart:
		return "▶"
	case BlockTypeStop:
		return "■"
	case BlockTypeTiltSensor:
		return "📐"
	case BlockTypeDistanceSensor:
		return "📏"
	case BlockTypeSound:
		return "🔊"
	case BlockTypeVoltageSensor:
		return "⚡"
	case BlockTypeCurrentSensor:
		return "🔌"
	default:
		return "◼"
	}
}