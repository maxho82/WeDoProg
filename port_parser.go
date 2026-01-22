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
    // Спецификация LPF2: после EventType может идти тип устройства
    // Положение зависит от типа сообщения
    
    if len(msg.Data) > 0 {
        // Попробуем несколько возможных позиций
        // Часто тип устройства в первом байте данных
        deviceType := msg.Data[0]
        
        // Проверяем, является ли это известным типом устройства
        if deviceType == DEVICE_TYPE_MOTOR ||
            deviceType == DEVICE_TYPE_TILT_SENSOR ||
            deviceType == DEVICE_TYPE_MOTION_SENSOR ||
            deviceType == DEVICE_TYPE_RGB_LIGHT ||
            deviceType == DEVICE_TYPE_PIEZO_TONE ||
            deviceType == DEVICE_TYPE_VOLTAGE ||
            deviceType == DEVICE_TYPE_CURRENT {
            return deviceType
        }
        
        // Если тип не распознан, пробуем другие позиции
        if len(msg.Data) > 3 {
            deviceType = msg.Data[3]
            if deviceType != 0x00 && deviceType != 0xFF {
                return deviceType
            }
        }
        
        if len(msg.Data) > 6 {
            deviceType = msg.Data[6]
            if deviceType != 0x00 && deviceType != 0xFF {
                return deviceType
            }
        }
    }
    
    return 0x00 // Неизвестный тип
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