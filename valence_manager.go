package main

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	//"fyne.io/fyne/v2/widget"
)

// ValenceManager управляет валентными точками для вставки блоков
type ValenceManager struct {
	programPanel *ProgramPanel
	points       []*ValencePoint
	pointWidgets []*ValencePointWidget
	container    *fyne.Container
	tempLines    []*canvas.Line // Временные линии для предварительного просмотра
	hoveredPoint *ValencePoint  // Точка, на которую наведен курсор
	selectedType BlockType      // Выбранный тип блока для вставки
}

// ValencePoint представляет точку для вставки нового блока
type ValencePoint struct {
	ID         int
	Position   fyne.Position
	InsertType ValenceInsertType
	FromBlock  *ProgramBlock
	ToBlock    *ProgramBlock
	Connection *ConnectionLine // Связь, в которую вставляем
}

// ValenceInsertType тип вставки
type ValenceInsertType int

const (
	ValenceInsertStart   ValenceInsertType = iota // В начало программы
	ValenceInsertBetween                          // Между двумя блоками
	ValenceInsertEnd                              // В конец программы
	ValenceInsertLoop                             // Внутри цикла
)

// NewValenceManager создает менеджер валентных точек
func NewValenceManager(programPanel *ProgramPanel) *ValenceManager {
	return &ValenceManager{
		programPanel: programPanel,
		points:       make([]*ValencePoint, 0),
		pointWidgets: make([]*ValencePointWidget, 0),
		container:    container.NewWithoutLayout(),
		tempLines:    make([]*canvas.Line, 0),
	}
}

// ShowValencePoints показывает валентные точки для указанного типа блока
func (vm *ValenceManager) ShowValencePoints(blockType BlockType) {
	vm.selectedType = blockType
	vm.ClearPoints()

	program := vm.programPanel.programMgr.program

	if len(program.Blocks) == 0 {
		// Если программа пустая, показываем точку для вставки первого блока
		vm.addStartPoint()
	} else {
		// Показываем точки для вставки в начало, между блоками и в конец
		vm.addPointsForAllPositions()
	}

	vm.refreshDisplay()
}

// addStartPoint добавляет точку для вставки первого блока
func (vm *ValenceManager) addStartPoint() {
	// Вычисляем позицию для первой точки (центр видимой области)
	scrollSize := vm.programPanel.scroll.Size()
	contentSize := vm.programPanel.content.Size()

	var pos fyne.Position
	if contentSize.Width > scrollSize.Width || contentSize.Height > scrollSize.Height {
		// Если контент больше видимой области, помещаем точку в центр видимой области
		offset := vm.programPanel.scroll.Offset
		pos = fyne.NewPos(
			scrollSize.Width/2+offset.X,
			scrollSize.Height/2+offset.Y,
		)
	} else {
		// Иначе в центр контента
		pos = fyne.NewPos(
			contentSize.Width/2,
			contentSize.Height/2,
		)
	}

	point := &ValencePoint{
		ID:         len(vm.points),
		Position:   pos,
		InsertType: ValenceInsertStart,
		FromBlock:  nil,
		ToBlock:    nil,
		Connection: nil,
	}

	vm.points = append(vm.points, point)
	vm.addPointWidget(point)
}

// addPointsForAllPositions добавляет точки для всех возможных позиций вставки
func (vm *ValenceManager) addPointsForAllPositions() {
	program := vm.programPanel.programMgr.program

	// 1. Точка для вставки в начало (перед первым блоком, если он не "Начать")
	if len(program.Blocks) > 0 && program.Blocks[0].Type != BlockTypeStart {
		vm.addPointBeforeBlock(program.Blocks[0], nil)
	}

	// 2. Точки между блоками
	for i := 0; i < len(program.Blocks)-1; i++ {
		currentBlock := program.Blocks[i]
		nextBlock := program.Blocks[i+1]

		// Нельзя вставлять между началом и концом цикла (только внутри цикла)
		if currentBlock.Type == BlockTypeLoopStart && nextBlock.Type == BlockTypeLoopEnd {
			vm.addPointInsideLoop(currentBlock, nextBlock)
		} else {
			vm.addPointBetweenBlocks(currentBlock, nextBlock)
		}
	}

	// 3. Точка для вставки в конец (после последнего блока, если он не "Стоп")
	if len(program.Blocks) > 0 {
		lastBlock := program.Blocks[len(program.Blocks)-1]
		if lastBlock.Type != BlockTypeStop {
			vm.addPointAfterBlock(lastBlock)
		}
	}
}

// addPointBetweenBlocks добавляет точку между двумя блоками
func (vm *ValenceManager) addPointBetweenBlocks(fromBlock, toBlock *ProgramBlock) {
	// Находим виджеты блоков
	fromWidget := vm.programPanel.GetBlockWidget(fromBlock.ID)
	toWidget := vm.programPanel.GetBlockWidget(toBlock.ID)

	if fromWidget == nil || toWidget == nil {
		return
	}

	// Находим соответствующую связь
	var targetConnection *ConnectionLine
	for _, conn := range vm.programPanel.connections {
		if conn.fromBlockID == fromBlock.ID && conn.toBlockID == toBlock.ID {
			targetConnection = conn
			break
		}
	}

	if targetConnection == nil {
		return
	}

	// Вычисляем позицию точки (середина линии)
	pos1 := fromWidget.GetBottomConnectorPosition()
	pos2 := toWidget.GetTopConnectorPosition()
	pos := fyne.NewPos(
		(pos1.X+pos2.X)/2,
		(pos1.Y+pos2.Y)/2,
	)

	point := &ValencePoint{
		ID:         len(vm.points),
		Position:   pos,
		InsertType: ValenceInsertBetween,
		FromBlock:  fromBlock,
		ToBlock:    toBlock,
		Connection: targetConnection,
	}

	vm.points = append(vm.points, point)
	vm.addPointWidget(point)
}

// addPointInsideLoop добавляет точку внутри цикла
func (vm *ValenceManager) addPointInsideLoop(loopStart, loopEnd *ProgramBlock) {
	// Находим виджеты блоков
	startWidget := vm.programPanel.GetBlockWidget(loopStart.ID)
	endWidget := vm.programPanel.GetBlockWidget(loopEnd.ID)

	if startWidget == nil || endWidget == nil {
		return
	}

	// Вычисляем позицию точки (середина между блоками цикла по вертикали)
	startPos := startWidget.Position()
	endPos := endWidget.Position()
	pos := fyne.NewPos(
		startPos.X+startWidget.Size().Width/2,
		(startPos.Y+endPos.Y)/2,
	)

	point := &ValencePoint{
		ID:         len(vm.points),
		Position:   pos,
		InsertType: ValenceInsertLoop,
		FromBlock:  loopStart,
		ToBlock:    loopEnd,
		Connection: nil,
	}

	vm.points = append(vm.points, point)
	vm.addPointWidget(point)
}

// addPointBeforeBlock добавляет точку перед указанным блоком
func (vm *ValenceManager) addPointBeforeBlock(block *ProgramBlock, fromBlock *ProgramBlock) {
	widget := vm.programPanel.GetBlockWidget(block.ID)
	if widget == nil {
		return
	}

	pos := widget.GetTopConnectorPosition()

	point := &ValencePoint{
		ID:         len(vm.points),
		Position:   pos,
		InsertType: ValenceInsertStart,
		FromBlock:  fromBlock,
		ToBlock:    block,
		Connection: nil,
	}

	vm.points = append(vm.points, point)
	vm.addPointWidget(point)
}

// addPointAfterBlock добавляет точку после указанного блока
func (vm *ValenceManager) addPointAfterBlock(block *ProgramBlock) {
	widget := vm.programPanel.GetBlockWidget(block.ID)
	if widget == nil {
		return
	}

	pos := widget.GetBottomConnectorPosition()

	point := &ValencePoint{
		ID:         len(vm.points),
		Position:   pos,
		InsertType: ValenceInsertEnd,
		FromBlock:  block,
		ToBlock:    nil,
		Connection: nil,
	}

	vm.points = append(vm.points, point)
	vm.addPointWidget(point)
}

// addPointWidget добавляет виджет для валентной точки
func (vm *ValenceManager) addPointWidget(point *ValencePoint) {
	pointWidget := NewValencePointWidget(point, vm)
	pointWidget.Resize(fyne.NewSize(16, 16))
	pointWidget.Move(fyne.NewPos(point.Position.X-8, point.Position.Y-8))

	vm.container.Add(pointWidget)
	vm.pointWidgets = append(vm.pointWidgets, pointWidget)
}

// ShowTempConnection показывает временную связь для предварительного просмотра
func (vm *ValenceManager) ShowTempConnection(point *ValencePoint) {
	vm.ClearTempLines()

	if point == nil {
		return
	}

	// Создаем временный блок для визуализации
	tempBlock := vm.createTempBlockForPreview()
	if tempBlock == nil {
		return
	}

	// Рассчитываем позиции для временных связей
	vm.createTempLinesForPoint(point, tempBlock)

	vm.hoveredPoint = point
	vm.refreshDisplay()
}

// createTempBlockForPreview создает временный блок для предварительного просмотра
func (vm *ValenceManager) createTempBlockForPreview() *ProgramBlock {
	// Создаем блок с нулевыми координатами
	block := &ProgramBlock{
		ID:         -1, // Временный ID
		Type:       vm.selectedType,
		Title:      "Новый блок",
		Parameters: make(map[string]interface{}),
		Width:      180,
		Height:     100,
	}

	// Настраиваем блок
	switch vm.selectedType {
	case BlockTypeStart:
		block.Title = "Начать"
	case BlockTypeMotor:
		block.Title = "Мотор"
	case BlockTypeLED:
		block.Title = "Светодиод"
	case BlockTypeWait:
		block.Title = "Ждать"
	case BlockTypeLoopStart:
		block.Title = "ДЛЯ"
	case BlockTypeLoopEnd:
		block.Title = "КЦ"
	case BlockTypeStop:
		block.Title = "Стоп"
		// ... остальные типы
	}

	return block
}

// createTempLinesForPoint создает временные линии для точки
func (vm *ValenceManager) createTempLinesForPoint(point *ValencePoint, tempBlock *ProgramBlock) {
	scale := vm.programPanel.GetScale()

	switch point.InsertType {
	case ValenceInsertStart:
		// Если вставляем в начало, линия от нового блока к первому блоку
		if point.ToBlock != nil {
			toWidget := vm.programPanel.GetBlockWidget(point.ToBlock.ID)
			if toWidget != nil {
				// Линия от нового блока к существующему
				line := canvas.NewLine(color.NRGBA{R: 255, G: 255, B: 0, A: 180}) // Желтый пунктир
				line.Position1 = fyne.NewPos(point.Position.X, point.Position.Y+50*scale)
				line.Position2 = toWidget.GetTopConnectorPosition()
				line.StrokeWidth = 2 * scale
				line.StrokeColor = color.NRGBA{R: 255, G: 255, B: 0, A: 180}

				// Создаем эффект пунктира (рисуем несколько коротких отрезков)
				vm.createDashedLine(line)
			}
		}

	case ValenceInsertBetween:
		// Если вставляем между блоками, две линии: от предыдущего к новому и от нового к следующему
		if point.FromBlock != nil && point.ToBlock != nil {
			fromWidget := vm.programPanel.GetBlockWidget(point.FromBlock.ID)
			toWidget := vm.programPanel.GetBlockWidget(point.ToBlock.ID)

			if fromWidget != nil && toWidget != nil {
				// Линия от предыдущего блока к новому
				line1 := canvas.NewLine(color.NRGBA{R: 255, G: 255, B: 0, A: 180})
				line1.Position1 = fromWidget.GetBottomConnectorPosition()
				line1.Position2 = fyne.NewPos(point.Position.X, point.Position.Y-50*scale)
				line1.StrokeWidth = 2 * scale
				line1.StrokeColor = color.NRGBA{R: 255, G: 255, B: 0, A: 180}
				vm.createDashedLine(line1)

				// Линия от нового блока к следующему
				line2 := canvas.NewLine(color.NRGBA{R: 255, G: 255, B: 0, A: 180})
				line2.Position1 = fyne.NewPos(point.Position.X, point.Position.Y+50*scale)
				line2.Position2 = toWidget.GetTopConnectorPosition()
				line2.StrokeWidth = 2 * scale
				line2.StrokeColor = color.NRGBA{R: 255, G: 255, B: 0, A: 180}
				vm.createDashedLine(line2)
			}
		}

	case ValenceInsertEnd:
		// Если вставляем в конец, линия от последнего блока к новому
		if point.FromBlock != nil {
			fromWidget := vm.programPanel.GetBlockWidget(point.FromBlock.ID)
			if fromWidget != nil {
				line := canvas.NewLine(color.NRGBA{R: 255, G: 255, B: 0, A: 180})
				line.Position1 = fromWidget.GetBottomConnectorPosition()
				line.Position2 = fyne.NewPos(point.Position.X, point.Position.Y-50*scale)
				line.StrokeWidth = 2 * scale
				line.StrokeColor = color.NRGBA{R: 255, G: 255, B: 0, A: 180}
				vm.createDashedLine(line)
			}
		}
	}
}

// createDashedLine создает эффект пунктирной линии
func (vm *ValenceManager) createDashedLine(baseLine *canvas.Line) {
	scale := vm.programPanel.GetScale()

	// Разбиваем линию на сегменты для эффекта пунктира
	dx := baseLine.Position2.X - baseLine.Position1.X
	dy := baseLine.Position2.Y - baseLine.Position1.Y
	length := float32(dx*dx + dy*dy)

	if length == 0 {
		return
	}

	// Создаем 10 сегментов (5 пунктирных линий)
	segmentCount := 10
	for i := 0; i < segmentCount; i += 2 {
		// Вычисляем начальную и конечную точку сегмента
		t1 := float32(i) / float32(segmentCount)
		t2 := float32(i+1) / float32(segmentCount)

		x1 := baseLine.Position1.X + dx*t1
		y1 := baseLine.Position1.Y + dy*t1
		x2 := baseLine.Position1.X + dx*t2
		y2 := baseLine.Position1.Y + dy*t2

		segment := canvas.NewLine(color.NRGBA{R: 255, G: 255, B: 0, A: 180})
		segment.Position1 = fyne.NewPos(x1, y1)
		segment.Position2 = fyne.NewPos(x2, y2)
		segment.StrokeWidth = 2 * scale
		segment.StrokeColor = color.NRGBA{R: 255, G: 255, B: 0, A: 180}

		vm.container.Add(segment)
		vm.tempLines = append(vm.tempLines, segment)
	}
}

// HideTempConnection скрывает временные связи
func (vm *ValenceManager) HideTempConnection() {
	vm.ClearTempLines()
	vm.hoveredPoint = nil
	vm.refreshDisplay()
}

// ClearTempLines очищает временные линии
func (vm *ValenceManager) ClearTempLines() {
	for _, line := range vm.tempLines {
		vm.container.Remove(line)
	}
	vm.tempLines = make([]*canvas.Line, 0)
}

// ClearPoints очищает все валентные точки
func (vm *ValenceManager) ClearPoints() {
	for _, widget := range vm.pointWidgets {
		vm.container.Remove(widget)
	}
	vm.points = make([]*ValencePoint, 0)
	vm.pointWidgets = make([]*ValencePointWidget, 0)
	vm.ClearTempLines()
}

// GetContainer возвращает контейнер с точками
func (vm *ValenceManager) GetContainer() fyne.CanvasObject {
	return vm.container
}

// InsertBlockAtPoint вставляет блок в указанной валентной точке
func (vm *ValenceManager) InsertBlockAtPoint(point *ValencePoint) bool {
	if point == nil {
		return false
	}

	log.Printf("Вставка блока типа %v в точку %d", vm.selectedType, point.ID)

	// Создаем новый блок
	newBlock := vm.programPanel.programMgr.CreateBlock(vm.selectedType, 0, 0)

	// Определяем позицию вставки
	var insertIndex int
	switch point.InsertType {
	case ValenceInsertStart:
		// В начало программы
		insertIndex = 0

	case ValenceInsertBetween:
		// Между двумя блоками
		for i, block := range vm.programPanel.programMgr.program.Blocks {
			if block.ID == point.FromBlock.ID {
				insertIndex = i + 1
				break
			}
		}

	case ValenceInsertEnd:
		// В конец программы
		insertIndex = len(vm.programPanel.programMgr.program.Blocks)

	case ValenceInsertLoop:
		// Внутри цикла
		for i, block := range vm.programPanel.programMgr.program.Blocks {
			if block.ID == point.FromBlock.ID {
				insertIndex = i + 1
				break
			}
		}
	}

	// Добавляем блок через programPanel
	vm.programPanel.AddBlockAtPosition(newBlock, insertIndex)

	return true
}

// refreshDisplay обновляет отображение
func (vm *ValenceManager) refreshDisplay() {
	vm.container.Refresh()
}
