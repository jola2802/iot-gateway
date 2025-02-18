package logic

import (
	"database/sql"
	"time"

	opcua "iot-gateway/driver/opcua"
	s7 "iot-gateway/driver/s7"

	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

// Define detailed device states
const (
	Stopped       = "0 (stopped)"
	Running       = "1 (running)"
	Initializing  = "2 (initializing)"
	Error         = "3 (error)"
	No_Datapoints = "4 (no datapoints)"
	No_Connection = "5 (no connection)"
	Paused        = "4 (paused)"
	deleted       = "9 (deleted)"
)

var (
	opcuaStopChans    = make(map[string]chan struct{})
	s7StopChans       = make(map[string]chan struct{})
	opcuaDeviceStates = make(map[string]*DeviceState)
	s7DeviceStates    = make(map[string]*DeviceState)
	server            *MQTT.Server
	db                *sql.DB
)

// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%% Handling-All-Driver %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

func StartAllDrivers(dbF *sql.DB, serverF *MQTT.Server) {
	server = serverF
	db = dbF
	logrus.Info("DM: Starting all drivers...")

	// Setze für alle Geräte initialen Status in der DB
	if _, err := db.Exec("UPDATE devices SET status = ?", Initializing); err != nil {
		logrus.Errorf("DM: Error updating devices to initializing: %v", err)
	}

	// Alle Gerätedaten zwischenlesen und in einem Slice speichern
	query := `SELECT id, type, name, address, acquisition_time FROM devices`
	rows, err := db.Query(query)
	if err != nil {
		logrus.Fatalf("DM: Error querying devices from database: %v", err)
	}
	defer rows.Close()

	// Verwende ein Slice von []interface{} pro Zeile
	var devicesData [][]interface{}
	for rows.Next() {
		var id, devType, name, address string
		var acqTime int
		if err := rows.Scan(&id, &devType, &name, &address, &acqTime); err != nil {
			logrus.Errorf("DM: Error scanning device data: %v", err)
			continue
		}
		// Speichere die Zeile als Slice – ohne extra Struct
		devicesData = append(devicesData, []interface{}{id, devType, name, address, acqTime})
	}

	// Am besten in einer eigenen Goroutine starten
	go func() {
		// Treiber anhand der im Slice gespeicherten Daten starten
		for _, d := range devicesData {
			// Da wir wissen, dass id, devType, name, address und acqTime in dieser Reihenfolge kommen:
			deviceID := d[0].(string)
			deviceType := d[1].(string)
			deviceName := d[2].(string)
			// deviceAddress := d[3].(string) // falls benötigt
			// acquisitionTime := d[4].(int)   // falls benötigt

			switch deviceType {
			case "opc-ua":
				StartOPCUADriver(db, deviceID)
			case "s7":
				StartS7Driver(db, deviceID)
			case "mqtt":
				// MQTT-Treiber starten, falls erforderlich
			default:
				logrus.Warnf("DM: Unknown device type %s for device %s", deviceType, deviceName)
			}
		}
	}()

	// Aktualisiere in der DB den Status auf Running, sofern noch Initializing gesetzt ist
	if _, err := db.Exec("UPDATE devices SET status = ? WHERE status = ?", Running, Initializing); err != nil {
		logrus.Errorf("DM: Error updating devices to running: %v", err)
	}

	logrus.Info("DM: All drivers started.")
}

func StopAllDrivers() {
	logrus.Info("DM: Stopping all drivers...")

	for deviceID := range opcuaDeviceStates {
		stopOPCUADriver(deviceID)
	}

	for deviceID := range s7DeviceStates {
		stopS7Driver(deviceID)
	}

	logrus.Info("DM: All drivers have been stopped.")
}

func RestartAllDrivers(db *sql.DB) {
	logrus.Info("DM: Restarting all drivers...")
	StopAllDrivers()
	time.Sleep(200 * time.Millisecond)
	StartAllDrivers(db, server)
}

// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%% OPC-UA-Part %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

// StartOPCUADriver-Funktion
func StartOPCUADriver(db *sql.DB, deviceID string) {
	state := getOrCreateDeviceState(deviceID, opcuaDeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	state.status = Initializing
	publishDeviceState(server, "opc-ua", deviceID, state.status)

	// Lese die Basis-Gerätekonfiguration
	opcuaConfig, err := readDeviceConfig(db, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("%v", err)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}

	// Lese optionale Sicherheitsdaten und wende diese an
	secOpts, err := readSecurityOptions(db, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("%v", err)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}
	applySecurityOptions(&opcuaConfig, secOpts)

	// Lese die zugehörigen OPC-UA-Knoten
	nodes, err := readOPCUANodes(db, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("%v", err)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}
	if len(nodes) == 0 {
		state.status = No_Datapoints
		logrus.Errorf("DM: No OPC-UA nodes found for device %s", opcuaConfig.Name)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}
	opcuaConfig.DataNode = nodes

	// Verbindungstest vor dem Start des Treibers
	if connected := opcua.TestConnection(opcuaConfig); !connected {
		state.status = No_Connection
		logrus.Errorf("DM: Keine Verbindung möglich zu OPC-UA Gerät %s", opcuaConfig.Name)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}

	// Treiber starten
	stopChan := make(chan struct{})
	opcuaStopChans[deviceID] = stopChan

	// Starte den OPC-UA-Treiber in einer separaten Goroutine
	go func() {
		if err := opcua.Run(opcuaConfig, db, stopChan, server); err != nil {
			st := getOrCreateDeviceState(deviceID, opcuaDeviceStates)
			st.mu.Lock()
			defer st.mu.Unlock()
			st.status = Error
			publishDeviceState(server, "opc-ua", deviceID, st.status)
			logrus.Errorf("DM: Error running OPC-UA driver for device %s: %v", opcuaConfig.Name, err)
		}
	}()

	state.running = true
	state.status = Running
	publishDeviceState(server, "opc-ua", deviceID, state.status)
	logrus.Infof("DM: OPC-UA driver started for device %s.", opcuaConfig.Name)
}

// StopOPCUADriver stops the OPC-UA driver for a given device.
//
// It locks the mutex, checks if the device is running, and if so, closes the stop channel,
// sets the device state to false, publishes the device state, and logs a message.
//
// Example:
//
//	stopOPCUADriver("device-123")
func stopOPCUADriver(deviceID string) {
	state := getOrCreateDeviceState(deviceID, opcuaDeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	// Falls der Treiber nicht läuft, ist nichts zu tun.
	if !state.running {
		logrus.Warnf("DM: OPC-UA driver for device %s is not running.", deviceID)
		return
	}

	// Stop-Channel schließen, falls vorhanden.
	if stopChan, ok := opcuaStopChans[deviceID]; ok && stopChan != nil {
		close(stopChan)
		delete(opcuaStopChans, deviceID)
	}

	// Aktualisiere den Gerätestatus
	state.running = false
	state.status = Stopped
	publishDeviceState(server, "opc-ua", deviceID, state.status)
	logrus.Infof("DM: Stopped OPC-UA driver for device %s.", deviceID)
}

// RestartOPCUADriver restarts the OPC-UA driver for a given device.
//
// It stops the OPC-UA driver, waits for 3 seconds, and then starts the OPC-UA driver again.
//
// Example:
//
//	db, _ := sql.Open("mysql", "user:password@tcp(localhost:3306)/database")
//	restartOPCUADriver(db, "device-123")
func restartOPCUADriver(db *sql.DB, deviceID string) {
	logrus.Infof("DM: Restarting OPC-UA driver for device %s...", deviceID)
	stopOPCUADriver(deviceID)
	time.Sleep(1 * time.Second)
	StartOPCUADriver(db, deviceID)
}

// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%% S7-Part %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

// Verbesserte StartS7Driver-Funktion
func StartS7Driver(db *sql.DB, deviceID string) {
	state := getOrCreateDeviceState(deviceID, s7DeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	state.status = Initializing
	publishDeviceState(server, "s7", deviceID, state.status)

	// Geräte-Konfiguration laden
	s7Config, err := readS7DeviceConfig(db, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("%v", err)
		publishDeviceState(server, "s7", deviceID, state.status)
		return
	}

	// Verbindungstest vor dem Start des Treibers
	if connected := s7.TestConnection(s7Config); !connected {
		state.status = No_Connection
		logrus.Errorf("DM: Keine Verbindung möglich zu S7 Gerät %s", s7Config.Name)
		publishDeviceState(server, "s7", deviceID, state.status)
		return
	}

	// 2. S7-Datenpunkte laden
	datapoints, err := readS7Datapoints(db, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("%v", err)
		publishDeviceState(server, "s7", deviceID, state.status)
		return
	}
	if len(datapoints) == 0 {
		state.status = No_Datapoints
		logrus.Errorf("DM: No S7 datapoints found for device %s", s7Config.Name)
		publishDeviceState(server, "s7", deviceID, state.status)
		return
	}
	s7Config.Datapoint = datapoints

	// 3. Starte den S7-Treiber
	stopChan := make(chan struct{})
	s7StopChans[deviceID] = stopChan

	// Starte den S7-Treiber in einer separaten Goroutine
	go func(config opcua.DeviceConfig) {
		if err := s7.Run(config, db, stopChan, server); err != nil {
			state := getOrCreateDeviceState(deviceID, s7DeviceStates)
			state.mu.Lock()
			defer state.mu.Unlock()
			state.status = Error
			publishDeviceState(server, "s7", deviceID, state.status)
			logrus.Errorf("DM: Error running S7 driver for device %s: %v", config.Name, err)
		}
	}(s7Config)

	state.running = true
	state.status = Running
	publishDeviceState(server, "s7", deviceID, state.status)
	logrus.Infof("DM: S7 driver started for device %s.", s7Config.Name)
}

// StopS7Driver stops the S7 driver for a given device.
func stopS7Driver(deviceID string) {
	state := getOrCreateDeviceState(deviceID, s7DeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.running {
		logrus.Warnf("DM: S7 driver for device %s is not running.", deviceID)
		return
	}

	close(s7StopChans[deviceID])
	state.running = false
	state.status = Stopped
	publishDeviceState(server, "s7", deviceID, state.status)
	logrus.Infof("DM: Stopped S7 driver for device %s.", deviceID)
}

// RestartS7Driver restarts the S7 driver for a given device.
func restartS7Driver(db *sql.DB, deviceID string) {
	logrus.Infof("DM: Restarting S7 driver for device %s...", deviceID)
	stopS7Driver(deviceID)
	time.Sleep(1 * time.Second)
	StartS7Driver(db, deviceID)
}
