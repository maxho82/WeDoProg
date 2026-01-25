// port_parser.go
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
/* func ParsePortMessage(data []byte) *PortMessage {
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
} */

// ParsePortMessageV2 правильный парсинг сообщений о портах для WeDo 2.0
func ParsePortMessageV2(data []byte) *PortMessage {
	if len(data) < 4 {
		log.Printf("Слишком короткое сообщение о порте: %x", data)
		return nil
	}

	// Формат WeDo 2.0: [PortID, Action, ???, DeviceType, ...]
	// Но в данных приходит: [MsgLen, HubID, PortID, EventType, ...]
	// Смотрим на реальные данные из лога

	// Из лога: данные=020101220000001000000010
	// Разбиваем: 02 01 01 22 00 00 00 10 00 00 00
	// Вероятно: MsgLen=0x02, HubID=0x01, PortID=0x01, EventType=0x22

	// Но согласно статье: PortID=0x01, Action=0x01, ???=0x22, DeviceType=???

	// Возможно, EventType и есть DeviceType для уведомлений о подключении

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

// GetDeviceTypeV2 правильное определение типа устройства
func (msg *PortMessage) GetDeviceTypeV2() byte {
	// В WeDo 2.0 тип устройства может быть в EventType для некоторых сообщений
	// Или в Data[0] для других

	// Проверяем, является ли EventType известным типом устройства
	switch msg.EventType {
	case DEVICE_TYPE_TILT_SENSOR: // 0x22
	case DEVICE_TYPE_MOTION_SENSOR: // 0x23
	case DEVICE_TYPE_RGB_LIGHT: // 0x17
	case DEVICE_TYPE_PIEZO_TONE: // 0x16
	case DEVICE_TYPE_VOLTAGE: // 0x14
	case DEVICE_TYPE_CURRENT: // 0x15
		return msg.EventType
	}

	// Если EventType не тип устройства, проверяем данные
	if len(msg.Data) > 0 {
		// Возможно, тип устройства в первом байте данных
		return msg.Data[0]
	}

	return 0x00
}

// IsConnectionEventV2 правильная проверка подключения
func (msg *PortMessage) IsConnectionEventV2() bool {
	// В WeDo 2.0 подключение может определяться по разным признакам
	// 1. EventType == 0x01 (как в статье)
	// 2. Или EventType является типом устройства (0x22, 0x23 и т.д.)
	// 3. Или по комбинации байтов

	// Проверяем, является ли EventType типом устройства
	if msg.GetDeviceTypeV2() != 0x00 {
		return true
	}

	// Или проверяем стандартный признак
	return msg.EventType == 0x01
}

// IsDisconnectionEventV2 правильная проверка отключения
func (msg *PortMessage) IsDisconnectionEventV2() bool {
	// В WeDo 2.0 отключение может быть 0x00
	return msg.EventType == 0x00
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
// GetDeviceType улучшенная версия определения типа устройства
func (msg *PortMessage) GetDeviceType() byte {
	if len(msg.Data) == 0 {
		return 0x00
	}

	// Пытаемся определить тип по известным форматам LPF2

	// Формат 1: [DeviceType, ...] - тип в первом байте
	if len(msg.Data) >= 1 {
		deviceType := msg.Data[0]
		if isValidDeviceType(deviceType) {
			return deviceType
		}
	}

	// Формат 2: [0x00, 0x00, DeviceType, ...] - тип в третьем байте
	if len(msg.Data) >= 3 && msg.Data[0] == 0x00 && msg.Data[1] == 0x00 {
		deviceType := msg.Data[2]
		if isValidDeviceType(deviceType) {
			return deviceType
		}
	}

	// Формат 3: [0x01, 0x00, 0x00, DeviceType, ...] - тип в четвертом байте
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

	// Формат: [PortID, ValueType, Value...]
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

// ParseWeDo2PortMessage парсит сообщения о портах в формате WeDo 2.0
func ParseWeDo2PortMessage(data []byte) (portID byte, isConnected bool, hubID byte, deviceType byte) {
	if len(data) < 4 {
		return 0, false, 0, 0
	}

	// Формат WeDo 2.0: [PortID, ConnectionEvent, HubID, DeviceType, ...]
	portID = data[0]
	connectionEvent := data[1]
	hubID = data[2]
	deviceType = data[3]

	isConnected = (connectionEvent == 0x01)

	return portID, isConnected, hubID, deviceType
}
