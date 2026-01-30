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
	gui           *MainGUI
	scroll        *container.Scroll
	content       *fyne.Container
	programMgr    *ProgramManager
	connections   []*ConnectionLine
	blockWidgets  map[int]*DraggableBlock
	lastBlockY    float64
	selectedBlock *ProgramBlock   // Выбранный блок для выделения
	gridContainer *fyne.Container // Контейнер для сетки
}

// ConnectionLine линия соединения между блоками
type ConnectionLine struct {
	line          *canvas.Line
	fromBlockID   int
	toBlockID     int
	isHighlighted bool
}

// NewProgramPanel создает панель программирования
func NewProgramPanel(gui *MainGUI, programMgr *ProgramManager) *ProgramPanel {
	panel := &ProgramPanel{
		gui:          gui,
		programMgr:   programMgr,
		connections:  make([]*ConnectionLine, 0),
		blockWidgets: make(map[int]*DraggableBlock),
		lastBlockY:   50,
	}

	// Создаем основной контейнер с сеткой и блоками
	panel.content = container.NewWithoutLayout()
	panel.addGrid()

	panel.scroll = container.NewScroll(panel.content)
	panel.scroll.SetMinSize(fyne.NewSize(800, 600))

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

	// Контейнер для линий сетки
	p.gridContainer = container.NewWithoutLayout()

	// Вертикальные линии
	for x := 0; x <= 2000; x += 20 {
		line := canvas.NewLine(color.NRGBA{R: 50, G: 50, B: 50, A: 255})
		line.Position1 = fyne.NewPos(float32(x), 0)
		line.Position2 = fyne.NewPos(float32(x), 2000)
		line.StrokeWidth = 1
		p.gridContainer.Add(line)
	}

	// Горизонтальные линии
	for y := 0; y <= 2000; y += 20 {
		line := canvas.NewLine(color.NRGBA{R: 50, G: 50, B: 50, A: 255})
		line.Position1 = fyne.NewPos(0, float32(y))
		line.Position2 = fyne.NewPos(2000, float32(y))
		line.StrokeWidth = 1
		p.gridContainer.Add(line)
	}

	p.content.Add(p.gridContainer)
}

// AddBlock добавляет блок на холст
func (p *ProgramPanel) AddBlock(block *ProgramBlock) {
	// Проверяем, не добавлен ли уже блок
	if _, exists := p.blockWidgets[block.ID]; exists {
		log.Printf("Блок %d уже добавлен на холст", block.ID)
		return
	}

	// Устанавливаем позицию блока
	block.X = 100
	block.Y = p.lastBlockY
	block.DragStartPos = fyne.NewPos(float32(block.X), float32(block.Y))

	// Создаем виджет блока
	blockWidget := NewDraggableBlock(block, p.programMgr, p.gui)
	blockWidget.Resize(fyne.NewSize(float32(block.Width), float32(block.Height)))
	blockWidget.Move(fyne.NewPos(float32(block.X), float32(block.Y)))

	// Добавляем на панель (после сетки, чтобы блоки были сверху)
	p.content.Add(blockWidget)
	p.blockWidgets[block.ID] = blockWidget

	// Обновляем lastBlockY для следующего блока
	p.lastBlockY = block.Y + block.Height + 40

	p.content.Refresh()

	// Автоматически соединяем с предыдущим блоком
	p.autoConnectBlock(block)

	log.Printf("Блок добавлен на холст: %s (ID: %d) на позиции (%.0f, %.0f)",
		block.Title, block.ID, block.X, block.Y)
}

// autoConnectBlock автоматически соединяет блок с предыдущим
func (p *ProgramPanel) autoConnectBlock(newBlock *ProgramBlock) {
	// Находим последний добавленный блок (кроме текущего)
	var lastBlock *ProgramBlock
	lastBlockID := 0

	for _, block := range p.programMgr.program.Blocks {
		if block.ID != newBlock.ID && block.ID > lastBlockID {
			lastBlockID = block.ID
			lastBlock = block
		}
	}

	if lastBlock != nil && lastBlock.NextBlockID == 0 {
		// Добавляем соединение в менеджер программ
		p.programMgr.AddConnection(lastBlock.ID, newBlock.ID)

		// Создаем визуальное соединение
		p.createVisualConnection(lastBlock.ID, newBlock.ID)

		log.Printf("Автоматическое соединение: блок %d -> блок %d", lastBlock.ID, newBlock.ID)
	}
}

// createVisualConnection создает визуальное соединение между блоками
func (p *ProgramPanel) createVisualConnection(fromBlockID, toBlockID int) {
	// Получаем виджеты блоков
	fromWidget, fromExists := p.blockWidgets[fromBlockID]
	toWidget, toExists := p.blockWidgets[toBlockID]

	if !fromExists || !toExists {
		log.Printf("Не удалось найти виджеты для соединения %d -> %d", fromBlockID, toBlockID)
		return
	}

	// Получаем позиции коннекторов
	fromPos := fromWidget.GetBottomConnectorPosition()
	toPos := toWidget.GetTopConnectorPosition()

	// Создаем линию соединения (синяя по умолчанию)
	line := canvas.NewLine(color.NRGBA{R: 0, G: 150, B: 255, A: 255})
	line.Position1 = fromPos
	line.Position2 = toPos
	line.StrokeWidth = 2

	// Добавляем линию на панель (после сетки, но до блоков)
	p.content.Add(line)

	// Сохраняем соединение
	connection := &ConnectionLine{
		line:          line,
		fromBlockID:   fromBlockID,
		toBlockID:     toBlockID,
		isHighlighted: false,
	}

	p.connections = append(p.connections, connection)
	p.content.Refresh()
}

// updateConnections обновляет все соединения
func (p *ProgramPanel) updateConnections() {
	for _, conn := range p.connections {
		// Получаем виджеты блоков
		fromWidget, fromExists := p.blockWidgets[conn.fromBlockID]
		toWidget, toExists := p.blockWidgets[conn.toBlockID]

		if fromExists && toExists {
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
	// Удаляем виджет блока из контейнера
	if blockWidget, exists := p.blockWidgets[blockID]; exists {
		for i, obj := range p.content.Objects {
			if obj == blockWidget {
				p.content.Objects = append(p.content.Objects[:i], p.content.Objects[i+1:]...)
				break
			}
		}
		delete(p.blockWidgets, blockID)
	}

	// Удаляем связанные соединения
	p.removeConnectionsForBlock(blockID)

	// Пересчитываем позиции оставшихся блоков
	p.repositionRemainingBlocks()

	p.content.Refresh()
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

// repositionRemainingBlocks перепозиционирует оставшиеся блоки
func (p *ProgramPanel) repositionRemainingBlocks() {
	// Сортируем блоки по ID
	var blockIDs []int
	for id := range p.blockWidgets {
		blockIDs = append(blockIDs, id)
	}

	// Простая сортировка
	for i := 0; i < len(blockIDs)-1; i++ {
		for j := i + 1; j < len(blockIDs); j++ {
			if blockIDs[i] > blockIDs[j] {
				blockIDs[i], blockIDs[j] = blockIDs[j], blockIDs[i]
			}
		}
	}

	// Располагаем блоки по порядку
	currentY := 50.0
	for _, blockID := range blockIDs {
		if widget, exists := p.blockWidgets[blockID]; exists {
			widget.block.Y = currentY
			widget.block.X = 100
			widget.Move(fyne.NewPos(100, float32(currentY)))
			currentY += widget.block.Height + 40
		}
	}

	p.lastBlockY = currentY

	// Обновляем соединения
	p.updateConnections()
}

// Clear очищает холст
func (p *ProgramPanel) Clear() {
	// Оставляем только фон и сетку
	var newObjects []fyne.CanvasObject
	newObjects = append(newObjects, p.content.Objects[0]) // Фон
	newObjects = append(newObjects, p.content.Objects[1]) // Сетка

	p.content.Objects = newObjects
	p.connections = make([]*ConnectionLine, 0)
	p.blockWidgets = make(map[int]*DraggableBlock)
	p.lastBlockY = 50
	p.content.Refresh()
}

// HighlightConnections выделяет соединения блока
func (p *ProgramPanel) HighlightConnections(blockID int) {
	// Сбрасываем выделение всех линий
	for _, conn := range p.connections {
		conn.isHighlighted = false
		conn.line.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255} // Синий
		conn.line.StrokeWidth = 2
	}

	// Выделяем линии, связанные с блоком
	for _, conn := range p.connections {
		if conn.fromBlockID == blockID || conn.toBlockID == blockID {
			conn.isHighlighted = true
			conn.line.StrokeColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255} // Золотой
			conn.line.StrokeWidth = 3
		}
	}

	p.content.Refresh()
}

// ResetHighlight сбрасывает выделение всех соединений
func (p *ProgramPanel) ResetHighlight() {
	for _, conn := range p.connections {
		conn.isHighlighted = false
		conn.line.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
		conn.line.StrokeWidth = 2
	}
	p.content.Refresh()
}

// GetBlockWidget возвращает виджет блока по ID
func (p *ProgramPanel) GetBlockWidget(blockID int) *DraggableBlock {
	return p.blockWidgets[blockID]
}

// SetSelectedBlock устанавливает выбранный блок
func (p *ProgramPanel) SetSelectedBlock(block *ProgramBlock) {
	p.selectedBlock = block
	if block != nil {
		p.HighlightConnections(block.ID)
	} else {
		p.ResetHighlight()
	}
}
