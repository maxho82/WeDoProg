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

	// Проверяем, подключен ли мотор
	device, exists := dm.GetDevice(portID)
	if !exists || !device.IsConnected || device.DeviceType != 0x01 {
		return fmt.Errorf("мотор не подключен к порту %d", portID)
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

	log.Printf("Установка мощности мотора на порту %d: %d%%", portID, power)
	return dm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", cmd)
}

// SetLEDColor устанавливает цвет светодиода
func (dm *DeviceManager) SetLEDColor(portID byte, red, green, blue byte) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверяем, подключен ли светодиод
	device, exists := dm.GetDevice(portID)
	if !exists || !device.IsConnected || device.DeviceType != 0x17 {
		return fmt.Errorf("RGB светодиод не подключен к порту %d", portID)
	}

	// Настраиваем режим RGB
	modeCmd := []byte{0x01, 0x02, portID, 0x17, 0x01, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
	if err := dm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", modeCmd); err != nil {
		log.Printf("Предупреждение при установке режима: %v", err)
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
