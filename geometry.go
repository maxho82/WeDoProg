package main

import (
	"math"

	"fyne.io/fyne/v2"
)

// BlockShapeType определяет тип фигуры блока
type BlockShapeType int

const (
	ShapeRectangle       BlockShapeType = iota // Прямоугольник
	ShapeHexagon                               // Шестиугольник для циклов
	ShapeHexagonInverted                       // Инвертированный шестиугольник для конца цикла
	ShapeEllipse                               // Эллипс для начала/конца программы
	ShapeDiamond                               // Ромб для условий
)

// calculateShapeVertices вычисляет вершины для фигуры
func calculateShapeVertices(shapeType BlockShapeType, size fyne.Size) []fyne.Position {
	width := size.Width
	height := size.Height

	switch shapeType {
	case ShapeHexagon:
		// Шестиугольник: верхняя часть - трапеция (1/3 высоты), нижняя - прямоугольник (2/3)
		topHeight := height / 3
		topWidthOffset := width / 4

		return []fyne.Position{
			{X: topWidthOffset, Y: 0},         // Верхний левый угол трапеции
			{X: width - topWidthOffset, Y: 0}, // Верхний правый угол трапеции
			{X: width, Y: topHeight},          // Правый верхний угол прямоугольника
			{X: width, Y: height},             // Правый нижний угол
			{X: 0, Y: height},                 // Левый нижний угол
			{X: 0, Y: topHeight},              // Левый верхний угол прямоугольника
		}

	case ShapeHexagonInverted:
		// Инвертированный шестиугольник (для конца цикла)
		topHeight := height * 2 / 3
		topWidthOffset := width / 4

		return []fyne.Position{
			{X: 0, Y: 0},                           // Левый верхний угол
			{X: width, Y: 0},                       // Правый верхний угол
			{X: width, Y: topHeight},               // Правый нижний угол прямоугольной части
			{X: width - topWidthOffset, Y: height}, // Правый нижний угол трапеции
			{X: topWidthOffset, Y: height},         // Левый нижний угол трапеции
			{X: 0, Y: topHeight},                   // Левый нижний угол прямоугольной части
		}

	case ShapeDiamond:
		// Ромб (для условий)
		centerX := width / 2
		centerY := height / 2

		return []fyne.Position{
			{X: centerX, Y: 0},      // Верх
			{X: width, Y: centerY},  // Право
			{X: centerX, Y: height}, // Низ
			{X: 0, Y: centerY},      // Лево
		}

	case ShapeEllipse:
		// Эллипс (аппроксимированный 16-угольником для гладкости)
		centerX := width / 2
		centerY := height / 2
		radiusX := width / 2
		radiusY := height / 2
		segments := 16 // Увеличиваем для более гладкого эллипса
		vertices := make([]fyne.Position, segments)

		for i := 0; i < segments; i++ {
			angle := 2 * math.Pi * float64(i) / float64(segments)
			// Используем стандартные математические функции
			cos := math.Cos(angle)
			sin := math.Sin(angle)

			vertices[i] = fyne.Position{
				X: centerX + float32(cos)*radiusX,
				Y: centerY + float32(sin)*radiusY,
			}
		}
		return vertices

	default: // ShapeRectangle
		return []fyne.Position{
			{X: 0, Y: 0},
			{X: width, Y: 0},
			{X: width, Y: height},
			{X: 0, Y: height},
		}
	}
}

// calculateConnectorPositions вычисляет позиции коннекторов
func calculateConnectorPositions(pos fyne.Position, size fyne.Size) (topConnector, bottomConnector fyne.Position) {
	width := size.Width
	height := size.Height
	// Для всех фигур коннекторы находятся по центру верхней и нижней грани
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

// getShapeType возвращает тип фигуры для блока
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

// getBlockIcon возвращает иконку для блока
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
