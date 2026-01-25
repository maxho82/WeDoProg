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
		//hm.CheckConnectedDevices()
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

// subscribeToPortNotifications подписывается на уведомления о портах
func (hm *HubManager) subscribeToPortNotifications() {
	portInfoUUID := PORT_INFO_UUID

	if char, exists := hm.characteristics[portInfoUUID]; exists {
		err := char.EnableNotifications(func(data []byte) {
			// Обрабатываем уведомление
			hm.handlePortNotification(data)
		})

		if err != nil {
			log.Printf("Ошибка подписки на информацию о портах: %v", err)
		} else {
			log.Println("Подписка на информацию о портах установлена")
			hm.subscribedCharacteristics[portInfoUUID] = true
		}
	} else {
		log.Printf("Характеристика информации о портах не найдена")
	}
}

// guessDeviceType пытается угадать тип устройства по порту и другим признакам
func (hm *HubManager) guessDeviceType(portID byte) byte {
	// Эвристика: порт 1 часто используется для мотора, порт 2 для датчиков
	if portID == 1 {
		// Пробуем настроить мотор и проверить реакцию
		setupCmd := []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
		if err == nil {
			log.Printf("Порт %d: успешно настроен как мотор", portID)
			return DEVICE_TYPE_MOTOR
		}
	} else if portID == 2 {
		// Пробуем настроить датчик расстояния
		setupCmd := []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
		if err == nil {
			log.Printf("Порт %d: успешно настроен как датчик расстояния", portID)
			return DEVICE_TYPE_MOTION_SENSOR
		}
	}

	return 0x00
}

// sendPortInformationRequest отправляет запрос информации о портах
func (hm *HubManager) sendPortInformationRequest() {
	log.Println("Отправка запроса информации о портах...")

	// Команда для запроса информации о всех портах
	// Hub Action: Request Port Information (0x21)
	cmd := []byte{0x01, 0x21}
	err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)

	if err != nil {
		log.Printf("Ошибка запроса информации о портах: %v", err)
	} else {
		log.Println("Запрос информации о портах отправлен")
	}
}

// registerDevice регистрирует устройство
func (hm *HubManager) registerDevice(portID byte, deviceType byte, name string) {
	log.Printf("Регистрация устройства: порт %d, тип 0x%02x, имя: %s", portID, deviceType, name)

	device := &Device{
		PortID:      portID,
		DeviceType:  deviceType,
		Name:        name,
		IsConnected: true,
		LastUpdate:  time.Now(),
		Properties:  make(map[string]interface{}),
	}

	hm.devices[portID] = device

	// Уведомляем GUI
	if hm.deviceUpdateCallback != nil {
		hm.deviceUpdateCallback(portID, device)
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

func (hm *HubManager) handleDeviceConnection(portID byte, deviceType byte, data []byte) {
	log.Printf("Устройство подключено к порту %d, тип: 0x%02x (%s)",
		portID, deviceType, hm.getDeviceName(deviceType))

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

	// Настраиваем устройство в отдельной горутине с задержкой
	go func() {
		// Ждем перед настройкой, чтобы избежать конфликтов
		time.Sleep(1 * time.Second)

		log.Printf("Настройка устройства на порту %d (тип: 0x%02x)", portID, deviceType)

		// Настраиваем устройство в зависимости от типа
		err := hm.configureDevice(portID, deviceType)
		if err != nil {
			log.Printf("Ошибка настройки устройства на порту %d: %v", portID, err)
			// Не помечаем как отключенное, т.к. устройство может работать без настройки
		} else {
			log.Printf("Устройство на порту %d успешно настроено", portID)
		}

		// Уведомляем GUI
		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}
	}()

	log.Printf("Устройство обнаружено: %s (порт %d)", device.Name, portID)
}

// handleDeviceDisconnection обрабатывает отключение устройства
func (hm *HubManager) handleDeviceDisconnection(portID byte) {
	log.Printf("Устройство отключено от порта %d", portID)

	// Проверяем, существует ли устройство
	if device, exists := hm.devices[portID]; exists {
		// Обновляем состояние устройства
		device.IsConnected = false
		device.LastUpdate = time.Now()

		log.Printf("Устройство отключено: %s (порт %d)", device.Name, portID)

		// Уведомляем GUI об изменении
		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}
	} else {
		// Создаем информацию об отключенном устройстве
		device := &Device{
			PortID:      portID,
			IsConnected: false,
			LastUpdate:  time.Now(),
			Properties:  make(map[string]interface{}),
		}

		// Уведомляем GUI
		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}
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

	if !hm.isConnected {
		hm.connectionMutex.RUnlock()
		return fmt.Errorf("не подключено к хабу")
	}

	char, exists := hm.characteristics[uuid]
	if !exists {
		hm.connectionMutex.RUnlock()
		return fmt.Errorf("характеристика %s не найдена", uuid)
	}

	// Проверяем, не отключился ли хаб во время операции
	if !hm.isConnected {
		hm.connectionMutex.RUnlock()
		return fmt.Errorf("потеряно подключение к хабу")
	}

	_, err := char.WriteWithoutResponse(data)
	hm.connectionMutex.RUnlock()

	if err != nil {
		log.Printf("Ошибка отправки данных: %v", err)
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

// autoDetectDevicesV2 - улучшенная функция обнаружения устройств
func (hm *HubManager) autoDetectDevicesV2() {
	log.Println("=== Автоматическое обнаружение устройств ===")

	if !hm.IsConnected() {
		log.Println("Не подключено к хабу, пропускаем обнаружение")
		return
	}

	// Ждем, чтобы уведомления о портах успели прийти
	log.Println("Ожидание уведомлений о подключенных устройствах...")
	time.Sleep(5 * time.Second)

	// Проверяем, какие устройства уже обнаружены через уведомления
	log.Println("Проверка обнаруженных устройств:")
	for port := byte(1); port <= 6; port++ {
		if device, exists := hm.devices[port]; exists && device.IsConnected {
			log.Printf("  Порт %d: %s", port, device.Name)
		}
	}

	// Если какие-то порты (1, 2, 6) не обнаружены, пытаемся обнаружить их вручную
	portsToCheck := []byte{1, 2, 6}

	for _, portID := range portsToCheck {
		if _, exists := hm.devices[portID]; !exists {
			log.Printf("Порт %d не обнаружен автоматически, запускаем ручное обнаружение...", portID)
			hm.manualDeviceDetection(portID)
			time.Sleep(3 * time.Second)
		}
	}

	log.Println("=== Обнаружение устройств завершено ===")
}

// manualDeviceDetection ручное обнаружение устройства на порту
func (hm *HubManager) manualDeviceDetection(portID byte) {
	log.Printf("Ручное обнаружение на порту %d", portID)

	// Проверяем наиболее вероятные устройства для данного порта
	if portID == 6 {
		// Порт 6 - встроенный RGB светодиод
		hm.detectBuiltInLED()
		return
	}

	// Для портов 1 и 2 пробуем разные типы устройств
	deviceTypes := []struct {
		name       string
		deviceType byte
		setupCmd   []byte
	}{
		{"Мотор", DEVICE_TYPE_MOTOR, []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}},
		{"Датчик наклона", DEVICE_TYPE_TILT_SENSOR, []byte{0x01, 0x02, portID, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}},
		{"Датчик расстояния", DEVICE_TYPE_MOTION_SENSOR, []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}},
	}

	for _, dev := range deviceTypes {
		log.Printf("Порт %d: проверка %s...", portID, dev.name)

		// Настраиваем устройство
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, dev.setupCmd)
		if err != nil {
			log.Printf("Порт %d: ошибка настройки %s - %v", portID, dev.name, err)
			continue
		}

		time.Sleep(2 * time.Second)

		// Для мотора тестируем запуск
		if dev.deviceType == DEVICE_TYPE_MOTOR {
			runCmd := []byte{portID, 0x01, 0x01, 0x05} // Очень низкая скорость
			err = hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, runCmd)
			if err != nil {
				log.Printf("Порт %d: не удалось запустить мотор - %v", portID, err)
				continue
			}

			time.Sleep(300 * time.Millisecond)

			// Останавливаем мотор
			stopCmd := []byte{portID, 0x01, 0x01, 0x00}
			hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
		}

		// Регистрируем устройство
		device := &Device{
			PortID:      portID,
			DeviceType:  dev.deviceType,
			Name:        dev.name,
			IsConnected: true,
			LastUpdate:  time.Now(),
			Properties:  make(map[string]interface{}),
		}

		hm.devices[portID] = device

		// Уведомляем GUI
		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}

		log.Printf("Порт %d: обнаружен %s", portID, dev.name)
		return // Успешно обнаружили устройство
	}

	log.Printf("Порт %d: устройства не обнаружены", portID)
}

// isPortOccupied проверяет, занят ли порт
func (hm *HubManager) isPortOccupied(portID byte) bool {
	if device, exists := hm.devices[portID]; exists {
		return device.IsConnected
	}
	return false
}

func (hm *HubManager) safeDetectPort(portID byte) {
	log.Printf("Безопасное обнаружение на порту %d", portID)

	// Проверяем, не обнаружили ли мы уже устройство через уведомления
	if device, exists := hm.devices[portID]; exists && device.IsConnected {
		log.Printf("Порт %d уже занят устройством: %s", portID, device.Name)
		return
	}

	// Список типов устройств для проверки (в порядке приоритета)
	deviceTypes := []struct {
		name       string
		deviceType byte
		setupCmd   []byte
		isSensor   bool
	}{
		// Для порта 1 сначала пробуем мотор
		{"Мотор", DEVICE_TYPE_MOTOR, []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}, false},
		{"Датчик наклона", DEVICE_TYPE_TILT_SENSOR, []byte{0x01, 0x02, portID, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}, true},
		{"Датчик расстояния", DEVICE_TYPE_MOTION_SENSOR, []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}, true},
	}

	for _, dev := range deviceTypes {
		log.Printf("Порт %d: проверка %s...", portID, dev.name)

		// Отправляем команду настройки
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, dev.setupCmd)
		if err != nil {
			log.Printf("Порт %d: ошибка настройки %s - %v", portID, dev.name, err)
			time.Sleep(2 * time.Second) // Ждем перед следующей попыткой
			continue
		}

		time.Sleep(2 * time.Second) // Даем время на настройку

		if dev.deviceType == DEVICE_TYPE_MOTOR {
			// Для мотора пробуем запустить и остановить
			runCmd := []byte{portID, 0x01, 0x01, 0x10} // Минимальная скорость
			err = hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, runCmd)
			if err != nil {
				log.Printf("Порт %d: ошибка запуска мотора - %v", portID, err)
				continue
			}

			time.Sleep(500 * time.Millisecond)

			// Останавливаем
			stopCmd := []byte{portID, 0x01, 0x01, 0x00}
			hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)

			log.Printf("Порт %d: обнаружен мотор", portID)
		} else {
			// Для датчиков читаем значение
			time.Sleep(1 * time.Second)
			data, err := hm.ReadCharacteristic(SENSOR_VALUES_UUID)
			if err == nil && len(data) >= 4 && data[1] == portID {
				log.Printf("Порт %d: обнаружен %s, данные: %x", portID, dev.name, data)
			} else {
				log.Printf("Порт %d: %s не отвечает", portID, dev.name)
				continue
			}
		}

		// Успешно обнаружили устройство
		device := &Device{
			PortID:      portID,
			DeviceType:  dev.deviceType,
			Name:        dev.name,
			IsConnected: true,
			LastUpdate:  time.Now(),
			Properties:  make(map[string]interface{}),
		}

		hm.devices[portID] = device

		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}

		return // Нашли устройство, выходим
	}

	log.Printf("Порт %d: устройства не обнаружены", portID)
}

// detectBuiltInLED проверяет встроенный RGB светодиод на порту 6
func (hm *HubManager) detectBuiltInLED() {
	log.Println("Обнаружение встроенного RGB светодиода на порту 6...")

	// Настраиваем светодиод в режиме RGB
	setupCmd := []byte{0x01, 0x02, 6, 0x17, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
	if err != nil {
		log.Printf("Порт 6: ошибка настройки RGB режима - %v", err)
		// Пробуем индексный режим
		setupCmd = []byte{0x01, 0x02, 6, 0x17, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
	}

	time.Sleep(1 * time.Second)

	// Включаем зеленый цвет
	colorCmd := []byte{0x06, 0x04, 0x03, 0x00, 0xFF, 0x00}
	err = hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, colorCmd)
	if err != nil {
		log.Printf("Порт 6: ошибка установки цвета - %v", err)
		return
	}

	// Регистрируем устройство
	device := &Device{
		PortID:      6,
		DeviceType:  DEVICE_TYPE_RGB_LIGHT,
		Name:        "RGB светодиод",
		IsConnected: true,
		LastUpdate:  time.Now(),
		Properties:  make(map[string]interface{}),
	}

	hm.devices[6] = device

	log.Println("Порт 6: RGB светодиод обнаружен (зеленый)")

	// Уведомляем GUI
	if hm.deviceUpdateCallback != nil {
		hm.deviceUpdateCallback(6, device)
	}
}

// smartDetectPort умное обнаружение устройств на порту
func (hm *HubManager) smartDetectPort(portID byte) {
	log.Printf("Умное обнаружение на порту %d...", portID)

	// Сначала запрашиваем информацию о порте
	hm.requestPortInfoDirect()
	time.Sleep(300 * time.Millisecond)

	// Пробуем разные типы устройств в правильном порядке
	deviceTypes := []struct {
		name       string
		deviceType byte
		setupCmd   []byte
		testCmd    []byte
	}{
		{
			name:       "Датчик наклона",
			deviceType: DEVICE_TYPE_TILT_SENSOR,
			setupCmd:   []byte{0x01, 0x02, portID, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01},
			testCmd:    nil, // Датчики не требуют тестовой команды
		},
		{
			name:       "Датчик расстояния",
			deviceType: DEVICE_TYPE_MOTION_SENSOR,
			setupCmd:   []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01},
			testCmd:    nil,
		},
		{
			name:       "Мотор",
			deviceType: DEVICE_TYPE_MOTOR,
			setupCmd:   []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01},
			testCmd:    []byte{portID, 0x01, 0x01, 0x10}, // Минимальная скорость вперед
		},
		{
			name:       "Пищалка",
			deviceType: DEVICE_TYPE_PIEZO_TONE,
			setupCmd:   []byte{0x01, 0x02, portID, 0x16, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01},
			testCmd:    []byte{portID, 0x02, 0x04, 0xB8, 0x01, 0xE8, 0x03}, // Тон 440 Гц, 1000 мс
		},
	}

	for _, deviceType := range deviceTypes {
		log.Printf("Порт %d: проверка на %s...", portID, deviceType.name)

		// Настраиваем устройство
		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, deviceType.setupCmd)
		if err != nil {
			log.Printf("Порт %d: ошибка настройки %s - %v", portID, deviceType.name, err)
			continue
		}

		time.Sleep(500 * time.Millisecond) // Даем время на настройку

		// Для моторов и пищалок отправляем тестовую команду
		if deviceType.testCmd != nil {
			err = hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, deviceType.testCmd)
			if err != nil {
				log.Printf("Порт %d: ошибка теста %s - возможно, не %s", portID, deviceType.name, deviceType.name)

				// Для мотора останавливаем, если был запущен
				if deviceType.deviceType == DEVICE_TYPE_MOTOR {
					hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, []byte{portID, 0x01, 0x01, 0x00})
				}
				continue
			}

			// Ждем и останавливаем тестовое действие
			time.Sleep(300 * time.Millisecond)
			if deviceType.deviceType == DEVICE_TYPE_MOTOR {
				hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, []byte{portID, 0x01, 0x01, 0x00})
			} else if deviceType.deviceType == DEVICE_TYPE_PIEZO_TONE {
				hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, []byte{portID, 0x03, 0x00})
			}
		}

		// Для датчиков проверяем ответ
		if deviceType.deviceType == DEVICE_TYPE_TILT_SENSOR || deviceType.deviceType == DEVICE_TYPE_MOTION_SENSOR {
			time.Sleep(300 * time.Millisecond)
			data, err := hm.ReadCharacteristic(SENSOR_VALUES_UUID)
			if err == nil && len(data) >= 4 && data[1] == portID {
				log.Printf("Порт %d: %s отвечает, данные: %x", portID, deviceType.name, data)
			} else {
				log.Printf("Порт %d: %s не отвечает", portID, deviceType.name)
				continue
			}
		}

		// Успешно обнаружили устройство
		log.Printf("Порт %d: обнаружен %s", portID, deviceType.name)

		device := &Device{
			PortID:      portID,
			DeviceType:  deviceType.deviceType,
			Name:        deviceType.name,
			IsConnected: true,
			LastUpdate:  time.Now(),
			Properties:  make(map[string]interface{}),
		}

		hm.devices[portID] = device

		// Уведомляем GUI
		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}

		break // Прерываем цикл, если нашли устройство
	}
}

// requestPortInfoDirect запрашивает информацию о порте напрямую
func (hm *HubManager) requestPortInfoDirect() {
	// Отправляем команду запроса информации о портах
	cmd := []byte{0x01, 0x00}
	hm.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)
}

// detectMotor пытается обнаружить мотор на порту
func (hm *HubManager) detectMotor(portID byte) {
	log.Printf("Проверка порта %d на наличие мотора...", portID)

	// Настраиваем мотор
	setupCmd := []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)

	if err != nil {
		log.Printf("Порт %d: не удалось настроить мотор", portID)
		return
	}

	log.Printf("Порт %d: мотор настроен", portID)

	// Регистрируем мотор
	hm.registerDevice(portID, DEVICE_TYPE_MOTOR, "Мотор")
}

// detectSensor пытается обнаружить датчик на порту
func (hm *HubManager) detectSensor(portID byte) {
	log.Printf("Проверка порта %d на наличие датчика...", portID)

	// Пробуем датчик расстояния
	setupCmd := []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)

	if err == nil {
		log.Printf("Порт %d: датчик расстояния настроен", portID)
		hm.registerDevice(portID, DEVICE_TYPE_MOTION_SENSOR, "Датчик расстояния")
		return
	}

	// Пробуем датчик наклона
	setupCmd = []byte{0x01, 0x02, portID, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	err = hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)

	if err == nil {
		log.Printf("Порт %d: датчик наклона настроен", portID)
		hm.registerDevice(portID, DEVICE_TYPE_TILT_SENSOR, "Датчик наклона")
		return
	}

	log.Printf("Порт %d: датчик не обнаружен", portID)
}

// readDeviceData читает данные с устройства
func (hm *HubManager) readDeviceData(portID byte, deviceType byte) {
	log.Printf("Чтение данных с устройства на порту %d (тип: 0x%02x)", portID, deviceType)

	// В зависимости от типа устройства, читаем данные
	switch deviceType {
	case DEVICE_TYPE_MOTION_SENSOR:
		// Для датчика расстояния читаем значение
		hm.readDistanceSensorValue(portID)
	case DEVICE_TYPE_TILT_SENSOR:
		// Для датчика наклона читаем значение
		hm.readTiltSensorValue(portID)
	case DEVICE_TYPE_VOLTAGE:
		// Для датчика напряжения читаем значение
		hm.readVoltageSensorValue(portID)
	case DEVICE_TYPE_CURRENT:
		// Для датчика тока читаем значение
		hm.readCurrentSensorValue(portID)
	default:
		// Для других устройств просто читаем сырые данные
		hm.readRawSensorData(portID)
	}
}

// readDistanceSensorValue читает значение датчика расстояния
func (hm *HubManager) readDistanceSensorValue(portID byte) {
	// Настраиваем датчик расстояния на режим измерения расстояния (если еще не настроен)
	setupCmd := []byte{0x01, 0x02, portID, 0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	_ = hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)

	// Ждем немного
	time.Sleep(200 * time.Millisecond)

	// Читаем значение
	data, err := hm.ReadCharacteristic(SENSOR_VALUES_UUID)
	if err != nil {
		log.Printf("Ошибка чтения датчика расстояния на порту %d: %v", portID, err)
		return
	}

	if len(data) >= 4 && data[1] == portID {
		// Значение датчика расстояния (обычно один байт)
		value := data[3]
		log.Printf("Датчик расстояния на порту %d: %d см", portID, value)

		// Обновляем значение в устройстве
		if device, exists := hm.devices[portID]; exists {
			device.LastValue = value
			device.LastUpdate = time.Now()

			// Уведомляем GUI
			if hm.deviceUpdateCallback != nil {
				hm.deviceUpdateCallback(portID, device)
			}
		}
	}
}

// readTiltSensorValue читает значение датчика наклона
func (hm *HubManager) readTiltSensorValue(portID byte) {
	// Настраиваем датчик наклона на режим определения наклона
	setupCmd := []byte{0x01, 0x02, portID, 0x22, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	_ = hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)

	time.Sleep(200 * time.Millisecond)

	data, err := hm.ReadCharacteristic(SENSOR_VALUES_UUID)
	if err != nil {
		log.Printf("Ошибка чтения датчика наклона на порту %d: %v", portID, err)
		return
	}

	if len(data) >= 4 && data[1] == portID {
		value := data[3]
		log.Printf("Датчик наклона на порту %d: %d", portID, value)

		if device, exists := hm.devices[portID]; exists {
			device.LastValue = value
			device.LastUpdate = time.Now()

			if hm.deviceUpdateCallback != nil {
				hm.deviceUpdateCallback(portID, device)
			}
		}
	}
}

// readVoltageSensorValue читает значение датчика напряжения
func (hm *HubManager) readVoltageSensorValue(portID byte) {
	// Настраиваем датчик напряжения
	setupCmd := []byte{0x01, 0x02, portID, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	_ = hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)

	time.Sleep(200 * time.Millisecond)

	data, err := hm.ReadCharacteristic(SENSOR_VALUES_UUID)
	if err != nil {
		log.Printf("Ошибка чтения датчика напряжения на порту %d: %v", portID, err)
		return
	}

	if len(data) >= 4 && data[1] == portID {
		value := data[3]
		log.Printf("Датчик напряжения на порту %d: %d мВ", portID, value)

		if device, exists := hm.devices[portID]; exists {
			device.LastValue = value
			device.LastUpdate = time.Now()

			if hm.deviceUpdateCallback != nil {
				hm.deviceUpdateCallback(portID, device)
			}
		}
	}
}

// readCurrentSensorValue читает значение датчика тока
func (hm *HubManager) readCurrentSensorValue(portID byte) {
	// Настраиваем датчик тока
	setupCmd := []byte{0x01, 0x02, portID, 0x15, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	_ = hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)

	time.Sleep(200 * time.Millisecond)

	data, err := hm.ReadCharacteristic(SENSOR_VALUES_UUID)
	if err != nil {
		log.Printf("Ошибка чтения датчика тока на порту %d: %v", portID, err)
		return
	}

	if len(data) >= 4 && data[1] == portID {
		value := data[3]
		log.Printf("Датчик тока на порту %d: %d мА", portID, value)

		if device, exists := hm.devices[portID]; exists {
			device.LastValue = value
			device.LastUpdate = time.Now()

			if hm.deviceUpdateCallback != nil {
				hm.deviceUpdateCallback(portID, device)
			}
		}
	}
}

// readRawSensorData читает сырые данные с датчика
func (hm *HubManager) readRawSensorData(portID byte) {
	data, err := hm.ReadCharacteristic(SENSOR_VALUES_UUID)
	if err != nil {
		log.Printf("Ошибка чтения сырых данных с порта %d: %v", portID, err)
		return
	}

	if len(data) > 0 {
		log.Printf("Сырые данные с порта %d: %x", portID, data)

		if device, exists := hm.devices[portID]; exists {
			device.LastValue = data
			device.LastUpdate = time.Now()

			if hm.deviceUpdateCallback != nil {
				hm.deviceUpdateCallback(portID, device)
			}
		}
	}
}

// isPortExternal проверяет, является ли порт внешним
func isPortExternal(portID byte) bool {
	return portID == 1 || portID == 2 || portID == 6
}

// forceDetectMotor - принудительное обнаружение мотора на порту
func (hm *HubManager) forceDetectMotor(portID byte) bool {
	log.Printf("Принудительное обнаружение мотора на порту %d", portID)

	// 1. Настройка мотора
	setupCmd := []byte{0x01, 0x02, portID, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
	if err != nil {
		log.Printf("Порт %d: ошибка настройки мотора - %v", portID, err)
		return false
	}

	time.Sleep(3 * time.Second)

	// 2. Запуск мотора на очень низкой скорости
	runCmd := []byte{portID, 0x01, 0x01, 0x05} // Очень низкая скорость
	err = hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, runCmd)
	if err != nil {
		log.Printf("Порт %d: ошибка запуска мотора - %v", portID, err)
		return false
	}

	time.Sleep(500 * time.Millisecond)

	// 3. Остановка мотора
	stopCmd := []byte{portID, 0x01, 0x01, 0x00}
	hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)

	// 4. Регистрация мотора
	device := &Device{
		PortID:      portID,
		DeviceType:  DEVICE_TYPE_MOTOR,
		Name:        "Мотор",
		IsConnected: true,
		LastUpdate:  time.Now(),
		Properties:  make(map[string]interface{}),
	}

	hm.devices[portID] = device

	if hm.deviceUpdateCallback != nil {
		hm.deviceUpdateCallback(portID, device)
	}

	log.Printf("Порт %d: мотор обнаружен принудительно", portID)
	return true
}

// parseWeDoPortMessageCorrect правильный парсинг сообщений WeDo 2.0
func parseWeDoPortMessageCorrect(data []byte) (portID byte, isConnected bool, hubID byte, deviceType byte) {
	if len(data) < 4 {
		log.Printf("Сообщение слишком короткое: %x", data)
		return 0, false, 0, 0
	}

	// Формат: [PortID, ConnectionEvent, HubID, DeviceType, ...]
	portID = data[0]
	connectionEvent := data[1]
	hubID = data[2]
	deviceType = data[3]

	isConnected = (connectionEvent == 0x01)

	log.Printf("Парсинг сообщения: порт=%d, подключено=%v, хаб=%d, тип=0x%02x",
		portID, isConnected, hubID, deviceType)

	return portID, isConnected, hubID, deviceType
}

// handlePortNotification обрабатывает уведомления о портах
func (hm *HubManager) handlePortNotification(data []byte) {
	if len(data) < 2 {
		log.Printf("Сообщение слишком короткое: %x", data)
		return
	}

	// Логируем все сообщения для отладки
	log.Printf("Обработка порта: данные=%x, длина=%d", data, len(data))

	// Обрабатываем разные форматы сообщений
	if len(data) == 2 {
		// Формат: [PortID, 0x00] - отключение устройства (2 байта)
		portID := data[0]
		eventType := data[1]

		if eventType == 0x00 {
			log.Printf("Короткое сообщение об отключении: порт %d", portID)

			// Фильтруем порты
			if !isExternalPort(portID) {
				log.Printf("Игнорируем внутренний порт %d", portID)
				return
			}

			hm.handleDeviceDisconnection(portID)
		} else {
			log.Printf("Неизвестное короткое сообщение: порт=%d, событие=0x%02x", portID, eventType)
		}
	} else if len(data) >= 4 {
		// Формат: [PortID, ConnectionEvent, HubID, DeviceType, ...] - подключение (>=4 байта)
		portID := data[0]
		connectionEvent := data[1]
		hubID := data[2]
		deviceType := data[3]

		log.Printf("Длинное сообщение: порт=%d, событие=0x%02x, хаб=%d, тип=0x%02x",
			portID, connectionEvent, hubID, deviceType)

		// Фильтруем порты
		if !isExternalPort(portID) {
			log.Printf("Игнорируем внутренний порт %d", portID)
			return
		}

		if connectionEvent == 0x01 {
			// Подключение устройства
			if deviceType == 0x00 {
				log.Printf("Порт %d: устройство подключено, но тип неизвестен (0x00)", portID)
				return
			}

			// Преобразуем тип устройства
			mappedDeviceType := hm.mapDeviceType(deviceType)
			if mappedDeviceType == 0x00 {
				log.Printf("Порт %d: неизвестный тип устройства 0x%02x", portID, deviceType)
				return
			}

			log.Printf("Порт %d: подключено устройство типа 0x%02x (%s)",
				portID, mappedDeviceType, hm.getDeviceName(mappedDeviceType))

			hm.handleDeviceConnection(portID, mappedDeviceType, data)
		} else if connectionEvent == 0x00 {
			// Отключение устройства (длинный формат)
			log.Printf("Порт %d: устройство отключено (длинный формат)", portID)
			hm.handleDeviceDisconnection(portID)
		} else {
			log.Printf("Порт %d: неизвестное событие 0x%02x", portID, connectionEvent)
		}
	} else {
		log.Printf("Неизвестный формат сообщения: %x", data)
	}
}

// isExternalPort проверяет, является ли порт внешним (1, 2, 6)
func isExternalPort(portID byte) bool {
	return portID == 1 || portID == 2 || portID == 6
}

// mapDeviceType преобразует WeDo 2.0 тип устройства в наш формат
func (hm *HubManager) mapDeviceType(deviceType byte) byte {
	switch deviceType {
	case 0x01:
		return DEVICE_TYPE_MOTOR
	case 0x22:
		return DEVICE_TYPE_TILT_SENSOR
	case 0x23:
		return DEVICE_TYPE_MOTION_SENSOR
	case 0x17:
		return DEVICE_TYPE_RGB_LIGHT
	case 0x16:
		return DEVICE_TYPE_PIEZO_TONE
	case 0x14:
		return DEVICE_TYPE_VOLTAGE
	case 0x15:
		return DEVICE_TYPE_CURRENT
	default:
		return 0x00 // Неизвестный тип
	}
}
