package main

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ValencePointWidget виджет валентной точки
type ValencePointWidget struct {
	widget.BaseWidget
	point     *ValencePoint
	manager   *ValenceManager
	circle    *canvas.Circle
	isHovered bool
}

func NewValencePointWidget(point *ValencePoint, manager *ValenceManager) *ValencePointWidget {
	w := &ValencePointWidget{
		point:     point,
		manager:   manager,
		isHovered: false,
	}
	w.ExtendBaseWidget(w)
	w.circle = canvas.NewCircle(color.NRGBA{R: 255, G: 215, B: 0, A: 255})
	w.circle.StrokeWidth = 1
	w.circle.StrokeColor = color.White
	return w
}

// CreateRenderer — использует константы для размера.
func (w *ValencePointWidget) CreateRenderer() fyne.WidgetRenderer {
	return &valencePointRenderer{widget: w, circle: w.circle}
}

// Tapped обработка клика по точке
func (w *ValencePointWidget) Tapped(e *fyne.PointEvent) {
	log.Printf("Левый клик по валентной точке %d", w.point.ID)

	// Вставляем блок в эту точку
	success := w.manager.InsertBlockAtPoint(w.point)
	if success {
		// Получаем ссылку на GUI для обновления состояния
		gui := w.manager.programPanel.gui

		// Очищаем все точки после успешной вставки
		w.manager.ClearPoints()

		// Отключаем режим вставки в programPanel
		w.manager.programPanel.isInsertMode = false
		w.manager.programPanel.insertBlockType = 0

		// Обновляем GUI в главном потоке
		fyne.Do(func() {
			// Скрываем кнопку отмены в GUI
			if gui != nil && gui.insertCancelButton != nil {
				gui.insertCancelButton.Hide()
			}

			// Обновляем состояние кнопок блоков
			if gui != nil {
				gui.updateBlockButtonsState()
			}

			// Обновляем панель инструментов
			if gui != nil && gui.toolbar != nil {
				gui.updateToolbarState()
			}

			log.Println("Режим вставки отключен после успешной вставки блока")
		})
	}
}

// TappedSecondary обработка правого клика
func (w *ValencePointWidget) TappedSecondary(e *fyne.PointEvent) {
	log.Println("Правый клик по валентной точке - отмена")

	// Получаем ссылку на GUI для обновления состояния
	gui := w.manager.programPanel.gui

	// Очищаем все точки
	w.manager.ClearPoints()

	// Отключаем режим вставки в programPanel
	w.manager.programPanel.isInsertMode = false
	w.manager.programPanel.insertBlockType = 0

	// Обновляем GUI в главном потоке
	fyne.Do(func() {
		// Скрываем кнопку отмены в GUI
		if gui != nil && gui.insertCancelButton != nil {
			gui.insertCancelButton.Hide()
		}

		// Обновляем состояние кнопок блоков
		if gui != nil {
			gui.updateBlockButtonsState()
		}

		// Обновляем панель инструментов
		if gui != nil && gui.toolbar != nil {
			gui.updateToolbarState()
		}

		log.Println("Режим вставки отключен после успешной вставки блока")
	})
}

// MouseIn обработка наведения мыши
func (w *ValencePointWidget) MouseIn(e *desktop.MouseEvent) {
	w.isHovered = true
	w.circle.FillColor = color.NRGBA{R: 255, G: 255, B: 0, A: 255} // Ярко-желтый при наведении
	w.circle.StrokeWidth = 2
	w.circle.StrokeColor = color.White
	w.circle.Refresh()

	// Показываем временные связи для предварительного просмотра
	w.manager.ShowTempConnection(w.point)
}

// MouseOut обработка ухода мыши
func (w *ValencePointWidget) MouseOut() {
	w.isHovered = false
	w.circle.FillColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255} // Возвращаем обычный цвет
	w.circle.StrokeWidth = 1
	w.circle.StrokeColor = color.White
	w.circle.Refresh()

	// Скрываем временные связи
	w.manager.HideTempConnection()
}

// MouseMoved обработка движения мыши
func (w *ValencePointWidget) MouseMoved(e *desktop.MouseEvent) {
	// Можно добавить дополнительные эффекты при движении
}

// Cursor возвращает тип курсора для точки
func (w *ValencePointWidget) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

// valencePointRenderer рендерер для виджета валентной точки
type valencePointRenderer struct {
	widget *ValencePointWidget
	circle *canvas.Circle
}

// Layout упорядочивает элементы с учетом масштаба
func (r *valencePointRenderer) Layout(size fyne.Size) {
	scale := r.widget.manager.programPanel.GetScale()

	// Размер точки должен масштабироваться
	pointSize := float32(16) * scale
	r.widget.Resize(fyne.NewSize(pointSize, pointSize))

	// Круг должен занимать всю доступную область
	r.circle.Resize(fyne.NewSize(pointSize, pointSize))
	r.circle.Move(fyne.NewPos(0, 0))
}

// MinSize возвращает минимальный размер с учетом масштаба
func (r *valencePointRenderer) MinSize() fyne.Size {
	scale := r.widget.manager.programPanel.GetScale()
	pointSize := float32(16) * scale
	return fyne.NewSize(pointSize, pointSize)
}

// Refresh обновляет отображение с учетом масштаба
func (r *valencePointRenderer) Refresh() {
	scale := r.widget.manager.programPanel.GetScale()

	if r.widget.isHovered {
		r.circle.FillColor = color.NRGBA{R: 255, G: 255, B: 0, A: 255}
		r.circle.StrokeWidth = 2 * scale
	} else {
		r.circle.FillColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}
		r.circle.StrokeWidth = 1 * scale
	}

	// Обновляем размер точки
	pointSize := float32(16) * scale
	r.circle.Resize(fyne.NewSize(pointSize, pointSize))
	r.circle.Refresh()
}

// Objects возвращает объекты для отрисовки
func (r *valencePointRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.circle}
}

// Destroy очищает ресурсы
func (r *valencePointRenderer) Destroy() {}
