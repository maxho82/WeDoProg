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
	connections   []*ConnectionLine
	blockWidgets  map[int]*DraggableBlock
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

// AddBlock добавляет блок на холст с учетом выделенного блока
func (p *ProgramPanel) AddBlock(block *ProgramBlock) {
	// Проверяем, не добавлен ли уже блок
	if _, exists := p.blockWidgets[block.ID]; exists {
		log.Printf("Блок %d уже добавлен на холст", block.ID)
		return
	}

	// Проверяем особые случаи для блоков "Начать" и "Стоп"
	if block.Type == BlockTypeStart {
		// Проверяем, есть ли уже блок "Начать"
		for _, b := range p.programMgr.program.Blocks {
			if b.Type == BlockTypeStart {
				log.Println("Блок 'Начать' уже существует в программе")
				return
			}
		}
	} else if block.Type == BlockTypeStop {
		// Проверяем, есть ли уже блок "Стоп"
		for _, b := range p.programMgr.program.Blocks {
			if b.Type == BlockTypeStop {
				log.Println("Блок 'Стоп' уже существует в программе")
				return
			}
		}
	}

	// Определяем индекс вставки в программу
	insertIndex := p.calculateInsertIndex()

	log.Printf("Вставка блока %d на позицию %d (всего блоков: %d)",
		block.ID, insertIndex, len(p.programMgr.program.Blocks))

	// Вставляем блок в программу по правильному индексу
	p.insertBlockToProgram(block, insertIndex)

	// Пересчитываем позиции всех блоков
	p.repositionAllBlocks()

	// Создаем виджет блока
	blockWidget := NewDraggableBlock(block, p.programMgr, p.gui)
	blockWidget.Resize(fyne.NewSize(float32(block.Width), float32(block.Height)))
	blockWidget.Move(fyne.NewPos(float32(block.X), float32(block.Y)))

	// Добавляем на панель
	p.content.Add(blockWidget)
	p.blockWidgets[block.ID] = blockWidget

	// Обновляем ВСЕ связи (после того как виджет создан)
	p.updateAllConnections()

	p.content.Refresh()

	log.Printf("Блок добавлен на холст: %s (ID: %d) на позиции (%.0f, %.0f)",
		block.Title, block.ID, block.X, block.Y)
}

// calculateInsertIndex вычисляет индекс вставки нового блока
func (p *ProgramPanel) calculateInsertIndex() int {
	// Если нет блоков в программе, вставляем в начало
	if len(p.programMgr.program.Blocks) == 0 {
		return 0
	}

	// Если нет выделенного блока
	if p.selectedBlock == nil {
		// Ищем блок "Стоп"
		for i, block := range p.programMgr.program.Blocks {
			if block.Type == BlockTypeStop {
				return i // Вставляем перед блоком "Стоп"
			}
		}
		// Если нет блока "Стоп", вставляем в конец
		return len(p.programMgr.program.Blocks)
	}

	// Определяем логику вставки в зависимости от типа выбранного блока
	switch p.selectedBlock.Type {
	case BlockTypeLoopStart:
		// Если выбран блок начала цикла, вставляем после него (в тело цикла)
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				// Находим конец цикла, если есть
				loopEndID, found := p.programMgr.FindLoopEndID(block.ID)
				if found {
					// Ищем индекс конца цикла
					for j, b := range p.programMgr.program.Blocks {
						if b.ID == loopEndID {
							// Вставляем перед концом цикла
							return j
						}
					}
				}
				// Если конца цикла нет, вставляем после начала
				return i + 1
			}
		}

	case BlockTypeLoopEnd:
		// Если выбран блок конца цикла, вставляем после него (после цикла)
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				return i + 1
			}
		}

	case BlockTypeStop:
		// Если выбран блок "Стоп", вставляем перед ним
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				return i
			}
		}

	default:
		// Для остальных блоков вставляем после выбранного
		for i, block := range p.programMgr.program.Blocks {
			if block.ID == p.selectedBlock.ID {
				return i + 1
			}
		}
	}

	// По умолчанию вставляем в конец
	return len(p.programMgr.program.Blocks)
}

// repositionAllBlocks перепозиционирует все блоки после вставки
func (p *ProgramPanel) repositionAllBlocks() {
	// Располагаем блоки вертикально с отступами
	currentY := 50.0
	indentLevel := 0 // Уровень вложенности для циклов

	for _, block := range p.programMgr.program.Blocks {
		// Вычисляем отступ в зависимости от типа блока
		switch block.Type {
		case BlockTypeLoopStart:
			// Начало цикла - отступ вправо
			block.X = 100 + float64(indentLevel*40)
			block.Y = currentY
			indentLevel++

		case BlockTypeLoopEnd:
			// Конец цикла - отступ обратно влево
			indentLevel = max(0, indentLevel-1)
			block.X = 100 + float64(indentLevel*40)
			block.Y = currentY

		default:
			// Обычные блоки
			block.X = 100 + float64(indentLevel*40)
			block.Y = currentY
		}

		// Обновляем позицию виджета, если он существует
		if widget, exists := p.blockWidgets[block.ID]; exists {
			widget.Move(fyne.NewPos(float32(block.X), float32(block.Y)))
		}

		currentY += block.Height + 40
	}
}

// updateAllConnections обновляет все связи между блоками
func (p *ProgramPanel) updateAllConnections() {
	// Очищаем все существующие визуальные соединения
	for _, conn := range p.connections {
		// Удаляем линию из контейнера
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

		// Устанавливаем связь в блоке
		currentBlock.NextBlockID = nextBlock.ID

		// Добавляем соединение в менеджер
		p.programMgr.program.Connections = append(p.programMgr.program.Connections, &Connection{
			FromBlockID: currentBlock.ID,
			ToBlockID:   nextBlock.ID,
		})

		// Создаем визуальное соединение
		p.createVisualConnection(currentBlock.ID, nextBlock.ID)
	}

	// У последнего блока нет следующего
	if len(p.programMgr.program.Blocks) > 0 {
		lastBlock := p.programMgr.program.Blocks[len(p.programMgr.program.Blocks)-1]
		lastBlock.NextBlockID = 0
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

	// Добавляем линию на панель
	p.content.Add(line)

	// Сохраняем соединение
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

	// Находим блок для удаления
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

	// Проверяем, нельзя ли удалять блоки "Начать" и "Стоп"
	if blockToRemove.Type == BlockTypeStart || blockToRemove.Type == BlockTypeStop {
		log.Printf("Блок '%s' (ID: %d) нельзя удалять", blockToRemove.Title, blockID)
		return
	}

	// Проверяем, является ли блок частью цикла
	if blockToRemove.Type == BlockTypeLoopStart || blockToRemove.Type == BlockTypeLoopEnd {
		// Для блоков цикла вызываем специальную функцию в GUI
		p.gui.deleteLoopWithConfirmation(blockID)
		return
	}

	// Находим индекс удаляемого блока
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
		// Ищем виджет в контейнере и удаляем его
		for i, obj := range p.content.Objects {
			if obj == blockWidget {
				p.content.Objects = append(p.content.Objects[:i], p.content.Objects[i+1:]...)
				break
			}
		}
		// Удаляем из карты виджетов
		delete(p.blockWidgets, blockID)
	}

	// Удаляем связанные соединения
	p.removeConnectionsForBlock(blockID)

	// Пересчитываем позиции оставшихся блоков
	p.repositionAllBlocks()

	// Обновляем все связи
	p.updateAllConnections()

	// Если удалили выбранный блок, сбрасываем выделение
	if p.selectedBlock != nil && p.selectedBlock.ID == blockID {
		p.selectedBlock = nil
		p.gui.selectedBlock = nil
		p.ResetHighlight()
	}

	p.content.Refresh()

	log.Printf("Блок %d удален с холста. Осталось блоков: %d", blockID, len(p.programMgr.program.Blocks))
}

// RemoveBlockInternal - внутренний метод для удаления блока без проверок (используется при удалении циклов)
func (p *ProgramPanel) RemoveBlockInternal(blockID int) {
	log.Printf("Внутреннее удаление блока %d", blockID)

	// Находим блок для удаления
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
		// Ищем виджет в контейнере и удаляем его
		for i, obj := range p.content.Objects {
			if obj == blockWidget {
				p.content.Objects = append(p.content.Objects[:i], p.content.Objects[i+1:]...)
				break
			}
		}
		// Удаляем из карты виджетов
		delete(p.blockWidgets, blockID)
	}

	// Удаляем связанные соединения
	p.removeConnectionsForBlock(blockID)

	// Если удалили выбранный блок, сбрасываем выделение
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

// Clear очищает холст
func (p *ProgramPanel) Clear() {
	// Оставляем только фон и сетку
	var newObjects []fyne.CanvasObject
	newObjects = append(newObjects, p.content.Objects[0]) // Фон
	newObjects = append(newObjects, p.content.Objects[1]) // Сетка

	p.content.Objects = newObjects
	p.connections = make([]*ConnectionLine, 0)
	p.blockWidgets = make(map[int]*DraggableBlock)
	p.selectedBlock = nil
	p.content.Refresh()
}

// HighlightConnections выделяет соединение, в которое будет вставлен новый блок
func (p *ProgramPanel) HighlightConnections(block *ProgramBlock) {
	// Сбрасываем выделение всех линий
	for _, conn := range p.connections {
		conn.isHighlighted = false
		conn.line.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255} // Синий
		conn.line.StrokeWidth = 2
	}

	if block == nil {
		p.content.Refresh()
		return
	}

	// В дракон-схеме подсвечиваем связь, которая идет ОТ выбранного блока (кроме блока "Стоп" и конца цикла)
	if block.Type != BlockTypeStop && block.Type != BlockTypeLoopEnd {
		for _, conn := range p.connections {
			if conn.fromBlockID == block.ID {
				conn.isHighlighted = true
				conn.line.StrokeColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255} // Золотой
				conn.line.StrokeWidth = 3
				break // только одну связь
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
	// Сбрасываем выделение со всех блоков
	for _, widget := range p.blockWidgets {
		widget.SetSelected(false)
	}

	p.selectedBlock = block
	if block != nil {
		// Выделяем выбранный блок
		if widget, exists := p.blockWidgets[block.ID]; exists {
			widget.SetSelected(true)
		}
		// Подсвечиваем соответствующую связь
		p.HighlightConnections(block)
	} else {
		p.ResetHighlight()
	}
}

// updateConnections обновляет позиции всех соединений
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

// insertBlockToProgram вставляет блок в программу по указанному индексу
func (p *ProgramPanel) insertBlockToProgram(block *ProgramBlock, index int) {
	// Проверяем корректность индекса
	if index < 0 {
		index = 0
	}
	if index > len(p.programMgr.program.Blocks) {
		index = len(p.programMgr.program.Blocks)
	}

	// Вставляем блок в срез
	if index == len(p.programMgr.program.Blocks) {
		p.programMgr.program.Blocks = append(p.programMgr.program.Blocks, block)
	} else {
		p.programMgr.program.Blocks = append(p.programMgr.program.Blocks[:index],
			append([]*ProgramBlock{block}, p.programMgr.program.Blocks[index:]...)...)
	}

	log.Printf("Блок %d вставлен в программу на позицию %d", block.ID, index)
}

// Метод для выделения блока выполнения
func (p *ProgramPanel) HighlightExecutingBlock(blockID int) {
	if blockID == -1 {
		// Сбрасываем выделение
		for _, widget := range p.blockWidgets {
			widget.SetExecuting(false)
		}
		p.content.Refresh()
		return
	}

	// Находим блок
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
	// Сбрасываем предыдущее выделение выполнения
	for _, widget := range p.blockWidgets {
		widget.SetExecuting(false)
	}

	// Устанавливаем выделение выполнения для текущего блока
	if widget, exists := p.blockWidgets[block.ID]; exists {
		widget.SetExecuting(true)
	}

	// Также подсвечиваем соответствующую связь
	p.HighlightConnections(block)
	p.content.Refresh()
}

// Удаляем все блоки цикла (включая начало и конец)
func (p *ProgramPanel) RemoveLoopBlocks(loopBlocks []*ProgramBlock) {
	log.Printf("Удаление всех блоков цикла (количество: %d)", len(loopBlocks))

	// Удаляем блоки начиная с конца (чтобы не нарушать индексы)
	for i := len(loopBlocks) - 1; i >= 0; i-- {
		block := loopBlocks[i]
		p.RemoveBlockInternal(block.ID)
	}

	// Пересчитываем позиции оставшихся блоков
	p.repositionAllBlocks()

	// Обновляем все связи
	p.updateAllConnections()

	p.content.Refresh()
}
