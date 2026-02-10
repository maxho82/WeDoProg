package main

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// ProgramPanel панель визуального программирования (дракон-схемы)
type ProgramPanel struct {
	gui           *MainGUI
	scroll        *container.Scroll
	content       *fyne.Container
	programMgr    *ProgramManager
	layout        *ProgramLayout
	connections   []*ConnectionLine
	blockWidgets  map[int]*DraggableBlock
	selectedBlock *ProgramBlock
	scale         float32

	// Новые поля для управления валентными точками
	valenceManager  *ValenceManager
	valenceOverlay  *fyne.Container // Оверлей для валентных точек и временных линий
	isInsertMode    bool            // Режим вставки нового блока
	insertBlockType BlockType       // Тип блока для вставки
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
		scale:        1.0,
		isInsertMode: false,
	}

	// Создаем кастомный layout
	panel.layout = NewProgramLayout(programMgr, panel)
	panel.layout.SetScale(panel.scale)

	// Создаем контейнер С layout, а не WithoutLayout!
	panel.content = container.New(panel.layout)

	// Создаем менеджер валентных точек
	panel.valenceManager = NewValenceManager(panel)

	// Создаем оверлей для валентных точек (располагаем поверх основного контента)
	panel.valenceOverlay = container.NewWithoutLayout(panel.valenceManager.GetContainer())

	// Создаем основной контейнер с наложением
	mainContainer := container.NewStack(
		panel.content,
		panel.valenceOverlay,
	)

	// Создаем скролл-контейнер
	panel.scroll = container.NewScroll(mainContainer)
	panel.scroll.SetMinSize(fyne.NewSize(400, 300))

	return panel
}

// GetContainer возвращает контейнер панели
func (p *ProgramPanel) GetContainer() fyne.CanvasObject {
	return p.scroll
}

// SetInsertMode устанавливает режим вставки нового блока
func (p *ProgramPanel) SetInsertMode(blockType BlockType) {
	p.isInsertMode = true
	p.insertBlockType = blockType

	// Показываем валентные точки для выбранного типа блока
	p.valenceManager.ShowValencePoints(blockType)

	// Обновляем оверлей
	p.valenceOverlay.Refresh()

	log.Printf("Режим вставки включен для типа блока: %v", blockType)
}

// CancelInsertMode отменяет режим вставки
func (p *ProgramPanel) CancelInsertMode() {
	p.isInsertMode = false
	p.insertBlockType = 0

	// Очищаем валентные точки
	p.valenceManager.ClearPoints()

	// Обновляем оверлей
	p.valenceOverlay.Refresh()

	log.Println("Режим вставки отменен")

	// Уведомляем GUI об отмене
	fyne.Do(func() {
		if p.gui != nil {
			p.gui.updateToolbarState()
			p.gui.updateBlockButtonsState()
		}
	})
}

// AddBlock добавляет блок на холст (старый метод для обратной совместимости)
func (p *ProgramPanel) AddBlock(block *ProgramBlock) {
	// Для обратной совместимости, если не в режиме вставки
	if !p.isInsertMode {
		p.AddBlockAtPosition(block, p.calculateInsertIndex())
	}
}

// AddBlockAtPosition добавляет блок на указанную позицию
func (p *ProgramPanel) AddBlockAtPosition(block *ProgramBlock, insertIndex int) {
	// Проверяем, не добавлен ли уже блок
	if _, exists := p.blockWidgets[block.ID]; exists {
		log.Printf("Блок %d уже добавлен на холст", block.ID)
		return
	}

	// Проверяем особые случаи для блоков "Начать" и "Стоп"
	switch block.Type {
	case BlockTypeStart:
		for _, b := range p.programMgr.program.Blocks {
			if b.Type == BlockTypeStart {
				log.Println("Блок 'Начать' уже существует в программе")
				return
			}
		}
	case BlockTypeStop:
		for _, b := range p.programMgr.program.Blocks {
			if b.Type == BlockTypeStop {
				log.Println("Блок 'Стоп' уже существует в программе")
				return
			}
		}
	}

	log.Printf("Вставка блока %d на позицию %d", block.ID, insertIndex)

	// Вставляем блок в программу
	p.insertBlockToProgram(block, insertIndex)

	// Создаем виджет блока
	blockWidget := NewDraggableBlock(block, p.programMgr, p.gui, p)

	// Устанавливаем начальный размер
	blockWidget.Resize(fyne.NewSize(float32(block.Width), float32(block.Height)))

	// Добавляем на панель
	p.content.Add(blockWidget)
	p.blockWidgets[block.ID] = blockWidget

	// Обновляем layout (это автоматически расставит блоки)
	p.content.Refresh()

	// Обновляем соединения
	p.updateAllConnections()

	// Если был режим вставки, отключаем его
	if p.isInsertMode {
		p.CancelInsertMode()
	}

	// Обновляем состояние кнопок в GUI
	p.gui.updateToolbarState()
	p.gui.updateBlockButtonsState()

	log.Printf("Блок добавлен на холст: %s (ID: %d)", block.Title, block.ID)
}

// calculateInsertIndex вычисляет индекс вставки нового блока (для обратной совместимости)
func (p *ProgramPanel) calculateInsertIndex() int {
	if len(p.programMgr.program.Blocks) == 0 {
		return 0
	}

	if p.selectedBlock == nil {
		for i, block := range p.programMgr.program.Blocks {
			if block.Type == BlockTypeStop {
				return i
			}
		}
		return len(p.programMgr.program.Blocks)
	}

	// Используем switch вместо цепочки if-else
	switch p.selectedBlock.Type {
	case BlockTypeLoopStart:
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				loopEndID, found := p.programMgr.FindLoopEndID(block.ID)
				if found {
					for j, b := range p.programMgr.program.Blocks {
						if b.ID == loopEndID {
							return j
						}
					}
				}
				return i + 1
			}
		}

	case BlockTypeLoopEnd:
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				return i + 1
			}
		}

	case BlockTypeStop:
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				return i
			}
		}

	default:
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				return i + 1
			}
		}
	}

	return len(p.programMgr.program.Blocks)
}

// insertBlockToProgram вставляет блок в программу по указанному индексу
func (p *ProgramPanel) insertBlockToProgram(block *ProgramBlock, index int) {
	if index < 0 {
		index = 0
	}
	if index > len(p.programMgr.program.Blocks) {
		index = len(p.programMgr.program.Blocks)
	}

	if index == len(p.programMgr.program.Blocks) {
		p.programMgr.program.Blocks = append(p.programMgr.program.Blocks, block)
	} else {
		p.programMgr.program.Blocks = append(p.programMgr.program.Blocks[:index],
			append([]*ProgramBlock{block}, p.programMgr.program.Blocks[index:]...)...)
	}

	log.Printf("Блок %d вставлен в программу на позицию %d", block.ID, index)
}

// updateAllConnections обновляет все связи между блоками
func (p *ProgramPanel) updateAllConnections() {
	// Очищаем все существующие визуальные соединения
	for _, conn := range p.connections {
		for i, obj := range p.content.Objects {
			if obj == conn.line {
				p.content.Objects = append(p.content.Objects[:i], p.content.Objects[i+1:]...)
				break
			}
		}
	}
	p.connections = make([]*ConnectionLine, 0)

	// Очищаем все связи в менеджере программ
	p.programMgr.program.Connections = make([]*Connection, 0)

	// Создаем связи между всеми блоками по порядку
	for i := 0; i < len(p.programMgr.program.Blocks)-1; i++ {
		currentBlock := p.programMgr.program.Blocks[i]
		nextBlock := p.programMgr.program.Blocks[i+1]

		currentBlock.NextBlockID = nextBlock.ID

		p.programMgr.program.Connections = append(p.programMgr.program.Connections, &Connection{
			FromBlockID: currentBlock.ID,
			ToBlockID:   nextBlock.ID,
		})

		p.createVisualConnection(currentBlock.ID, nextBlock.ID)
	}

	if len(p.programMgr.program.Blocks) > 0 {
		lastBlock := p.programMgr.program.Blocks[len(p.programMgr.program.Blocks)-1]
		lastBlock.NextBlockID = 0
	}
}

// createVisualConnection создает визуальное соединение между блоками
func (p *ProgramPanel) createVisualConnection(fromBlockID, toBlockID int) {
	fromWidget, fromExists := p.blockWidgets[fromBlockID]
	toWidget, toExists := p.blockWidgets[toBlockID]

	if !fromExists || !toExists {
		log.Printf("Не удалось найти виджеты для соединения %d -> %d", fromBlockID, toBlockID)
		return
	}

	fromPos := fromWidget.GetBottomConnectorPosition()
	toPos := toWidget.GetTopConnectorPosition()

	line := canvas.NewLine(color.NRGBA{R: 0, G: 150, B: 255, A: 255})
	line.Position1 = fromPos
	line.Position2 = toPos
	line.StrokeWidth = 2 * p.scale

	p.content.Add(line)

	connection := &ConnectionLine{
		line:          line,
		fromBlockID:   fromBlockID,
		toBlockID:     toBlockID,
		isHighlighted: false,
	}

	p.connections = append(p.connections, connection)
}

// RemoveBlock удаляет блок с холста
func (p *ProgramPanel) RemoveBlock(blockID int) {
	log.Printf("Начинаем удаление блока %d с холста", blockID)

	var blockToRemove *ProgramBlock
	for _, block := range p.programMgr.program.Blocks {
		if block.ID == blockID {
			blockToRemove = block
			break
		}
	}

	if blockToRemove == nil {
		log.Printf("Блок %d не найден в программе", blockID)
		return
	}

	if blockToRemove.Type == BlockTypeStart || blockToRemove.Type == BlockTypeStop {
		log.Printf("Блок '%s' (ID: %d) нельзя удалять", blockToRemove.Title, blockID)
		return
	}

	if blockToRemove.Type == BlockTypeLoopStart || blockToRemove.Type == BlockTypeLoopEnd {
		p.gui.deleteLoopWithConfirmation(blockID)
		return
	}

	removeIndex := -1
	for i, block := range p.programMgr.program.Blocks {
		if block.ID == blockID {
			removeIndex = i
			break
		}
	}

	if removeIndex == -1 {
		log.Printf("Блок %d не найден в программе", blockID)
		return
	}

	// Удаляем блок из программы
	if removeIndex == 0 {
		p.programMgr.program.Blocks = p.programMgr.program.Blocks[1:]
	} else if removeIndex == len(p.programMgr.program.Blocks)-1 {
		p.programMgr.program.Blocks = p.programMgr.program.Blocks[:removeIndex]
	} else {
		p.programMgr.program.Blocks = append(
			p.programMgr.program.Blocks[:removeIndex],
			p.programMgr.program.Blocks[removeIndex+1:]...,
		)
	}

	// Удаляем виджет блока
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

	// Обновляем layout
	p.content.Refresh()

	// Обновляем все связи
	p.updateAllConnections()

	if p.selectedBlock != nil && p.selectedBlock.ID == blockID {
		p.selectedBlock = nil
		p.gui.selectedBlock = nil
		p.ResetHighlight()
	}

	log.Printf("Блок %d удален с холста. Осталось блоков: %d", blockID, len(p.programMgr.program.Blocks))
}

// RemoveBlockInternal - внутренний метод для удаления блока без проверок
func (p *ProgramPanel) RemoveBlockInternal(blockID int) {
	log.Printf("Внутреннее удаление блока %d", blockID)

	var blockToRemove *ProgramBlock
	var removeIndex = -1

	for i, block := range p.programMgr.program.Blocks {
		if block.ID == blockID {
			blockToRemove = block
			removeIndex = i
			break
		}
	}

	if blockToRemove == nil {
		log.Printf("Блок %d не найден в программе", blockID)
		return
	}

	if removeIndex == 0 {
		p.programMgr.program.Blocks = p.programMgr.program.Blocks[1:]
	} else if removeIndex == len(p.programMgr.program.Blocks)-1 {
		p.programMgr.program.Blocks = p.programMgr.program.Blocks[:removeIndex]
	} else {
		p.programMgr.program.Blocks = append(
			p.programMgr.program.Blocks[:removeIndex],
			p.programMgr.program.Blocks[removeIndex+1:]...,
		)
	}

	if blockWidget, exists := p.blockWidgets[blockID]; exists {
		for i, obj := range p.content.Objects {
			if obj == blockWidget {
				p.content.Objects = append(p.content.Objects[:i], p.content.Objects[i+1:]...)
				break
			}
		}
		delete(p.blockWidgets, blockID)
	}

	p.removeConnectionsForBlock(blockID)

	if p.selectedBlock != nil && p.selectedBlock.ID == blockID {
		p.selectedBlock = nil
		p.gui.selectedBlock = nil
		p.ResetHighlight()
	}

	log.Printf("Блок %d удален из программы. Осталось блоков: %d", blockID, len(p.programMgr.program.Blocks))
}

// removeConnectionsForBlock удаляет соединения для блока
func (p *ProgramPanel) removeConnectionsForBlock(blockID int) {
	var newConnections []*ConnectionLine
	for _, conn := range p.connections {
		if conn.fromBlockID == blockID || conn.toBlockID == blockID {
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

// Clear очищает холст
func (p *ProgramPanel) Clear() {
	p.content.Objects = nil
	p.connections = make([]*ConnectionLine, 0)
	p.blockWidgets = make(map[int]*DraggableBlock)
	p.selectedBlock = nil

	// Очищаем валентные точки
	p.valenceManager.ClearPoints()
	p.valenceOverlay.Refresh()

	p.content.Refresh()
}

// HighlightConnections выделяет соединение
func (p *ProgramPanel) HighlightConnections(block *ProgramBlock) {
	for _, conn := range p.connections {
		conn.isHighlighted = false
		conn.line.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
		conn.line.StrokeWidth = 2 * p.scale
	}

	if block == nil {
		p.content.Refresh()
		return
	}

	if block.Type != BlockTypeStop && block.Type != BlockTypeLoopEnd {
		for _, conn := range p.connections {
			if conn.fromBlockID == block.ID {
				conn.isHighlighted = true
				conn.line.StrokeColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}
				conn.line.StrokeWidth = 3 * p.scale
				break
			}
		}
	}

	p.content.Refresh()
}

// ResetHighlight сбрасывает выделение всех соединений
func (p *ProgramPanel) ResetHighlight() {
	for _, conn := range p.connections {
		conn.isHighlighted = false
		conn.line.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
		conn.line.StrokeWidth = 2 * p.scale
	}
	p.content.Refresh()
}

// GetBlockWidget возвращает виджет блока по ID
func (p *ProgramPanel) GetBlockWidget(blockID int) *DraggableBlock {
	return p.blockWidgets[blockID]
}

// SetSelectedBlock устанавливает выбранный блок
func (p *ProgramPanel) SetSelectedBlock(block *ProgramBlock) {
	for _, widget := range p.blockWidgets {
		widget.SetSelected(false)
	}

	p.selectedBlock = block
	if block != nil {
		if widget, exists := p.blockWidgets[block.ID]; exists {
			widget.SetSelected(true)
		}
		p.HighlightConnections(block)
	} else {
		p.ResetHighlight()
	}
}

// updateConnections обновляет позиции всех соединений
func (p *ProgramPanel) updateConnections() {
	for _, conn := range p.connections {
		fromWidget, fromExists := p.blockWidgets[conn.fromBlockID]
		toWidget, toExists := p.blockWidgets[conn.toBlockID]

		if fromExists && toExists {
			fromPos := fromWidget.GetBottomConnectorPosition()
			toPos := toWidget.GetTopConnectorPosition()

			conn.line.Position1 = fromPos
			conn.line.Position2 = toPos
			conn.line.StrokeWidth = 2 * p.scale
			if conn.isHighlighted {
				conn.line.StrokeWidth = 3 * p.scale
			}
			conn.line.Refresh()
		}
	}

	// Обновляем оверлей с валентными точками
	p.valenceOverlay.Refresh()
}

// HighlightExecutingBlock выделяет выполняющийся блок
func (p *ProgramPanel) HighlightExecutingBlock(blockID int) {
	if blockID == -1 {
		for _, widget := range p.blockWidgets {
			widget.SetExecuting(false)
		}
		p.content.Refresh()
		return
	}

	var block *ProgramBlock
	for _, b := range p.programMgr.program.Blocks {
		if b.ID == blockID {
			block = b
			break
		}
	}

	if block != nil {
		p.highlightBlockAsExecuting(block)
	}
}

// highlightBlockAsExecuting выделяет блок как выполняющийся
func (p *ProgramPanel) highlightBlockAsExecuting(block *ProgramBlock) {
	for _, widget := range p.blockWidgets {
		widget.SetExecuting(false)
	}

	if widget, exists := p.blockWidgets[block.ID]; exists {
		widget.SetExecuting(true)
	}

	p.HighlightConnections(block)
	p.content.Refresh()
}

// RemoveLoopBlocks удаляет все блоки цикла
func (p *ProgramPanel) RemoveLoopBlocks(loopBlocks []*ProgramBlock) {
	log.Printf("Удаление всех блоков цикла (количество: %d)", len(loopBlocks))

	for i := len(loopBlocks) - 1; i >= 0; i-- {
		block := loopBlocks[i]
		p.RemoveBlockInternal(block.ID)
	}

	p.content.Refresh()
	p.updateAllConnections()
	p.content.Refresh()
}

// ApplyScale применяет масштаб ко всем элементам холста
func (p *ProgramPanel) ApplyScale(newScale float32) {
	if newScale < 0.5 || newScale > 3.0 {
		return
	}

	oldScale := p.scale
	p.scale = newScale

	p.layout.SetScale(newScale)

	// Обновляем layout
	p.content.Layout.Layout(p.content.Objects, p.content.Size())

	p.content.Refresh()

	// Обновляем валентные точки с новым масштабом
	if p.isInsertMode {
		p.valenceManager.ShowValencePoints(p.insertBlockType)
		// Обновляем позиции точек с учетом нового масштаба
		p.valenceManager.UpdatePointsPositions()
	}
	p.valenceOverlay.Refresh()

	p.scroll.Refresh()

	log.Printf("Масштаб изменен: %.2f -> %.2f", oldScale, newScale)
}

// ZoomIn увеличивает масштаб
func (p *ProgramPanel) ZoomIn() {
	newScale := p.scale * 1.2
	if newScale > 3.0 {
		newScale = 3.0
	}
	p.ApplyScale(newScale)
}

// ZoomOut уменьшает масштаб
func (p *ProgramPanel) ZoomOut() {
	newScale := p.scale / 1.2
	if newScale < 0.5 {
		newScale = 0.5
	}
	p.ApplyScale(newScale)
}

// ResetZoom сбрасывает масштаб к 100%
func (p *ProgramPanel) ResetZoom() {
	p.ApplyScale(1.0)
}

// GetScale возвращает текущий масштаб
func (p *ProgramPanel) GetScale() float32 {
	return p.scale
}

// scrollToBlock прокручивает панель к указанному блоку
func (p *ProgramPanel) scrollToBlock(block *ProgramBlock) {
	if widget, exists := p.blockWidgets[block.ID]; exists {
		pos := widget.Position()
		size := widget.Size()

		centerX := pos.X + size.Width/2
		centerY := pos.Y + size.Height/2

		scrollWidth := p.scroll.Size().Width
		scrollHeight := p.scroll.Size().Height

		offsetX := centerX - scrollWidth/2
		offsetY := centerY - scrollHeight/2

		contentSize := p.content.Size()
		maxOffsetX := contentSize.Width - scrollWidth
		maxOffsetY := contentSize.Height - scrollHeight

		if offsetX < 0 {
			offsetX = 0
		} else if offsetX > maxOffsetX {
			offsetX = maxOffsetX
		}

		if offsetY < 0 {
			offsetY = 0
		} else if offsetY > maxOffsetY {
			offsetY = maxOffsetY
		}

		p.scroll.Offset = fyne.NewPos(offsetX, offsetY)
		p.scroll.Refresh()
	}
}

// RefreshAllConnections обновляет все соединения
func (p *ProgramPanel) RefreshAllConnections() {
	p.updateConnections()
	p.content.Refresh()
}

// IsInsertMode возвращает состояние режима вставки
func (p *ProgramPanel) IsInsertMode() bool {
	return p.isInsertMode
}
