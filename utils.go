// utils.go
package main

import (
	"fmt"
	"strings"
)

// DeviceTypeToBlockType преобразует тип устройства в тип блока
func DeviceTypeToBlockType(deviceType byte) BlockType {
	switch deviceType {
	case DEVICE_TYPE_MOTOR:
		return BlockTypeMotor
	case DEVICE_TYPE_RGB_LIGHT:
		return BlockTypeLED
	case DEVICE_TYPE_TILT_SENSOR:
		return BlockTypeTiltSensor
	case DEVICE_TYPE_MOTION_SENSOR:
		return BlockTypeDistanceSensor
	case DEVICE_TYPE_PIEZO_TONE:
		return BlockTypeSound
	case DEVICE_TYPE_VOLTAGE:
		return BlockTypeVoltageSensor
	case DEVICE_TYPE_CURRENT:
		return BlockTypeCurrentSensor
	default:
		return BlockTypeStart
	}
}

// IsBlockAvailable проверяет доступность блока
func (gui *MainGUI) IsBlockAvailable(blockType BlockType) bool {
	if enabled, exists := gui.availableBlocks[blockType]; exists {
		return enabled
	}
	return false
}

// FormatHubInfo форматирует информацию о хабе для отображения
func FormatHubInfo(info *HubInfo) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Имя: %s\n", info.Name))
	builder.WriteString(fmt.Sprintf("Адрес: %s\n", info.Address))

	if info.Manufacturer != "" {
		builder.WriteString(fmt.Sprintf("Производитель: %s\n", info.Manufacturer))
	}

	if info.FirmwareVersion != "" {
		builder.WriteString(fmt.Sprintf("Версия прошивки: %s\n", info.FirmwareVersion))
	}

	if info.SoftwareVersion != "" {
		builder.WriteString(fmt.Sprintf("Версия софта: %s\n", info.SoftwareVersion))
	}

	if info.SystemID != "" {
		builder.WriteString(fmt.Sprintf("System ID: %s\n", info.SystemID))
	}

	if info.Battery > 0 {
		builder.WriteString(fmt.Sprintf("Батарея: %d%%\n", info.Battery))
	}

	return builder.String()
}

// GetDeviceFromPort получает устройство по порту
func (hm *HubManager) GetDeviceFromPort(portID byte) (*Device, bool) {
	device, exists := hm.devices[portID]
	return device, exists
}

// GetConnectedDevices возвращает список подключенных устройств
func (hm *HubManager) GetConnectedDevices() []*Device {
	var devices []*Device
	for _, device := range hm.devices {
		if device.IsConnected {
			devices = append(devices, device)
		}
	}
	return devices
}
