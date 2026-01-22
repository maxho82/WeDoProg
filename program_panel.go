package main

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// ProgramPanel панель визуального программирования
type ProgramPanel struct {
	gui         *MainGUI
	scroll      *container.Scroll
	content     *fyne.Container
	programMgr  *ProgramManager
	connections []*ConnectionLine
}

// ConnectionLine линия соединения между блоками
type ConnectionLine struct {
	line        *canvas.Line
	fromBlockID int
	toBlockID   int
}

// NewProgramPanel создает панель программирования
func NewProgramPanel(gui *MainGUI, programMgr *ProgramManager) *ProgramPanel {
	panel := &ProgramPanel{
		gui:         gui,
		programMgr:  programMgr,
		connections: make([]*ConnectionLine, 0),
	}

	panel.content = container.NewWithoutLayout()
	panel.scroll = container.NewScroll(panel.content)
	panel.scroll.SetMinSize(fyne.NewSize(800, 600))

	// Добавляем сетку
	panel.addGrid()

	return panel
}

// GetContainer возвращает контейнер панели
func (p *ProgramPanel) GetContainer() fyne.CanvasObject {
	return p.scroll
}

// addGrid добавляет сетку на холст
func (p *ProgramPanel) addGrid() {
	// Фон сетки
	bg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 30, A: 255})
	bg.SetMinSize(fyne.NewSize(2000, 2000))
	p.content.Add(bg)

	// Линии сетки
	gridLines := container.NewWithoutLayout()

	// Вертикальные линии
	for x := 0; x <= 2000; x += 20 {
		line := canvas.NewLine(color.NRGBA{R: 50, G: 50, B: 50, A: 255})
		line.Position1 = fyne.NewPos(float32(x), 0)
		line.Position2 = fyne.NewPos(float32(x), 2000)
		line.StrokeWidth = 1
		gridLines.Add(line)
	}

	// Горизонтальные линии
	for y := 0; y <= 2000; y += 20 {
		line := canvas.NewLine(color.NRGBA{R: 50, G: 50, B: 50, A: 255})
		line.Position1 = fyne.NewPos(0, float32(y))
		line.Position2 = fyne.NewPos(2000, float32(y))
		line.StrokeWidth = 1
		gridLines.Add(line)
	}

	p.content.Add(gridLines)
}

// AddBlock добавляет блок на холст
func (p *ProgramPanel) AddBlock(block *ProgramBlock) {
	blockWidget := NewDraggableBlock(block, p.programMgr)
	p.content.Add(blockWidget)
	p.content.Refresh()

	log.Printf("Блок добавлен на холст: %s (ID: %d)", block.Title, block.ID)
}

// RemoveBlock удаляет блок с холста
func (p *ProgramPanel) RemoveBlock(blockID int) {
	for i, obj := range p.content.Objects {
		if block, ok := obj.(*DraggableBlock); ok && block.block.ID == blockID {
			p.content.Objects = append(p.content.Objects[:i], p.content.Objects[i+1:]...)
			p.content.Refresh()
			break
		}
	}

	// Удаляем связанные соединения
	p.removeConnectionsForBlock(blockID)
}

// removeConnectionsForBlock удаляет соединения для блока
func (p *ProgramPanel) removeConnectionsForBlock(blockID int) {
	var newConnections []*ConnectionLine
	for _, conn := range p.connections {
		if conn.fromBlockID != blockID && conn.toBlockID != blockID {
			newConnections = append(newConnections, conn)
			p.content.Remove(conn.line)
		}
	}
	p.connections = newConnections
}

// AddConnection добавляет соединение между блоками
func (p *ProgramPanel) AddConnection(fromBlockID, toBlockID int, fromPos, toPos fyne.Position) {
	line := canvas.NewLine(color.NRGBA{R: 0, G: 150, B: 255, A: 255})
	line.Position1 = fromPos
	line.Position2 = toPos
	line.StrokeWidth = 2

	p.content.Add(line)

	connection := &ConnectionLine{
		line:        line,
		fromBlockID: fromBlockID,
		toBlockID:   toBlockID,
	}

	p.connections = append(p.connections, connection)
	p.content.Refresh()
}

// Clear очищает холст
func (p *ProgramPanel) Clear() {
	// Оставляем только фон и сетку
	var newObjects []fyne.CanvasObject
	for _, obj := range p.content.Objects {
		if _, ok := obj.(*canvas.Rectangle); ok {
			// Это фон
			newObjects = append(newObjects, obj)
		} else if container, ok := obj.(*fyne.Container); ok && len(container.Objects) > 0 {
			// Это сетка
			newObjects = append(newObjects, obj)
		}
	}

	p.content.Objects = newObjects
	p.connections = make([]*ConnectionLine, 0)
	p.content.Refresh()
}

// UpdateConnectionPositions обновляет позиции соединений
func (p *ProgramPanel) UpdateConnectionPositions() {
	for _, conn := range p.connections {
		// Здесь должна быть логика обновления позиций линий
		// когда блоки перемещаются
		conn.line.Refresh()
	}
}

// GetBlockAtPosition возвращает блок в указанной позиции
func (p *ProgramPanel) GetBlockAtPosition(pos fyne.Position) *ProgramBlock {
	for _, obj := range p.content.Objects {
		if block, ok := obj.(*DraggableBlock); ok {
			blockPos := block.Position()
			blockSize := block.Size()

			if pos.X >= blockPos.X && pos.X <= blockPos.X+blockSize.Width &&
				pos.Y >= blockPos.Y && pos.Y <= blockPos.Y+blockSize.Height {
				return block.block
			}
		}
	}
	return nil
}

// ShowProperties показывает свойства выбранного блока
func (p *ProgramPanel) ShowProperties(block *ProgramBlock) {
	// Реализуется в главном GUI
	p.gui.showBlockProperties(block)
}
