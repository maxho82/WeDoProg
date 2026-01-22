// debug.go
package main

import (
    "fmt"
    "log"
    "time"
)

// DebugMode включает/выключает отладочный вывод
var DebugMode = true

// DebugLog выводит отладочное сообщение
func DebugLog(format string, args ...interface{}) {
    if DebugMode {
        timestamp := time.Now().Format("15:04:05.000")
        message := fmt.Sprintf(format, args...)
        log.Printf("[DEBUG %s] %s", timestamp, message)
    }
}

// TestAddMockDevices добавляет тестовые устройства для отладки
func (gui *MainGUI) TestAddMockDevices() {
    DebugLog("Добавление тестовых устройств")
    
    // Тестовый мотор
    motor := &Device{
        PortID:      1,
        DeviceType:  DEVICE_TYPE_MOTOR,
        Name:        "Мотор",
        IsConnected: true,
        LastUpdate:  time.Now(),
    }
    gui.UpdateDeviceDisplay(1, motor)
    
    // Тестовый светодиод
    led := &Device{
        PortID:      6,
        DeviceType:  DEVICE_TYPE_RGB_LIGHT,
        Name:        "RGB Светодиод",
        IsConnected: true,
        LastUpdate:  time.Now(),
    }
    gui.UpdateDeviceDisplay(6, led)
    
    // Тестовый датчик наклона
    tilt := &Device{
        PortID:      1,
        DeviceType:  DEVICE_TYPE_TILT_SENSOR,
        Name:        "Датчик наклона",
        IsConnected: true,
        LastUpdate:  time.Now(),
    }
    gui.UpdateDeviceDisplay(2, tilt)
    
    DebugLog("Тестовые устройства добавлены")
}