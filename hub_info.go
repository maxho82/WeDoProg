package main

import "time"

// HubInfo содержит информацию о подключенном хабе
type HubInfo struct {
	Name            string
	Address         string
	RSSI            int
	Manufacturer    string
	FirmwareVersion string
	SoftwareVersion string
	SystemID        string
	Battery         int
	LastUpdated     time.Time
}

// Device представляет подключенное устройство
type Device struct {
	PortID      byte
	DeviceType  byte
	Name        string
	IsConnected bool
	LastValue   interface{}
	LastUpdate  time.Time
	Properties  map[string]interface{}
}

// PortInfo информация о порте хаба
type PortInfo struct {
	PortID      byte
	DeviceType  byte
	DeviceName  string
	IsConnected bool
	LastValue   []byte
	Mode        byte
	LastUpdate  time.Time
}
