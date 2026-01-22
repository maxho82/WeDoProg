package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// hexStringToBytes преобразует hex строку в байты
func hexStringToBytes(hexStr string) ([]byte, error) {
	// Убираем пробелы и другие разделители
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ReplaceAll(hexStr, "0x", "")
	hexStr = strings.ReplaceAll(hexStr, "\\x", "")
	hexStr = strings.ReplaceAll(hexStr, ",", "")
	hexStr = strings.ReplaceAll(hexStr, ":", "")

	// Проверяем четность длины
	if len(hexStr)%2 != 0 {
		// Добавляем ведущий ноль
		hexStr = "0" + hexStr
	}

	// Преобразуем hex в байты
	data := make([]byte, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		hexByte := hexStr[i : i+2]
		b, err := strconv.ParseUint(hexByte, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("неверный hex байт '%s': %v", hexByte, err)
		}
		data[i/2] = byte(b)
	}

	return data, nil
}

// bytesToHexString преобразует байты в hex строку
func bytesToHexString(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	hexStr := make([]string, len(data))
	for i, b := range data {
		hexStr[i] = fmt.Sprintf("%02X", b)
	}

	return strings.Join(hexStr, " ")
}

// bytesToStringSafe безопасно преобразует байты в строку
func bytesToStringSafe(data []byte) string {
	// Проверяем, можно ли преобразовать в UTF-8
	if utf8.Valid(data) {
		return strings.TrimSpace(string(data))
	}

	// Если не UTF-8, возвращаем hex представление
	return bytesToHexString(data)
}

// formatBatteryLevel форматирует уровень батареи
func formatBatteryLevel(level int) string {
	if level < 0 {
		return "Н/Д"
	}

	if level > 100 {
		level = 100
	}

	// Определяем иконку по уровню
	var icon string
	if level > 80 {
		icon = "🔋"
	} else if level > 40 {
		icon = "🔋"
	} else if level > 20 {
		icon = "🪫"
	} else {
		icon = "🪫"
	}

	return fmt.Sprintf("%s %d%%", icon, level)
}

// formatDeviceName форматирует имя устройства
func formatDeviceName(deviceType byte, portID byte) string {
	name := DeviceTypeName(deviceType)
	if name == "" {
		name = fmt.Sprintf("Устройство 0x%02X", deviceType)
	}

	return fmt.Sprintf("%s (Порт %d)", name, portID)
}

// isDeviceConnected проверяет, подключено ли устройство
func isDeviceConnected(devices map[byte]*Device, portID byte, deviceType byte) bool {
	device, exists := devices[portID]
	if !exists {
		return false
	}

	return device.IsConnected && device.DeviceType == deviceType
}

// clamp ограничивает значение в заданном диапазоне
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// mapRange отображает значение из одного диапазона в другой
func mapRange(value, inMin, inMax, outMin, outMax float64) float64 {
	return (value-inMin)*(outMax-outMin)/(inMax-inMin) + outMin
}

// getShortUUID возвращает короткий UUID
func getShortUUID(uuid string) string {
	// Убираем дефисы и берем первые 8 символов
	short := strings.ReplaceAll(uuid, "-", "")
	if len(short) >= 8 {
		return strings.ToUpper(short[:8])
	}
	return strings.ToUpper(short)
}

// isPrintable проверяет, можно ли отобразить данные как текст
func isPrintable(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for _, b := range data {
		if b < 32 && b != 10 && b != 13 && b != 9 {
			return false
		}
		if b > 126 {
			return false
		}
	}
	return true
}

// splitString разбивает строку на части заданной длины
func splitString(str string, length int) []string {
	var parts []string
	for i := 0; i < len(str); i += length {
		end := i + length
		if end > len(str) {
			end = len(str)
		}
		parts = append(parts, str[i:end])
	}
	return parts
}

// contains проверяет, содержит ли слайс элемент
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// max возвращает максимальное из двух чисел
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
