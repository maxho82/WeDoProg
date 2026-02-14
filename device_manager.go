package main

import (
	"fmt"
	"log"
	"time"
)

// DeviceManager управляет устройствами хаба, используя AppState как единственное хранилище.
type DeviceManager struct {
	hubMgr *HubManager
	state  *AppState // ссылка на общее состояние
}

// NewDeviceManager создаёт новый менеджер устройств.
func NewDeviceManager(hubMgr *HubManager, state *AppState) *DeviceManager {
	return &DeviceManager{
		hubMgr: hubMgr,
		state:  state,
	}
}

// GetDevice возвращает устройство по порту из AppState.
func (dm *DeviceManager) GetDevice(portID byte) (*Device, bool) {
	dev := dm.state.GetDevice(portID)
	return dev, dev != nil
}

// GetConnectedDevices возвращает список подключенных устройств из AppState.
func (dm *DeviceManager) GetConnectedDevices() []*Device {
	all := dm.state.GetAllDevices()
	var connected []*Device
	for _, dev := range all {
		if dev.IsConnected {
			connected = append(connected, dev)
		}
	}
	return connected
}

// GetDevicesByType возвращает подключенные устройства определённого типа.
func (dm *DeviceManager) GetDevicesByType(deviceType byte) []*Device {
	all := dm.state.GetAllDevices()
	var filtered []*Device
	for _, dev := range all {
		if dev.DeviceType == deviceType && dev.IsConnected {
			filtered = append(filtered, dev)
		}
	}
	return filtered
}

// SetMotorPower устанавливает мощность мотора.
func (dm *DeviceManager) SetMotorPower(portID byte, power int8, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверяем наличие устройства (опционально)
	dev, _ := dm.GetDevice(portID)
	if dev == nil {
		log.Printf("Устройство на порту %d не найдено в AppState, но команда будет отправлена", portID)
	} else if !dev.IsConnected {
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
			_ = dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
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

	// Проверяем наличие устройства, но для встроенного светодиода (порт 6) не обязательно
	if portID != PortBuiltInLED {
		dev, _ := dm.GetDevice(portID)
		if dev == nil || !dev.IsConnected {
			return fmt.Errorf("устройство на порту %d не подключено", portID)
		}
	}

	// Настройка режима RGB
	modeCmd := LPF2Protocol{}.EncodeLEDModeCommand(portID, 0x01)
	if err := dm.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, modeCmd); err != nil {
		log.Printf("Предупреждение при установке режима светодиода: %v", err)
	}

	colorCmd := LPF2Protocol{}.EncodeLEDCommand(portID, red, green, blue)
	log.Printf("Установка цвета светодиода на порту %d: RGB(%d,%d,%d)", portID, red, green, blue)
	return dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, colorCmd)
}

// PlayTone воспроизводит тон.
func (dm *DeviceManager) PlayTone(portID byte, frequency uint16, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверяем наличие пищалки
	dev, _ := dm.GetDevice(portID)
	if dev == nil || !dev.IsConnected || dev.DeviceType != DEVICE_TYPE_PIEZO_TONE {
		return fmt.Errorf("пищалка не подключена к порту %d", portID)
	}

	cmd := LPF2Protocol{}.EncodePiezoToneCommand(portID, frequency, duration)
	log.Printf("Проигрывание тона на порту %d: частота=%d Гц, длительность=%d мс", portID, frequency, duration)
	return dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
}

// PlayToneAndWait - с ожиданием.
func (dm *DeviceManager) PlayToneAndWait(portID byte, frequency uint16, duration uint16) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	// Проверка необязательна, но для совместимости оставим
	dev, _ := dm.GetDevice(portID)
	if dev == nil {
		log.Printf("Предупреждение: устройство на порту %d не найдено, но попытка воспроизведения будет выполнена", portID)
	}

	cmd := LPF2Protocol{}.EncodePiezoToneCommand(portID, frequency, duration)
	err := dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
	if err != nil {
		return err
	}

	if duration > 0 {
		time.Sleep(time.Duration(duration) * time.Millisecond)
		stopCmd := LPF2Protocol{}.EncodeStopPiezoToneCommand(portID)
		_ = dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
	}
	return nil
}

// StopTone останавливает пищалку.
func (dm *DeviceManager) StopTone(portID byte) error {
	if !dm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}
	cmd := LPF2Protocol{}.EncodeStopPiezoToneCommand(portID)
	log.Printf("Остановка пищалки на порту %d", portID)
	return dm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, cmd)
}

// UpdateDeviceValue обновляет значение датчика в AppState.
func (dm *DeviceManager) UpdateDeviceValue(portID byte, value interface{}) {
	dm.state.UpdateDeviceValue(portID, value)
}

// ForceDetectAllDevices принудительно запускает обнаружение через HubManager.
func (dm *DeviceManager) ForceDetectAllDevices() {
	if dm.hubMgr == nil || !dm.hubMgr.IsConnected() {
		return
	}
	log.Println("Принудительное обнаружение всех устройств...")
	dm.hubMgr.autoDetectDevicesV2()
}
