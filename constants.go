package main

// ============================================================================
// Константы приложения
// ============================================================================

// Размеры блоков (базовые, без учёта масштаба)
const (
	DefaultBlockWidth  = 180
	DefaultBlockHeight = 100
	BlockSpacing       = 40 // вертикальный отступ между блоками
)

// Порты устройств
const (
	PortMotorA     = 1
	PortMotorB     = 2
	PortBuiltInLED = 6 // встроенный RGB-светодиод
)

// Длительности по умолчанию (в миллисекундах или секундах)
const (
	DefaultMotorDurationMs   = 1000
	DefaultWaitDurationSec   = 1.0
	DefaultToneDurationMs    = 1000
	DefaultToneFrequencyHz   = 440
	DefaultLoopCount         = 5
)

// Границы масштабирования
const (
	MinScale = 0.5
	MaxScale = 3.0
)

// Размеры валентных точек (базовые)
const (
	ValencePointBaseSize = 16
)

// Идентификаторы для коллбэков (используются в AppState)
const (
	CallbackIDBattery     = "battery"
	CallbackIDHubInfo     = "hubInfo"
	CallbackIDDevice      = "device"
	CallbackIDConnection  = "connection"
	CallbackIDProgramState = "programState"
	CallbackIDCurrentBlock = "currentBlock"
)

// Размеры отступов и толщины линий
const (
	LineStrokeWidthNormal = 2.0
	LineStrokeWidthHighlighted = 3.0
	LineStrokeWidthExecuting = 4.0
)