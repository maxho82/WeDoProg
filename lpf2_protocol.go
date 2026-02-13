package main

// UUID характеристик WeDo 2.0/LPF2
const (
	LPF2_HUB_SERVICE_UUID       = "00001523-1212-efde-1523-785feabcd123"
	LPF2_EXTENDED_SERVICE_UUID  = "00004f0e-1212-efde-1523-785feabcd123"
	DEVICE_INFO_SERVICE_UUID    = "0000180a-0000-1000-8000-00805f9b34fb"
	BATTERY_SERVICE_UUID        = "0000180f-0000-1000-8000-00805f9b34fb"
	WEDO2_SPECIFIC_SERVICE_UUID = "5833ff01-9b8b-5191-6142-22a4536ef123"

	SENSOR_VALUES_UUID  = "00001560-1212-efde-1523-785feabcd123"
	PORT_INFO_UUID      = "00001527-1212-efde-1523-785feabcd123"
	INPUT_COMMAND_UUID  = "00001563-1212-efde-1523-785feabcd123"
	OUTPUT_COMMAND_UUID = "00001565-1212-efde-1523-785feabcd123"
	NAME_UUID           = "00001524-1212-efde-1523-785feabcd123"

	MANUFACTURER_NAME_UUID = "00002a29-0000-1000-8000-00805f9b34fb"
	FIRMWARE_REVISION_UUID = "00002a26-0000-1000-8000-00805f9b34fb"
	SOFTWARE_REVISION_UUID = "00002a28-0000-1000-8000-00805f9b34fb"
	SYSTEM_ID_UUID         = "00002a23-0000-1000-8000-00805f9b34fb"
	BATTERY_LEVEL_UUID     = "00002a19-0000-1000-8000-00805f9b34fb"

	FIRMWARE_CHAR_UUID = "00004f01-1212-efde-1523-785feabcd123"
)

// LPF2Protocol реализует протокол LPF2.
type LPF2Protocol struct{}

// EncodeMotorCommand кодирует команду для мотора.
func (p LPF2Protocol) EncodeMotorCommand(portID byte, speed float64) []byte {
	var speedByte byte
	if speed < 0 {
		speedByte = byte((0x54 * maxFloat(speed, -1)) + 0xF0)
	} else if speed > 0 {
		speedByte = byte((0x54 * minFloat(speed, 1)) + 0x10)
	} else {
		speedByte = 0x00
	}
	return []byte{portID, 0x01, 0x01, speedByte}
}

// EncodeLEDCommand кодирует команду для RGB светодиода.
func (p LPF2Protocol) EncodeLEDCommand(portID byte, red, green, blue byte) []byte {
	return []byte{0x06, 0x04, 0x03, red, green, blue}
}

// EncodeLEDIndexCommand кодирует команду для индексного цвета.
func (p LPF2Protocol) EncodeLEDIndexCommand(portID byte, colorIndex byte) []byte {
	return []byte{0x06, 0x04, 0x01, colorIndex}
}

// EncodeLEDModeCommand кодирует команду установки режима светодиода.
func (p LPF2Protocol) EncodeLEDModeCommand(portID byte, mode byte) []byte {
	return []byte{0x01, 0x02, portID, 0x17, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
}

// EncodeTiltSensorModeCommand кодирует команду настройки датчика наклона.
func (p LPF2Protocol) EncodeTiltSensorModeCommand(portID byte, mode byte) []byte {
	return []byte{0x01, 0x02, portID, 0x22, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
}

// EncodeDistanceSensorModeCommand кодирует команду настройки датчика расстояния.
func (p LPF2Protocol) EncodeDistanceSensorModeCommand(portID byte, mode byte) []byte {
	return []byte{0x01, 0x02, portID, 0x23, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
}

// EncodePiezoToneCommand кодирует команду для пищалки.
func (p LPF2Protocol) EncodePiezoToneCommand(portID byte, frequency uint16, duration uint16) []byte {
	freqLow := byte(frequency & 0xFF)
	freqHigh := byte((frequency >> 8) & 0xFF)
	durLow := byte(duration & 0xFF)
	durHigh := byte((duration >> 8) & 0xFF)
	return []byte{portID, 0x02, 0x04, freqLow, freqHigh, durLow, durHigh}
}

// EncodeStopPiezoToneCommand кодирует команду остановки пищалки.
func (p LPF2Protocol) EncodeStopPiezoToneCommand(portID byte) []byte {
	return []byte{portID, 0x03, 0x00}
}

// EncodeVoltageSensorModeCommand кодирует команду настройки датчика напряжения.
func (p LPF2Protocol) EncodeVoltageSensorModeCommand(portID byte) []byte {
	return []byte{0x01, 0x02, portID, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
}

// EncodeCurrentSensorModeCommand кодирует команду настройки датчика тока.
func (p LPF2Protocol) EncodeCurrentSensorModeCommand(portID byte) []byte {
	return []byte{0x01, 0x02, portID, 0x15, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
}

// EncodeDeviceSetupCommand возвращает универсальную команду настройки устройства.
func (p LPF2Protocol) EncodeDeviceSetupCommand(portID byte, deviceType byte, mode byte) []byte {
	return []byte{0x01, 0x02, portID, deviceType, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
}

// maxFloat возвращает максимальное из двух чисел.
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// minFloat возвращает минимальное из двух чисел.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
