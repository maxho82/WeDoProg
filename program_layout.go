package main

import (
	"fyne.io/fyne/v2"
)

// ProgramLayout кастомный макет для панели программирования.
type ProgramLayout struct {
	programMgr *ProgramManager
	scale      float32
	panel      *ProgramPanel
}

// NewProgramLayout создаёт новый макет.
func NewProgramLayout(programMgr *ProgramManager, panel *ProgramPanel) *ProgramLayout {
	return &ProgramLayout{
		programMgr: programMgr,
		scale:      1.0,
		panel:      panel,
	}
}

// Layout упорядочивает объекты в контейнере.
func (l *ProgramLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if l.programMgr == nil {
		return
	}

	prog := l.programMgr.state.GetProgram()
	if prog == nil {
		return
	}

	currentY := float32(50 * l.scale)

	for _, block := range prog.Blocks {
		// Используем карту виджетов из панели для быстрого доступа
		widget := l.panel.GetBlockWidget(block.ID)
		if widget == nil {
			continue
		}

		// Обновляем позицию блока в модели (опционально, если нужно сохранять)
		block.X = 100
		block.Y = float64(currentY / l.scale)

		scaledWidth := float32(block.Width) * l.scale
		scaledHeight := float32(block.Height) * l.scale
		scaledX := float32(block.X) * l.scale

		widget.Resize(fyne.NewSize(scaledWidth, scaledHeight))
		widget.Move(fyne.NewPos(scaledX, currentY))

		currentY += scaledHeight + float32(40*l.scale)
	}

	// Обновляем соединения после изменения позиций
	l.panel.updateConnections()

	// Если в режиме вставки, обновляем валентные точки
	if l.panel.IsInsertMode() {
		l.panel.valenceManager.ShowValencePoints(l.panel.insertBlockType)
	}
}

// MinSize возвращает минимальный размер контейнера.
func (l *ProgramLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if l.programMgr == nil {
		return fyne.NewSize(2000*l.scale, 2000*l.scale)
	}

	prog := l.programMgr.state.GetProgram()
	if prog == nil || len(prog.Blocks) == 0 {
		return fyne.NewSize(2000*l.scale, 2000*l.scale)
	}

	var totalHeight float32
	var maxWidth float32 = 300 * l.scale

	for _, block := range prog.Blocks {
		blockHeight := float32(block.Height) * l.scale
		blockWidth := float32(block.Width) * l.scale

		totalHeight += blockHeight + float32(40*l.scale)

		if blockWidth > maxWidth {
			maxWidth = blockWidth
		}
	}

	padding := float32(100 * l.scale)
	return fyne.NewSize(maxWidth+padding, totalHeight+padding)
}

// SetScale устанавливает масштаб макета.
func (l *ProgramLayout) SetScale(scale float32) {
	if scale < 0.5 || scale > 3.0 {
		return
	}
	l.scale = scale
}

// GetScale возвращает текущий масштаб.
func (l *ProgramLayout) GetScale() float32 {
	return l.scale
}
