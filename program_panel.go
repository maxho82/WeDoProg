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
	lastBlockY  float64
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
		lastBlockY:  50, // Начальная Y-координата для первого блока
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
	// Устанавливаем позицию блока
	block.X = 100
	block.Y = p.lastBlockY
	block.DragStartPos = fyne.NewPos(float32(block.X), float32(block.Y))

	blockWidget := NewDraggableBlock(block, p.programMgr, p.gui)
	blockWidget.Resize(fyne.NewSize(float32(block.Width), float32(block.Height)))
	blockWidget.Move(fyne.NewPos(float32(block.X), float32(block.Y)))

	p.content.Add(blockWidget)
	p.content.Refresh()

	// Автоматически соединяем с предыдущим блоком
	p.autoConnectBlock(block)

	// Обновляем позицию для следующего блока
	p.lastBlockY += block.Height + 40 // Отступ между блоками

	log.Printf("Блок добавлен на холст: %s (ID: %d) на позиции (%.0f, %.0f)",
		block.Title, block.ID, block.X, block.Y)
}

// autoConnectBlock автоматически соединяет блок с предыдущим
func (p *ProgramPanel) autoConnectBlock(newBlock *ProgramBlock) {
	// Находим последний добавленный блок (кроме текущего)
	var lastBlock *ProgramBlock
	for _, block := range p.programMgr.program.Blocks {
		if block.ID != newBlock.ID {
			lastBlock = block
		}
	}

	if lastBlock != nil {
		// Добавляем соединение в менеджер программ
		p.programMgr.AddConnection(lastBlock.ID, newBlock.ID)

		// Создаем визуальное соединение
		p.createVisualConnection(lastBlock.ID, newBlock.ID)

		log.Printf("Автоматическое соединение: блок %d -> блок %d", lastBlock.ID, newBlock.ID)
	}
}

// createVisualConnection создает визуальное соединение между блоками
func (p *ProgramPanel) createVisualConnection(fromBlockID, toBlockID int) {
	// Находим виджеты блоков
	var fromWidget, toWidget *DraggableBlock
	for _, obj := range p.content.Objects {
		if block, ok := obj.(*DraggableBlock); ok {
			if block.block.ID == fromBlockID {
				fromWidget = block
			} else if block.block.ID == toBlockID {
				toWidget = block
			}
		}
	}

	if fromWidget == nil || toWidget == nil {
		log.Printf("Не удалось найти виджеты для соединения %d -> %d", fromBlockID, toBlockID)
		return
	}

	// Получаем позиции коннекторов
	fromPos := fromWidget.GetBottomConnectorPosition()
	toPos := toWidget.GetTopConnectorPosition()

	// Создаем линию соединения
	line := canvas.NewLine(color.NRGBA{R: 0, G: 150, B: 255, A: 255})
	line.Position1 = fromPos
	line.Position2 = toPos
	line.StrokeWidth = 2

	// Добавляем стрелку на конце линии (простой вариант)
	p.addSimpleArrowHead(line, fromPos, toPos)

	p.content.Add(line)
	// Перемещаем линию на задний план (после фона, но перед блоками)
	// Для простоты просто добавляем в конец
	p.content.Objects = append([]fyne.CanvasObject{line}, p.content.Objects...)

	// Сохраняем соединение
	connection := &ConnectionLine{
		line:        line,
		fromBlockID: fromBlockID,
		toBlockID:   toBlockID,
	}

	p.connections = append(p.connections, connection)
	p.content.Refresh()
}

// addSimpleArrowHead добавляет простую стрелку на конце линии
func (p *ProgramPanel) addSimpleArrowHead(line *canvas.Line, fromPos, toPos fyne.Position) {
	// Простая реализация: просто делаем линию немного толще на конце
	// В реальном приложении можно добавить настоящую стрелку
	line.StrokeWidth = 2
}

// updateConnections обновляет все соединения
func (p *ProgramPanel) updateConnections() {
	for _, conn := range p.connections {
		// Находим виджеты блоков
		var fromWidget, toWidget *DraggableBlock
		for _, obj := range p.content.Objects {
			if block, ok := obj.(*DraggableBlock); ok {
				if block.block.ID == conn.fromBlockID {
					fromWidget = block
				} else if block.block.ID == conn.toBlockID {
					toWidget = block
				}
			}
		}

		if fromWidget != nil && toWidget != nil {
			// Обновляем позиции линии
			fromPos := fromWidget.GetBottomConnectorPosition()
			toPos := toWidget.GetTopConnectorPosition()

			conn.line.Position1 = fromPos
			conn.line.Position2 = toPos
			conn.line.Refresh()
		}
	}
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
		if conn.fromBlockID == blockID || conn.toBlockID == blockID {
			// Удаляем линию из контейнера
			for i, obj := range p.content.Objects {
				if obj == conn.line {
					p.content.Objects = append(p.content.Objects[:i], p.content.Objects[i+1:]...)
					break
				}
			}
		} else {
			newConnections = append(newConnections, conn)
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
	p.lastBlockY = 50
	p.content.Refresh()
}

// UpdateConnectionPositions обновляет позиции соединений
func (p *ProgramPanel) UpdateConnectionPositions() {
	for _, conn := range p.connections {
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
	p.gui.showBlockProperties(block)
}

// GetBlockWidget возвращает виджет блока по ID
func (p *ProgramPanel) GetBlockWidget(blockID int) *DraggableBlock {
	for _, obj := range p.content.Objects {
		if block, ok := obj.(*DraggableBlock); ok && block.block.ID == blockID {
			return block
		}
	}
	return nil
}

// RecreateConnections перерисовывает все соединения
func (p *ProgramPanel) RecreateConnections() {
	// Удаляем все существующие соединения
	for _, conn := range p.connections {
		p.content.Remove(conn.line)
	}
	p.connections = make([]*ConnectionLine, 0)

	// Создаем новые соединения
	for _, conn := range p.programMgr.program.Connections {
		p.createVisualConnection(conn.FromBlockID, conn.ToBlockID)
	}

	p.content.Refresh()
}
