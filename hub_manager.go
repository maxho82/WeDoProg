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

	// Callback'и
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

			adapter.StopScan()
			cancel()
		}
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования: %v", err)
	}

	<-ctx.Done()
	hm.adapter.StopScan()

	log.Printf("Сканирование завершено. Найдено хабов: %d", len(foundHubs))
	return foundHubs, nil
}

// Connect подключается к хабу
func (hm *HubManager) Connect(address string) error {
	hm.connectionMutex.Lock()
	defer hm.connectionMutex.Unlock()

	if hm.isConnected {
		hm.Disconnect()
	}

	log.Printf("Подключение к хабу: %s", address)

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

	log.Printf("Устанавливаем соединение с %s...", address)
	device, err := hm.adapter.Connect(targetDevice.Address, tinybluetooth.ConnectionParams{})
	if err != nil {
		return fmt.Errorf("ошибка подключения: %v", err)
	}

	hm.device = device
	hm.deviceAddress = address
	hm.isConnected = true

	hm.hubInfo.Name = targetDevice.LocalName()
	hm.hubInfo.Address = address
	hm.hubInfo.LastUpdated = time.Now()

	log.Println("Обнаружение служб и характеристик...")
	err = hm.discoverAllServices()
	if err != nil {
		log.Printf("Предупреждение: %v", err)
	}

	log.Println("Чтение информации об устройстве...")
	go hm.readAllDeviceInfo()

	go hm.subscribeToImportantNotifications()

	if hm.connectionStateCallback != nil {
		hm.connectionStateCallback(true)
	}

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
					value = bytesToHexString(data)
					log.Printf("%s (HEX): %s", name, value)
				} else {
					value = strings.TrimSpace(string(data))
					log.Printf("%s: %s", name, value)
				}

				hm.updateHubInfo(uuid, value)
			}
		}
	}

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
	hm.subscribeToBatteryNotifications()
	hm.subscribeToPortNotifications()
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

// handlePortNotification обрабатывает уведомления о портах
func (hm *HubManager) handlePortNotification(data []byte) {
	if len(data) < 2 {
		log.Printf("Сообщение слишком короткое: %x", data)
		return
	}

	log.Printf("Обработка порта: данные=%x, длина=%d", data, len(data))

	if len(data) == 2 {
		portID := data[0]
		eventType := data[1]

		if eventType == 0x00 {
			log.Printf("Короткое сообщение об отключении: порт %d", portID)
			if isExternalPort(portID) {
				hm.handleDeviceDisconnection(portID)
			}
		}
	} else if len(data) >= 4 {
		portID := data[0]
		connectionEvent := data[1]
		hubID := data[2]
		deviceType := data[3]

		log.Printf("Длинное сообщение: порт=%d, событие=0x%02x, хаб=%d, тип=0x%02x",
			portID, connectionEvent, hubID, deviceType)

		if !isExternalPort(portID) {
			return
		}

		switch connectionEvent {
		case 0x01:
			if deviceType == 0x00 {
				log.Printf("Порт %d: устройство подключено, но тип неизвестен (0x00)", portID)
				return
			}

			mappedDeviceType := hm.mapDeviceType(deviceType)
			if mappedDeviceType == 0x00 {
				log.Printf("Порт %d: неизвестный тип устройства 0x%02x", portID, deviceType)
				return
			}

			log.Printf("Порт %d: подключено устройство типа 0x%02x (%s)",
				portID, mappedDeviceType, hm.getDeviceName(mappedDeviceType))

			hm.handleDeviceConnection(portID, mappedDeviceType, data)
		case 0x00:
			log.Printf("Порт %d: устройство отключено (длинный формат)", portID)
			hm.handleDeviceDisconnection(portID)
		}
	}
}

// handleDeviceConnection обрабатывает подключение устройства
func (hm *HubManager) handleDeviceConnection(portID byte, deviceType byte, _ []byte) {
	log.Printf("Устройство подключено к порту %d, тип: 0x%02x (%s)",
		portID, deviceType, hm.getDeviceName(deviceType))

	device := &Device{
		PortID:      portID,
		DeviceType:  deviceType,
		Name:        hm.getDeviceName(deviceType),
		IsConnected: true,
		LastUpdate:  time.Now(),
		Properties:  make(map[string]interface{}),
	}

	hm.devices[portID] = device

	go func() {
		time.Sleep(1 * time.Second)
		log.Printf("Настройка устройства на порту %d (тип: 0x%02x)", portID, deviceType)

		err := hm.configureDevice(portID, deviceType)
		if err != nil {
			log.Printf("Ошибка настройки устройства на порту %d: %v", portID, err)
		} else {
			log.Printf("Устройство на порту %d успешно настроено", portID)
		}

		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}
	}()

	log.Printf("Устройство обнаружено: %s (порт %d)", device.Name, portID)
}

// handleDeviceDisconnection обрабатывает отключение устройства
func (hm *HubManager) handleDeviceDisconnection(portID byte) {
	log.Printf("Устройство отключено от порта %d", portID)

	if device, exists := hm.devices[portID]; exists {
		device.IsConnected = false
		device.LastUpdate = time.Now()
		log.Printf("Устройство отключено: %s (порт %d)", device.Name, portID)

		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}
	} else {
		device := &Device{
			PortID:      portID,
			IsConnected: false,
			LastUpdate:  time.Now(),
			Properties:  make(map[string]interface{}),
		}

		if hm.deviceUpdateCallback != nil {
			hm.deviceUpdateCallback(portID, device)
		}
	}
}

// configureDevice настраивает устройство
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

	log.Println("Ожидание уведомлений о подключенных устройствах...")
	time.Sleep(5 * time.Second)

	log.Println("Проверка обнаруженных устройств:")
	for port := byte(1); port <= 6; port++ {
		if device, exists := hm.devices[port]; exists && device.IsConnected {
			log.Printf("  Порт %d: %s", port, device.Name)
		}
	}

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

	if portID == 6 {
		hm.detectBuiltInLED()
		return
	}

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

		err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, dev.setupCmd)
		if err != nil {
			log.Printf("Порт %d: ошибка настройки %s - %v", portID, dev.name, err)
			continue
		}

		time.Sleep(2 * time.Second)

		if dev.deviceType == DEVICE_TYPE_MOTOR {
			runCmd := []byte{portID, 0x01, 0x01, 0x05}
			err = hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, runCmd)
			if err != nil {
				log.Printf("Порт %d: не удалось запустить мотор - %v", portID, err)
				continue
			}

			time.Sleep(300 * time.Millisecond)
			stopCmd := []byte{portID, 0x01, 0x01, 0x00}
			hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
		}

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

		log.Printf("Порт %d: обнаружен %s", portID, dev.name)
		return
	}

	log.Printf("Порт %d: устройства не обнаружены", portID)
}

// detectBuiltInLED проверяет встроенный RGB светодиод
func (hm *HubManager) detectBuiltInLED() {
	log.Println("Обнаружение встроенного RGB светодиода на порту 6...")

	setupCmd := []byte{0x01, 0x02, 6, 0x17, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	err := hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
	if err != nil {
		log.Printf("Порт 6: ошибка настройки RGB режима - %v", err)
		setupCmd = []byte{0x01, 0x02, 6, 0x17, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		hm.WriteCharacteristic(INPUT_COMMAND_UUID, setupCmd)
	}

	time.Sleep(1 * time.Second)

	colorCmd := []byte{0x06, 0x04, 0x03, 0x00, 0xFF, 0x00}
	err = hm.WriteCharacteristic(OUTPUT_COMMAND_UUID, colorCmd)
	if err != nil {
		log.Printf("Порт 6: ошибка установки цвета - %v", err)
		return
	}

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

	if hm.deviceUpdateCallback != nil {
		hm.deviceUpdateCallback(6, device)
	}
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
		return 0x00
	}
}

// isExternalPort проверяет, является ли порт внешним
func isExternalPort(portID byte) bool {
	return portID == 1 || portID == 2 || portID == 6
}
