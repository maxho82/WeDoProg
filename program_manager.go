package main

import (
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
)

// BlockType определяет тип блока программирования
type BlockType int

const (
	BlockTypeStart BlockType = iota
	BlockTypeMotor
	BlockTypeLED
	BlockTypeWait
	BlockTypeLoopStart
	BlockTypeLoopEnd
	BlockTypeCondition
	BlockTypeTiltSensor
	BlockTypeDistanceSensor
	BlockTypeSound
	BlockTypeVoltageSensor
	BlockTypeCurrentSensor
	BlockTypeStop
)

// ProgramState состояние выполнения программы
type ProgramState int

const (
	ProgramStateStopped ProgramState = iota
	ProgramStateRunning
	ProgramStatePaused
	ProgramStateError
)

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
	DragStartPos fyne.Position // из пакета fyne, но импорт не показан для краткости
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

// LoopContext контекст выполнения цикла (используется внутри executeProgram)
type LoopContext struct {
	StartBlockID int
	EndBlockID   int
	CurrentIter  int
	MaxIter      int
	IsForever    bool
}

// ProgramManager управляет программами, используя AppState как хранилище.
type ProgramManager struct {
	hubMgr    *HubManager
	deviceMgr *DeviceManager
	state     *AppState

	stateChangeCB  func(state ProgramState)
	currentBlockCB func(blockID int)
}

// NewProgramManager создаёт менеджер программ
func NewProgramManager(hubMgr *HubManager, deviceMgr *DeviceManager, state *AppState) *ProgramManager {
	return &ProgramManager{
		hubMgr:    hubMgr,
		deviceMgr: deviceMgr,
		state:     state,
	}
}

// --- Методы работы с программой ---

// CreateBlock создаёт новый блок (не добавляет в программу)
func (pm *ProgramManager) CreateBlock(blockType BlockType, x, y float64) *ProgramBlock {
	prog := pm.state.GetProgram()
	newID := 1
	for _, block := range prog.Blocks {
		if block.ID >= newID {
			newID = block.ID + 1
		}
	}
	block := &ProgramBlock{
		ID:          newID,
		Type:        blockType,
		X:           x,
		Y:           y,
		Width:       DefaultBlockWidth,
		Height:      DefaultBlockHeight,
		Parameters:  make(map[string]interface{}),
		IsStart:     (blockType == BlockTypeStart),
		NextBlockID: 0,
		Color:       getBlockColor(blockType),
	}
	pm.configureBlock(block)
	log.Printf("Создан блок: %s (ID: %d)", block.Title, block.ID)
	return block
}

// InsertBlock вставляет блок в программу
func (pm *ProgramManager) InsertBlock(block *ProgramBlock, afterBlockID int) bool {
	prog := pm.state.GetProgram()
	blocks := prog.Blocks

	if afterBlockID == -1 {
		blocks = append(blocks, block)
		var prevBlock *ProgramBlock
		for _, b := range blocks {
			if b.ID != block.ID && b.Type != BlockTypeStop && b.NextBlockID == 0 {
				prevBlock = b
			}
		}
		if prevBlock != nil {
			prevBlock.NextBlockID = block.ID
			pm.AddConnection(prevBlock.ID, block.ID)
		}
		pm.state.UpdateProgramBlocks(blocks)
		return true
	}
	if afterBlockID == 0 {
		for _, b := range blocks {
			b.IsStart = false
		}
		block.IsStart = true
		block.NextBlockID = 0
		if len(blocks) > 0 {
			block.NextBlockID = blocks[0].ID
			pm.AddConnection(block.ID, blocks[0].ID)
		}
		blocks = append([]*ProgramBlock{block}, blocks...)
		pm.state.UpdateProgramBlocks(blocks)
		return true
	}

	insertIndex := -1
	for i, b := range blocks {
		if b.ID == afterBlockID {
			insertIndex = i + 1
			break
		}
	}
	if insertIndex == -1 {
		blocks = append(blocks, block)
	} else {
		blocks = append(blocks[:insertIndex], append([]*ProgramBlock{block}, blocks[insertIndex:]...)...)
	}
	pm.state.UpdateProgramBlocks(blocks)
	pm.rebuildConnections()
	return true
}

// AddConnection добавляет соединение между блоками
func (pm *ProgramManager) AddConnection(fromBlockID, toBlockID int) bool {
	prog := pm.state.GetProgram()
	fromBlock := pm.findBlockByID(fromBlockID)
	toBlock := pm.findBlockByID(toBlockID)
	if fromBlock == nil || toBlock == nil {
		return false
	}
	fromBlock.NextBlockID = toBlockID
	conn := &Connection{FromBlockID: fromBlockID, ToBlockID: toBlockID}
	prog.Connections = append(prog.Connections, conn)
	pm.state.SetProgram(prog)
	return true
}

// RemoveConnection удаляет соединение по fromBlockID
func (pm *ProgramManager) RemoveConnection(fromBlockID int) bool {
	prog := pm.state.GetProgram()
	newConns := make([]*Connection, 0, len(prog.Connections))
	for _, conn := range prog.Connections {
		if conn.FromBlockID != fromBlockID {
			newConns = append(newConns, conn)
		}
	}
	prog.Connections = newConns
	if block := pm.findBlockByID(fromBlockID); block != nil {
		block.NextBlockID = 0
	}
	pm.state.SetProgram(prog)
	return true
}

// RemoveBlock удаляет блок из программы
func (pm *ProgramManager) RemoveBlock(blockID int) bool {
	prog := pm.state.GetProgram()
	newBlocks := make([]*ProgramBlock, 0, len(prog.Blocks))
	for _, b := range prog.Blocks {
		if b.ID != blockID {
			newBlocks = append(newBlocks, b)
		}
	}
	prog.Blocks = newBlocks
	pm.rebuildConnections()
	pm.state.SetProgram(prog)
	return true
}

// rebuildConnections перестраивает все связи в порядке следования блоков
func (pm *ProgramManager) rebuildConnections() {
	prog := pm.state.GetProgram()
	prog.Connections = make([]*Connection, 0)
	for i := 0; i < len(prog.Blocks)-1; i++ {
		current := prog.Blocks[i]
		next := prog.Blocks[i+1]
		current.NextBlockID = next.ID
		prog.Connections = append(prog.Connections, &Connection{FromBlockID: current.ID, ToBlockID: next.ID})
	}
	if len(prog.Blocks) > 0 {
		prog.Blocks[len(prog.Blocks)-1].NextBlockID = 0
	}
	pm.state.SetProgram(prog)
}

// GetBlock возвращает блок по ID
func (pm *ProgramManager) GetBlock(blockID int) (*ProgramBlock, bool) {
	block := pm.findBlockByID(blockID)
	return block, block != nil
}

// UpdateBlock обновляет параметры блока
func (pm *ProgramManager) UpdateBlock(blockID int, params map[string]interface{}) bool {
	prog := pm.state.GetProgram()
	for _, block := range prog.Blocks {
		if block.ID == blockID {
			for k, v := range params {
				block.Parameters[k] = v
			}
			pm.state.SetProgram(prog)
			return true
		}
	}
	return false
}

// ClearProgram удаляет все блоки и соединения
func (pm *ProgramManager) ClearProgram() {
	prog := &Program{
		Name:        "Новая программа",
		Blocks:      []*ProgramBlock{},
		Connections: []*Connection{},
		Created:     time.Now(),
		Modified:    time.Now(),
	}
	pm.state.SetProgram(prog)
	pm.state.SetProgramState(ProgramStateStopped)
	pm.notifyStateChange()
	log.Println("Программа очищена")
}

// StopProgram останавливает выполнение программы
func (pm *ProgramManager) StopProgram() {
	if pm.state.GetProgramState() == ProgramStateRunning {
		pm.state.SetProgramState(ProgramStateStopped)
		pm.notifyStateChange()
		log.Println("Программа остановлена")
		pm.ensureAllMotorsStopped()
		pm.stopAllSounds()
		if pm.currentBlockCB != nil {
			pm.currentBlockCB(-1)
		}
	}
}

// --- Методы выполнения программы ---

// RunProgram запускает выполнение программы
func (pm *ProgramManager) RunProgram() error {
	if pm.state.GetProgramState() == ProgramStateRunning {
		return fmt.Errorf("программа уже выполняется")
	}
	if !pm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}
	prog := pm.state.GetProgram()
	if len(prog.Blocks) == 0 {
		return fmt.Errorf("нет блоков в программе")
	}
	var startBlock *ProgramBlock
	for _, block := range prog.Blocks {
		if block.Type == BlockTypeStart {
			startBlock = block
			break
		}
	}
	if startBlock == nil {
		return fmt.Errorf("программа должна содержать блок 'Начать'")
	}
	hasStop := false
	for _, block := range prog.Blocks {
		if block.Type == BlockTypeStop {
			hasStop = true
			break
		}
	}
	if !hasStop {
		return fmt.Errorf("программа должна содержать блок 'Стоп'")
	}

	pm.state.SetProgramState(ProgramStateRunning)
	pm.notifyStateChange()
	log.Println("Запуск программы...")
	go pm.executeProgram(startBlock)
	return nil
}

// executeProgram выполняет программу
func (pm *ProgramManager) executeProgram(startBlock *ProgramBlock) {
	log.Println("=== Начало выполнения программы ===")
	loopStack := make([]*LoopContext, 0)

	currentBlock := startBlock
	for pm.state.GetProgramState() == ProgramStateRunning && currentBlock != nil {
		if pm.currentBlockCB != nil {
			pm.currentBlockCB(currentBlock.ID)
		}

		switch currentBlock.Type {
		case BlockTypeStop:
			pm.handleStopBlock(currentBlock)
			return

		case BlockTypeLoopStart:
			// Проверяем, не находимся ли мы уже внутри этого цикла (возврат после итерации)
			found := false
			for _, ctx := range loopStack {
				if ctx.StartBlockID == currentBlock.ID {
					found = true
					break
				}
			}
			if !found {
				// Первый вход – создаём контекст
				loopCtx, err := pm.handleLoopStart(currentBlock)
				if err != nil {
					log.Printf("Ошибка в начале цикла: %v", err)
					pm.state.SetProgramState(ProgramStateError)
					pm.finishExecution()
					return
				}
				loopStack = append(loopStack, loopCtx)
			}
			// После начала цикла (или при возврате) переходим к следующему блоку (первому внутри цикла)
			if currentBlock.NextBlockID != 0 {
				next, _ := pm.GetBlock(currentBlock.NextBlockID)
				currentBlock = next
				continue
			}

		case BlockTypeLoopEnd:
			if len(loopStack) == 0 {
				log.Printf("Ошибка: блок конца цикла без начала")
				pm.state.SetProgramState(ProgramStateError)
				pm.finishExecution()
				return
			}
			loopCtx := loopStack[len(loopStack)-1]
			shouldContinue, err := pm.handleLoopEnd(currentBlock, loopCtx)
			if err != nil {
				log.Printf("Ошибка в конце цикла: %v", err)
				pm.state.SetProgramState(ProgramStateError)
				pm.finishExecution()
				return
			}
			if shouldContinue {
				// Переходим к первому блоку внутри цикла (следующий за LoopStart)
				startLoop, _ := pm.GetBlock(loopCtx.StartBlockID)
				if startLoop.NextBlockID != 0 {
					next, _ := pm.GetBlock(startLoop.NextBlockID)
					currentBlock = next
				} else {
					// Пустое тело цикла – переходим на конец цикла (блок LoopEnd)
					endBlock, _ := pm.GetBlock(loopCtx.EndBlockID)
					currentBlock = endBlock
				}
				continue
			} else {
				// Завершаем цикл – удаляем контекст и переходим к следующему за LoopEnd блоку
				loopStack = loopStack[:len(loopStack)-1]
			}

		default:
			if err := pm.handleRegularBlock(currentBlock); err != nil {
				log.Printf("Ошибка выполнения блока %d: %v", currentBlock.ID, err)
				pm.state.SetProgramState(ProgramStateError)
				pm.finishExecution()
				return
			}
		}

		// Переход к следующему блоку по цепочке (если не было continue)
		if currentBlock.NextBlockID == 0 {
			break
		}
		next, exists := pm.GetBlock(currentBlock.NextBlockID)
		if !exists {
			log.Printf("Ошибка: следующий блок %d не найден", currentBlock.NextBlockID)
			pm.state.SetProgramState(ProgramStateError)
			pm.finishExecution()
			return
		}
		currentBlock = next

		if currentBlock.Type != BlockTypeWait {
			time.Sleep(100 * time.Millisecond)
		}
	}
	pm.finishExecution()
}

// handleLoopStart обрабатывает начало цикла
func (pm *ProgramManager) handleLoopStart(block *ProgramBlock) (*LoopContext, error) {
	loopEndID, ok := block.Parameters["loopEndID"].(int)
	if !ok || loopEndID == 0 {
		return nil, fmt.Errorf("блок цикла %d не имеет связанного конца цикла", block.ID)
	}
	forever, _ := block.Parameters["forever"].(bool)
	maxIter := 1
	if !forever {
		if c, ok := block.Parameters["count"].(int); ok {
			maxIter = c
		}
	}
	ctx := &LoopContext{
		StartBlockID: block.ID,
		EndBlockID:   loopEndID,
		CurrentIter:  0,
		MaxIter:      maxIter,
		IsForever:    forever,
	}
	if block.OnExecute != nil {
		if err := block.OnExecute(); err != nil {
			return nil, err
		}
	}
	log.Printf("Начало цикла %d. Параметры: forever=%v, count=%d", block.ID, forever, maxIter)
	return ctx, nil
}

// handleLoopEnd обрабатывает конец цикла, возвращает true, если нужно продолжать
func (pm *ProgramManager) handleLoopEnd(block *ProgramBlock, ctx *LoopContext) (bool, error) {
	if block.OnExecute != nil {
		if err := block.OnExecute(); err != nil {
			return false, err
		}
	}
	ctx.CurrentIter++
	shouldContinue := ctx.IsForever || ctx.CurrentIter < ctx.MaxIter
	log.Printf("Конец цикла %d. Итерация %d/%d. Продолжать: %v", ctx.StartBlockID, ctx.CurrentIter, ctx.MaxIter, shouldContinue)
	return shouldContinue, nil
}

// handleRegularBlock выполняет обычный блок
func (pm *ProgramManager) handleRegularBlock(block *ProgramBlock) error {
	if block.OnExecute != nil {
		return block.OnExecute()
	}
	return nil
}

// handleStopBlock выполняет блок "Стоп"
func (pm *ProgramManager) handleStopBlock(block *ProgramBlock) {
	if block.OnExecute != nil {
		_ = block.OnExecute()
	}
	pm.finishExecution()
}

// finishExecution завершает выполнение, останавливает моторы и звуки
func (pm *ProgramManager) finishExecution() {
	pm.state.SetProgramState(ProgramStateStopped)
	pm.notifyStateChange()
	pm.ensureAllMotorsStopped()
	pm.stopAllSounds()
	if pm.currentBlockCB != nil {
		pm.currentBlockCB(-1)
	}
	log.Println("=== Программа завершена ===")
}

// ensureAllMotorsStopped останавливает все моторы
func (pm *ProgramManager) ensureAllMotorsStopped() {
	log.Println("Гарантированная остановка всех моторов...")
	for port := byte(1); port <= 6; port++ {
		if pm.deviceMgr != nil && pm.hubMgr != nil && pm.hubMgr.IsConnected() {
			stopCmd := []byte{port, 0x01, 0x01, 0x00}
			_ = pm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
		}
	}
}

// stopAllSounds останавливает все звуки
func (pm *ProgramManager) stopAllSounds() {
	log.Println("Остановка всех звуков...")
	for port := byte(1); port <= 6; port++ {
		if pm.deviceMgr != nil && pm.hubMgr != nil && pm.hubMgr.IsConnected() {
			stopCmd := []byte{port, 0x03, 0x00}
			_ = pm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
		}
	}
}

// --- Вспомогательные методы ---

// configureBlock настраивает блок в зависимости от типа
func (pm *ProgramManager) configureBlock(block *ProgramBlock) {
	// Цвет извлекается из отдельной функции
	block.Color = getBlockColor(block.Type)

	switch block.Type {
	case BlockTypeStart:
		block.Title = "Начать"
		block.Description = "Начало программы"
		block.IsStart = true
		block.OnExecute = func() error {
			log.Println("Начало программы")
			return nil
		}
	case BlockTypeMotor:
		block.Title = "Мотор"
		block.Description = "Управление мотором"
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
		block.Parameters["duration"] = 1.0
		block.OnExecute = func() error {
			duration := block.Parameters["duration"].(float64)
			log.Printf("Пауза: %.1f секунд", duration)
			time.Sleep(time.Duration(duration*1000) * time.Millisecond)
			return nil
		}
	case BlockTypeLoopStart:
		block.Title = "ДЛЯ"
		block.Description = "Начало цикла"
		block.Parameters["count"] = 5
		block.Parameters["forever"] = false
		block.Parameters["loopEndID"] = 0
		block.OnExecute = func() error {
			log.Println("Начало цикла")
			return nil
		}
	case BlockTypeLoopEnd:
		block.Title = "КЦ"
		block.Description = "Конец цикла"
		block.Parameters["loopStartID"] = 0
		block.OnExecute = func() error {
			log.Println("Конец цикла")
			return nil
		}
	case BlockTypeCondition:
		block.Title = "Условие"
		block.Description = "Условный оператор"
		block.OnExecute = func() error {
			log.Println("Проверка условия")
			return nil
		}
	case BlockTypeTiltSensor:
		block.Title = "Датчик наклона"
		block.Description = "Чтение датчика наклона"
		block.Parameters["port"] = byte(1)
		block.Parameters["mode"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			mode := block.Parameters["mode"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x22, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)
		}
	case BlockTypeDistanceSensor:
		block.Title = "Датчик расстояния"
		block.Description = "Измерение расстояния"
		block.Parameters["port"] = byte(1)
		block.Parameters["mode"] = byte(0)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			mode := block.Parameters["mode"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x23, mode, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)
		}
	case BlockTypeSound:
		block.Title = "Звук"
		block.Description = "Воспроизведение звука"
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
		block.Parameters["port"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)
		}
	case BlockTypeCurrentSensor:
		block.Title = "Датчик тока"
		block.Description = "Измерение тока"
		block.Parameters["port"] = byte(1)
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			cmd := []byte{0x01, 0x02, port, 0x15, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01}
			return pm.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)
		}
	case BlockTypeStop:
		block.Title = "Стоп"
		block.Description = "Остановка программы"
		block.OnExecute = func() error {
			pm.StopProgram()
			return nil
		}
	}
}

// getBlockColor возвращает цвет для типа блока
func getBlockColor(blockType BlockType) string {
	switch blockType {
	case BlockTypeStart:
		return "#4CAF50"
	case BlockTypeMotor:
		return "#2196F3"
	case BlockTypeLED:
		return "#FF9800"
	case BlockTypeWait:
		return "#9E9E9E"
	case BlockTypeLoopStart, BlockTypeLoopEnd:
		return "#9C27B0"
	case BlockTypeCondition:
		return "#3F51B5"
	case BlockTypeTiltSensor:
		return "#673AB7"
	case BlockTypeDistanceSensor:
		return "#00BCD4"
	case BlockTypeSound:
		return "#FF5722"
	case BlockTypeVoltageSensor:
		return "#8BC34A"
	case BlockTypeCurrentSensor:
		return "#F44336"
	case BlockTypeStop:
		return "#F44336"
	default:
		return "#607D8B"
	}
}

// findBlockByID ищет блок по ID в программе
func (pm *ProgramManager) findBlockByID(id int) *ProgramBlock {
	prog := pm.state.GetProgram()
	for _, b := range prog.Blocks {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// GetLoopBlocks возвращает все блоки цикла (включая начало и конец)
func (pm *ProgramManager) GetLoopBlocks(loopStartID int) ([]*ProgramBlock, bool) {
	prog := pm.state.GetProgram()
	loopStart := pm.findBlockByID(loopStartID)
	if loopStart == nil || loopStart.Type != BlockTypeLoopStart {
		return nil, false
	}
	loopEndID, ok := loopStart.Parameters["loopEndID"].(int)
	if !ok {
		return nil, false
	}
	var result []*ProgramBlock
	inLoop := false
	for _, block := range prog.Blocks {
		if block.ID == loopStartID {
			inLoop = true
		}
		if inLoop {
			result = append(result, block)
		}
		if block.ID == loopEndID {
			break
		}
	}
	return result, true
}

// GetProgramState возвращает состояние программы
func (pm *ProgramManager) GetProgramState() ProgramState {
	return pm.state.GetProgramState()
}

// SetStateChangeCallback устанавливает callback для изменения состояния
func (pm *ProgramManager) SetStateChangeCallback(callback func(state ProgramState)) {
	pm.stateChangeCB = callback
}

// SetCurrentBlockCallback устанавливает callback для отслеживания текущего блока
func (pm *ProgramManager) SetCurrentBlockCallback(callback func(blockID int)) {
	pm.currentBlockCB = callback
}

// notifyStateChange уведомляет об изменении состояния
func (pm *ProgramManager) notifyStateChange() {
	if pm.stateChangeCB != nil {
		pm.stateChangeCB(pm.state.GetProgramState())
	}
}

// FindLoopEndID находит ID конца цикла по ID начала цикла.
func (pm *ProgramManager) FindLoopEndID(loopStartID int) (int, bool) {
	prog := pm.state.GetProgram()
	for _, block := range prog.Blocks {
		if block.Type == BlockTypeLoopStart && block.ID == loopStartID {
			if endID, ok := block.Parameters["loopEndID"].(int); ok && endID > 0 {
				return endID, true
			}
			break
		}
	}
	return 0, false
}
