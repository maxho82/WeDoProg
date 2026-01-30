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
		program:      &Program{Name: "Новая программа", Created: time.Now(), Modified: time.Now()},
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
		DragStartPos: fyne.NewPos(float32(x), float32(y)),
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
		block.Color = "#4CAF50"
		block.IsStart = true
		block.OnExecute = func() error {
			log.Println("Начало программы")
			return nil
		}

	case BlockTypeMotor:
		block.Title = "Мотор"
		block.Description = "Управление мотором"
		block.Color = "#2196F3"
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
			return pm.deviceMgr.SetMotorPowerAndWait(port, power, duration)
		}

	case BlockTypeLED:
		block.Title = "Светодиод"
		block.Description = "Управление светодиодом"
		block.Color = "#FF9800"
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
		block.Color = "#9E9E9E"
		block.Parameters["duration"] = 1.0
		block.OnExecute = func() error {
			duration := block.Parameters["duration"].(float64)
			log.Printf("Пауза: %.1f секунд", duration)
			time.Sleep(time.Duration(duration*1000) * time.Millisecond)
			return nil
		}

	case BlockTypeLoop:
		block.Title = "Повторять"
		block.Description = "Цикл повторений"
		block.Color = "#9C27B0"
		block.Parameters["count"] = 5
		block.Parameters["forever"] = false
		block.OnExecute = func() error {
			log.Println("Цикл выполняется")
			return nil
		}

	case BlockTypeCondition:
		block.Title = "Условие"
		block.Description = "Условный оператор"
		block.Color = "#3F51B5"
		block.OnExecute = func() error {
			log.Println("Проверка условия")
			return nil
		}

	case BlockTypeTiltSensor:
		block.Title = "Датчик наклона"
		block.Description = "Чтение датчика наклона"
		block.Color = "#673AB7"
		block.Parameters["port"] = byte(1)
		block.Parameters["mode"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			mode := block.Parameters["mode"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x22, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeDistanceSensor:
		block.Title = "Датчик расстояния"
		block.Description = "Измерение расстояния"
		block.Color = "#00BCD4"
		block.Parameters["port"] = byte(1)
		block.Parameters["mode"] = byte(0)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			mode := block.Parameters["mode"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x23, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeSound:
		block.Title = "Звук"
		block.Description = "Воспроизведение звука"
		block.Color = "#FF5722"
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
			return pm.deviceMgr.PlayToneAndWait(port, frequency, duration)
		}

	case BlockTypeVoltageSensor:
		block.Title = "Датчик напряжения"
		block.Description = "Измерение напряжения"
		block.Color = "#8BC34A"
		block.Parameters["port"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeCurrentSensor:
		block.Title = "Датчик тока"
		block.Description = "Измерение тока"
		block.Color = "#F44336"
		block.Parameters["port"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x15, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic("00001563-1212-efde-1523-785feabcd123", cmd)
		}

	case BlockTypeStop:
		block.Title = "Стоп"
		block.Description = "Остановка программы"
		block.Color = "#F44336"
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
		if len(pm.program.Blocks) > 0 {
			startBlock = pm.program.Blocks[0]
			log.Println("Стартовый блок не найден, используем первый блок в программе")
		} else {
			return fmt.Errorf("нет блоков для выполнения")
		}
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
	executedBlocks := make(map[int]bool)

	log.Println("=== Начало выполнения программы ===")

	for pm.currentState == ProgramStateRunning && currentBlock != nil {
		if executedBlocks[currentBlock.ID] {
			log.Printf("Предотвращение бесконечного цикла: блок %d уже выполнялся", currentBlock.ID)
			break
		}
		executedBlocks[currentBlock.ID] = true

		log.Printf(">>> Выполнение блока: %s (ID: %d) <<<", currentBlock.Title, currentBlock.ID)

		// Выполняем блок
		if currentBlock.OnExecute != nil {
			startTime := time.Now()

			if err := currentBlock.OnExecute(); err != nil {
				log.Printf("ОШИБКА выполнения блока %d: %v", currentBlock.ID, err)
				pm.currentState = ProgramStateError
				break
			}

			executionTime := time.Since(startTime)
			log.Printf("Блок %d выполнен за %v", currentBlock.ID, executionTime)
		} else {
			log.Printf("Блок %d не имеет функции выполнения", currentBlock.ID)
		}

		// Ищем следующий блок
		if currentBlock.NextBlockID > 0 {
			nextBlock := pm.findBlockByID(currentBlock.NextBlockID)
			if nextBlock == nil {
				log.Printf("ОШИБКА: следующий блок %d не найден", currentBlock.NextBlockID)
				pm.currentState = ProgramStateError
				break
			}
			currentBlock = nextBlock
		} else {
			log.Printf("Достигнут конец программы (блок %d не имеет следующего блока)", currentBlock.ID)
			break
		}

		if pm.currentState != ProgramStateRunning {
			break
		}

		if currentBlock.Type != BlockTypeWait {
			time.Sleep(10 * time.Millisecond)
		}
	}

	switch pm.currentState {
	case ProgramStateRunning:
		pm.currentState = ProgramStateStopped
		log.Println("=== Программа завершена успешно ===")
	case ProgramStateError:
		log.Println("=== Программа завершена с ошибкой ===")
	}

	pm.ensureAllMotorsStopped()
	log.Println("Все моторы остановлены")
}

// ensureAllMotorsStopped гарантирует остановку всех моторов
func (pm *ProgramManager) ensureAllMotorsStopped() {
	log.Println("Гарантированная остановка всех моторов...")
	for port := byte(1); port <= 6; port++ {
		if pm.deviceMgr != nil && pm.hubMgr != nil && pm.hubMgr.IsConnected() {
			stopCmd := []byte{port, 0x01, 0x01, 0x00}
			pm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", stopCmd)
		}
	}
}

// StopProgram останавливает программу
func (pm *ProgramManager) StopProgram() {
	if pm.currentState == ProgramStateRunning {
		pm.currentState = ProgramStateStopped
		log.Println("Программа остановлена")
		pm.ensureAllMotorsStopped()
		pm.stopAllSounds()
	}
}

// stopAllSounds останавливает все звуки
func (pm *ProgramManager) stopAllSounds() {
	log.Println("Остановка всех звуков...")
	for port := byte(1); port <= 6; port++ {
		if pm.deviceMgr != nil && pm.hubMgr != nil && pm.hubMgr.IsConnected() {
			stopCmd := []byte{port, 0x03, 0x00}
			pm.hubMgr.WriteCharacteristic("00001565-1212-efde-1523-785feabcd123", stopCmd)
		}
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
	fromBlock, fromExists := pm.GetBlock(fromBlockID)
	_, toExists := pm.GetBlock(toBlockID)

	if !fromExists || !toExists {
		return false
	}

	fromBlock.NextBlockID = toBlockID

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
			pm.program.Connections = append(pm.program.Connections[:i], pm.program.Connections[i+1:]...)
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

// RemoveBlock полностью удаляет блок
func (pm *ProgramManager) RemoveBlock(blockID int) bool {
	var blockToRemove *ProgramBlock
	var newBlocks []*ProgramBlock

	for _, block := range pm.program.Blocks {
		if block.ID != blockID {
			newBlocks = append(newBlocks, block)
		} else {
			blockToRemove = block
		}
	}

	if blockToRemove == nil {
		return false
	}

	pm.program.Blocks = newBlocks

	// Удаляем все соединения, связанные с блоком
	var newConnections []*Connection
	for _, conn := range pm.program.Connections {
		if conn.FromBlockID != blockID && conn.ToBlockID != blockID {
			newConnections = append(newConnections, conn)
		} else {
			// Если соединение вело к удаляемому блоку, сбрасываем NextBlockID
			if conn.FromBlockID != blockID {
				for _, block := range newBlocks {
					if block.ID == conn.FromBlockID {
						block.NextBlockID = 0
					}
				}
			}
		}
	}
	pm.program.Connections = newConnections

	// Если удаляемый блок был начальным, делаем первый блок начальным
	if blockToRemove.IsStart && len(newBlocks) > 0 {
		newBlocks[0].IsStart = true
	}

	pm.program.Modified = time.Now()
	log.Printf("Блок %d полностью удален из программы", blockID)
	return true
}

// GetProgramState возвращает состояние программы
func (pm *ProgramManager) GetProgramState() ProgramState {
	return pm.currentState
}

// UpdateBlockPosition обновляет позицию блока
func (pm *ProgramManager) UpdateBlockPosition(blockID int, x, y float64) bool {
	for _, block := range pm.program.Blocks {
		if block.ID == blockID {
			block.X = x
			block.Y = y
			pm.program.Modified = time.Now()
			return true
		}
	}
	return false
}
