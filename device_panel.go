package main

import (
	"fmt"
	"image/color"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout" // добавлен недостающий импорт
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// DevicePanel отвечает за отображение информации о хабе и подключённых устройствах.
type DevicePanel struct {
	gui              *MainGUI
	state            *AppState
	container        *fyne.Container
	batteryProgress  *widget.ProgressBar
	hubInfoContainer *fyne.Container
	deviceList       *widget.List
}

// NewDevicePanel создаёт новую панель устройств.
func NewDevicePanel(gui *MainGUI, state *AppState) *DevicePanel {
	panel := &DevicePanel{
		gui:   gui,
		state: state,
	}
	panel.buildUI()
	return panel
}

// GetContainer возвращает корневой контейнер панели.
func (dp *DevicePanel) GetContainer() fyne.CanvasObject {
	return dp.container
}

// buildUI строит интерфейс панели.
func (dp *DevicePanel) buildUI() {
	mainContainer := container.NewVBox()

	// Заголовок
	title := canvas.NewText("Информация о хабе", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	title.TextSize = 16
	title.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(title))
	mainContainer.Add(widget.NewSeparator())

	// Батарея
	dp.batteryProgress = widget.NewProgressBar()
	dp.batteryProgress.Min = 0
	dp.batteryProgress.Max = 1
	dp.batteryProgress.SetValue(0)
	dp.batteryProgress.TextFormatter = func() string {
		if dp.batteryProgress.Value <= 0 {
			return "--%"
		}
		return fmt.Sprintf("%.0f%%", dp.batteryProgress.Value*100)
	}
	mainContainer.Add(container.NewVBox(
		canvas.NewText("Батарея", color.NRGBA{R: 240, G: 240, B: 240, A: 255}),
		dp.batteryProgress,
	))
	mainContainer.Add(widget.NewSeparator())

	// Информация о хабе
	hubTitle := canvas.NewText("Хаб", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	hubTitle.TextSize = 14
	hubTitle.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(hubTitle))

	dp.hubInfoContainer = container.NewVBox()
	mainContainer.Add(dp.hubInfoContainer)
	mainContainer.Add(widget.NewSeparator())

	// Список устройств
	devicesTitle := canvas.NewText("Подключенные устройства", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	devicesTitle.TextSize = 14
	devicesTitle.TextStyle.Bold = true
	mainContainer.Add(container.NewCenter(devicesTitle))

	dp.deviceList = dp.createDeviceList()
	mainContainer.Add(dp.deviceList)

	// Кнопка ручного обнаружения
	detectButton := widget.NewButton("Обнаружить устройства", func() {
		log.Println("Ручное обнаружение устройств...")
		go func() {
			if dp.gui.deviceMgr != nil {
				dp.gui.deviceMgr.ForceDetectAllDevices()
			}
			time.Sleep(500 * time.Millisecond)
			fyne.Do(func() {
				dp.Refresh()
				dp.gui.updateAvailableBlocks()
			})
		}()
	})
	detectButton.Importance = widget.MediumImportance
	mainContainer.Add(detectButton)

	dp.container = mainContainer
}

// createDeviceList создаёт список устройств.
func (dp *DevicePanel) createDeviceList() *widget.List {
	list := widget.NewList(
		func() int {
			devices := dp.state.GetAllDevices()
			count := 0
			for _, dev := range devices {
				if dev.IsConnected {
					count++
				}
			}
			return count
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.ComputerIcon())
			info := widget.NewLabel("Порт X: Устройство")
			info.TextStyle.Bold = true
			status := widget.NewLabel("✓ Подключено")
			status.TextStyle.Italic = true
			return container.NewHBox(icon, info, layout.NewSpacer(), status)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			devices := dp.state.GetAllDevices()
			type pair struct {
				port byte
				dev  *Device
			}
			connected := make([]pair, 0, len(devices))
			for port, dev := range devices {
				if dev.IsConnected {
					connected = append(connected, pair{port, dev})
				}
			}
			// Сортировка по порту (пузырьком для простоты)
			for i := 0; i < len(connected)-1; i++ {
				for j := i + 1; j < len(connected); j++ {
					if connected[i].port > connected[j].port {
						connected[i], connected[j] = connected[j], connected[i]
					}
				}
			}

			if id < len(connected) {
				port := connected[id].port
				dev := connected[id].dev

				containerObj := obj.(*fyne.Container)
				if len(containerObj.Objects) >= 4 {
					var iconRes fyne.Resource
					switch dev.DeviceType {
					case DEVICE_TYPE_MOTOR:
						iconRes = theme.StorageIcon()
					case DEVICE_TYPE_RGB_LIGHT:
						iconRes = theme.VisibilityIcon()
					case DEVICE_TYPE_TILT_SENSOR:
						iconRes = theme.ViewRefreshIcon()
					case DEVICE_TYPE_MOTION_SENSOR:
						iconRes = theme.MoveDownIcon()
					case DEVICE_TYPE_PIEZO_TONE:
						iconRes = theme.MediaFastForwardIcon()
					default:
						iconRes = theme.ComputerIcon()
					}
					iconWidget := containerObj.Objects[0].(*widget.Icon)
					iconWidget.SetResource(iconRes)

					infoWidget := containerObj.Objects[1].(*widget.Label)
					infoWidget.SetText(fmt.Sprintf("Порт %d: %s", port, dev.Name))
				}
			}
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		list.Unselect(id)
	}

	return list
}

// UpdateBattery обновляет отображение заряда батареи.
func (dp *DevicePanel) UpdateBattery(level int) {
	dp.batteryProgress.SetValue(float64(level) / 100)
	dp.batteryProgress.Refresh()
}

// UpdateHubInfo обновляет информацию о хабе.
func (dp *DevicePanel) UpdateHubInfo(info *HubInfo) {
	dp.hubInfoContainer.Objects = nil

	nameLabel := widget.NewLabel(fmt.Sprintf("Имя: %s", info.Name))
	dp.hubInfoContainer.Add(nameLabel)

	addressLabel := widget.NewLabel(fmt.Sprintf("Адрес: %s", info.Address))
	dp.hubInfoContainer.Add(addressLabel)

	if info.Manufacturer != "" {
		manufacturerLabel := widget.NewLabel(fmt.Sprintf("Производитель: %s", info.Manufacturer))
		dp.hubInfoContainer.Add(manufacturerLabel)
	}

	if info.FirmwareVersion != "" {
		firmwareLabel := widget.NewLabel(fmt.Sprintf("Прошивка: %s", info.FirmwareVersion))
		dp.hubInfoContainer.Add(firmwareLabel)
	}

	if info.SoftwareVersion != "" {
		softwareLabel := widget.NewLabel(fmt.Sprintf("Софт: %s", info.SoftwareVersion))
		dp.hubInfoContainer.Add(softwareLabel)
	}

	dp.hubInfoContainer.Refresh()
}

// Refresh обновляет список устройств и информацию о хабе.
func (dp *DevicePanel) Refresh() {
	if dp.deviceList != nil {
		dp.deviceList.Refresh()
	}
	hubInfo := dp.state.GetConnectedHub()
	if hubInfo != nil {
		dp.UpdateHubInfo(hubInfo)
	}
	if dp.batteryProgress != nil {
		// батарея обновляется отдельно через UpdateBattery, но можем показать текущую
		if hubInfo != nil {
			dp.batteryProgress.SetValue(float64(hubInfo.Battery) / 100)
		} else {
			dp.batteryProgress.SetValue(0)
		}
		dp.batteryProgress.Refresh()
	}
}

// Clear очищает отображение (при отключении).
func (dp *DevicePanel) Clear() {
	dp.hubInfoContainer.Objects = nil
	dp.hubInfoContainer.Refresh()
	dp.deviceList.Refresh()
	dp.batteryProgress.SetValue(0)
	dp.batteryProgress.Refresh()
}
