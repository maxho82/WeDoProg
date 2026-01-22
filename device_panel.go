package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// DevicePanel панель для отображения устройств
type DevicePanel struct {
	gui    *MainGUI
	scroll *container.Scroll
}

// NewDevicePanel создает новую панель устройств
func NewDevicePanel(gui *MainGUI) *DevicePanel {
	panel := &DevicePanel{
		gui: gui,
	}

	panel.scroll = container.NewVScroll(panel.buildUI())
	panel.scroll.SetMinSize(fyne.NewSize(300, 400))

	return panel
}

// GetContainer возвращает контейнер панели
func (p *DevicePanel) GetContainer() fyne.CanvasObject {
	return p.scroll
}

// buildUI строит интерфейс панели
func (p *DevicePanel) buildUI() *fyne.Container {
	mainContainer := container.NewVBox()

	// Заголовок
	title := canvas.NewText("Подключенные устройства", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter

	mainContainer.Add(title)
	mainContainer.Add(widget.NewSeparator())

	// Контейнер для устройств
	devicesContainer := container.NewVBox()

	// Если нет подключения, показываем сообщение
	if !p.gui.hubMgr.IsConnected() {
		noConnectionLabel := widget.NewLabel("Не подключено к хабу")
		noConnectionLabel.Alignment = fyne.TextAlignCenter
		noConnectionLabel.TextStyle.Italic = true
		devicesContainer.Add(noConnectionLabel)
	} else {
		// Показываем устройства
		connectedDevices := p.gui.deviceMgr.GetConnectedDevices()

		if len(connectedDevices) == 0 {
			noDevicesLabel := widget.NewLabel("Нет подключенных устройств")
			noDevicesLabel.Alignment = fyne.TextAlignCenter
			noDevicesLabel.TextStyle.Italic = true
			devicesContainer.Add(noDevicesLabel)
		} else {
			for _, device := range connectedDevices {
				deviceCard := p.createDeviceCard(device)
				devicesContainer.Add(deviceCard)
			}
		}
	}

	mainContainer.Add(devicesContainer)

	return container.NewPadded(mainContainer)
}

// createDeviceCard создает карточку устройства
func (p *DevicePanel) createDeviceCard(device *Device) fyne.CanvasObject {
	// Основной контейнер карточки
	card := container.NewVBox()

	// Заголовок карточки
	header := container.NewHBox()

	// Иконка устройства
	icon := p.getDeviceIcon(device.DeviceType)

	// Название и порт
	nameLabel := widget.NewLabel(fmt.Sprintf("%s (Порт %d)", device.Name, device.PortID))
	nameLabel.TextStyle.Bold = true

	// Статус
	statusLabel := widget.NewLabel("✓")
	statusLabel.TextStyle.Bold = true

	header.Add(icon)
	header.Add(nameLabel)
	header.Add(layout.NewSpacer())
	header.Add(statusLabel)

	card.Add(header)
	card.Add(widget.NewSeparator())

	// Информация об устройстве
	if device.LastValue != nil {
		valueLabel := widget.NewLabel(fmt.Sprintf("Значение: %v", device.LastValue))
		card.Add(valueLabel)
	}

	if !device.LastUpdate.IsZero() {
		updateLabel := widget.NewLabel(fmt.Sprintf("Обновлено: %s",
			device.LastUpdate.Format("15:04:05")))
		updateLabel.TextStyle.Italic = true
		card.Add(updateLabel)
	}

	// Кнопки управления
	if p.isDeviceControllable(device.DeviceType) {
		card.Add(widget.NewSeparator())
		card.Add(p.createDeviceControls(device))
	}

	// Фон карточки
	cardContainer := container.NewStack(
		&canvas.Rectangle{
			FillColor:   color.NRGBA{R: 60, G: 60, B: 60, A: 255},
			StrokeColor: color.NRGBA{R: 100, G: 100, B: 100, A: 255},
			StrokeWidth: 1,
		},
		container.NewPadded(card),
	)

	return cardContainer
}

// getDeviceIcon возвращает иконку для типа устройства
func (p *DevicePanel) getDeviceIcon(deviceType byte) *widget.Icon {
	// В реальном приложении здесь должны быть кастомные иконки
	// Пока используем стандартные
	return widget.NewIcon(nil)
}

// isDeviceControllable проверяет, можно ли управлять устройством
func (p *DevicePanel) isDeviceControllable(deviceType byte) bool {
	switch deviceType {
	case DEVICE_TYPE_MOTOR, DEVICE_TYPE_RGB_LIGHT, DEVICE_TYPE_PIEZO_TONE:
		return true
	default:
		return false
	}
}

// createDeviceControls создает элементы управления для устройства
func (p *DevicePanel) createDeviceControls(device *Device) fyne.CanvasObject {
	switch device.DeviceType {
	case DEVICE_TYPE_MOTOR:
		return p.createMotorControls(device.PortID)
	case DEVICE_TYPE_RGB_LIGHT:
		return p.createLEDControls(device.PortID)
	case DEVICE_TYPE_PIEZO_TONE:
		return p.createPiezoControls(device.PortID)
	default:
		return widget.NewLabel("Управление не поддерживается")
	}
}

// createMotorControls создает элементы управления для мотора
func (p *DevicePanel) createMotorControls(portID byte) fyne.CanvasObject {
	controls := container.NewVBox()

	powerLabel := widget.NewLabel("Мощность:")
	powerSlider := widget.NewSlider(-100, 100)
	powerSlider.Value = 0
	powerValueLabel := widget.NewLabel("0%")

	powerSlider.OnChanged = func(value float64) {
		powerValueLabel.SetText(fmt.Sprintf("%.0f%%", value))
	}

	// Кнопки управления
	buttonContainer := container.NewGridWithColumns(3)

	forwardBtn := widget.NewButton("▶", func() {
		power := powerSlider.Value
		p.gui.deviceMgr.SetMotorPower(portID, int8(power), 1000)
	})

	stopBtn := widget.NewButton("⏹", func() {
		p.gui.deviceMgr.SetMotorPower(portID, 0, 0)
	})

	backwardBtn := widget.NewButton("◀", func() {
		power := -powerSlider.Value
		p.gui.deviceMgr.SetMotorPower(portID, int8(power), 1000)
	})

	buttonContainer.Add(forwardBtn)
	buttonContainer.Add(stopBtn)
	buttonContainer.Add(backwardBtn)

	controls.Add(powerLabel)
	controls.Add(container.NewHBox(powerSlider, powerValueLabel))
	controls.Add(buttonContainer)

	return controls
}

// createLEDControls создает элементы управления для светодиода
func (p *DevicePanel) createLEDControls(portID byte) fyne.CanvasObject {
	controls := container.NewVBox()

	colorLabel := widget.NewLabel("Цвет светодиода:")

	// Простые цвета
	colorsContainer := container.NewGridWithColumns(4)

	colors := []struct {
		name    string
		r, g, b byte
	}{
		{"Красный", 255, 0, 0},
		{"Зеленый", 0, 255, 0},
		{"Синий", 0, 0, 255},
		{"Выкл", 0, 0, 0},
	}

	for _, color := range colors {
		btn := widget.NewButton("", func(r, g, b byte) func() {
			return func() {
				p.gui.deviceMgr.SetLEDColor(portID, r, g, b)
			}
		}(color.r, color.g, color.b))

		btn.Importance = widget.LowImportance
		colorsContainer.Add(btn)
	}

	controls.Add(colorLabel)
	controls.Add(colorsContainer)

	return controls
}

// createPiezoControls создает элементы управления для пищалки
func (p *DevicePanel) createPiezoControls(portID byte) fyne.CanvasObject {
	controls := container.NewVBox()

	freqLabel := widget.NewLabel("Частота (Гц):")
	freqEntry := widget.NewEntry()
	freqEntry.SetText("440")

	durationLabel := widget.NewLabel("Длительность (мс):")
	durationEntry := widget.NewEntry()
	durationEntry.SetText("500")

	// Кнопки
	buttonContainer := container.NewHBox()

	playBtn := widget.NewButton("▶ Воспроизвести", func() {
		// Реализация в gui/main_gui.go
	})

	stopBtn := widget.NewButton("⏹ Стоп", func() {
		p.gui.deviceMgr.StopTone(portID)
	})

	buttonContainer.Add(playBtn)
	buttonContainer.Add(stopBtn)

	controls.Add(freqLabel)
	controls.Add(freqEntry)
	controls.Add(durationLabel)
	controls.Add(durationEntry)
	controls.Add(buttonContainer)

	return controls
}

// Update обновляет отображение панели
func (p *DevicePanel) Update() {
	p.scroll.Content = p.buildUI()
	p.scroll.Refresh()
}
