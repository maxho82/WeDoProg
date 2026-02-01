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

// CreateBlock создает новый блок для дракон-схемы
func (pm *ProgramManager) CreateBlock(blockType BlockType, x, y float64) *ProgramBlock {
	// Генерируем новый уникальный ID
	newID := 1
	for _, block := range pm.program.Blocks {
		if block.ID >= newID {
			newID = block.ID + 1
		}
	}

	block := &ProgramBlock{
		ID:          newID,
		Type:        blockType,
		X:           x,
		Y:           y,
		Width:       150,
		Height:      80,
		Parameters:  make(map[string]interface{}), // Инициализируем здесь!
		IsStart:     (blockType == BlockTypeStart),
		NextBlockID: 0,
		Color:       "#4CAF50", // Значение по умолчанию
	}

	// Настраиваем блок в зависимости от типа
	pm.configureBlock(block)

	log.Printf("Создан блок: %s (ID: %d)", block.Title, block.ID)

	// ВАЖНО: НЕ добавляем блок в программу здесь!
	// Это сделает programPanel.AddBlock после определения позиции вставки

	return block
}

// InsertBlock вставляет блок в программу в указанную позицию
func (pm *ProgramManager) InsertBlock(block *ProgramBlock, afterBlockID int) bool {
	// Если afterBlockID = 0, добавляем в начало
	// Если afterBlockID = -1, добавляем в конец

	if afterBlockID == -1 {
		// Добавляем в конец
		pm.program.Blocks = append(pm.program.Blocks, block)

		// Находим предыдущий блок (последний не-стоп блок)
		var prevBlock *ProgramBlock
		for _, b := range pm.program.Blocks {
			if b.ID != block.ID && b.Type != BlockTypeStop && b.NextBlockID == 0 {
				prevBlock = b
			}
		}

		if prevBlock != nil {
			prevBlock.NextBlockID = block.ID
			pm.AddConnection(prevBlock.ID, block.ID)
		}

		pm.program.Modified = time.Now()
		return true
	}

	if afterBlockID == 0 {
		// Добавляем в начало
		// Делаем все существующие блоки не стартовыми
		for _, b := range pm.program.Blocks {
			b.IsStart = false
		}

		block.IsStart = true
		block.NextBlockID = 0

		// Если есть другие блоки, устанавливаем связь
		if len(pm.program.Blocks) > 0 {
			block.NextBlockID = pm.program.Blocks[0].ID
			pm.AddConnection(block.ID, pm.program.Blocks[0].ID)
		}

		// Вставляем в начало
		pm.program.Blocks = append([]*ProgramBlock{block}, pm.program.Blocks...)
		pm.program.Modified = time.Now()
		return true
	}

	// Вставляем после указанного блока
	var insertIndex = -1
	for i, b := range pm.program.Blocks {
		if b.ID == afterBlockID {
			insertIndex = i + 1
			break
		}
	}

	if insertIndex == -1 {
		// Блок не найден, добавляем в конец
		pm.program.Blocks = append(pm.program.Blocks, block)
		pm.program.Modified = time.Now()
		return true
	}

	// Вставляем блок
	pm.program.Blocks = append(pm.program.Blocks[:insertIndex],
		append([]*ProgramBlock{block}, pm.program.Blocks[insertIndex:]...)...)

	// Обновляем связи
	prevBlock, _ := pm.GetBlock(afterBlockID)
	if prevBlock != nil {
		block.NextBlockID = prevBlock.NextBlockID
		prevBlock.NextBlockID = block.ID

		// Обновляем соединения
		pm.RemoveConnection(afterBlockID)
		pm.AddConnection(afterBlockID, block.ID)
		if block.NextBlockID > 0 {
			pm.AddConnection(block.ID, block.NextBlockID)
		}
	}

	pm.program.Modified = time.Now()
	return true
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

			// Безопасное получение параметров
			var port byte
			var power int8
			var duration uint16

			if p, ok := block.Parameters["port"].(byte); ok {
				port = p
			} else {
				port = 1
			}

			if p, ok := block.Parameters["power"].(int8); ok {
				power = p
			} else {
				power = 50
			}

			if d, ok := block.Parameters["duration"].(uint16); ok {
				duration = d
			} else {
				duration = 1000
			}

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

// GetProgram возвращает текущую программу.
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

// RemoveBlock полностью удаляет блок из программы
func (pm *ProgramManager) RemoveBlock(blockID int) bool {
	log.Printf("Начинаем удаление блока %d из программы", blockID)

	// Находим блок для удаления
	var blockToRemove *ProgramBlock
	var removeIndex int = -1

	for i, block := range pm.program.Blocks {
		if block.ID == blockID {
			blockToRemove = block
			removeIndex = i
			break
		}
	}

	if blockToRemove == nil {
		log.Printf("Блок %d не найден в программе", blockID)
		return false
	}

	// Удаляем блок из списка
	if removeIndex == 0 {
		pm.program.Blocks = pm.program.Blocks[1:]
	} else if removeIndex == len(pm.program.Blocks)-1 {
		pm.program.Blocks = pm.program.Blocks[:removeIndex]
	} else {
		pm.program.Blocks = append(
			pm.program.Blocks[:removeIndex],
			pm.program.Blocks[removeIndex+1:]...,
		)
	}

	// Удаляем все соединения, связанные с блоком
	var newConnections []*Connection
	for _, conn := range pm.program.Connections {
		if conn.FromBlockID != blockID && conn.ToBlockID != blockID {
			newConnections = append(newConnections, conn)
		}
	}
	pm.program.Connections = newConnections

	// Обновляем связи в оставшихся блоках
	pm.rebuildConnections()

	// Если удаляемый блок был начальным и остались другие блоки
	if blockToRemove.IsStart && len(pm.program.Blocks) > 0 {
		pm.program.Blocks[0].IsStart = true
	}

	pm.program.Modified = time.Now()
	log.Printf("Блок %d удален из программы. Осталось блоков: %d", blockID, len(pm.program.Blocks))
	return true
}

// GetProgramState возвращает состояние программы
func (pm *ProgramManager) GetProgramState() ProgramState {
	return pm.currentState
}

// GetBlockBeforeStop возвращает блок, который идет перед блоком "Стоп"
func (pm *ProgramManager) GetBlockBeforeStop() (*ProgramBlock, bool) {
	// Находим блок "Стоп"
	var stopBlock *ProgramBlock
	for _, block := range pm.program.Blocks {
		if block.Type == BlockTypeStop {
			stopBlock = block
			break
		}
	}

	if stopBlock == nil {
		return nil, false
	}

	// Находим блок, который ссылается на блок "Стоп"
	for _, block := range pm.program.Blocks {
		if block.NextBlockID == stopBlock.ID {
			return block, true
		}
	}

	return nil, false
}

// GetBlocksInOrder возвращает блоки в порядке их выполнения
func (pm *ProgramManager) GetBlocksInOrder() []*ProgramBlock {
	var ordered []*ProgramBlock
	visited := make(map[int]bool)

	// Находим стартовый блок
	var current *ProgramBlock
	for _, block := range pm.program.Blocks {
		if block.IsStart {
			current = block
			break
		}
	}

	// Если нет стартового блока, берем первый
	if current == nil && len(pm.program.Blocks) > 0 {
		current = pm.program.Blocks[0]
	}

	// Проходим по цепочке
	for current != nil && !visited[current.ID] {
		visited[current.ID] = true
		ordered = append(ordered, current)

		if current.NextBlockID == 0 {
			break
		}

		next, exists := pm.GetBlock(current.NextBlockID)
		if !exists {
			break
		}
		current = next
	}

	return ordered
}

// rebuildConnections перестраивает все связи после удаления блока
func (pm *ProgramManager) rebuildConnections() {
	// Очищаем все существующие связи
	pm.program.Connections = make([]*Connection, 0)

	// Очищаем NextBlockID у всех блоков
	for _, block := range pm.program.Blocks {
		block.NextBlockID = 0
	}

	// Создаем новые связи по порядку
	for i := 0; i < len(pm.program.Blocks)-1; i++ {
		currentBlock := pm.program.Blocks[i]
		nextBlock := pm.program.Blocks[i+1]

		currentBlock.NextBlockID = nextBlock.ID
		pm.program.Connections = append(pm.program.Connections, &Connection{
			FromBlockID: currentBlock.ID,
			ToBlockID:   nextBlock.ID,
		})
	}

	log.Printf("Связи перестроены. Создано %d соединений", len(pm.program.Connections))
}
