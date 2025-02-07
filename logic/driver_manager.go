package logic

import (
	"database/sql"
	"strconv"
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
	// Jetzt ist der DB-Query abgeschlossen und die DB-Sperre freigegeben

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

// OPC-UA-Part START
func StartOPCUADriver(db *sql.DB, deviceID string) {
	state := getOrCreateDeviceState(deviceID, opcuaDeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	state.status = Initializing
	publishDeviceState(server, "opc-ua", deviceID, state.status)

	// Verwende sql.NullString für Felder, die NULL sein könnten
	var deviceAddress, deviceName string
	var securityMode, securityPolicy, certificate, key, username, password sql.NullString
	var acquisitionTime int

	// Lade die Gerätedaten inklusive Sicherheitsdaten aus der `devices`-Tabelle
	deviceQuery := `SELECT name, address, acquisition_time, security_mode, security_policy, certificate, key, username, password FROM devices WHERE id = ?`
	err := db.QueryRow(deviceQuery, deviceID).Scan(&deviceName, &deviceAddress, &acquisitionTime, &securityMode, &securityPolicy, &certificate, &key, &username, &password)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying device data from devices table: %v", err)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}

	// Konfiguration für OPC-UA-Gerät erstellen
	opcuaConfig := opcua.DeviceConfig{
		ID:              deviceID,
		Name:            deviceName,
		Address:         deviceAddress,
		AcquisitionTime: acquisitionTime,
	}

	// Lade die OPC-UA-Knoten basierend auf der device_id aus der `opcua_datanodes`-Tabelle
	nodeQuery := `SELECT name, node_identifier FROM opcua_datanodes WHERE device_id = ?`
	rows, err := db.Query(nodeQuery, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying OPC-UA nodes from opcua_datanodes table: %v", err)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}
	defer rows.Close()

	found := false

	// Iteriere über alle gefundenen Zeilen
	for rows.Next() {
		found = true
		var nodeName, nodeIdentifier string
		if err := rows.Scan(&nodeName, &nodeIdentifier); err != nil {
			state.status = No_Datapoints
			logrus.Errorf("DM: Error scanning OPC-UA node data: %v", err)
			publishDeviceState(server, "opc-ua", deviceID, state.status)
			return
		}
		opcuaConfig.DataNode = append(opcuaConfig.DataNode, opcua.DataNode{
			Name: nodeName,
			Node: nodeIdentifier,
		})
	}

	// Falls keine Zeilen gefunden wurden, gib einen Fehler aus und verlasse die Funktion
	if !found {
		state.status = No_Datapoints
		logrus.Errorf("DM: No OPC-UA nodes found for device %s", deviceName)
		publishDeviceState(server, "opc-ua", deviceID, state.status)
		return
	}

	// Nur die nicht-null Werte übernehmen
	if securityMode.Valid {
		opcuaConfig.SecurityMode = securityMode.String
	}
	if securityPolicy.Valid {
		opcuaConfig.SecurityPolicy = securityPolicy.String
	}
	if certificate.Valid {
		opcuaConfig.CertFile = certificate.String
		opcuaConfig.KeyFile = key.String
	}
	if username.Valid {
		opcuaConfig.Username = username.String
	}
	if password.Valid {
		opcuaConfig.Password = password.String
	}

	// Lade die node_identifiers in die OPC-UA-Konfiguration
	for rows.Next() {
		var nodeIdentifier, nodeName string
		if err := rows.Scan(&nodeName, &nodeIdentifier); err != nil {
			state.status = Error
			logrus.Errorf("DM: Error scanning OPC-UA node data: %v", err)
			publishDeviceState(server, "opc-ua", deviceID, state.status)
			return
		}
		// Füge die Knoten zur Konfiguration hinzu
		opcuaConfig.DataNode = append(opcuaConfig.DataNode, opcua.DataNode{
			Name: nodeName,
			Node: nodeIdentifier,
		}) // hier wurde der nodeName entfernt
	}

	// Starte den OPC-UA-Treiber mit den geladenen Daten
	stopChan := make(chan struct{})
	opcuaStopChans[deviceID] = stopChan

	// Starte den OPC-UA-Treiber mit den geladenen Daten
	go func() {
		err := opcua.Run(opcuaConfig, db, stopChan, server)
		if err != nil {
			// Setze den Gerätestatus auf Error, wenn ein Fehler auftritt
			state := getOrCreateDeviceState(deviceID, opcuaDeviceStates)
			state.mu.Lock()
			defer state.mu.Unlock()
			state.status = Error
			publishDeviceState(server, "opc-ua", deviceID, state.status)
			logrus.Errorf("DM: Error running OPC-UA driver for device %s: %v", deviceName, err)
		}
	}()

	state.running = true
	state.status = Running
	publishDeviceState(server, "opc-ua", deviceID, state.status)
	logrus.Infof("DM: OPC-UA driver started for device %s.", deviceName)
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

// S7-PART
func StartS7Driver(db *sql.DB, deviceID string) {
	state := getOrCreateDeviceState(deviceID, s7DeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	state.status = Initializing
	publishDeviceState(server, "s7", deviceID, state.status)

	// 1. Lade die Hauptgerätekonfiguration aus der devices-Tabelle
	var s7Config opcua.DeviceConfig
	var rack, slot string
	deviceQuery := `SELECT name, address, rack, slot, acquisition_time FROM devices WHERE id = ?`
	err := db.QueryRow(deviceQuery, deviceID).Scan(&s7Config.Name, &s7Config.Address, &rack, &slot, &s7Config.AcquisitionTime)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying S7 device config from devices table: %v", err)
		publishDeviceState(server, "s7", deviceID, state.status)
		return
	}
	s7Config.Rack, _ = strconv.Atoi(rack)
	s7Config.Slot, _ = strconv.Atoi(slot)

	// 2. Lade die S7-Datenpunkte aus der s7_datapoints-Tabelle in ein Slice ein
	datapointQuery := `SELECT name, datatype, address FROM s7_datapoints WHERE device_id = ?`
	rows, err := db.Query(datapointQuery, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying S7 datapoints from s7_datapoints table: %v", err)
		publishDeviceState(server, "s7", deviceID, state.status)
		return
	}

	var datapoints []opcua.Datapoint
	for rows.Next() {
		var dpName, dpDatatype, dpAddress string
		if err := rows.Scan(&dpName, &dpDatatype, &dpAddress); err != nil {
			state.status = Error
			logrus.Errorf("DM: Error scanning S7 datapoint data: %v", err)
			publishDeviceState(server, "s7", deviceID, state.status)
			rows.Close()
			return
		}
		datapoints = append(datapoints, opcua.Datapoint{
			Name:     dpName,
			Datatype: dpDatatype,
			Address:  dpAddress,
		})
	}
	rows.Close()

	// Falls keine Datenpunkte gefunden wurden, Abbruch
	if len(datapoints) == 0 {
		state.status = No_Datapoints
		logrus.Errorf("DM: No S7 datapoints found for device %s", s7Config.Name)
		publishDeviceState(server, "s7", deviceID, state.status)
		return
	}

	// Setze die eingelesenen Datenpunkte in die Konfiguration
	s7Config.Datapoint = datapoints

	// 4. Starte den S7-Treiber mit den geladenen Daten
	stopChan := make(chan struct{})
	s7StopChans[deviceID] = stopChan

	go func() {
		err := s7.Run(s7Config, db, stopChan)
		if err != nil {
			// Setze den Gerätestatus auf Error, wenn ein Fehler auftritt
			state := getOrCreateDeviceState(deviceID, s7DeviceStates)
			state.mu.Lock()
			defer state.mu.Unlock()
			state.status = Error
			publishDeviceState(server, "s7", deviceID, state.status)
			logrus.Errorf("DM: Error running S7 driver for device %s: %v", s7Config.Name, err)
		}
	}()

	state.running = true
	state.status = Running
	publishDeviceState(server, "s7", deviceID, state.status)
	logrus.Infof("DM: S7 driver started for device %s.", s7Config.Name)
}

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

func restartS7Driver(db *sql.DB, deviceID string) {
	logrus.Infof("DM: Restarting S7 driver for device %s...", deviceID)
	stopS7Driver(deviceID)
	time.Sleep(1 * time.Second)
	StartS7Driver(db, deviceID)
}
