package logic

import (
	"database/sql"
	"strconv"
	"time"

	opcua "iot-gateway/driver/opcua"
	s7 "iot-gateway/driver/s7"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

// Define detailed device states
const (
	Stopped      = "0 (stopped)"
	Running      = "1 (running)"
	Initializing = "2 (initializing)"
	Error        = "3 (error)"
	Paused       = "4 (paused)"
	deleted      = "9 (deleted)"
)

var (
	opcuaStopChans = make(map[string]chan struct{})
	s7StopChans    = make(map[string]chan struct{})
)

// Individuelle Zustände für OPC-UA und S7-Geräte
var (
	opcuaDeviceStates = make(map[string]*DeviceState)
	s7DeviceStates    = make(map[string]*DeviceState)
)

// General

func StartAllDrivers(db *sql.DB) {
	logrus.Info("DM: Starting all drivers...")

	// Lade die Gerätedaten aus der Datenbank
	query := `SELECT id, type, name, address, aquisition_time FROM devices`
	rows, err := db.Query(query)
	if err != nil {
		logrus.Fatalf("DM: Error querying devices from database: %v", err)
	}
	defer rows.Close()

	mqttClient := getPooledMQTTClient(mqttClientPool, db)
	defer releaseMQTTClient(mqttClientPool, mqttClient)

	for rows.Next() {
		var deviceID, deviceType, deviceName, deviceAddress string
		var acquisitionTime int
		if err := rows.Scan(&deviceID, &deviceType, &deviceName, &deviceAddress, &acquisitionTime); err != nil {
			logrus.Errorf("DM: Error scanning device data: %v", err)
			continue
		}

		// Starte den entsprechenden Treiber basierend auf dem Gerätetyp
		switch deviceType {
		case "opcua":
			StartOPCUADriver(db, deviceName)
		case "s7":
			StartS7Driver(db, deviceName)
		case "mqtt":
			state := getOrCreateDeviceState(deviceName, opcuaDeviceStates)
			state.mu.Lock()
			defer state.mu.Unlock()

			state.status = Running
			publishDeviceState(mqttClient, deviceType, deviceName, state.status)
		default:
			logrus.Warnf("DM: Unknown device type %s for device %s", deviceType, deviceName)
		}
	}
	logrus.Info("DM: All drivers started.")
}

func StopAllDrivers() {
	logrus.Info("DM: Stopping all drivers...")

	for deviceName := range opcuaDeviceStates {
		stopOPCUADriver(deviceName)
	}

	for deviceName := range s7DeviceStates {
		stopS7Driver(deviceName)
	}

	logrus.Info("DM: All drivers have been stopped.")
}

func RestartAllDrivers(db *sql.DB) {
	logrus.Info("DM: Restarting all drivers...")
	StopAllDrivers()
	time.Sleep(200 * time.Millisecond)
	StartAllDrivers(db)
}

// OPC-UA-Part START
func StartOPCUADriver(db *sql.DB, deviceName string) {
	state := getOrCreateDeviceState(deviceName, opcuaDeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	mqttClient := getPooledMQTTClient(mqttClientPool, db)
	defer releaseMQTTClient(mqttClientPool, mqttClient)

	state.status = Initializing
	publishDeviceState(mqttClient, "opcua", deviceName, state.status)

	// Verwende sql.NullString für Felder, die NULL sein könnten
	var deviceAddress, deviceID string
	var securityMode, securityPolicy, certificate, key, username, password sql.NullString
	var acquisitionTime int

	// Lade die Gerätedaten inklusive Sicherheitsdaten aus der `devices`-Tabelle
	deviceQuery := `SELECT id, name, address, aquisition_time, security_mode, security_policy, certificate, key, username, password FROM devices WHERE name = ?`
	err := db.QueryRow(deviceQuery, deviceName).Scan(&deviceID, &deviceName, &deviceAddress, &acquisitionTime, &securityMode, &securityPolicy, &certificate, &key, &username, &password)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying device data from devices table: %v", err)
		publishDeviceState(mqttClient, "opcua", deviceName, state.status)
		return
	}

	// Lade die OPC-UA-Knoten basierend auf der device_id aus der `opcua_datanodes`-Tabelle
	nodeQuery := `SELECT name, node_identifier FROM opcua_datanodes WHERE device_id = ?`
	rows, err := db.Query(nodeQuery, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying OPC-UA nodes from opcua_datanodes table: %v", err)
		publishDeviceState(mqttClient, "opcua", deviceName, state.status)
		return
	}
	defer rows.Close()

	// Konfiguration für OPC-UA-Gerät erstellen
	opcuaConfig := opcua.DeviceConfig{
		Name:            deviceName,
		Address:         deviceAddress,
		AcquisitionTime: acquisitionTime,
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
			publishDeviceState(mqttClient, "opcua", deviceName, state.status)
			return
		}
		// Füge die Knoten zur Konfiguration hinzu
		opcuaConfig.DataNode = append(opcuaConfig.DataNode, nodeIdentifier) // hier wurde der nodeName entfernt
	}

	// Starte den OPC-UA-Treiber mit den geladenen Daten
	stopChan := make(chan struct{})
	opcuaStopChans[deviceName] = stopChan

	// Starte den OPC-UA-Treiber mit den geladenen Daten
	go func() {
		err := opcua.Run(opcuaConfig, db, stopChan)
		if err != nil {
			// Setze den Gerätestatus auf Error, wenn ein Fehler auftritt
			state := getOrCreateDeviceState(deviceName, opcuaDeviceStates)
			state.mu.Lock()
			defer state.mu.Unlock()
			state.status = Error
			publishDeviceState(mqttClient, "opcua", deviceName, state.status)
			logrus.Errorf("DM: Error running OPC-UA driver for device %s: %v", deviceName, err)
		}
	}()

	state.running = true
	state.status = Running
	publishDeviceState(mqttClient, "opcua", deviceName, state.status)
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
func stopOPCUADriver(deviceName string) {
	state := getOrCreateDeviceState(deviceName, opcuaDeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.running {
		logrus.Warnf("DM: OPC-UA driver for device %s is not running.", deviceName)
		return
	}

	mqttClient := getPooledMQTTClient(mqttClientPool)
	defer releaseMQTTClient(mqttClientPool, mqttClient)

	close(opcuaStopChans[deviceName])
	state.running = false
	state.status = Stopped
	publishDeviceState(mqttClient, "opcua", deviceName, state.status)
	logrus.Infof("DM: Stopped OPC-UA driver for device %s.", deviceName)
}

// RestartOPCUADriver restarts the OPC-UA driver for a given device.
//
// It stops the OPC-UA driver, waits for 3 seconds, and then starts the OPC-UA driver again.
//
// Example:
//
//	db, _ := sql.Open("mysql", "user:password@tcp(localhost:3306)/database")
//	restartOPCUADriver(db, "device-123")
func restartOPCUADriver(db *sql.DB, deviceName string) {
	logrus.Infof("DM: Restarting OPC-UA driver for device %s...", deviceName)
	stopOPCUADriver(deviceName)
	time.Sleep(1 * time.Second)
	StartOPCUADriver(db, deviceName)
}

// S7-PART
func StartS7Driver(db *sql.DB, deviceName string) {
	state := getOrCreateDeviceState(deviceName, s7DeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	mqttClient := getPooledMQTTClient(mqttClientPool, db)
	defer releaseMQTTClient(mqttClientPool, mqttClient)

	state.status = Initializing
	publishDeviceState(mqttClient, "s7", deviceName, state.status)

	// 1. Lade die Hauptgerätekonfiguration (z.B. Name, Adresse, Rack, Slot) aus der devices-Tabelle
	var s7Config opcua.DeviceConfig
	var deviceID int
	var rack, slot string
	deviceQuery := `SELECT id, name, address, rack, slot, aquisition_time FROM devices WHERE name = ?`
	err := db.QueryRow(deviceQuery, deviceName).Scan(&deviceID, &s7Config.Name, &s7Config.Address, &rack, &slot, &s7Config.AcquisitionTime)
	s7Config.Rack, err = strconv.Atoi(rack)
	s7Config.Slot, err = strconv.Atoi(slot)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying S7 device config from devices table: %v", err)
		publishDeviceState(mqttClient, "s7", deviceName, state.status)
		return
	}

	// 2. Lade die S7-Datenpunkte (name und address) aus der s7_datapoints-Tabelle
	datapointQuery := `SELECT name, datatype, address FROM s7_datapoints WHERE device_id = ?`
	rows, err := db.Query(datapointQuery, deviceID)
	if err != nil {
		state.status = Error
		logrus.Errorf("DM: Error querying S7 datapoints from s7_datapoints table: %v", err)
		publishDeviceState(mqttClient, "s7", deviceName, state.status)
		return
	}
	defer rows.Close()

	// 3. Füge die Datenpunkte zur Konfiguration hinzu
	for rows.Next() {
		var dpName, dpDatatype, dpAddress string
		if err := rows.Scan(&dpName, &dpDatatype, &dpAddress); err != nil {
			state.status = Error
			logrus.Errorf("DM: Error scanning S7 datapoint data: %v", err)
			publishDeviceState(mqttClient, "s7", deviceName, state.status)
			return
		}
		// Füge den Datenpunkt zur Konfiguration hinzu
		s7Config.Datapoint = append(s7Config.Datapoint, opcua.Datapoint{
			Name:     dpName,
			Datatype: dpDatatype,
			Address:  dpAddress,
		})
	}

	// 4. Starte den S7-Treiber mit den geladenen Daten
	stopChan := make(chan struct{})
	s7StopChans[deviceName] = stopChan

	// Starte den S7-Treiber mit den geladenen Daten
	go func() {
		err := s7.Run(s7Config, db, stopChan)
		if err != nil {
			// Setze den Gerätestatus auf Error, wenn ein Fehler auftritt
			state := getOrCreateDeviceState(deviceName, s7DeviceStates)
			state.mu.Lock()
			defer state.mu.Unlock()
			state.status = Error
			publishDeviceState(mqttClient, "s7", deviceName, state.status)
			logrus.Errorf("DM: Error running S7 driver for device %s: %v", deviceName, err)
		}
	}()

	state.running = true
	state.status = Running
	publishDeviceState(mqttClient, "s7", deviceName, state.status)
	logrus.Infof("DM: S7 driver started for device %s.", deviceName)
}

func stopS7Driver(deviceName string) {
	state := getOrCreateDeviceState(deviceName, s7DeviceStates)
	state.mu.Lock()
	defer state.mu.Unlock()

	mqttClient := getPooledMQTTClient(mqttClientPool)
	defer releaseMQTTClient(mqttClientPool, mqttClient)

	if !state.running {
		logrus.Warnf("DM: S7 driver for device %s is not running.", deviceName)
		return
	}

	close(s7StopChans[deviceName])
	state.running = false
	state.status = Stopped
	publishDeviceState(mqttClient, "s7", deviceName, state.status)
	logrus.Infof("DM: Stopped S7 driver for device %s.", deviceName)
}

func restartS7Driver(db *sql.DB, deviceName string) {
	logrus.Infof("DM: Restarting S7 driver for device %s...", deviceName)
	stopS7Driver(deviceName)
	time.Sleep(1 * time.Second)
	StartS7Driver(db, deviceName)
}

func handleCommand(db *sql.DB, job DeviceCommand) {
	topic := job.Topic
	command := job.Command
	deviceType, deviceName := parseTopic(topic)

	switch command {
	case "start":
		if deviceType == "opcua" {
			StartOPCUADriver(db, deviceName)
		} else if deviceType == "s7" {
			StartS7Driver(db, deviceName)
		}
	case "stop":
		if deviceType == "opcua" {
			stopOPCUADriver(deviceName)
		} else if deviceType == "s7" {
			stopS7Driver(deviceName)
		}
	case "restart":
		if deviceType == "opcua" {
			restartOPCUADriver(db, deviceName)
		} else if deviceType == "s7" {
			restartS7Driver(db, deviceName)
		}
	case "1 (running)", "0 (stopped)", "2 (initializing)", "9 (deleted)", "3 (error)", "4 (paused)":
		return
	default:
		logrus.Warnf("DM: Received unknown command for device %s: %s", deviceName, command)
	}

	logrus.Infof("received command %s for device %s", command, deviceName)
}

func handleMqttDriverCommands(message mqtt.Message, db *sql.DB) {
	client := getPooledMQTTClient(mqttClientPool, db)
	defer releaseMQTTClient(mqttClientPool, client)

	job := DeviceCommand{
		Topic:   message.Topic(),
		Command: string(message.Payload()),
	}
	handleCommand(db, job)
}

// // Dynamischer Worker Pool
// var jobQueue chan DeviceCommand
// var workers sync.WaitGroup
// var stopChan []chan struct{}
// var stopWorkers = make(chan struct{})

// // Worker function modified to support graceful shutdown
// func worker(id int, jobs <-chan DeviceCommand, db *sql.DB, stopCh <-chan struct{}) {
// 	defer workers.Done()
// 	logrus.Infof("Worker %d started.", id)
// 	for {
// 		select {
// 		case job := <-jobs:
// 			logrus.Info("worker %d: received job %v", id, job)
// 			handleCommand(db, job)
// 		case <-stopCh:
// 			logrus.Errorf("Worker %d stopping.", id)
// 			return
// 		}
// 	}
// }

// func initWorkerPool(numWorkers int, db *sql.DB) {
// 	jobQueue = make(chan DeviceCommand, 10000)
// 	workers.Add(numWorkers)
// 	for i := 1; i <= numWorkers; i++ {
// 		go worker(i, jobQueue, db, stopWorkers)
// 	}
// }

// // Stopping all workers
// func stopAllWorkers() {
// 	close(stopWorkers)
// 	workers.Wait()
// 	logrus.Info("All workers stopped")
// }
