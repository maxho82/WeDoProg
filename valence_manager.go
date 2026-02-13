package main

import (
	"image/color"
	"log"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

	// Получаем программу через state
	program := vm.programPanel.programMgr.state.GetProgram()

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
	// Получаем программу через state
	program := vm.programPanel.programMgr.state.GetProgram()

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

// addPointWidget добавляет виджет для валентной точки с учетом масштаба
func (vm *ValenceManager) addPointWidget(point *ValencePoint) {
	scale := vm.programPanel.GetScale()

	pointWidget := NewValencePointWidget(point, vm)

	// Размер точки должен масштабироваться
	pointSize := float32(ValencePointBaseSize) * scale
	pointWidget.Resize(fyne.NewSize(pointSize, pointSize))

	// Позиция должна учитывать смещение для центрирования
	offset := pointSize / 2
	pointWidget.Move(fyne.NewPos(
		point.Position.X-offset,
		point.Position.Y-offset,
	))

	vm.container.Add(pointWidget)
	vm.pointWidgets = append(vm.pointWidgets, pointWidget)

	// Обновляем отображение точки с учетом масштаба
	pointWidget.Refresh()
}

// ShowTempConnection показывает временную связь для предварительного просмотра
func (vm *ValenceManager) ShowTempConnection(point *ValencePoint) {
	vm.ClearTempLines()

	if point == nil {
		return
	}

	// Создаем временные линии для предварительного просмотра
	vm.createTempLinesForPoint(point)

	vm.hoveredPoint = point
	vm.refreshDisplay()
}

// createTempLinesForPoint создает временные линии для точки
func (vm *ValenceManager) createTempLinesForPoint(point *ValencePoint) {
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

// createDashedLine создает эффект пунктирной линии с учетом масштаба
func (vm *ValenceManager) createDashedLine(baseLine *canvas.Line) {
	scale := vm.programPanel.GetScale()

	// Разбиваем линию на сегменты для эффекта пунктира
	dx := baseLine.Position2.X - baseLine.Position1.X
	dy := baseLine.Position2.Y - baseLine.Position1.Y
	length := float32(dx*dx + dy*dy)

	if length == 0 {
		return
	}

	// Длина пунктира должна масштабироваться
	dashLength := float32(10) * scale
	gapLength := float32(5) * scale

	// Нормализуем вектор направления
	normalizedLength := float32(math.Sqrt(float64(length)))
	if normalizedLength == 0 {
		return
	}

	dxNormalized := dx / normalizedLength
	dyNormalized := dy / normalizedLength

	// Создаем сегменты пунктира
	currentPos := fyne.Position{X: baseLine.Position1.X, Y: baseLine.Position1.Y}
	totalLength := normalizedLength

	for currentLength := float32(0); currentLength < totalLength; {
		// Начало пунктира
		startX := currentPos.X
		startY := currentPos.Y

		// Конец пунктира (не более общей длины)
		dashProgress := minFloat32(dashLength, totalLength-currentLength)
		endX := startX + dxNormalized*dashProgress
		endY := startY + dyNormalized*dashProgress

		// Создаем сегмент пунктира
		segment := canvas.NewLine(color.NRGBA{R: 255, G: 255, B: 0, A: 180})
		segment.Position1 = fyne.NewPos(startX, startY)
		segment.Position2 = fyne.NewPos(endX, endY)
		segment.StrokeWidth = 2 * scale
		segment.StrokeColor = color.NRGBA{R: 255, G: 255, B: 0, A: 180}

		vm.container.Add(segment)
		vm.tempLines = append(vm.tempLines, segment)

		// Перемещаем текущую позицию
		currentPos = fyne.Position{X: endX, Y: endY}
		currentLength += dashProgress

		// Добавляем промежуток
		gapProgress := minFloat32(gapLength, totalLength-currentLength)
		if gapProgress > 0 {
			currentPos.X += dxNormalized * gapProgress
			currentPos.Y += dyNormalized * gapProgress
			currentLength += gapProgress
		}
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

// InsertBlockAtPoint вставляет блок (или пару) в указанной валентной точке.
func (vm *ValenceManager) InsertBlockAtPoint(point *ValencePoint) bool {
	if point == nil {
		return false
	}

	log.Printf("Вставка блока типа %v в точку %d", vm.programPanel.insertBlockType, point.ID)

	// Определяем afterBlockID на основе типа точки
	var afterBlockID int
	switch point.InsertType {
	case ValenceInsertStart:
		afterBlockID = 0
	case ValenceInsertBetween, ValenceInsertLoop:
		if point.FromBlock != nil {
			afterBlockID = point.FromBlock.ID
		} else {
			afterBlockID = -1 // fallback
		}
	case ValenceInsertEnd:
		afterBlockID = -1
	}

	// Если есть пара для вставки, передаём её в панель вместе с afterBlockID
	if vm.programPanel.insertPairFirst != nil && vm.programPanel.insertPairSecond != nil {
		vm.programPanel.AddBlockAtPosition(vm.programPanel.insertPairFirst, afterBlockID)
	} else {
		// Обычный одиночный блок
		newBlock := vm.programPanel.programMgr.CreateBlock(vm.programPanel.insertBlockType, 0, 0)
		vm.programPanel.AddBlockAtPosition(newBlock, afterBlockID)
	}

	return true
}

// refreshDisplay обновляет отображение
func (vm *ValenceManager) refreshDisplay() {
	vm.container.Refresh()
}

// UpdatePointsPositions обновляет позиции всех точек при изменении масштаба
func (vm *ValenceManager) UpdatePointsPositions() {
	if !vm.programPanel.isInsertMode {
		return
	}

	scale := vm.programPanel.GetScale()

	// Пересчитываем позиции для каждой точки
	for i, point := range vm.points {
		// Пересчитываем позицию в зависимости от типа точки
		switch point.InsertType {
		case ValenceInsertBetween:
			if point.FromBlock != nil && point.ToBlock != nil {
				fromWidget := vm.programPanel.GetBlockWidget(point.FromBlock.ID)
				toWidget := vm.programPanel.GetBlockWidget(point.ToBlock.ID)

				if fromWidget != nil && toWidget != nil {
					pos1 := fromWidget.GetBottomConnectorPosition()
					pos2 := toWidget.GetTopConnectorPosition()

					// Пересчитываем с учетом масштаба
					point.Position = fyne.NewPos(
						(pos1.X+pos2.X)/2,
						(pos1.Y+pos2.Y)/2,
					)
				}
			}

		case ValenceInsertStart:
			if point.ToBlock != nil {
				toWidget := vm.programPanel.GetBlockWidget(point.ToBlock.ID)
				if toWidget != nil {
					point.Position = toWidget.GetTopConnectorPosition()
				}
			}

		case ValenceInsertEnd:
			if point.FromBlock != nil {
				fromWidget := vm.programPanel.GetBlockWidget(point.FromBlock.ID)
				if fromWidget != nil {
					point.Position = fromWidget.GetBottomConnectorPosition()
				}
			}

		case ValenceInsertLoop:
			if point.FromBlock != nil && point.ToBlock != nil {
				startWidget := vm.programPanel.GetBlockWidget(point.FromBlock.ID)
				endWidget := vm.programPanel.GetBlockWidget(point.ToBlock.ID)

				if startWidget != nil && endWidget != nil {
					startPos := startWidget.Position()
					endPos := endWidget.Position()
					point.Position = fyne.NewPos(
						startPos.X+startWidget.Size().Width/2,
						(startPos.Y+endPos.Y)/2,
					)
				}
			}
		}

		// Обновляем позицию виджета точки
		if i < len(vm.pointWidgets) {
			pointSize := float32(ValencePointBaseSize) * scale
			offset := pointSize / 2
			vm.pointWidgets[i].Move(fyne.NewPos(
				point.Position.X-offset,
				point.Position.Y-offset,
			))
			vm.pointWidgets[i].Resize(fyne.NewSize(pointSize, pointSize))
			vm.pointWidgets[i].Refresh()
		}
	}

	vm.refreshDisplay()
}

// minFloat32 возвращает минимальное из двух float32
func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
