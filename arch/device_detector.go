package main

import (
    "log"
    "time"
)

// DeviceDetector централизованный детектор устройств
type DeviceDetector struct {
    hubMgr *HubManager
}

// NewDeviceDetector создает новый детектор устройств
func NewDeviceDetector(hubMgr *HubManager) *DeviceDetector {
    return &DeviceDetector{
        hubMgr: hubMgr,
    }
}

// DetectAllDevices обнаруживает все устройства
func (dd *DeviceDetector) DetectAllDevices() {
    log.Println("=== Запуск полного обнаружения устройств ===")
    
    // 1. Встроенный RGB светодиод (порт 6)
    dd.detectBuiltInDevices()
    time.Sleep(500 * time.Millisecond)
    
    // 2. Порт 1
    dd.detectPortWithPriority(1)
    time.Sleep(1000 * time.Millisecond)
    
    // 3. Порт 2
    dd.detectPortWithPriority(2)
    
    log.Println("=== Обнаружение устройств завершено ===")
}

// detectBuiltInDevices обнаруживает встроенные устройства
func (dd *DeviceDetector) detectBuiltInDevices() {
    log.Println("Обнаружение встроенных устройств...")
    
    // RGB светодиод на порту 6
    dd.detectRGBLED(6)
}

// detectRGBLED обнаруживает RGB светодиод
func (dd *DeviceDetector) detectRGBLED(portID byte) {
    log.Printf("Проверка RGB светодиода на порту %d...", portID)
    
    // Настраиваем светодиод в режиме RGB
    setupCmd := []byte{0x01, 0x02, portID, 0x17, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
    err := dd.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
    
    if err != nil {
        log.Printf("Порт %d: ошибка настройки RGB светодиода", portID)
        return
    }
    
    log.Printf("Порт %d: RGB светодиод настроен", portID)
    
    // Тестируем светодиод
    testCmd := []byte{0x06, 0x04, 0x03, 0x00, 0x00, 0xFF} // Синий
    err = dd.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, testCmd)
    
    if err != nil {
        log.Printf("Порт %d: ошибка теста светодиода", portID)
        return
    }
    
    // Успешно обнаружили
    device := &Device{
        PortID:      portID,
        DeviceType:  DEVICE_TYPE_RGB_LIGHT,
        Name:        "RGB светодиод",
        IsConnected: true,
        LastUpdate:  time.Now(),
        Properties:  make(map[string]interface{}),
    }
    
    // Сохраняем устройство
    if dd.hubMgr.devices == nil {
        dd.hubMgr.devices = make(map[byte]*Device)
    }
    dd.hubMgr.devices[portID] = device
    
    // Уведомляем GUI
    if dd.hubMgr.deviceUpdateCallback != nil {
        dd.hubMgr.deviceUpdateCallback(portID, device)
    }
    
    log.Printf("Порт %d: RGB светодиод обнаружен", portID)
    
    // Возвращаем исходное состояние
    time.Sleep(300 * time.Millisecond)
    dd.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, []byte{0x06, 0x04, 0x03, 0x00, 0x00, 0x00})
}

// detectPortWithPriority обнаруживает устройство на порту с приоритетом
func (dd *DeviceDetector) detectPortWithPriority(portID byte) {
    log.Printf("Обнаружение устройств на порту %d...", portID)
    
    // Приоритет типов устройств для данного порта
    // (порт 1 обычно мотор, порт 2 обычно датчик)
    var priorityList []byte
    
    if portID == 1 {
        priorityList = []byte{
            DEVICE_TYPE_MOTOR,
            DEVICE_TYPE_TILT_SENSOR,
            DEVICE_TYPE_MOTION_SENSOR,
            DEVICE_TYPE_PIEZO_TONE,
        }
    } else if portID == 2 {
        priorityList = []byte{
            DEVICE_TYPE_TILT_SENSOR,
            DEVICE_TYPE_MOTION_SENSOR,
            DEVICE_TYPE_MOTOR,
            DEVICE_TYPE_PIEZO_TONE,
        }
    } else {
        priorityList = []byte{
            DEVICE_TYPE_MOTOR,
            DEVICE_TYPE_TILT_SENSOR,
            DEVICE_TYPE_MOTION_SENSOR,
            DEVICE_TYPE_PIEZO_TONE,
        }
    }
    
    for _, deviceType := range priorityList {
        if dd.testDeviceType(portID, deviceType) {
            log.Printf("Порт %d: обнаружено устройство типа 0x%02x", portID, deviceType)
            return
        }
        time.Sleep(300 * time.Millisecond)
    }
    
    log.Printf("Порт %d: устройства не обнаружены", portID)
}

// testDeviceType тестирует конкретный тип устройства на порту
func (dd *DeviceDetector) testDeviceType(portID, deviceType byte) bool {
    var setupCmd []byte
    var testCmd []byte
    var name string
    
    switch deviceType {
    case DEVICE_TYPE_MOTOR:
        setupCmd = []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
        testCmd = []byte{portID, 0x01, 0x01, 0x10} // Минимальная скорость вперед
        name = "Мотор"
    case DEVICE_TYPE_TILT_SENSOR:
        setupCmd = []byte{0x01, 0x02, portID, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
        name = "Датчик наклона"
    case DEVICE_TYPE_MOTION_SENSOR:
        setupCmd = []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
        name = "Датчик расстояния"
    case DEVICE_TYPE_PIEZO_TONE:
        setupCmd = []byte{0x01, 0x02, portID, 0x16, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
        testCmd = []byte{portID, 0x02, 0x04, 0xB8, 0x01, 0xE8, 0x03} // 440 Гц, 1000 мс
        name = "Пищалка"
    default:
        return false
    }
    
    // Настраиваем устройство
    err := dd.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
    if err != nil {
        log.Printf("Порт %d: ошибка настройки %s", portID, name)
        return false
    }
    
    time.Sleep(500 * time.Millisecond) // Даем время на настройку
    
    // Для устройств с тестовой командой
    if testCmd != nil {
        err = dd.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, testCmd)
        if err != nil {
            log.Printf("Порт %d: ошибка теста %s", portID, name)
            
            // Останавливаем мотор, если был запущен
            if deviceType == DEVICE_TYPE_MOTOR {
                dd.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, []byte{portID, 0x01, 0x01, 0x00})
            }
            return false
        }
        
        // Ждем и останавливаем
        time.Sleep(300 * time.Millisecond)
        if deviceType == DEVICE_TYPE_MOTOR {
            dd.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, []byte{portID, 0x01, 0x01, 0x00})
        } else if deviceType == DEVICE_TYPE_PIEZO_TONE {
            dd.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, []byte{portID, 0x03, 0x00})
        }
    }
    
    // Для датчиков проверяем ответ
    if deviceType == DEVICE_TYPE_TILT_SENSOR || deviceType == DEVICE_TYPE_MOTION_SENSOR {
        time.Sleep(300 * time.Millisecond)
        data, err := dd.hubMgr.ReadCharacteristic(SENSOR_VALUES_UUID)
        if err != nil || len(data) < 4 || data[1] != portID {
            log.Printf("Порт %d: %s не отвечает", portID, name)
            return false
        }
        log.Printf("Порт %d: %s отвечает (данные: %x)", portID, name, data)
    }
    
    // Успешно обнаружили устройство
    device := &Device{
        PortID:      portID,
        DeviceType:  deviceType,
        Name:        name,
        IsConnected: true,
        LastUpdate:  time.Now(),
        Properties:  make(map[string]interface{}),
    }
    
    if dd.hubMgr.devices == nil {
        dd.hubMgr.devices = make(map[byte]*Device)
    }
    dd.hubMgr.devices[portID] = device
    
    // Уведомляем GUI
    if dd.hubMgr.deviceUpdateCallback != nil {
        dd.hubMgr.deviceUpdateCallback(portID, device)
    }
    
    log.Printf("Порт %d: %s обнаружен", portID, name)
    return true
}