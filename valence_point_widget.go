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
	point       *ValencePoint
	manager     *ValenceManager
	circle      *canvas.Circle
	isHovered   bool
}

// NewValencePointWidget создает виджет валентной точки
func NewValencePointWidget(point *ValencePoint, manager *ValenceManager) *ValencePointWidget {
	w := &ValencePointWidget{
		point:     point,
		manager:   manager,
		isHovered: false,
	}
	
	w.ExtendBaseWidget(w)
	
	// Создаем круг для отображения точки
	w.circle = canvas.NewCircle(color.NRGBA{R: 255, G: 215, B: 0, A: 255}) // Золотой
	w.circle.StrokeWidth = 1
	w.circle.StrokeColor = color.White
	
	return w
}

// CreateRenderer создает рендерер для виджета
func (w *ValencePointWidget) CreateRenderer() fyne.WidgetRenderer {
	return &valencePointRenderer{
		widget: w,
		circle: w.circle,
	}
}

// Tapped обработка клика по точке
func (w *ValencePointWidget) Tapped(e *fyne.PointEvent) {
	log.Printf("Левый клик по валентной точке %d", w.point.ID)
	
	// Вставляем блок в эту точку
	success := w.manager.InsertBlockAtPoint(w.point)
	if success {
		// Очищаем все точки после успешной вставки
		w.manager.ClearPoints()
	}
}

// TappedSecondary обработка правого клика
func (w *ValencePointWidget) TappedSecondary(e *fyne.PointEvent) {
	log.Println("Правый клик по валентной точке - отмена")
	
	// Очищаем все точки
	w.manager.ClearPoints()
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

// Layout упорядочивает элементы
func (r *valencePointRenderer) Layout(size fyne.Size) {
	// Круг занимает всю доступную область
	diameter := float32(16) * r.widget.manager.programPanel.GetScale()
	r.widget.Resize(fyne.NewSize(diameter, diameter))
	r.circle.Resize(fyne.NewSize(diameter, diameter))
	r.circle.Move(fyne.NewPos(0, 0))
}

// MinSize возвращает минимальный размер
func (r *valencePointRenderer) MinSize() fyne.Size {
	diameter := float32(16) * r.widget.manager.programPanel.GetScale()
	return fyne.NewSize(diameter, diameter)
}

// Refresh обновляет отображение
func (r *valencePointRenderer) Refresh() {
	if r.widget.isHovered {
		r.circle.FillColor = color.NRGBA{R: 255, G: 255, B: 0, A: 255}
		r.circle.StrokeWidth = 2
	} else {
		r.circle.FillColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}
		r.circle.StrokeWidth = 1
	}
	r.circle.Refresh()
}

// Objects возвращает объекты для отрисовки
func (r *valencePointRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.circle}
}

// Destroy очищает ресурсы
func (r *valencePointRenderer) Destroy() {}