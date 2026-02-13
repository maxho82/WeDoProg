// app_state.go
package main

import "sync"

// AppState хранит всё разделяемое состояние приложения, доступ к которому
// может осуществляться из нескольких горутин (например, callback'и Bluetooth).
// Все методы используют мьютекс для безопасного доступа.
type AppState struct {
	mu               sync.RWMutex
	connectedHub     *HubInfo
	connectedDevices map[byte]*Device
	availableBlocks  map[BlockType]bool
	selectedBlock    *ProgramBlock
}

// NewAppState создаёт новый экземпляр состояния с инициализированными картами.
func NewAppState() *AppState {
	return &AppState{
		connectedDevices: make(map[byte]*Device),
		availableBlocks:  make(map[BlockType]bool),
	}
}

// --- connectedHub ---

// GetConnectedHub возвращает текущую информацию о хабе (или nil).
func (s *AppState) GetConnectedHub() *HubInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectedHub
}

// SetConnectedHub сохраняет информацию о хабе. Делает копию, чтобы избежать
// случайного изменения извне.
func (s *AppState) SetConnectedHub(hub *HubInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hub == nil {
		s.connectedHub = nil
		return
	}
	// Копируем, так как переданный указатель может быть использован где-то ещё.
	copyHub := *hub
	s.connectedHub = &copyHub
}

// UpdateHubBattery обновляет только уровень заряда батареи (если хаб существует).
func (s *AppState) UpdateHubBattery(level int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connectedHub != nil {
		s.connectedHub.Battery = level
	}
}

// --- connectedDevices ---

// GetDevice возвращает устройство по порту (или nil).
func (s *AppState) GetDevice(port byte) *Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectedDevices[port]
}

// GetAllDevices возвращает копию карты подключённых устройств.
// Возвращается новая карта, чтобы вызывающий мог безопасно итерировать.
func (s *AppState) GetAllDevices() map[byte]*Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	devices := make(map[byte]*Device, len(s.connectedDevices))
	for k, v := range s.connectedDevices {
		// Делаем поверхностную копию, т.к. поля Device не изменяются после создания.
		devCopy := *v
		devices[k] = &devCopy
	}
	return devices
}

// UpdateDevice сохраняет или обновляет устройство на порту.
// device копируется, чтобы избежать внешних изменений.
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

// RemoveDevice удаляет устройство на порту.
func (s *AppState) RemoveDevice(port byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connectedDevices, port)
}

// ClearDevices очищает карту устройств.
func (s *AppState) ClearDevices() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedDevices = make(map[byte]*Device)
}

// --- availableBlocks ---

// GetAvailableBlocks возвращает копию карты доступных блоков.
func (s *AppState) GetAvailableBlocks() map[BlockType]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blocks := make(map[BlockType]bool, len(s.availableBlocks))
	for k, v := range s.availableBlocks {
		blocks[k] = v
	}
	return blocks
}

// SetAvailableBlocks заменяет карту доступных блоков.
func (s *AppState) SetAvailableBlocks(blocks map[BlockType]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.availableBlocks = make(map[BlockType]bool, len(blocks))
	for k, v := range blocks {
		s.availableBlocks[k] = v
	}
}

// SetAvailableBlock устанавливает доступность конкретного блока.
func (s *AppState) SetAvailableBlock(bt BlockType, available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.availableBlocks[bt] = available
}

// --- selectedBlock ---

// GetSelectedBlock возвращает текущий выбранный блок.
func (s *AppState) GetSelectedBlock() *ProgramBlock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedBlock
}

// SetSelectedBlock сохраняет выбранный блок.
func (s *AppState) SetSelectedBlock(block *ProgramBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectedBlock = block
}