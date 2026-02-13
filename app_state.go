package main

import "sync"

// AppState хранит всё разделяемое состояние приложения, доступ к которому
// может осуществляться из нескольких горутин (например, callback'и Bluetooth).
// Содержит программу и состояние её выполнения.
type AppState struct {
	mu               sync.RWMutex
	connectedHub     *HubInfo
	connectedDevices map[byte]*Device
	availableBlocks  map[BlockType]bool
	selectedBlock    *ProgramBlock
	program          *Program     // текущая программа
	programState     ProgramState // состояние выполнения
}

// NewAppState создаёт новый экземпляр состояния с инициализированными картами.
func NewAppState() *AppState {
	return &AppState{
		connectedDevices: make(map[byte]*Device),
		availableBlocks:  make(map[BlockType]bool),
		program:          &Program{Name: "Новая программа", Blocks: []*ProgramBlock{}, Connections: []*Connection{}},
		programState:     ProgramStateStopped,
	}
}

// --- connectedHub ---
func (s *AppState) GetConnectedHub() *HubInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectedHub
}

func (s *AppState) SetConnectedHub(hub *HubInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hub == nil {
		s.connectedHub = nil
		return
	}
	copyHub := *hub
	s.connectedHub = &copyHub
}

func (s *AppState) UpdateHubBattery(level int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connectedHub != nil {
		s.connectedHub.Battery = level
	}
}

// --- connectedDevices ---
func (s *AppState) GetDevice(port byte) *Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectedDevices[port]
}

func (s *AppState) GetAllDevices() map[byte]*Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	devices := make(map[byte]*Device, len(s.connectedDevices))
	for k, v := range s.connectedDevices {
		devCopy := *v
		devices[k] = &devCopy
	}
	return devices
}

func (s *AppState) UpdateDevice(port byte, device *Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if device == nil {
		delete(s.connectedDevices, port)
		return
	}
	devCopy := *device
	s.connectedDevices[port] = &devCopy
}

func (s *AppState) RemoveDevice(port byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connectedDevices, port)
}

func (s *AppState) ClearDevices() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedDevices = make(map[byte]*Device)
}

// --- availableBlocks ---
func (s *AppState) GetAvailableBlocks() map[BlockType]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blocks := make(map[BlockType]bool, len(s.availableBlocks))
	for k, v := range s.availableBlocks {
		blocks[k] = v
	}
	return blocks
}

func (s *AppState) SetAvailableBlocks(blocks map[BlockType]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.availableBlocks = make(map[BlockType]bool, len(blocks))
	for k, v := range blocks {
		s.availableBlocks[k] = v
	}
}

func (s *AppState) SetAvailableBlock(bt BlockType, available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.availableBlocks[bt] = available
}

// --- selectedBlock ---
func (s *AppState) GetSelectedBlock() *ProgramBlock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedBlock
}

func (s *AppState) SetSelectedBlock(block *ProgramBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectedBlock = block
}

// ========== Методы для работы с программой ==========

// GetProgram возвращает текущую программу.
// Внимание: возвращаемый объект нельзя модифицировать напрямую.
// Для изменений используйте методы ProgramManager или SetProgram.
func (s *AppState) GetProgram() *Program {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.program
}

// SetProgram заменяет текущую программу.
func (s *AppState) SetProgram(prog *Program) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.program = prog
}

// UpdateProgramBlocks заменяет список блоков программы (с копированием).
func (s *AppState) UpdateProgramBlocks(blocks []*ProgramBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Создаём глубокую копию блоков, чтобы избежать случайных изменений извне.
	newBlocks := make([]*ProgramBlock, len(blocks))
	for i, b := range blocks {
		copyBlock := *b
		// Копируем Parameters, так как это map
		if b.Parameters != nil {
			copyBlock.Parameters = make(map[string]interface{}, len(b.Parameters))
			for k, v := range b.Parameters {
				copyBlock.Parameters[k] = v
			}
		}
		newBlocks[i] = &copyBlock
	}
	s.program.Blocks = newBlocks
}

// GetProgramState возвращает состояние выполнения программы.
func (s *AppState) GetProgramState() ProgramState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.programState
}

// SetProgramState устанавливает состояние выполнения программы.
func (s *AppState) SetProgramState(state ProgramState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.programState = state
}

// FindBlockByID находит блок по ID внутри программы.
func (s *AppState) FindBlockByID(id int) *ProgramBlock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.program.Blocks {
		if b.ID == id {
			return b
		}
	}
	return nil
}
