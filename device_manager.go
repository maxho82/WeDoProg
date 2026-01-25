package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DeviceManager управляет устройствами хаба
type DeviceManager struct {
	hubMgr    *HubManager
	devices   map[byte]*Device
	devicesMu sync.RWMutex

	// Callback для обновлений GUI
	deviceChangedCallback func(portID byte, device *Device)
}

// NewDeviceManager создает менеджер устройств
func NewDeviceManager(hubMgr *HubManager) *DeviceManager {
	return &DeviceManager{
		hubMgr:  hubMgr,
		devices: make(map[byte]*Device),
	}
}

// AddOrUpdateDevice добавляет или обновляет устройство
func (dm *DeviceManager) AddOrUpdateDevice(device *Device) {
	dm.devicesMu.Lock()
	defer dm.devicesMu.Unlock()

	dm.devices[device.PortID] = device

	// Уведомляем об изменении
	if dm.deviceChangedCallback != nil {
		dm.deviceChangedCallback(device.PortID, device)
	}
}

// GetDevice возвращает устройство по порту
func (dm *DeviceManager) GetDevice(portID byte) (*Device, bool) {
	dm.devicesMu.RLock()
	defer dm.devicesMu.RUnlock()

	device, exists := dm.devices[portID]
	return device, exists
}

// GetConnectedDevices возвращает список подключенных устройств
func (dm *DeviceManager) GetConnectedDevices() []*Device {
	dm.devicesMu.RLock()
	defer dm.devicesMu.RUnlock()

	var connected []*Device
	for _, device := range dm.devices {
		if device.IsConnected {
			connected = append(connected, device)
		}
	}

	return connected
}

// GetDevicesByType возвращает устройства определенного типа
func (dm *DeviceManager) GetDevicesByType(deviceType byte) []*Device {
	dm.devicesMu.RLock()
	defer dm.devicesMu.RUnlock()

	var filtered []*Device
	for _, device := range dm.devices {
		if device.DeviceType == deviceType && device.IsConnected {
			filtered = append(filtered, device)
		}
	}

	return filtered
}

// SetMotorPower устанавливает мощность мотора
func (dm *DeviceManager) SetMotorPower(portID byte, power int8, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Сначала проверяем в своем хранилище
	device, exists := dm.GetDevice(portID)

	// Если не нашли, проверяем в HubManager
	if !exists && dm.hubMgr != nil {
		if hubDevice, hubExists := dm.hubMgr.GetDeviceFromPort(portID); hubExists {
			device = hubDevice
			exists = true
			// Сохраняем для будущего использования
			dm.AddOrUpdateDevice(device)
		}
	}

	if !exists {
		log.Printf("Устройство на порту %d не найдено ни в DeviceManager, ни в HubManager", portID)
		// Пытаемся выполнить команду даже если устройство не найдено
		log.Printf("Пытаемся выполнить команду для порта %d без проверки устройства", portID)
	}

	if exists && !device.IsConnected {
		log.Printf("Устройство на порту %d существует, но не подключено", portID)
		// Все равно пытаемся выполнить команду
	}

	// Преобразуем мощность в байт
	var speedByte byte
	powerFloat := float64(power) / 100.0

	if powerFloat < 0 {
		speedByte = byte(int(0x54*powerFloat) + 0xF0)
	} else if powerFloat > 0 {
		speedByte = byte(int(0x54*powerFloat) + 0x10)
	} else {
		speedByte = 0x00
	}

	cmd := []byte{portID, 0x01, 0x01, speedByte}

	log.Printf("Установка мощности мотора на порту %d: %d%% (байт: 0x%02x)", portID, power, speedByte)

	err := dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", cmd)

	if err != nil {
		return err
	}

	// Если есть длительность, ждем ее завершения
	if duration > 0 {
		log.Printf("Мотор на порту %d будет работать %d мс", portID, duration)

		// Создаем канал для синхронизации
		done := make(chan bool)

		go func() {
			time.Sleep(time.Duration(duration) * time.Millisecond)
			stopCmd := []byte{portID, 0x01, 0x01, 0x00}
			dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", stopCmd)
			log.Printf("Мотор на порту %d автоматически остановлен после %d мс", portID, duration)
			done <- true
		}()

		// Ждем завершения в отдельной горутине, чтобы не блокировать основной поток
		// для тестового режима
		return nil
	}

	return nil
}

// Новая функция: SetMotorPowerAndWait - с ожиданием завершения
func (dm *DeviceManager) SetMotorPowerAndWait(portID byte, power int8, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Преобразуем мощность в байт
	var speedByte byte
	powerFloat := float64(power) / 100.0

	if powerFloat < 0 {
		speedByte = byte(int(0x54*powerFloat) + 0xF0)
	} else if powerFloat > 0 {
		speedByte = byte(int(0x54*powerFloat) + 0x10)
	} else {
		speedByte = 0x00
	}

	cmd := []byte{portID, 0x01, 0x01, speedByte}

	log.Printf("Установка мощности мотора на порту %d: %d%% на %d мс", portID, power, duration)

	err := dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", cmd)

	if err != nil {
		return err
	}

	// Если есть длительность, ждем ее завершения СИНХРОННО
	if duration > 0 {
		log.Printf("Мотор на порту %d работает %d мс...", portID, duration)
		time.Sleep(time.Duration(duration) * time.Millisecond)

		// Останавливаем мотор
		stopCmd := []byte{portID, 0x01, 0x01, 0x00}
		err = dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", stopCmd)
		if err != nil {
			log.Printf("Ошибка остановки мотора на порту %d: %v", portID, err)
		}
		log.Printf("Мотор на порту %d остановлен", portID)
	}

	return nil
}

// SetLEDColor устанавливает цвет светодиода
func (dm *DeviceManager) SetLEDColor(portID byte, red, green, blue byte) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Сначала проверяем в своем хранилище
	device, exists := dm.GetDevice(portID)

	// Если не нашли, проверяем в HubManager
	if !exists && dm.hubMgr != nil {
		if hubDevice, hubExists := dm.hubMgr.GetDeviceFromPort(portID); hubExists {
			device = hubDevice
			exists = true
			// Сохраняем для будущего использования
			dm.AddOrUpdateDevice(device)
		}
	}

	if !exists {
		log.Printf("Устройство на порту %d не найдено ни в DeviceManager, ни в HubManager", portID)
		// Для порта 6 (встроенного светодиода) продолжаем без проверки
		if portID != 6 {
			return fmt.Errorf("устройство на порту %d не найдено", portID)
		}
		log.Printf("Используем встроенный светодиод на порту 6")
	} else if !device.IsConnected {
		log.Printf("Устройство на порту %d существует, но не подключено", portID)
		// Для порта 6 все равно продолжаем
		if portID != 6 {
			return fmt.Errorf("устройство на порту %d не подключено", portID)
		}
	} else if device.DeviceType != DEVICE_TYPE_RGB_LIGHT && portID != 6 {
		log.Printf("Устройство на порту %d имеет тип %v (0x%02x), ожидается светодиод (0x%02x)",
			portID, device.Name, device.DeviceType, DEVICE_TYPE_RGB_LIGHT)
		// Для порта 6 игнорируем проверку типа
		if portID != 6 {
			return fmt.Errorf("устройство на порту %d не является светодиодом", portID)
		}
	}

	// Настраиваем режим RGB (если нужно)
	modeCmd := []byte{0x01, 0x02, portID, 0x17, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	if err := dm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", modeCmd); err != nil {
		log.Printf("Предупреждение при установке режима светодиода: %v", err)
		// Пробуем альтернативный режим
		modeCmd = []byte{0x01, 0x02, portID, 0x17, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
		dm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", modeCmd)
	}

	// Устанавливаем цвет
	colorCmd := []byte{0x06, 0x04, 0x03, red, green, blue}

	log.Printf("Установка цвета светодиода на порту %d: RGB(%d,%d,%d)", portID, red, green, blue)
	return dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", colorCmd)
}

// PlayTone воспроизводит тон на пищалке
func (dm *DeviceManager) PlayTone(portID byte, frequency uint16, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверяем, подключена ли пищалка
	device, exists := dm.GetDevice(portID)
	if !exists || !device.IsConnected || device.DeviceType != 0x16 {
		return fmt.Errorf("пищалка не подключена к порту %d", portID)
	}

	// Формируем команду
	freqLow := byte(frequency & 0xFF)
	freqHigh := byte((frequency >> 8) & 0xFF)
	durLow := byte(duration & 0xFF)
	durHigh := byte((duration >> 8) & 0xFF)

	cmd := []byte{
		portID,   // connectId
		0x02,     // commandId
		0x04,     // dataLength
		freqLow,  // frequency low byte
		freqHigh, // frequency high byte
		durLow,   // duration low byte
		durHigh,  // duration high byte
	}

	log.Printf("Проигрывание тона на порту %d: частота=%d Гц, длительность=%d мс", portID, frequency, duration)
	return dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", cmd)
}

// StopTone останавливает пищалку
func (dm *DeviceManager) StopTone(portID byte) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	cmd := []byte{
		portID, // connectId
		0x03,   // commandId
		0x00,   // dataLength
	}

	log.Printf("Остановка пищалки на порту %d", portID)
	return dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", cmd)
}

// SetDeviceChangedCallback устанавливает callback для обновлений
func (dm *DeviceManager) SetDeviceChangedCallback(callback func(portID byte, device *Device)) {
	dm.deviceChangedCallback = callback
}

// UpdateDeviceValue обновляет значение устройства
func (dm *DeviceManager) UpdateDeviceValue(portID byte, value interface{}) {
	dm.devicesMu.Lock()
	defer dm.devicesMu.Unlock()

	if device, exists := dm.devices[portID]; exists {
		device.LastValue = value
		device.LastUpdate = time.Now()

		// Уведомляем об изменении
		if dm.deviceChangedCallback != nil {
			dm.deviceChangedCallback(portID, device)
		}
	}
}

// SyncDevices синхронизирует устройства с HubManager
func (dm *DeviceManager) SyncDevices() {
	if dm.hubMgr == nil {
		return
	}

	log.Println("Синхронизация устройств между HubManager и DeviceManager...")

	// Получаем все устройства из HubManager
	for portID := byte(1); portID <= 6; portID++ {
		if device, exists := dm.hubMgr.GetDeviceFromPort(portID); exists {
			dm.AddOrUpdateDevice(device)
			log.Printf("Синхронизировано устройство на порту %d: %s", portID, device.Name)
		}
	}
}

// ForceDetectAllDevices принудительно обнаруживает все устройства
func (dm *DeviceManager) ForceDetectAllDevices() {
	if dm.hubMgr == nil || !dm.hubMgr.IsConnected() {
		return
	}

	log.Println("Принудительное обнаружение всех устройств...")
	dm.hubMgr.autoDetectDevicesV2()

	// Ждем и синхронизируем
	time.Sleep(3 * time.Second)
	dm.SyncDevices()
}

// device_manager.go - добавляем функцию PlayToneAndWait
func (dm *DeviceManager) PlayToneAndWait(portID byte, frequency uint16, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверяем, подключена ли пищалка
	device, exists := dm.GetDevice(portID)
	if !exists && dm.hubMgr != nil {
		if hubDevice, hubExists := dm.hubMgr.GetDeviceFromPort(portID); hubExists {
			device = hubDevice
			exists = true
			dm.AddOrUpdateDevice(device)
		}
	}

	// Формируем команду
	freqLow := byte(frequency & 0xFF)
	freqHigh := byte((frequency >> 8) & 0xFF)
	durLow := byte(duration & 0xFF)
	durHigh := byte((duration >> 8) & 0xFF)

	cmd := []byte{
		portID,   // connectId
		0x02,     // commandId
		0x04,     // dataLength
		freqLow,  // frequency low byte
		freqHigh, // frequency high byte
		durLow,   // duration low byte
		durHigh,  // duration high byte
	}

	log.Printf("Проигрывание тона на порту %d: частота=%d Гц, длительность=%d мс", portID, frequency, duration)

	err := dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", cmd)
	if err != nil {
		return err
	}

	// Ждем завершения звука СИНХРОННО
	if duration > 0 {
		log.Printf("Звук на порту %d воспроизводится %d мс...", portID, duration)
		time.Sleep(time.Duration(duration) * time.Millisecond)

		// Останавливаем звук (на всякий случай)
		stopCmd := []byte{portID, 0x03, 0x00}
		dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", stopCmd)
		log.Printf("Звук на порту %d завершен", portID)
	}

	return nil
}
