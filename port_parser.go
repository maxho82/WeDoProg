package main

import (
	"encoding/binary"
	"log"
)

// PortMessage парсит сообщения о портах
type PortMessage struct {
	MsgLen    byte
	HubID     byte
	PortID    byte
	EventType byte
	Data      []byte
}

// ParsePortMessage парсит сообщение о порте
func ParsePortMessage(data []byte) *PortMessage {
	if len(data) < 4 {
		log.Printf("Слишком короткое сообщение о порте: %x", data)
		return nil
	}

	msg := &PortMessage{
		MsgLen:    data[0],
		HubID:     data[1],
		PortID:    data[2],
		EventType: data[3],
	}

	if len(data) > 4 {
		msg.Data = data[4:]
	}

	return msg
}

// IsConnectionEvent проверяет, является ли событие подключением устройства
func (msg *PortMessage) IsConnectionEvent() bool {
	return msg.EventType == 0x01
}

// IsDisconnectionEvent проверяет, является ли событие отключением устройства
func (msg *PortMessage) IsDisconnectionEvent() bool {
	return msg.EventType == 0x00
}

// GetDeviceType пытается извлечь тип устройства из сообщения
func (msg *PortMessage) GetDeviceType() byte {
	if len(msg.Data) == 0 {
		return 0x00
	}

	// Формат 1: [DeviceType, ...]
	if len(msg.Data) >= 1 {
		deviceType := msg.Data[0]
		if isValidDeviceType(deviceType) {
			return deviceType
		}
	}

	// Формат 2: [0x00, 0x00, DeviceType, ...]
	if len(msg.Data) >= 3 && msg.Data[0] == 0x00 && msg.Data[1] == 0x00 {
		deviceType := msg.Data[2]
		if isValidDeviceType(deviceType) {
			return deviceType
		}
	}

	// Формат 3: [0x01, 0x00, 0x00, DeviceType, ...]
	if len(msg.Data) >= 4 && msg.Data[0] == 0x01 && msg.Data[1] == 0x00 && msg.Data[2] == 0x00 {
		deviceType := msg.Data[3]
		if isValidDeviceType(deviceType) {
			return deviceType
		}
	}

	return 0x00
}

// isValidDeviceType проверяет, является ли байт валидным типом устройства
func isValidDeviceType(deviceType byte) bool {
	switch deviceType {
	case DEVICE_TYPE_MOTOR,
		DEVICE_TYPE_TILT_SENSOR,
		DEVICE_TYPE_MOTION_SENSOR,
		DEVICE_TYPE_RGB_LIGHT,
		DEVICE_TYPE_PIEZO_TONE,
		DEVICE_TYPE_VOLTAGE,
		DEVICE_TYPE_CURRENT:
		return true
	default:
		return false
	}
}

// DecodeSensorValues декодирует значения сенсоров
func DecodeSensorValues(data []byte, portID byte) interface{} {
	if len(data) < 3 {
		return nil
	}

	if data[1] != portID {
		return nil
	}

	valueType := data[2]

	switch valueType {
	case 0x01: // Одно байтовое значение
		if len(data) >= 4 {
			return data[3]
		}
	case 0x02: // Двухбайтовое значение
		if len(data) >= 5 {
			return binary.LittleEndian.Uint16(data[3:5])
		}
	case 0x03: // Четырехбайтовое значение
		if len(data) >= 7 {
			return binary.LittleEndian.Uint32(data[3:7])
		}
	}

	return nil
}
