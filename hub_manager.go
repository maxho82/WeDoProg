package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tinybluetooth "tinygo.org/x/bluetooth"
)

// HubManager управляет подключением к WeDo 2.0 хабу
type HubManager struct {
	adapter                   *tinybluetooth.Adapter
	device                    tinybluetooth.Device
	deviceAddress             string
	isConnected               bool
	connectionMutex           sync.RWMutex
	hubInfo                   *HubInfo
	stopScan                  context.CancelFunc
	services                  map[string]tinybluetooth.DeviceService
	characteristics           map[string]tinybluetooth.DeviceCharacteristic
	subscribedCharacteristics map[string]bool
	devices                   map[byte]*Device

	// Callback'и для обновлений
	batteryUpdateCallback   func(batteryLevel int)
	hubInfoUpdateCallback   func(info *HubInfo)
	deviceUpdateCallback    func(portID byte, device *Device)
	connectionStateCallback func(isConnected bool)
}

// NewHubManager создает новый менеджер хаба
func NewHubManager() (*HubManager, error) {
	adapter := tinybluetooth.DefaultAdapter
	if adapter == nil {
		return nil, fmt.Errorf("BLE адаптер не найден")
	}

	// Включение адаптера
	if err := adapter.Enable(); err != nil {
		return nil, fmt.Errorf("ошибка включения BLE адаптера: %v", err)
	}

	return &HubManager{
		adapter:                   adapter,
		hubInfo:                   &HubInfo{},
		services:                  make(map[string]tinybluetooth.DeviceService),
		characteristics:           make(map[string]tinybluetooth.DeviceCharacteristic),
		subscribedCharacteristics: make(map[string]bool),
		devices:                   make(map[byte]*Device),
	}, nil
}

// ScanForHubs сканирует WeDo 2.0 хабы
func (hm *HubManager) ScanForHubs(timeout time.Duration) ([]HubInfo, error) {
	var foundHubs []HubInfo
	var scanMutex sync.Mutex

	log.Println("=== Начало сканирования WeDo 2.0 хабов ===")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	hm.stopScan = cancel

	err := hm.adapter.Scan(func(adapter *tinybluetooth.Adapter, result tinybluetooth.ScanResult) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		name := result.LocalName()
		address := result.Address.String()
		rssi := result.RSSI

		// Ищем WeDo 2.0 хаб
		if (strings.Contains(strings.ToUpper(name), "WEDO") ||
			strings.Contains(strings.ToUpper(name), "LEGO") ||
			strings.Contains(strings.ToUpper(name), "LPF2") ||
			strings.HasPrefix(address, "24:71:89:")) && rssi > -80 {

			log.Printf("!!! Найден WeDo 2.0 хаб: %s [%s] RSSI: %d", name, address, rssi)

			scanMutex.Lock()
			foundHubs = append(foundHubs, HubInfo{
				Name:    name,
				Address: address,
				RSSI:    int(rssi),
			})
			scanMutex.Unlock()

			// Останавливаем сканирование при нахождении
			adapter.StopScan()
			cancel()
		}
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования: %v", err)
	}

	// Ждем завершения
	<-ctx.Done()
	hm.adapter.StopScan()

	log.Printf("Сканирование завершено. Найдено хабов: %d", len(foundHubs))
	return foundHubs, nil
}

// Connect подключается к хабу и читает всю информацию
func (hm *HubManager) Connect(address string) error {
	hm.connectionMutex.Lock()
	defer hm.connectionMutex.Unlock()

	if hm.isConnected {
		hm.Disconnect()
	}

	log.Printf("Подключение к хабу: %s", address)

	// Находим устройство через сканирование
	var targetDevice tinybluetooth.ScanResult
	found := false

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Поиск устройства для подключения...")

	err := hm.adapter.Scan(func(adapter *tinybluetooth.Adapter, result tinybluetooth.ScanResult) {
		if result.Address.String() == address {
			log.Printf("Найдено устройство: %s", result.LocalName())
			adapter.StopScan()
			targetDevice = result
			found = true
			cancel()
		}
	})

	if err != nil {
		return fmt.Errorf("ошибка сканирования: %v", err)
	}

	<-ctx.Done()
	hm.adapter.StopScan()

	if !found {
		return fmt.Errorf("устройство с адресом %s не найдено", address)
	}

	// Подключаемся
	log.Printf("Устанавливаем соединение с %s...", address)
	device, err := hm.adapter.Connect(targetDevice.Address, tinybluetooth.ConnectionParams{})
	if err != nil {
		return fmt.Errorf("ошибка подключения: %v", err)
	}

	hm.device = device
	hm.deviceAddress = address
	hm.isConnected = true

	// Обновляем информацию о хабе
	hm.hubInfo.Name = targetDevice.LocalName()
	hm.hubInfo.Address = address
	hm.hubInfo.LastUpdated = time.Now()

	// Обнаруживаем службы и характеристики
	log.Println("Обнаружение служб и характеристик...")
	err = hm.discoverAllServices()
	if err != nil {
		log.Printf("Предупреждение: %v", err)
	}

	// Читаем информацию об устройстве
	log.Println("Чтение информации об устройстве...")
	go hm.readAllDeviceInfo()

	// Подписываемся на важные уведомления
	go hm.subscribeToImportantNotifications()

	// Уведомляем о подключении
	if hm.connectionStateCallback != nil {
		hm.connectionStateCallback(true)
	}

	// После успешного подключения проверяем устройства
	go func() {
		time.Sleep(2 * time.Second) // Ждем, пока все службы инициализируются
		hm.CheckConnectedDevices()
	}()

	return nil

}

// discoverAllServices обнаруживает все службы и характеристики
func (hm *HubManager) discoverAllServices() error {
	services, err := hm.device.DiscoverServices(nil)
	if err != nil {
		return fmt.Errorf("ошибка обнаружения служб: %v", err)
	}

	log.Printf("Найдено служб: %d", len(services))

	for _, service := range services {
		uuid := service.UUID().String()
		hm.services[uuid] = service

		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			log.Printf("Ошибка обнаружения характеристик в службе %s: %v", uuid, err)
			continue
		}

		for _, char := range chars {
			charUUID := char.UUID().String()
			hm.characteristics[charUUID] = char
		}
	}

	log.Printf("Обнаружено характеристик: %d", len(hm.characteristics))
	return nil
}

// readAllDeviceInfo читает всю информацию об устройстве
func (hm *HubManager) readAllDeviceInfo() {
	log.Println("Чтение полной информации об устройстве...")

	// Читаем Device Name (если доступен)
	if char, exists := hm.characteristics["00002a00-0000-1000-8000-00805f9b34fb"]; exists {
		data, err := hm.readCharacteristic(char)
		if err == nil && len(data) > 0 {
			name := strings.TrimSpace(string(data))
			if name != "" {
				hm.hubInfo.Name = name
				log.Printf("Device Name: %s", name)
			}
		}
	}

	// Массив UUID для чтения информации об устройстве
	deviceInfoUUIDs := map[string]string{
		"00002a29-0000-1000-8000-00805f9b34fb": "Производитель",
		"00002a26-0000-1000-8000-00805f9b34fb": "Версия прошивки",
		"00002a28-0000-1000-8000-00805f9b34fb": "Версия софта",
		"00002a23-0000-1000-8000-00805f9b34fb": "System ID",
	}

	for uuid, name := range deviceInfoUUIDs {
		if char, exists := hm.characteristics[uuid]; exists {
			data, err := hm.readCharacteristic(char)
			if err != nil {
				log.Printf("Ошибка чтения %s: %v", name, err)
				continue
			}

			if len(data) > 0 {
				var value string
				if uuid == "00002a23-0000-1000-8000-00805f9b34fb" {
					// System ID - преобразуем байты в hex строку
					hexStr := bytesToHexString(data)
					value = hexStr
					log.Printf("%s (HEX): %s", name, hexStr)
				} else {
					value = strings.TrimSpace(string(data))
					log.Printf("%s: %s", name, value)
				}

				// Обновляем информацию в хабе
				hm.updateHubInfo(uuid, value)
			}
		}
	}

	// Читаем уровень батареи
	hm.readBatteryLevel()
}

// readCharacteristic читает данные из характеристики
func (hm *HubManager) readCharacteristic(char tinybluetooth.DeviceCharacteristic) ([]byte, error) {
	buf := make([]byte, 512)
	n, err := char.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// updateHubInfo обновляет информацию о хабе
func (hm *HubManager) updateHubInfo(uuid string, value string) {
	hm.connectionMutex.Lock()
	defer hm.connectionMutex.Unlock()

	switch uuid {
	case "00002a29-0000-1000-8000-00805f9b34fb":
		hm.hubInfo.Manufacturer = value
	case "00002a26-0000-1000-8000-00805f9b34fb":
		hm.hubInfo.FirmwareVersion = value
	case "00002a28-0000-1000-8000-00805f9b34fb":
		hm.hubInfo.SoftwareVersion = value
	case "00002a23-0000-1000-8000-00805f9b34fb":
		hm.hubInfo.SystemID = value
	}

	// Уведомляем об обновлении
	if hm.hubInfoUpdateCallback != nil {
		hm.hubInfoUpdateCallback(hm.hubInfo)
	}
}

// readBatteryLevel читает уровень батареи
func (hm *HubManager) readBatteryLevel() {
	batteryUUID := "00002a19-0000-1000-8000-00805f9b34fb"

	if char, exists := hm.characteristics[batteryUUID]; exists {
		data, err := hm.readCharacteristic(char)
		if err != nil {
			log.Printf("Ошибка чтения батареи: %v", err)
			return
		}

		if len(data) > 0 {
			batteryLevel := int(data[0])
			hm.hubInfo.Battery = batteryLevel

			if hm.batteryUpdateCallback != nil {
				hm.batteryUpdateCallback(batteryLevel)
			}
		}
	}
}

// subscribeToImportantNotifications подписывается на важные уведомления
func (hm *HubManager) subscribeToImportantNotifications() {
	// Подписываемся на обновления батареи
	hm.subscribeToBatteryNotifications()

	// Подписываемся на уведомления портов
	hm.subscribeToPortNotifications()

	// Подписываемся на обновления прошивки
	hm.subscribeToFirmwareNotifications()
}

// subscribeToBatteryNotifications подписывается на уведомления батареи
func (hm *HubManager) subscribeToBatteryNotifications() {
	batteryUUID := "00002a19-0000-1000-8000-00805f9b34fb"

	if char, exists := hm.characteristics[batteryUUID]; exists {
		err := char.EnableNotifications(func(data []byte) {
			if len(data) > 0 {
				batteryLevel := int(data[0])
				hm.hubInfo.Battery = batteryLevel

				if hm.batteryUpdateCallback != nil {
					hm.batteryUpdateCallback(batteryLevel)
				}
			}
		})

		if err != nil {
			log.Printf("Ошибка подписки на батарею: %v", err)
		} else {
			log.Println("Подписка на обновления батареи установлена")
			hm.subscribedCharacteristics[batteryUUID] = true
		}
	}
}

// subscribeToPortNotifications подписывается на уведомления портов
func (hm *HubManager) subscribeToPortNotifications() {
	portInfoUUID := PORT_INFO_UUID // "00001527-1212-efde-1523-785feabcd123"

	if char, exists := hm.characteristics[portInfoUUID]; exists {
		err := char.EnableNotifications(func(data []byte) {
			if len(data) >= 3 {
				portID := data[1]
				eventType := data[2]

				log.Printf("Информация о порте: порт=%d, событие=%d, данные=%x",
					portID, eventType, data)

				// Обрабатываем подключение/отключение устройства
				if eventType == 0x01 && len(data) >= 8 {
					deviceType := data[4]
					hm.handleDeviceConnection(portID, deviceType, data)
				} else if eventType == 0x00 {
					hm.handleDeviceDisconnection(portID)
				}
			}
		})

		if err != nil {
			log.Printf("Ошибка подписки на информацию о портах: %v", err)
			// Если подписка не поддерживается, запрашиваем информацию о портах напрямую
			hm.requestPortInfoDirectly()
		} else {
			log.Println("Подписка на информацию о портах установлена")
			hm.subscribedCharacteristics[portInfoUUID] = true

			// Запрашиваем информацию о портах
			hm.requestPortInfo()
		}
	} else {
		log.Printf("Характеристика информации о портах не найдена")
		// Пытаемся найти порты другим способом
		hm.discoverPortsManually()
	}
}

// requestPortInfo запрашивает информацию о портах
func (hm *HubManager) requestPortInfo() {
	log.Println("Запрос информации о портах...")

	// Запрашиваем информацию о портах 1 и 2
	for port := byte(1); port <= 2; port++ {
		// Команда запроса информации о порте
		cmd := []byte{0x01, 0x00, port, 0x00}
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)

		if err != nil {
			log.Printf("Ошибка запроса информации о порте %d: %v", port, err)
		} else {
			log.Printf("Запрос информации о порте %d отправлен", port)
		}

		// Небольшая задержка между запросами
		time.Sleep(100 * time.Millisecond)
	}
}

// requestPortInfoDirectly пытается прочитать информацию о портах напрямую
func (hm *HubManager) requestPortInfoDirectly() {
	log.Println("Прямой запрос информации о портах...")

	portInfoUUID := PORT_INFO_UUID

	if char, exists := hm.characteristics[portInfoUUID]; exists {
		for port := byte(1); port <= 2; port++ {
			// Отправляем команду запроса информации о порте
			cmd := []byte{0x01, 0x00, port, 0x00}
			err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)

			if err != nil {
				log.Printf("Ошибка отправки запроса порта %d: %v", port, err)
				continue
			}

			// Ждем немного, затем читаем характеристику
			time.Sleep(200 * time.Millisecond)

			data, err := hm.readCharacteristic(char)
			if err != nil {
				log.Printf("Ошибка чтения информации о порте %d: %v", port, err)
				continue
			}

			if len(data) >= 3 {
				portID := data[1]
				eventType := data[2]

				log.Printf("Прямое чтение порта %d: событие=%d, данные=%x",
					portID, eventType, data)

				if eventType == 0x01 && len(data) >= 8 {
					deviceType := data[4]
					hm.handleDeviceConnection(portID, deviceType, data)
				}
			}
		}
	}
}

// discoverPortsManually пытается обнаружить порты вручную
func (hm *HubManager) discoverPortsManually() {
	log.Println("Ручное обнаружение портов...")

	// Попробуем отправить команды настройки для всех возможных портов
	// и посмотреть, какие ответят

	portsToCheck := []byte{1, 2} // Порты WeDo 2.0

	for _, port := range portsToCheck {
		// Пытаемся настроить мотор (даже если его нет)
		motorCmd := []byte{0x01, 0x02, port, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, motorCmd)

		if err != nil {
			log.Printf("Порт %d: ошибка настройки мотора - устройство может отсутствовать", port)
		} else {
			log.Printf("Порт %d: команда настройки мотора отправлена", port)

			// Предполагаем, что устройство есть
			// В реальном приложении нужно ждать ответа
			device := &Device{
				PortID:      port,
				DeviceType:  DEVICE_TYPE_MOTOR,
				Name:        "Мотор (предположительно)",
				IsConnected: true,
				LastUpdate:  time.Now(),
			}

			hm.devices[port] = device

			// Уведомляем GUI
			if hm.deviceUpdateCallback != nil {
				hm.deviceUpdateCallback(port, device)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// CheckConnectedDevices проверяет подключенные устройства
func (hm *HubManager) CheckConnectedDevices() {
	log.Println("Проверка подключенных устройств...")

	// Отправляем команды для проверки каждого порта
	for port := byte(1); port <= 2; port++ {
		// Команда запроса информации о порте
		cmd := []byte{0x01, 0x00, port, 0x00}
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)

		if err != nil {
			log.Printf("Ошибка проверки порта %d: %v", port, err)
		} else {
			log.Printf("Проверка порта %d отправлена", port)
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// subscribeToFirmwareNotifications подписывается на обновления прошивки
func (hm *HubManager) subscribeToFirmwareNotifications() {
	firmwareUUID := "00004f01-1212-efde-1523-785feabcd123"

	if char, exists := hm.characteristics[firmwareUUID]; exists {
		err := char.EnableNotifications(func(data []byte) {
			if len(data) > 0 {
				firmware := string(data)
				log.Printf("Обновление прошивки: %s", firmware)
				hm.hubInfo.FirmwareVersion = firmware

				if hm.hubInfoUpdateCallback != nil {
					hm.hubInfoUpdateCallback(hm.hubInfo)
				}
			}
		})

		if err != nil {
			log.Printf("Ошибка подписки на обновления прошивки: %v", err)
		} else {
			log.Println("Подписка на обновления прошивки установлена")
			hm.subscribedCharacteristics[firmwareUUID] = true
		}
	}
}

// handleDeviceConnection обрабатывает подключение устройства
/* func (hm *HubManager) handleDeviceConnection(portID byte, deviceType byte, data []byte) {
	log.Printf("Устройство подключено к порту %d, тип: 0x%02x", portID, deviceType)

	// Создаем информацию об устройстве
	device := &Device{
		PortID:      portID,
		DeviceType:  deviceType,
		Name:        hm.getDeviceName(deviceType),
		IsConnected: true,
		LastUpdate:  time.Now(),
	}

	// Настраиваем устройство в зависимости от типа
	hm.configureDevice(portID, deviceType)

	// Уведомляем об обновлении
	if hm.deviceUpdateCallback != nil {
		hm.deviceUpdateCallback(portID, device)
	}
} */
func (hm *HubManager) handleDeviceConnection(portID byte, deviceType byte, data []byte) {
	log.Printf("Устройство подключено к порту %d, тип: 0x%02x, данные: %x",
		portID, deviceType, data)

	// Создаем информацию об устройстве
	device := &Device{
		PortID:      portID,
		DeviceType:  deviceType,
		Name:        hm.getDeviceName(deviceType),
		IsConnected: true,
		LastUpdate:  time.Now(),
		Properties:  make(map[string]interface{}),
	}

	// Сохраняем устройство
	hm.devices[portID] = device

	// Настраиваем устройство в зависимости от типа
	go func() {
		time.Sleep(500 * time.Millisecond) // Даем время на подключение
		err := hm.configureDevice(portID, deviceType)
		if err != nil {
			log.Printf("Ошибка настройки устройства: %v", err)
			// Не помечаем как отключенное, т.к. оно может работать без настройки
		}

		// Уведомляем об обновлении
		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}
	}()

	log.Printf("Устройство обнаружено: %s (порт %d)", device.Name, portID)
}

// handleDeviceDisconnection обрабатывает отключение устройства
func (hm *HubManager) handleDeviceDisconnection(portID byte) {
	log.Printf("Устройство отключено от порта %d", portID)

	// Создаем информацию об отключенном устройстве
	device := &Device{
		PortID:      portID,
		IsConnected: false,
		LastUpdate:  time.Now(),
	}

	// Уведомляем об обновлении
	if hm.deviceUpdateCallback != nil {
		hm.deviceUpdateCallback(portID, device)
	}
}

// configureDevice настраивает устройство в зависимости от типа
func (hm *HubManager) configureDevice(portID byte, deviceType byte) error {
	log.Printf("Настройка устройства на порту %d (тип: 0x%02x)", portID, deviceType)

	var cmd []byte

	switch deviceType {
	case DEVICE_TYPE_MOTOR:
		cmd = []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	case DEVICE_TYPE_TILT_SENSOR:
		cmd = []byte{0x01, 0x02, portID, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	case DEVICE_TYPE_MOTION_SENSOR:
		cmd = []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	case DEVICE_TYPE_RGB_LIGHT:
		cmd = []byte{0x01, 0x02, portID, 0x17, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	case DEVICE_TYPE_PIEZO_TONE:
		cmd = []byte{0x01, 0x02, portID, 0x16, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	case DEVICE_TYPE_VOLTAGE:
		cmd = []byte{0x01, 0x02, portID, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	case DEVICE_TYPE_CURRENT:
		cmd = []byte{0x01, 0x02, portID, 0x15, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	default:
		log.Printf("Неизвестный тип устройства 0x%02x, пропускаем настройку", deviceType)
		return nil
	}

	// Отправляем команду настройки
	if err := hm.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd); err != nil {
		return fmt.Errorf("ошибка настройки устройства: %v", err)
	}

	log.Printf("Устройство на порту %d успешно настроено", portID)
	return nil
}

// getDeviceName возвращает имя устройства по типу
func (hm *HubManager) getDeviceName(deviceType byte) string {
	switch deviceType {
	case 0x01:
		return "Мотор"
	case 0x22:
		return "Датчик наклона"
	case 0x23:
		return "Датчик расстояния"
	case 0x17:
		return "RGB светодиод"
	case 0x16:
		return "Пищалка"
	case 0x14:
		return "Датчик напряжения"
	case 0x15:
		return "Датчик тока"
	default:
		return fmt.Sprintf("Неизвестное (0x%02x)", deviceType)
	}
}

// WriteCharacteristic записывает данные в характеристику
func (hm *HubManager) WriteCharacteristic(uuid string, data []byte) error {
	hm.connectionMutex.RLock()
	defer hm.connectionMutex.RUnlock()

	if !hm.isConnected {
		return fmt.Errorf("не подключено к хабу")
	}

	char, exists := hm.characteristics[uuid]
	if !exists {
		return fmt.Errorf("характеристика %s не найдена", uuid)
	}

	_, err := char.WriteWithoutResponse(data)
	if err != nil {
		return fmt.Errorf("ошибка отправки данных: %v", err)
	}

	log.Printf("Данные отправлены: %v (HEX: %x)", data, data)
	return nil
}

// ReadCharacteristic читает данные из характеристики
func (hm *HubManager) ReadCharacteristic(uuid string) ([]byte, error) {
	hm.connectionMutex.RLock()
	defer hm.connectionMutex.RUnlock()

	if !hm.isConnected {
		return nil, fmt.Errorf("не подключено к хабу")
	}

	char, exists := hm.characteristics[uuid]
	if !exists {
		return nil, fmt.Errorf("характеристика %s не найдена", uuid)
	}

	buf := make([]byte, 512)
	n, err := char.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения данных: %v", err)
	}

	return buf[:n], nil
}

// Disconnect отключается от хаба
func (hm *HubManager) Disconnect() {
	hm.connectionMutex.Lock()
	defer hm.connectionMutex.Unlock()

	if hm.isConnected {
		log.Println("Отключение от хаба...")

		hm.device.Disconnect()
		hm.isConnected = false
		hm.hubInfo = &HubInfo{}

		// Уведомляем об отключении
		if hm.connectionStateCallback != nil {
			hm.connectionStateCallback(false)
		}

		log.Println("Отключено")
	}
}

// IsConnected возвращает статус подключения
func (hm *HubManager) IsConnected() bool {
	hm.connectionMutex.RLock()
	defer hm.connectionMutex.RUnlock()
	return hm.isConnected
}

// GetHubInfo возвращает информацию о хабе
func (hm *HubManager) GetHubInfo() *HubInfo {
	hm.connectionMutex.RLock()
	defer hm.connectionMutex.RUnlock()

	// Возвращаем копию
	infoCopy := *hm.hubInfo
	return &infoCopy
}

// Callback функции

func (hm *HubManager) SetBatteryUpdateCallback(callback func(batteryLevel int)) {
	hm.batteryUpdateCallback = callback
}

func (hm *HubManager) SetHubInfoUpdateCallback(callback func(info *HubInfo)) {
	hm.hubInfoUpdateCallback = callback
}

func (hm *HubManager) SetDeviceUpdateCallback(callback func(portID byte, device *Device)) {
	hm.deviceUpdateCallback = callback
}

func (hm *HubManager) SetConnectionStateCallback(callback func(isConnected bool)) {
	hm.connectionStateCallback = callback
}
