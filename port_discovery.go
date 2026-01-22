// port_discovery.go
package main

import (
    "log"
    "time"
)

// PortDiscovery управляет обнаружением портов
type PortDiscovery struct {
    hubMgr *HubManager
}

// NewPortDiscovery создает новый объект обнаружения портов
func NewPortDiscovery(hubMgr *HubManager) *PortDiscovery {
    return &PortDiscovery{
        hubMgr: hubMgr,
    }
}

// DiscoverPorts обнаруживает устройства на портах
func (pd *PortDiscovery) DiscoverPorts() {
    if !pd.hubMgr.IsConnected() {
        log.Println("Не подключено к хабу для обнаружения портов")
        return
    }
    
    log.Println("=== Начало обнаружения портов ===")
    
    // Метод 1: Чтение портов через команды
    pd.discoverViaCommands()
    
    // Метод 2: Попытка настроить устройства
    time.Sleep(1 * time.Second)
    pd.discoverViaConfiguration()
    
    // Метод 3: Чтение сенсоров
    time.Sleep(1 * time.Second)
    pd.discoverViaSensors()
    
    log.Println("=== Завершение обнаружения портов ===")
}

// discoverViaCommands обнаруживает порты через команды
func (pd *PortDiscovery) discoverViaCommands() {
    log.Println("Обнаружение через команды...")
    
    // Запрашиваем информацию о портах
    for port := byte(1); port <= 2; port++ {
        // Команда: запрос информации о порте
        cmd := []byte{0x01, 0x00, port, 0x00}
        err := pd.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)
        
        if err != nil {
            log.Printf("Порт %d: ошибка запроса - %v", port, err)
        } else {
            log.Printf("Порт %d: запрос информации отправлен", port)
        }
        
        time.Sleep(300 * time.Millisecond)
    }
}

// discoverViaConfiguration обнаруживает порты через настройку
func (pd *PortDiscovery) discoverViaConfiguration() {
    log.Println("Обнаружение через настройку...")
    
    // Пробуем настроить различные типы устройств
    deviceTypes := []struct {
        port byte
        cmd  []byte
        name string
    }{
        {1, []byte{0x01, 0x02, 1, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}, "Мотор A"},
        {2, []byte{0x01, 0x02, 2, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}, "Мотор B"},
        {1, []byte{0x01, 0x02, 1, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}, "Датчик наклона"},
        {2, []byte{0x01, 0x02, 2, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}, "Датчик наклона"},
    }
    
    for _, device := range deviceTypes {
        err := pd.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, device.cmd)
        if err != nil {
            log.Printf("Настройка %s: ошибка - %v", device.name, err)
        } else {
            log.Printf("Настройка %s отправлена", device.name)
            
            // Предполагаем, что устройство есть
            // В реальности нужно проверять ответ
        }
        
        time.Sleep(200 * time.Millisecond)
    }
}

// discoverViaSensors обнаруживает порты через чтение сенсоров
func (pd *PortDiscovery) discoverViaSensors() {
    log.Println("Обнаружение через чтение сенсоров...")
    
    // Пытаемся прочитать значения сенсоров
    for port := byte(1); port <= 2; port++ {
        // Настраиваем порт для чтения значений
        setupCmd := []byte{0x01, 0x02, port, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
        err := pd.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
        
        if err != nil {
            log.Printf("Порт %d: ошибка настройки сенсора - %v", port, err)
        } else {
            log.Printf("Порт %d: настройка сенсора отправлена", port)
            
            // Читаем значение сенсора
            time.Sleep(200 * time.Millisecond)
            data, err := pd.hubMgr.ReadCharacteristic(SENSOR_VALUES_UUID)
            if err != nil {
                log.Printf("Порт %d: ошибка чтения сенсора - %v", port, err)
            } else if len(data) > 0 {
                log.Printf("Порт %d: получены данные сенсора (%d байт): %x", 
                    port, len(data), data)
                
                // Если есть данные, возможно устройство подключено
                if len(data) >= 4 && data[1] == port {
                    log.Printf("Порт %d: устройство отвечает", port)
                }
            }
        }
        
        time.Sleep(300 * time.Millisecond)
    }
}