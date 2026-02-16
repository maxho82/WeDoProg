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
	gui          *MainGUI
	scroll       *container.Scroll
	content      *fyne.Container
	programMgr   *ProgramManager
	layout       *ProgramLayout
	connections  []*ConnectionLine
	blockWidgets map[int]*DraggableBlock
	scale        float32

	valenceManager  *ValenceManager
	valenceOverlay  *fyne.Container
	isInsertMode    bool
	insertBlockType BlockType

	// Для вставки пары блоков (цикл)
	insertPairFirst  *ProgramBlock
	insertPairSecond *ProgramBlock

	// для если
	isMergeMode             bool                      // режим выбора точки слияния
	mergeConditionID        int                       // ID блока условия, для которого выбирается слияние
	altConnections          map[int][]*ConnectionLine // ключ - ID блока условия // специальные линии для альтернативных ветвей
	pendingMergeConditionID int                       // ID условия, для которого нужно включить режим слияния после перерисовки

	lastConnectorPositions map[int]fyne.Position
}

// ConnectionLine линия соединения между блоками
type ConnectionLine struct {
	line          *canvas.Line
	fromBlockID   int
	toBlockID     int
	isHighlighted bool
}

// NewProgramPanel создаёт панель программирования.
func NewProgramPanel(gui *MainGUI, programMgr *ProgramManager) *ProgramPanel {
	panel := &ProgramPanel{
		gui:                    gui,
		programMgr:             programMgr,
		connections:            make([]*ConnectionLine, 0),
		blockWidgets:           make(map[int]*DraggableBlock),
		scale:                  1.0,
		isInsertMode:           false,
		lastConnectorPositions: make(map[int]fyne.Position),
		altConnections:         make(map[int][]*ConnectionLine),
	}

	panel.layout = NewProgramLayout(programMgr, panel)
	panel.layout.SetScale(panel.scale)

	panel.content = container.New(panel.layout)
	panel.valenceManager = NewValenceManager(panel)
	panel.valenceOverlay = container.NewWithoutLayout(panel.valenceManager.GetContainer())

	mainContainer := container.NewStack(
		panel.content,
		panel.valenceOverlay,
	)

	panel.scroll = container.NewScroll(mainContainer)
	panel.scroll.SetMinSize(fyne.NewSize(400, 300))

	// Подписываемся на изменения программы
	programMgr.SetProgramChangedCallback(func() {
		fyne.Do(func() {
			panel.ReloadFromProgram()
		})
	})

	return panel
}

// GetContainer возвращает контейнер панели.
func (p *ProgramPanel) GetContainer() fyne.CanvasObject {
	return p.scroll
}

// ReloadFromProgram полностью перестраивает отображение на основе текущей программы из ProgramManager.
func (p *ProgramPanel) ReloadFromProgram() {
	// Очищаем текущее содержимое
	p.content.Objects = nil
	p.connections = make([]*ConnectionLine, 0)

	// Удаляем старые альтернативные линии
	for _, lines := range p.altConnections {
		for _, conn := range lines {
			p.content.Remove(conn.line)
		}
	}
	p.altConnections = make(map[int][]*ConnectionLine)

	p.blockWidgets = make(map[int]*DraggableBlock)
	p.lastConnectorPositions = make(map[int]fyne.Position)

	prog := p.programMgr.state.GetProgram()
	for _, block := range prog.Blocks {
		blockWidget := NewDraggableBlock(block, p.programMgr, p.gui, p)
		blockWidget.Resize(fyne.NewSize(float32(block.Width), float32(block.Height)))
		p.content.Add(blockWidget)
		p.blockWidgets[block.ID] = blockWidget
	}

	// Восстанавливаем обычные соединения
	for _, conn := range prog.Connections {
		p.createVisualConnection(conn.FromBlockID, conn.ToBlockID)
	}

	// Восстанавливаем альтернативные линии для условий
	p.rebuildAlternativeLines()

	p.content.Refresh()

	// Если есть ожидающий режим слияния, активируем его
	if p.pendingMergeConditionID != 0 {
		p.StartMergePointSelection(p.pendingMergeConditionID)
		p.pendingMergeConditionID = 0
	}
}

// SetInsertMode устанавливает режим вставки одного блока.
func (p *ProgramPanel) SetInsertMode(blockType BlockType) {
	p.isInsertMode = true
	p.insertBlockType = blockType
	p.insertPairFirst = nil
	p.insertPairSecond = nil
	p.valenceManager.ShowValencePoints(blockType)
	p.valenceOverlay.Refresh()
	log.Printf("Режим вставки включен для типа блока: %v", blockType)
}

// SetInsertLoopPair сохраняет пару блоков для последующей вставки.
func (p *ProgramPanel) SetInsertLoopPair(first, second *ProgramBlock) {
	if p.isInsertMode {
		// Если уже в режиме вставки, сбрасываем
		p.CancelInsertMode()
	}
	p.isInsertMode = true
	p.insertBlockType = BlockTypeLoopStart // условно
	p.insertPairFirst = first
	p.insertPairSecond = second
	p.valenceManager.ShowValencePoints(BlockTypeLoopStart)
	p.valenceOverlay.Refresh()
	log.Printf("Режим вставки пары циклов: блоки %d и %d", first.ID, second.ID)
}

// AddBlockAtPosition добавляет блок (или пару) в программу после блока с указанным ID.
// afterBlockID = 0 означает вставку в начало, -1 – в конец.
func (p *ProgramPanel) AddBlockAtPosition(block *ProgramBlock, afterBlockID int) {
	// Если это не пара, вставляем один блок
	if p.insertPairFirst == nil {
		// Проверяем, не добавлен ли уже блок с таким ID
		if _, exists := p.blockWidgets[block.ID]; exists {
			log.Printf("Блок %d уже присутствует на панели", block.ID)
			return
		}
		p.programMgr.InsertBlock(block, afterBlockID)
	} else {
		// Вставляем пару: первый блок с указанным afterBlockID,
		// второй – после первого.
		p.programMgr.InsertBlock(p.insertPairFirst, afterBlockID)
		p.programMgr.InsertBlock(p.insertPairSecond, p.insertPairFirst.ID)

		// Очищаем временное хранение пары
		p.insertPairFirst = nil
		p.insertPairSecond = nil
	}

	// Если был режим вставки, отключаем его
	if p.isInsertMode {
		p.CancelInsertMode()
	}

	// Если это блок условия, устанавливаем ожидание режима слияния
	if block.Type == BlockTypeCondition && p.gui != nil {
		p.pendingMergeConditionID = block.ID
		// Показываем подсказку в статусной строке
		p.gui.statusLabel.SetText("Выберите точку слияния для ветвей условия")
		p.gui.statusLabel.Refresh()
	}

	// Принудительно обновляем layout, чтобы позиции блоков стали актуальными
	p.content.Layout.Layout(p.content.Objects, p.content.Size())
	p.lastConnectorPositions = make(map[int]fyne.Position)

}

func (p *ProgramPanel) CancelInsertMode() {
	p.isInsertMode = false
	p.insertBlockType = 0
	p.insertPairFirst = nil
	p.insertPairSecond = nil
	p.valenceManager.ClearPoints()
	p.valenceOverlay.Refresh()
	log.Println("Режим вставки отменен")
	if p.gui != nil {
		p.gui.updateToolbarState()
		// Было: p.gui.updateBlockButtonsState()
		// Теперь обращаемся к блоку палитры:
		p.gui.blocksPalette.UpdateButtonsState(false)
	}
}

// createVisualConnection создаёт визуальное соединение между блоками.
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

// RemoveBlock удаляет блок с холста.
func (p *ProgramPanel) RemoveBlock(blockID int) {
	p.programMgr.RemoveBlock(blockID)
}

// RemoveLoopBlocks удаляет все блоки цикла.
func (p *ProgramPanel) RemoveLoopBlocks(loopBlocks []*ProgramBlock) {
	for _, block := range loopBlocks {
		p.programMgr.RemoveBlock(block.ID)
	}
}

// GetBlockWidget возвращает виджет блока по ID.
func (p *ProgramPanel) GetBlockWidget(blockID int) *DraggableBlock {
	return p.blockWidgets[blockID]
}

// SetSelectedBlock устанавливает выбранный блок.
func (p *ProgramPanel) SetSelectedBlock(block *ProgramBlock) {
	p.gui.SetSelectedBlock(block)
	p.updateBlockStyle()
}

// updateBlockStyle обновляет стиль блоков в зависимости от выбранного.
func (p *ProgramPanel) updateBlockStyle() {
	selected := p.gui.GetSelectedBlock()
	for id, widget := range p.blockWidgets {
		widget.SetSelected(selected != nil && id == selected.ID)
	}
}

// HighlightExecutingBlock выделяет выполняющийся блок.
func (p *ProgramPanel) HighlightExecutingBlock(blockID int) {
	if blockID == -1 {
		for _, widget := range p.blockWidgets {
			widget.SetExecuting(false)
		}
		p.content.Refresh()
		return
	}

	block := p.programMgr.state.FindBlockByID(blockID)
	if block != nil {
		for _, widget := range p.blockWidgets {
			widget.SetExecuting(false)
		}
		if widget, exists := p.blockWidgets[block.ID]; exists {
			widget.SetExecuting(true)
		}
		p.HighlightConnections(block)
		p.content.Refresh()
	}
}

// HighlightConnections выделяет соединение, исходящее из блока.
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

// ResetHighlight сбрасывает выделение всех соединений.
func (p *ProgramPanel) ResetHighlight() {
	for _, conn := range p.connections {
		conn.isHighlighted = false
		conn.line.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
		conn.line.StrokeWidth = 2 * p.scale
	}
	p.content.Refresh()
}

// updateConnections обновляет позиции всех соединений.
func (p *ProgramPanel) updateConnections() {
	// Обновляем обычные соединения (как и раньше)
	for _, conn := range p.connections {
		fromWidget, fromExists := p.blockWidgets[conn.fromBlockID]
		toWidget, toExists := p.blockWidgets[conn.toBlockID]
		if fromExists && toExists {
			fromPos := fromWidget.GetBottomConnectorPosition()
			toPos := toWidget.GetTopConnectorPosition()

			fromKey := conn.fromBlockID*2 + 0
			toKey := conn.toBlockID*2 + 1
			lastFrom, okFrom := p.lastConnectorPositions[fromKey]
			lastTo, okTo := p.lastConnectorPositions[toKey]

			if okFrom && okTo && lastFrom == fromPos && lastTo == toPos {
				continue
			}

			conn.line.Position1 = fromPos
			conn.line.Position2 = toPos
			conn.line.StrokeWidth = 2 * p.scale
			if conn.isHighlighted {
				conn.line.StrokeWidth = 3 * p.scale
			}
			conn.line.Refresh()

			p.lastConnectorPositions[fromKey] = fromPos
			p.lastConnectorPositions[toKey] = toPos
		}
	}

	// Обновляем альтернативные линии для всех условий
	prog := p.programMgr.state.GetProgram()
	for _, block := range prog.Blocks {
		if block.Type == BlockTypeCondition && block.OutBlockID != 0 && block.OutBlockID != block.NextBlockID {
			p.updateAlternativeLines(block)
		}
	}
}

// drawAlternativeBranch рисует ветвь «Ложь» для блока условия.
/* func (p *ProgramPanel) drawAlternativeBranch(condition *ProgramBlock) {
	fromWidget := p.blockWidgets[condition.ID]
	toWidget := p.blockWidgets[condition.OutBlockID]
	if fromWidget == nil || toWidget == nil {
		return
	}

	scale := p.scale

	// Вычисляем позиции (как и раньше)
	fromPos := fromWidget.Position()
	fromSize := fromWidget.Size()
	toPos := toWidget.Position()
	toSize := toWidget.Size()

	offsetX := fromSize.Width * 1.5

	topX := fromPos.X + fromSize.Width + offsetX
	topY := fromPos.Y + fromSize.Height/2
	bottomX := toPos.X + toSize.Width + offsetX
	bottomY := toPos.Y + toSize.Height/2

	// Если линии ещё не созданы — создаём и сохраняем
	if len(p.altConnections) == 0 {
		horLine1 := canvas.NewLine(color.NRGBA{R: 255, G: 100, B: 100, A: 255})
		vertLine := canvas.NewLine(color.NRGBA{R: 255, G: 100, B: 100, A: 255})
		horLine2 := canvas.NewLine(color.NRGBA{R: 255, G: 100, B: 100, A: 255})

		p.altConnections = []*ConnectionLine{
			{line: horLine1},
			{line: vertLine},
			{line: horLine2},
		}

		for _, conn := range p.altConnections {
			p.content.Add(conn.line)
		}
	}

	// Обновляем позиции существующих линий
	if len(p.altConnections) >= 3 {
		p.altConnections[0].line.Position1 = fyne.NewPos(fromPos.X+fromSize.Width, fromPos.Y+fromSize.Height/2)
		p.altConnections[0].line.Position2 = fyne.NewPos(topX, topY)

		p.altConnections[1].line.Position1 = fyne.NewPos(topX, topY)
		p.altConnections[1].line.Position2 = fyne.NewPos(bottomX, bottomY)

		p.altConnections[2].line.Position1 = fyne.NewPos(bottomX, bottomY)
		p.altConnections[2].line.Position2 = fyne.NewPos(toPos.X, toPos.Y+toSize.Height/2)

		// Устанавливаем толщину с учётом масштаба
		for _, conn := range p.altConnections {
			conn.line.StrokeWidth = 2 * scale
		}
	}
}
*/
// Clear очищает холст.
func (p *ProgramPanel) Clear() {
	p.content.Objects = nil
	p.connections = make([]*ConnectionLine, 0)
	p.blockWidgets = make(map[int]*DraggableBlock)
	p.lastConnectorPositions = make(map[int]fyne.Position)
	p.valenceManager.ClearPoints()
	p.valenceOverlay.Refresh()
	p.content.Refresh()
}

// ApplyScale применяет масштаб ко всем элементам.
func (p *ProgramPanel) ApplyScale(newScale float32) {
	if newScale < MinScale || newScale > MaxScale {
		return
	}
	p.scale = newScale
	p.layout.SetScale(newScale)
	p.content.Layout.Layout(p.content.Objects, p.content.Size())
	p.content.Refresh()
	p.lastConnectorPositions = make(map[int]fyne.Position)
	if p.isInsertMode {
		p.valenceManager.ShowValencePoints(p.insertBlockType)
		p.valenceManager.UpdatePointsPositions()
	}
	p.valenceOverlay.Refresh()
	p.scroll.Refresh()
}

// ZoomIn увеличивает масштаб.
func (p *ProgramPanel) ZoomIn() {
	newScale := p.scale * 1.2
	if newScale > MaxScale {
		newScale = MaxScale
	}
	p.ApplyScale(newScale)
}

// ZoomOut уменьшает масштаб.
func (p *ProgramPanel) ZoomOut() {
	newScale := p.scale / 1.2
	if newScale < MinScale {
		newScale = MinScale
	}
	p.ApplyScale(newScale)
}

// ResetZoom сбрасывает масштаб к 100%.
func (p *ProgramPanel) ResetZoom() {
	p.ApplyScale(1.0)
}

// GetScale возвращает текущий масштаб.
func (p *ProgramPanel) GetScale() float32 {
	return p.scale
}

// IsInsertMode возвращает состояние режима вставки.
func (p *ProgramPanel) IsInsertMode() bool {
	return p.isInsertMode
}

// scrollToBlock прокручивает панель к указанному блоку.
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

// StartMergePointSelection переводит панель в режим выбора точки слияния для указанного условия.
func (p *ProgramPanel) StartMergePointSelection(conditionID int) {
	// Убедимся, что все виджеты имеют актуальные позиции
	p.content.Layout.Layout(p.content.Objects, p.content.Size())

	p.isMergeMode = true
	p.mergeConditionID = conditionID
	p.valenceManager.ShowMergePoints(conditionID)
	p.valenceOverlay.Refresh()
	log.Printf("Режим выбора точки слияния для условия %d", conditionID)
}

// CancelMergeMode отменяет режим слияния.
func (p *ProgramPanel) CancelMergeMode() {
	p.isMergeMode = false
	p.mergeConditionID = 0
	p.valenceManager.ClearPoints()
	p.valenceOverlay.Refresh()
}

// ensureAlternativeLines создаёт линии для конкретного условия, если их ещё нет
func (p *ProgramPanel) ensureAlternativeLines(conditionID int) {
	// Если линии уже есть для этого условия, ничего не делаем
	if _, exists := p.altConnections[conditionID]; exists {
		return
	}
	condition := p.programMgr.state.FindBlockByID(conditionID)
	if condition == nil || condition.OutBlockID == 0 {
		return
	}
	fromWidget := p.blockWidgets[condition.ID]
	toWidget := p.blockWidgets[condition.OutBlockID]
	if fromWidget == nil || toWidget == nil {
		return
	}

	// Создаём три линии
	horLine1 := canvas.NewLine(color.NRGBA{R: 0, G: 150, B: 255, A: 255}) // синий
	vertLine := canvas.NewLine(color.NRGBA{R: 0, G: 150, B: 255, A: 255})
	horLine2 := canvas.NewLine(color.NRGBA{R: 0, G: 150, B: 255, A: 255})

	lines := []*ConnectionLine{
		{line: horLine1},
		{line: vertLine},
		{line: horLine2},
	}

	// Сохраняем в карту по ID условия
	p.altConnections[conditionID] = lines

	// Добавляем в контейнер
	for _, conn := range lines {
		p.content.Add(conn.line)
	}

	// Сразу обновим позиции
	p.updateAlternativeLines(condition)
}

// updateAlternativeLines обновляет позиции линий для указанного условия
func (p *ProgramPanel) updateAlternativeLines(condition *ProgramBlock) {
	lines, ok := p.altConnections[condition.ID]
	if !ok || len(lines) < 3 {
		return
	}
	fromWidget := p.blockWidgets[condition.ID]
	toWidget := p.blockWidgets[condition.OutBlockID]
	if fromWidget == nil || toWidget == nil {
		return
	}

	scale := p.scale
	fromPos := fromWidget.Position()
	fromSize := fromWidget.Size()
	toPos := toWidget.Position()
	toSize := toWidget.Size()

	offsetX := fromSize.Width * 1.5

	topX := fromPos.X + fromSize.Width + offsetX
	topY := fromPos.Y + fromSize.Height/2
	bottomX := toPos.X + toSize.Width + offsetX
	bottomY := toPos.Y + toSize.Height/2

	// Горизонталь от условия к вертикали
	lines[0].line.Position1 = fyne.NewPos(fromPos.X+fromSize.Width, fromPos.Y+fromSize.Height/2)
	lines[0].line.Position2 = fyne.NewPos(topX, topY)

	// Вертикальная линия
	lines[1].line.Position1 = fyne.NewPos(topX, topY)
	lines[1].line.Position2 = fyne.NewPos(bottomX, bottomY-fromSize.Height/1.5)

	// Горизонталь от вертикали к точке слияния
	lines[2].line.Position1 = fyne.NewPos(bottomX, bottomY-fromSize.Height/1.5)
	lines[2].line.Position2 = fyne.NewPos(bottomX-fromSize.Width*2, bottomY-fromSize.Height/1.5) //toWidget.GetTopConnectorPosition()

	for _, conn := range lines {
		conn.line.StrokeWidth = 2 * scale
		conn.line.Refresh()
	}
}

// rebuildAlternativeLines создаёт линии для всех блоков условия с OutBlockID
func (p *ProgramPanel) rebuildAlternativeLines() {
	prog := p.programMgr.state.GetProgram()
	for _, block := range prog.Blocks {
		if block.Type == BlockTypeCondition && block.OutBlockID != 0 && block.OutBlockID != block.NextBlockID {
			p.ensureAlternativeLines(block.ID)
		}
	}
}
