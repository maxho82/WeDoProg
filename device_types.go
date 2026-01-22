package main

import "fmt"

// Типы устройств LPF2
const (
	DEVICE_TYPE_MOTOR         = 0x01 // Мотор
	DEVICE_TYPE_VOLTAGE       = 0x14 // Датчик напряжения
	DEVICE_TYPE_CURRENT       = 0x15 // Датчик тока
	DEVICE_TYPE_PIEZO_TONE    = 0x16 // Пищалка
	DEVICE_TYPE_RGB_LIGHT     = 0x17 // RGB светодиод
	DEVICE_TYPE_TILT_SENSOR   = 0x22 // Датчик наклона
	DEVICE_TYPE_MOTION_SENSOR = 0x23 // Датчик расстояния/движения
)

// Режимы работы устройств
const (
	LED_ABSOLUTE_MODE = 0 // Режим индексных цветов
	LED_DISCRETE_MODE = 1 // Режим RGB цветов

	DIST_DETECT_MODE = 0 // Режим измерения расстояния
	DIST_COUNT_MODE  = 1 // Режим подсчета

	TILT_ANGLE_MODE = 0 // Режим угла наклона
	TILT_TILT_MODE  = 1 // Режим определения наклона
	TILT_CRASH_MODE = 2 // Режим определения удара
)

// Индексные цвета для светодиода
const (
	LED_INDEX_PINK   = 0x01 // Розовый
	LED_INDEX_PURPLE = 0x02 // Фиолетовый
	LED_INDEX_BLUE   = 0x03 // Синий
	LED_INDEX_GREEN  = 0x05 // Зеленый
	LED_INDEX_RED    = 0x09 // Красный
	LED_INDEX_WHITE  = 0x0A // Белый
)

// DeviceTypeName возвращает имя типа устройства
func DeviceTypeName(deviceType byte) string {
	switch deviceType {
	case DEVICE_TYPE_MOTOR:
		return "Мотор"
	case DEVICE_TYPE_VOLTAGE:
		return "Датчик напряжения"
	case DEVICE_TYPE_CURRENT:
		return "Датчик тока"
	case DEVICE_TYPE_PIEZO_TONE:
		return "Пищалка"
	case DEVICE_TYPE_RGB_LIGHT:
		return "RGB светодиод"
	case DEVICE_TYPE_TILT_SENSOR:
		return "Датчик наклона"
	case DEVICE_TYPE_MOTION_SENSOR:
		return "Датчик расстояния"
	default:
		return fmt.Sprintf("Неизвестное (0x%02x)", deviceType)
	}
}
