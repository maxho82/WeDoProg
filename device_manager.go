package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DeviceManager управляет устройствами хаба. Является единственным источником истины.
type DeviceManager struct {
	hubMgr    *HubManager
	devices   map[byte]*Device
	devicesMu sync.RWMutex

	deviceChangedCallback func(portID byte, device *Device)
}

func NewDeviceManager(hubMgr *HubManager) *DeviceManager {
	return &DeviceManager{
		hubMgr:  hubMgr,
		devices: make(map[byte]*Device),
	}
}

// AddOrUpdateDevice добавляет или обновляет устройство.
// Вызывается из callback'ов HubManager.
func (dm *DeviceManager) AddOrUpdateDevice(device *Device) {
	dm.devicesMu.Lock()
	defer dm.devicesMu.Unlock()

	dm.devices[device.PortID] = device

	if dm.deviceChangedCallback != nil {
		dm.deviceChangedCallback(device.PortID, device)
	}
}

// GetDevice возвращает устройство по порту.
func (dm *DeviceManager) GetDevice(portID byte) (*Device, bool) {
	dm.devicesMu.RLock()
	defer dm.devicesMu.RUnlock()

	device, exists := dm.devices[portID]
	return device, exists
}

// GetConnectedDevices возвращает список подключенных устройств.
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

// GetDevicesByType возвращает устройства определенного типа.
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

// SetMotorPower устанавливает мощность мотора.
func (dm *DeviceManager) SetMotorPower(portID byte, power int8, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверка наличия устройства (теперь только через собственное хранилище)
	device, exists := dm.GetDevice(portID)
	if !exists {
		log.Printf("Устройство на порту %d не найдено в DeviceManager", portID)
		// Всё равно пытаемся выполнить команду
	} else if !device.IsConnected {
		log.Printf("Устройство на порту %d существует, но не подключено", portID)
	}

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
	log.Printf("Установка мощности мотора на порту %d: %d%%", portID, power)

	err := dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
	if err != nil {
		return err
	}

	if duration > 0 {
		log.Printf("Мотор на порту %d будет работать %d мс", portID, duration)
		go func() {
			time.Sleep(time.Duration(duration) * time.Millisecond)
			stopCmd := []byte{portID, 0x01, 0x01, 0x00}
			dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
			log.Printf("Мотор на порту %d автоматически остановлен", portID)
		}()
	}

	return nil
}

// SetMotorPowerAndWait - с ожиданием завершения.
func (dm *DeviceManager) SetMotorPowerAndWait(portID byte, power int8, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

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

	err := dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
	if err != nil {
		return err
	}

	if duration > 0 {
		time.Sleep(time.Duration(duration) * time.Millisecond)
		stopCmd := []byte{portID, 0x01, 0x01, 0x00}
		_ = dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
	}
	return nil
}

// SetLEDColor устанавливает цвет светодиода.
func (dm *DeviceManager) SetLEDColor(portID byte, red, green, blue byte) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверка через DeviceManager
	device, exists := dm.GetDevice(portID)
	if !exists && portID != PortBuiltInLED {
		log.Printf("Устройство на порту %d не найдено", portID)
	} else if exists && !device.IsConnected && portID != PortBuiltInLED {
		return fmt.Errorf("устройство на порту %d не подключено", portID)
	}

	// Настройка режима RGB
	modeCmd := []byte{0x01, 0x02, portID, 0x17, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	if err := dm.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, modeCmd); err != nil {
		log.Printf("Предупреждение при установке режима светодиода: %v", err)
	}

	colorCmd := []byte{0x06, 0x04, 0x03, red, green, blue}
	log.Printf("Установка цвета светодиода на порту %d: RGB(%d,%d,%d)", portID, red, green, blue)
	return dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, colorCmd)
}

// PlayTone воспроизводит тон.
func (dm *DeviceManager) PlayTone(portID byte, frequency uint16, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	device, exists := dm.GetDevice(portID)
	if !exists || !device.IsConnected || device.DeviceType != DEVICE_TYPE_PIEZO_TONE {
		return fmt.Errorf("пищалка не подключена к порту %d", portID)
	}

	freqLow := byte(frequency & 0xFF)
	freqHigh := byte((frequency >> 8) & 0xFF)
	durLow := byte(duration & 0xFF)
	durHigh := byte((duration >> 8) & 0xFF)

	cmd := []byte{portID, 0x02, 0x04, freqLow, freqHigh, durLow, durHigh}
	log.Printf("Проигрывание тона на порту %d: частота=%d Гц, длительность=%d мс", portID, frequency, duration)
	return dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
}

// PlayToneAndWait - с ожиданием.
func (dm *DeviceManager) PlayToneAndWait(portID byte, frequency uint16, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверка необязательна, но для совместимости оставим
	_, exists := dm.GetDevice(portID)
	if !exists {
		log.Printf("Предупреждение: устройство на порту %d не найдено, но попытка воспроизведения будет выполнена", portID)
	}

	freqLow := byte(frequency & 0xFF)
	freqHigh := byte((frequency >> 8) & 0xFF)
	durLow := byte(duration & 0xFF)
	durHigh := byte((duration >> 8) & 0xFF)

	cmd := []byte{portID, 0x02, 0x04, freqLow, freqHigh, durLow, durHigh}
	err := dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
	if err != nil {
		return err
	}

	if duration > 0 {
		time.Sleep(time.Duration(duration) * time.Millisecond)
		stopCmd := []byte{portID, 0x03, 0x00}
		_ = dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
	}
	return nil
}

// StopTone останавливает пищалку.
func (dm *DeviceManager) StopTone(portID byte) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}
	cmd := []byte{portID, 0x03, 0x00}
	log.Printf("Остановка пищалки на порту %d", portID)
	return dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
}

// SetDeviceChangedCallback устанавливает callback.
func (dm *DeviceManager) SetDeviceChangedCallback(callback func(portID byte, device *Device)) {
	dm.deviceChangedCallback = callback
}

// UpdateDeviceValue обновляет значение устройства.
func (dm *DeviceManager) UpdateDeviceValue(portID byte, value interface{}) {
	dm.devicesMu.Lock()
	defer dm.devicesMu.Unlock()

	if device, exists := dm.devices[portID]; exists {
		device.LastValue = value
		device.LastUpdate = time.Now()
		if dm.deviceChangedCallback != nil {
			dm.deviceChangedCallback(portID, device)
		}
	}
}

// SyncDevices синхронизирует устройства с HubManager (теперь просто логирует, так как HubManager не хранит устройства)
func (dm *DeviceManager) SyncDevices() {
	log.Println("Синхронизация устройств не требуется: DeviceManager является единственным источником")
	// Можно оставить пустым или удалить этот метод, если он нигде не используется.
	// Оставим для совместимости.
}

// ForceDetectAllDevices принудительно запускает обнаружение через HubManager.
func (dm *DeviceManager) ForceDetectAllDevices() {
	if dm.hubMgr == nil || !dm.hubMgr.IsConnected() {
		return
	}
	log.Println("Принудительное обнаружение всех устройств...")
	dm.hubMgr.autoDetectDevicesV2()
}
