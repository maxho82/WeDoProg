package main

import (
	"fyne.io/fyne/v2"
)

// ProgramLayout кастомный макет для панели программирования
type ProgramLayout struct {
	programMgr *ProgramManager
	scale      float32
	panel      *ProgramPanel
}

// NewProgramLayout создает новый макет для панели программирования
func NewProgramLayout(programMgr *ProgramManager, panel *ProgramPanel) *ProgramLayout {
	return &ProgramLayout{
		programMgr: programMgr,
		scale:      1.0,
		panel:      panel,
	}
}

// Layout упорядочивает объекты в контейнере
func (l *ProgramLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if l.programMgr == nil {
		return
	}

	// Располагаем блоки вертикально с отступами
	currentY := float32(50 * l.scale)

	for _, block := range l.programMgr.program.Blocks {
		// Ищем виджет для текущего блока
		for _, obj := range objects {
			if widget, ok := obj.(*DraggableBlock); ok && widget.block.ID == block.ID {
				// Обновляем позицию блока в модели
				block.X = 100
				block.Y = float64(currentY / l.scale)

				// Вычисляем размер с учетом масштаба
				scaledWidth := float32(block.Width) * l.scale
				scaledHeight := float32(block.Height) * l.scale
				scaledX := float32(block.X) * l.scale

				// Устанавливаем позицию и размер виджета
				widget.Resize(fyne.NewSize(scaledWidth, scaledHeight))
				widget.Move(fyne.NewPos(scaledX, currentY))

				currentY += scaledHeight + float32(40*l.scale)
				break
			}
		}
	}

	// Обновляем соединения после изменения позиций
	if l.panel != nil {
		l.panel.updateConnections()

		// Если в режиме вставки, обновляем валентные точки
		if l.panel.IsInsertMode() {
			l.panel.valenceManager.ShowValencePoints(l.panel.insertBlockType)
		}
	}
}

// MinSize возвращает минимальный размер контейнера
func (l *ProgramLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if l.programMgr == nil || len(l.programMgr.program.Blocks) == 0 {
		// Возвращаем базовый размер, если нет блоков
		return fyne.NewSize(2000*l.scale, 2000*l.scale)
	}

	// Рассчитываем необходимый размер на основе всех блоков
	var totalHeight float32
	var maxWidth float32 = 300 * l.scale // Минимальная ширина

	for _, block := range l.programMgr.program.Blocks {
		blockHeight := float32(block.Height) * l.scale
		blockWidth := float32(block.Width) * l.scale

		totalHeight += blockHeight + float32(40*l.scale)

		if blockWidth > maxWidth {
			maxWidth = blockWidth
		}
	}

	// Добавляем отступы сверху и снизу
	padding := float32(100 * l.scale)
	return fyne.NewSize(maxWidth+padding, totalHeight+padding)
}

// SetScale устанавливает масштаб макета
func (l *ProgramLayout) SetScale(scale float32) {
	if scale < 0.5 || scale > 3.0 {
		return
	}
	l.scale = scale
}

// GetScale возвращает текущий масштаб
func (l *ProgramLayout) GetScale() float32 {
	return l.scale
}
