package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// ProgramManager управляет программами
type ProgramManager struct {
	hubMgr       *HubManager
	deviceMgr    *DeviceManager
	program      *Program
	programs     map[string]*Program
	programsMu   sync.RWMutex
	currentState ProgramState
}

// Program представляет программу
type Program struct {
	Name        string
	Blocks      []*ProgramBlock
	Connections []*Connection
	Created     time.Time
	Modified    time.Time
}

// ProgramBlock блок программы
type ProgramBlock struct {
	ID           int
	Type         BlockType
	Title        string
	Description  string
	X, Y         float64
	DragStartPos fyne.Position
	Width        float64
	Height       float64
	Parameters   map[string]interface{}
	NextBlockID  int
	IsStart      bool
	Color        string
	OnExecute    func() error
}

// Connection соединение между блоками
type Connection struct {
	FromBlockID int
	ToBlockID   int
}

// ProgramState состояние выполнения программы
type ProgramState int

const (
	ProgramStateStopped ProgramState = iota
	ProgramStateRunning
	ProgramStatePaused
	ProgramStateError
)

// BlockType тип блока программирования
type BlockType int

const (
	BlockTypeStart BlockType = iota
	BlockTypeMotor
	BlockTypeLED
	BlockTypeWait
	BlockTypeLoop
	BlockTypeCondition
	BlockTypeTiltSensor
	BlockTypeDistanceSensor
	BlockTypeSound
	BlockTypeVoltageSensor
	BlockTypeCurrentSensor
	BlockTypeStop
)

// NewProgramManager создает менеджер программ
func NewProgramManager(hubMgr *HubManager, deviceMgr *DeviceManager) *ProgramManager {
	return &ProgramManager{
		hubMgr:       hubMgr,
		deviceMgr:    deviceMgr,
		program:      &Program{Name: "Новая программа"},
		programs:     make(map[string]*Program),
		currentState: ProgramStateStopped,
	}
}

// CreateBlock создает новый блок
func (pm *ProgramManager) CreateBlock(blockType BlockType, x, y float64) *ProgramBlock {
	block := &ProgramBlock{
		ID:           len(pm.program.Blocks) + 1,
		Type:         blockType,
		X:            x,
		Y:            y,
		DragStartPos: fyne.NewPos(float32(x), float32(y)), // Инициализируем
		Width:        150,
		Height:       80,
		Parameters:   make(map[string]interface{}),
		IsStart:      (blockType == BlockTypeStart),
	}

	// Настраиваем блок в зависимости от типа
	pm.configureBlock(block)

	pm.program.Blocks = append(pm.program.Blocks, block)
	pm.program.Modified = time.Now()

	log.Printf("Создан блок: %s (ID: %d)", block.Title, block.ID)
	return block
}

// configureBlock настраивает блок
func (pm *ProgramManager) configureBlock(block *ProgramBlock) {
	switch block.Type {
	case BlockTypeStart:
		block.Title = "Начать"
		block.Description = "Начало программы"
		block.Color = "#4CAF50" // Зеленый
		block.IsStart = true
		block.OnExecute = func() error {
			log.Println("Начало программы")
			return nil
		}

	case BlockTypeMotor:
		block.Title = "Мотор"
		block.Description = "Управление мотором"
		block.Color = "#2196F3" // Синий
		block.Parameters["port"] = byte(1)
		block.Parameters["power"] = int8(50)
		block.Parameters["duration"] = uint16(1000)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}

			port := block.Parameters["port"].(byte)
			power := block.Parameters["power"].(int8)
			duration := block.Parameters["duration"].(uint16)

			return pm.deviceMgr.SetMotorPower(port, power, duration)
		}

	case BlockTypeLED:
		block.Title = "Светодиод"
		block.Description = "Управление светодиодом"
		block.Color = "#FF9800" // Оранжевый
		block.Parameters["port"] = byte(6)
		block.Parameters["red"] = byte(255)
		block.Parameters["green"] = byte(0)
		block.Parameters["blue"] = byte(0)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}

			port := block.Parameters["port"].(byte)
			red := block.Parameters["red"].(byte)
			green := block.Parameters["green"].(byte)
			blue := block.Parameters["blue"].(byte)

			return pm.deviceMgr.SetLEDColor(port, red, green, blue)
		}

	case BlockTypeWait:
		block.Title = "Ждать"
		block.Description = "Пауза в программе"
		block.Color = "#9E9E9E" // Серый
		block.Parameters["duration"] = 1.0
		block.OnExecute = func() error {
			duration := block.Parameters["duration"].(float64)
			time.Sleep(time.Duration(duration*1000) * time.Millisecond)
			return nil
		}

	case BlockTypeLoop:
		block.Title = "Повторять"
		block.Description = "Цикл повторений"
		block.Color = "#9C27B0" // Фиолетовый
		block.Parameters["count"] = 5
		block.Parameters["forever"] = false

	case BlockTypeCondition:
		block.Title = "Условие"
		block.Description = "Условный оператор"
		block.Color = "#3F51B5" // Индиго

	case BlockTypeTiltSensor:
		block.Title = "Датчик наклона"
		block.Description = "Чтение датчика наклона"
		block.Color = "#673AB7" // Глубокий фиолетовый
		block.Parameters["port"] = byte(1)
		block.Parameters["mode"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}

			port := block.Parameters["port"].(byte)
			mode := block.Parameters["mode"].(byte)

			// Настраиваем датчик
			cmd := []byte{0x01, 0x02, port, 0x22, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeDistanceSensor:
		block.Title = "Датчик расстояния"
		block.Description = "Измерение расстояния"
		block.Color = "#00BCD4" // Голубой
		block.Parameters["port"] = byte(1)
		block.Parameters["mode"] = byte(0)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}

			port := block.Parameters["port"].(byte)
			mode := block.Parameters["mode"].(byte)

			// Настраиваем датчик
			cmd := []byte{0x01, 0x02, port, 0x23, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeSound:
		block.Title = "Звук"
		block.Description = "Воспроизведение звука"
		block.Color = "#FF5722" // Глубокий оранжевый
		block.Parameters["port"] = byte(1)
		block.Parameters["frequency"] = uint16(440)
		block.Parameters["duration"] = uint16(1000)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}

			port := block.Parameters["port"].(byte)
			frequency := block.Parameters["frequency"].(uint16)
			duration := block.Parameters["duration"].(uint16)

			return pm.deviceMgr.PlayTone(port, frequency, duration)
		}

	case BlockTypeVoltageSensor:
		block.Title = "Датчик напряжения"
		block.Description = "Измерение напряжения"
		block.Color = "#8BC34A" // Светло-зеленый
		block.Parameters["port"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}

			port := block.Parameters["port"].(byte)

			// Настраиваем датчик
			cmd := []byte{0x01, 0x02, port, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeCurrentSensor:
		block.Title = "Датчик тока"
		block.Description = "Измерение тока"
		block.Color = "#F44336" // Красный
		block.Parameters["port"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}

			port := block.Parameters["port"].(byte)

			// Настраиваем датчик
			cmd := []byte{0x01, 0x02, port, 0x15, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeStop:
		block.Title = "Стоп"
		block.Description = "Остановка программы"
		block.Color = "#F44336" // Красный
		block.OnExecute = func() error {
			pm.StopProgram()
			return nil
		}
	}
}

// RunProgram запускает выполнение программы
func (pm *ProgramManager) RunProgram() error {
	if pm.currentState == ProgramStateRunning {
		return fmt.Errorf("программа уже выполняется")
	}

	if !pm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}

	if len(pm.program.Blocks) == 0 {
		return fmt.Errorf("нет блоков в программе")
	}

	// Находим стартовый блок
	var startBlock *ProgramBlock
	for _, block := range pm.program.Blocks {
		if block.IsStart {
			startBlock = block
			break
		}
	}

	if startBlock == nil {
		return fmt.Errorf("стартовый блок не найден")
	}

	pm.currentState = ProgramStateRunning
	log.Println("Запуск программы...")

	// Запускаем выполнение в отдельной горутине
	go pm.executeProgram(startBlock)

	return nil
}

// executeProgram выполняет программу
func (pm *ProgramManager) executeProgram(startBlock *ProgramBlock) {
	currentBlock := startBlock

	for pm.currentState == ProgramStateRunning && currentBlock != nil {
		log.Printf("Выполнение блока: %s (ID: %d)", currentBlock.Title, currentBlock.ID)

		// Выполняем блок
		if currentBlock.OnExecute != nil {
			if err := currentBlock.OnExecute(); err != nil {
				log.Printf("Ошибка выполнения блока %d: %v", currentBlock.ID, err)
				pm.currentState = ProgramStateError
				break
			}
		}

		// Ищем следующий блок
		if currentBlock.NextBlockID > 0 {
			nextBlock := pm.findBlockByID(currentBlock.NextBlockID)
			currentBlock = nextBlock
		} else {
			// Конец программы
			break
		}

		// Небольшая задержка между блоками
		time.Sleep(100 * time.Millisecond)
	}

	if pm.currentState == ProgramStateRunning {
		pm.currentState = ProgramStateStopped
		log.Println("Программа завершена успешно")
	}
}

// findBlockByID находит блок по ID
func (pm *ProgramManager) findBlockByID(blockID int) *ProgramBlock {
	for _, block := range pm.program.Blocks {
		if block.ID == blockID {
			return block
		}
	}
	return nil
}

// StopProgram останавливает выполнение программы
func (pm *ProgramManager) StopProgram() {
	if pm.currentState == ProgramStateRunning {
		pm.currentState = ProgramStateStopped
		log.Println("Программа остановлена")

		// Останавливаем все моторы
		for _, block := range pm.program.Blocks {
			if block.Type == BlockTypeMotor {
				if port, ok := block.Parameters["port"].(byte); ok {
					pm.deviceMgr.SetMotorPower(port, 0, 0)
				}
			}
		}
	}
}

// ClearProgram очищает программу
func (pm *ProgramManager) ClearProgram() {
	pm.program.Blocks = make([]*ProgramBlock, 0)
	pm.program.Connections = make([]*Connection, 0)
	pm.currentState = ProgramStateStopped
	pm.program.Modified = time.Now()

	log.Println("Программа очищена")
}

// GetProgram возвращает текущую программу
func (pm *ProgramManager) GetProgram() *Program {
	return pm.program
}

// GetBlock возвращает блок по ID
func (pm *ProgramManager) GetBlock(blockID int) (*ProgramBlock, bool) {
	for _, block := range pm.program.Blocks {
		if block.ID == blockID {
			return block, true
		}
	}
	return nil, false
}

// UpdateBlock обновляет параметры блока
func (pm *ProgramManager) UpdateBlock(blockID int, params map[string]interface{}) bool {
	for _, block := range pm.program.Blocks {
		if block.ID == blockID {
			for key, value := range params {
				block.Parameters[key] = value
			}
			pm.program.Modified = time.Now()
			return true
		}
	}
	return false
}

// AddConnection добавляет соединение между блоками
func (pm *ProgramManager) AddConnection(fromBlockID, toBlockID int) bool {
	// Проверяем, существуют ли блоки
	fromBlock, fromExists := pm.GetBlock(fromBlockID)
	_, toExists := pm.GetBlock(toBlockID)

	if !fromExists || !toExists {
		return false
	}

	// Устанавливаем следующий блок
	fromBlock.NextBlockID = toBlockID

	// Добавляем соединение
	connection := &Connection{
		FromBlockID: fromBlockID,
		ToBlockID:   toBlockID,
	}

	pm.program.Connections = append(pm.program.Connections, connection)
	pm.program.Modified = time.Now()

	log.Printf("Добавлено соединение: блок %d -> блок %d", fromBlockID, toBlockID)
	return true
}

// RemoveConnection удаляет соединение
func (pm *ProgramManager) RemoveConnection(fromBlockID int) bool {
	for i, conn := range pm.program.Connections {
		if conn.FromBlockID == fromBlockID {
			// Удаляем соединение
			pm.program.Connections = append(pm.program.Connections[:i], pm.program.Connections[i+1:]...)

			// Сбрасываем следующий блок
			if block, exists := pm.GetBlock(fromBlockID); exists {
				block.NextBlockID = 0
			}

			pm.program.Modified = time.Now()
			log.Printf("Удалено соединение для блока %d", fromBlockID)
			return true
		}
	}
	return false
}

// GetProgramState возвращает состояние программы
func (pm *ProgramManager) GetProgramState() ProgramState {
	return pm.currentState
}
