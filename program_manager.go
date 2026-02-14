package main

import (
	"fmt"
	"log"
	"strconv"
	"sync"
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
	BlockTypeVariable
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

// LoopContext контекст выполнения цикла
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

	stateChangeCB    func(state ProgramState)
	currentBlockCB   func(blockID int)
	programChangedCB func() // callback для уведомления об изменении программы

	variables map[string]VariableValue
	varsMutex sync.RWMutex
}

// VariableValue представляет значение переменной с типом
type VariableValue struct {
	Type  string // "bool", "int", "string"
	Value interface{}
}

// NewProgramManager создаёт менеджер программ
func NewProgramManager(hubMgr *HubManager, deviceMgr *DeviceManager, state *AppState) *ProgramManager {
	return &ProgramManager{
		hubMgr:    hubMgr,
		deviceMgr: deviceMgr,
		state:     state,
		variables: make(map[string]VariableValue),
		varsMutex: sync.RWMutex{},
	}
}

// SetProgramChangedCallback устанавливает callback, вызываемый при любом изменении программы.
func (pm *ProgramManager) SetProgramChangedCallback(cb func()) {
	pm.programChangedCB = cb
}

// notifyProgramChanged вызывает callback об изменении программы (если установлен).
func (pm *ProgramManager) notifyProgramChanged() {
	if pm.programChangedCB != nil {
		pm.programChangedCB()
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

// InsertBlock вставляет блок в программу.
// afterBlockID = -1 означает вставку в конец, 0 – в начало.
func (pm *ProgramManager) InsertBlock(block *ProgramBlock, afterBlockID int) bool {
	prog := pm.state.GetProgram()
	blocks := prog.Blocks

	// Создаём копию слайса блоков, чтобы не модифицировать оригинал напрямую
	newBlocks := make([]*ProgramBlock, len(blocks))
	copy(newBlocks, blocks)

	if afterBlockID == -1 {
		// Вставка в конец
		newBlocks = append(newBlocks, block)
		// Обновляем NextBlockID предыдущего блока, если он есть
		if len(blocks) > 0 {
			prev := blocks[len(blocks)-1]
			// Ищем индекс предыдущего в newBlocks (он остался на том же месте)
			for _, b := range newBlocks {
				if b.ID == prev.ID {
					// Если предыдущий блок не является блоком, после которого нельзя вставлять?
					// Просто установим связь.
					b.NextBlockID = block.ID
					pm.addConnection(b.ID, block.ID)
					break
				}
			}
		}
	} else if afterBlockID == 0 {
		// Вставка в начало
		for _, b := range newBlocks {
			b.IsStart = false
		}
		block.IsStart = true
		if len(newBlocks) > 0 {
			block.NextBlockID = newBlocks[0].ID
			pm.addConnection(block.ID, newBlocks[0].ID)
		}
		newBlocks = append([]*ProgramBlock{block}, newBlocks...)
	} else {
		// Вставка после конкретного блока
		insertIndex := -1
		for i, b := range newBlocks {
			if b.ID == afterBlockID {
				insertIndex = i + 1
				break
			}
		}
		if insertIndex == -1 {
			// Если блок не найден, вставляем в конец
			newBlocks = append(newBlocks, block)
		} else {
			// Вставляем на позицию insertIndex
			newBlocks = append(newBlocks[:insertIndex], append([]*ProgramBlock{block}, newBlocks[insertIndex:]...)...)
			// Обновляем связи: предыдущий блок (afterBlockID) теперь указывает на новый
			for _, b := range newBlocks {
				if b.ID == afterBlockID {
					b.NextBlockID = block.ID
					pm.addConnection(b.ID, block.ID)
				}
				// Новый блок должен указывать на следующий (который был после afterBlockID)
				if b.ID == block.ID && insertIndex+1 < len(newBlocks) {
					nextBlock := newBlocks[insertIndex+1]
					block.NextBlockID = nextBlock.ID
					pm.addConnection(block.ID, nextBlock.ID)
				}
			}
		}
	}

	// Сохраняем программу
	prog.Blocks = newBlocks
	pm.state.SetProgram(prog)
	pm.rebuildConnectionsFromBlocks() // перестроим список соединений на основе порядка блоков
	pm.notifyProgramChanged()
	return true
}

// addConnection добавляет соединение в программу (без дублирования)
func (pm *ProgramManager) addConnection(fromID, toID int) {
	prog := pm.state.GetProgram()
	// Проверим, нет ли уже такого соединения
	for _, conn := range prog.Connections {
		if conn.FromBlockID == fromID && conn.ToBlockID == toID {
			return
		}
	}
	prog.Connections = append(prog.Connections, &Connection{FromBlockID: fromID, ToBlockID: toID})
}

// rebuildConnectionsFromBlocks перестраивает список соединений на основе порядка блоков.
func (pm *ProgramManager) rebuildConnectionsFromBlocks() {
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
}

// RemoveBlock удаляет блок из программы.
func (pm *ProgramManager) RemoveBlock(blockID int) bool {
	prog := pm.state.GetProgram()
	newBlocks := make([]*ProgramBlock, 0, len(prog.Blocks))
	for _, b := range prog.Blocks {
		if b.ID != blockID {
			newBlocks = append(newBlocks, b)
		}
	}
	if len(newBlocks) == len(prog.Blocks) {
		return false // блок не найден
	}
	prog.Blocks = newBlocks
	pm.rebuildConnectionsFromBlocks()
	pm.state.SetProgram(prog)
	pm.notifyProgramChanged()
	return true
}

// GetBlock возвращает блок по ID.
func (pm *ProgramManager) GetBlock(blockID int) (*ProgramBlock, bool) {
	block := pm.state.FindBlockByID(blockID)
	return block, block != nil
}

// UpdateBlock обновляет параметры блока.
func (pm *ProgramManager) UpdateBlock(blockID int, params map[string]interface{}) bool {
	prog := pm.state.GetProgram()
	for _, block := range prog.Blocks {
		if block.ID == blockID {
			for k, v := range params {
				block.Parameters[k] = v
			}
			pm.state.SetProgram(prog)
			pm.notifyProgramChanged()
			return true
		}
	}
	return false
}

// ClearProgram удаляет все блоки и соединения.
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
	pm.notifyProgramChanged()
	log.Println("Программа очищена")
}

// EnsureTerminalBlocks проверяет наличие блоков "Начать" и "Стоп" и добавляет их при необходимости.
func (pm *ProgramManager) EnsureTerminalBlocks() {
	prog := pm.state.GetProgram()
	hasStart := false
	hasStop := false
	for _, b := range prog.Blocks {
		if b.Type == BlockTypeStart {
			hasStart = true
		}
		if b.Type == BlockTypeStop {
			hasStop = true
		}
	}
	if !hasStart {
		startBlock := pm.CreateBlock(BlockTypeStart, 0, 0)
		pm.InsertBlock(startBlock, 0) // вставляем в начало
	}
	if !hasStop {
		stopBlock := pm.CreateBlock(BlockTypeStop, 0, 0)
		pm.InsertBlock(stopBlock, -1) // вставляем в конец
	}
}

// GetLoopBlocks возвращает все блоки цикла (включая начало и конец).
func (pm *ProgramManager) GetLoopBlocks(loopStartID int) ([]*ProgramBlock, bool) {
	prog := pm.state.GetProgram()
	loopStart := pm.state.FindBlockByID(loopStartID)
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

// FindLoopEndID находит ID конца цикла по ID начала цикла.
func (pm *ProgramManager) FindLoopEndID(loopStartID int) (int, bool) {
	loopStart := pm.state.FindBlockByID(loopStartID)
	if loopStart == nil || loopStart.Type != BlockTypeLoopStart {
		return 0, false
	}
	if endID, ok := loopStart.Parameters["loopEndID"].(int); ok && endID > 0 {
		return endID, true
	}
	return 0, false
}

// --- Методы выполнения программы ---

// RunProgram запускает выполнение программы.
func (pm *ProgramManager) RunProgram() error {
	if pm.state.GetProgramState() == ProgramStateRunning {
		return fmt.Errorf("программа уже выполняется")
	}
	if !pm.hubMgr.IsConnected() {
		return fmt.Errorf("не подключено к хабу")
	}
	// Больше не вызываем EnsureTerminalBlocks здесь
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
	pm.clearVariables() // очистка переменнх
	pm.state.SetProgramState(ProgramStateRunning)
	pm.notifyStateChange()
	log.Println("Запуск программы...")
	go pm.executeProgram(startBlock)
	return nil
}

// StopProgram останавливает выполнение программы.
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

// GetProgramState возвращает состояние программы.
func (pm *ProgramManager) GetProgramState() ProgramState {
	return pm.state.GetProgramState()
}

// SetStateChangeCallback устанавливает callback для изменения состояния.
func (pm *ProgramManager) SetStateChangeCallback(callback func(state ProgramState)) {
	pm.stateChangeCB = callback
}

// SetCurrentBlockCallback устанавливает callback для отслеживания текущего блока.
func (pm *ProgramManager) SetCurrentBlockCallback(callback func(blockID int)) {
	pm.currentBlockCB = callback
}

// notifyStateChange уведомляет об изменении состояния.
func (pm *ProgramManager) notifyStateChange() {
	if pm.stateChangeCB != nil {
		pm.stateChangeCB(pm.state.GetProgramState())
	}
}

// --- Вспомогательные методы для выполнения ---

// Методы для работы с переменными
func (pm *ProgramManager) SetVariable(name string, val VariableValue) {
	pm.varsMutex.Lock()
	defer pm.varsMutex.Unlock()
	pm.variables[name] = val
}

func (pm *ProgramManager) GetVariable(name string) (VariableValue, bool) {
	pm.varsMutex.RLock()
	defer pm.varsMutex.RUnlock()
	val, ok := pm.variables[name]
	return val, ok
}

// Очистка переменных перед запуском
func (pm *ProgramManager) clearVariables() {
	pm.varsMutex.Lock()
	defer pm.varsMutex.Unlock()
	pm.variables = make(map[string]VariableValue)
}

// ------------------------------------------------------------------------------

// EvaluateExpression вычисляет строку выражения в контексте текущих переменных
func (pm *ProgramManager) EvaluateExpression(expr string) (interface{}, error) {
	if expr == "" {
		return nil, nil
	}
	node, err := ParseExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга выражения: %v", err)
	}
	// Копируем переменные для безопасности
	varsCopy := make(map[string]VariableValue)
	pm.varsMutex.RLock()
	for k, v := range pm.variables {
		varsCopy[k] = v
	}
	pm.varsMutex.RUnlock()
	return node.Evaluate(varsCopy)
}

//------------------------------------------------------------------------------

// executeProgram выполняет программу.
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
	}
	pm.finishExecution()
}

// handleLoopStart обрабатывает начало цикла.
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

// handleLoopEnd обрабатывает конец цикла, возвращает true, если нужно продолжать.
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

// handleRegularBlock выполняет обычный блок.
func (pm *ProgramManager) handleRegularBlock(block *ProgramBlock) error {
	if block.OnExecute != nil {
		return block.OnExecute()
	}
	return nil
}

// handleStopBlock выполняет блок "Стоп".
func (pm *ProgramManager) handleStopBlock(block *ProgramBlock) {
	if block.OnExecute != nil {
		_ = block.OnExecute()
	}
	pm.finishExecution()
}

// finishExecution завершает выполнение, останавливает моторы и звуки.
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

// ensureAllMotorsStopped останавливает все моторы.
func (pm *ProgramManager) ensureAllMotorsStopped() {
	log.Println("Гарантированная остановка всех моторов...")
	for port := byte(1); port <= 6; port++ {
		if pm.deviceMgr != nil && pm.hubMgr != nil && pm.hubMgr.IsConnected() {
			stopCmd := []byte{port, 0x01, 0x01, 0x00}
			_ = pm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
		}
	}
}

// stopAllSounds останавливает все звуки.
func (pm *ProgramManager) stopAllSounds() {
	log.Println("Остановка всех звуков...")
	for port := byte(1); port <= 6; port++ {
		if pm.deviceMgr != nil && pm.hubMgr != nil && pm.hubMgr.IsConnected() {
			stopCmd := []byte{port, 0x03, 0x00}
			_ = pm.hubMgr.WriteCharacteristic(OUTPUT_COMMAND_UUID, stopCmd)
		}
	}
}

// configureBlock настраивает блок в зависимости от типа.
func (pm *ProgramManager) configureBlock(block *ProgramBlock) {
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
		block.Parameters["mode"] = byte(0) // 0 - RGB, 1 - индексный
		block.Parameters["red"] = byte(255)
		block.Parameters["green"] = byte(0)
		block.Parameters["blue"] = byte(0)
		block.Parameters["colorExpr"] = ""
		block.OnExecute = func() error {
			if !pm.hubMgr.IsConnected() {
				return fmt.Errorf("не подключено к хабу")
			}
			port := block.Parameters["port"].(byte)
			mode := block.Parameters["mode"].(byte)
			if mode == 0 {
				red := block.Parameters["red"].(byte)
				green := block.Parameters["green"].(byte)
				blue := block.Parameters["blue"].(byte)
				return pm.deviceMgr.SetLEDColor(port, red, green, blue)
			} else {
				expr, _ := block.Parameters["colorExpr"].(string)
				if expr == "" {
					// по умолчанию розовый
					return pm.deviceMgr.SetLEDIndexColor(port, 1)
				}
				// вычисляем выражение
				result, err := pm.EvaluateExpression(expr)
				if err != nil {
					return fmt.Errorf("ошибка вычисления цвета: %v", err)
				}
				// конвертируем в цвет
				conv, err := pm.convertValue(VarTypeColor, result)
				if err != nil {
					return err
				}
				colorIdx := conv.(byte)
				return pm.deviceMgr.SetLEDIndexColor(port, colorIdx)
			}
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
			cmd := LPF2Protocol{}.EncodeTiltSensorModeCommand(port, mode)
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
			cmd := LPF2Protocol{}.EncodeDistanceSensorModeCommand(port, mode)
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
			cmd := LPF2Protocol{}.EncodeVoltageSensorModeCommand(port)
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
			cmd := LPF2Protocol{}.EncodeCurrentSensorModeCommand(port)
			return pm.hubMgr.WriteCharacteristic(INPUT_COMMAND_UUID, cmd)
		}
	case BlockTypeStop:
		block.Title = "Стоп"
		block.Description = "Остановка программы"
		block.OnExecute = func() error {
			pm.StopProgram()
			return nil
		}
	case BlockTypeVariable:
		block.Title = "Переменная"
		block.Description = "Объявление переменной"
		block.Parameters["name"] = "var"
		block.Parameters["varType"] = "int"
		block.Parameters["expression"] = ""
		block.OnExecute = func() error {
			name, ok := block.Parameters["name"].(string)
			if !ok || name == "" {
				return fmt.Errorf("имя переменной не задано")
			}
			varType, _ := block.Parameters["varType"].(string)
			expr, _ := block.Parameters["expression"].(string)

			var rawVal interface{}
			if expr == "" {
				// значение по умолчанию
				switch varType {
				case "bool":
					rawVal = false
				case "int":
					rawVal = 0
				case "string":
					rawVal = ""
				case VarTypeColor:
					rawVal = byte(0)
				default:
					return fmt.Errorf("неизвестный тип переменной: %s", varType)
				}
			} else {
				// вычисляем выражение
				result, err := pm.EvaluateExpression(expr)
				if err != nil {
					return fmt.Errorf("ошибка вычисления выражения для переменной %s: %v", name, err)
				}
				rawVal = result
			}
			// конвертируем в целевой тип
			converted, err := pm.convertValue(varType, rawVal)
			if err != nil {
				return err
			}
			pm.SetVariable(name, VariableValue{Type: varType, Value: converted})
			log.Printf("Переменная %s = %v (%s)", name, converted, varType)
			return nil
		}
	}
}

// getBlockColor возвращает цвет для типа блока.
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

// ----конвертация переменных
// convertValue конвертирует значение val в целевой тип targetType
func (pm *ProgramManager) convertValue(targetType string, val interface{}) (interface{}, error) {
	// Допустимые индексы цветов (включая 0 как "выключено")
	validColorIndices := []byte{0, 0x01, 0x02, 0x03, 0x05, 0x09, 0x0A}

	// поиск ближайшего индекса
	nearestColorIndex := func(x int) byte {
		if x <= 0 {
			return 0
		}
		best := validColorIndices[0]
		bestDist := abs(x - int(best))
		for _, idx := range validColorIndices[1:] {
			dist := abs(x - int(idx))
			if dist < bestDist {
				bestDist = dist
				best = idx
			}
		}
		return best
	}

	switch targetType {
	case "bool":
		switch v := val.(type) {
		case bool:
			return v, nil
		case int, int64, float64:
			num := toInt(v)
			return num != 0, nil
		case string:
			return v != "", nil
		default:
			return false, nil
		}

	case "int":
		switch v := val.(type) {
		case bool:
			if v {
				return 1, nil
			}
			return 0, nil
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			if num, err := strconv.Atoi(v); err == nil {
				return num, nil
			}
			return 0, nil
		default:
			return 0, nil
		}

	case "string":
		switch v := val.(type) {
		case bool:
			if v {
				return "Да", nil
			}
			return "Нет", nil
		case int:
			return strconv.Itoa(v), nil
		case int64:
			return strconv.FormatInt(v, 10), nil
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		case string:
			return v, nil
		case byte:
			if name, ok := IndexColorNames[v]; ok {
				return name, nil
			}
			return strconv.Itoa(int(v)), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}

	case VarTypeColor: // "color"
		switch v := val.(type) {
		case bool:
			if v {
				return byte(1), nil // розовый
			}
			return byte(0), nil
		case int:
			return nearestColorIndex(v), nil
		case int64:
			return nearestColorIndex(int(v)), nil
		case float64:
			return nearestColorIndex(int(v)), nil
		case string:
			// сначала по имени цвета
			if idx, ok := colorNameToIndex(v); ok {
				return idx, nil
			}
			// затем как число
			if num, err := strconv.Atoi(v); err == nil {
				return nearestColorIndex(num), nil
			}
			return byte(0), nil
		case byte:
			return nearestColorIndex(int(v)), nil
		default:
			return byte(0), nil
		}

	default:
		return nil, fmt.Errorf("неизвестный тип переменной: %s", targetType)
	}
}

// toInt преобразует различные числовые типы в int
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
