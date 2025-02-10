package logic

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	opcua "iot-gateway/driver/opcua"
	"strconv"
	"time"

	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%% Utils for user_manager.go %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

// genRandomPW generiert ein zufälliges Passwort
func genRandomPW() string {
	b := make([]byte, 10)
	_, err := rand.Read(b)
	if err != nil {
		logrus.Fatal(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

// LoadDevices liest die Gerätedaten aus der Datenbank
func LoadDevices(db *sql.DB) ([]map[string]interface{}, error) {
	query := `SELECT type, name FROM devices`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying devices from database: %v", err)
	}
	defer rows.Close()

	var deviceList []map[string]interface{}

	for rows.Next() {
		var deviceType, name string

		// Lese Daten aus der Datenbankzeile
		err := rows.Scan(&deviceType, &name)
		if err != nil {
			return nil, fmt.Errorf("error scanning device row: %v", err)
		}

		// Konvertiere die Daten in eine Map
		deviceMap := map[string]interface{}{
			"type": deviceType,
			"name": name,
		}
		deviceList = append(deviceList, deviceMap)
	}

	return deviceList, nil
}

// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%% Utils for driver_manager.go %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

// Generische Funktion für das Starten eines Gerätezustands
func getOrCreateDeviceState(deviceName string, deviceStates map[string]*DeviceState) *DeviceState {
	state, exists := deviceStates[deviceName]
	if !exists {
		state = &DeviceState{}
		deviceStates[deviceName] = state
	}
	return state
}

// MQTT-Publikation mit exponentiellem Backoff
func publishDeviceState(server *MQTT.Server, deviceType, deviceID string, status string) {
	topic := "iot-gateway/driver/states/" + deviceType + "/" + deviceID
	publishWithBackoff(server, topic, status, 5)

	// Publish the state to the db
	_, err := db.Exec("UPDATE devices SET status = ? WHERE id = ?", status, deviceID)
	if err != nil {
		logrus.Errorf("Error updating device state in the database: %v", err)
	}
}

// Implementiere exponentiellen Backoff
func publishWithBackoff(server *MQTT.Server, topic string, payload string, maxRetries int) {
	backoff := 1000 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		err := server.Publish(topic, []byte(payload), true, 1)
		if err == nil {
			return
		}
		time.Sleep(backoff)
		backoff *= 2 // Exponentielles Wachstum der Wartezeit
	}
	logrus.Errorf("Failed to publish message after %d retries", maxRetries)
}

// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%% OPC-UA-Part %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

// Hilfsfunktion: Lese Geräte–Konfiguration (ohne Sicherheitsdaten)
func readDeviceConfig(db *sql.DB, deviceID string) (opcua.DeviceConfig, error) {
	var config opcua.DeviceConfig
	var deviceAddress, deviceName string
	var acquisitionTime int
	query := `SELECT name, address, acquisition_time FROM devices WHERE id = ?`
	if err := db.QueryRow(query, deviceID).Scan(&deviceName, &deviceAddress, &acquisitionTime); err != nil {
		return config, fmt.Errorf("DM: Error querying device config: %v", err)
	}
	config = opcua.DeviceConfig{
		ID:              deviceID,
		Name:            deviceName,
		Address:         deviceAddress,
		AcquisitionTime: acquisitionTime,
	}
	return config, nil
}

// Hilfsfunktion: Lese optionale Sicherheitsdaten
func readSecurityOptions(db *sql.DB, deviceID string) (security struct {
	Mode, Policy, Cert, Key, Username, Password sql.NullString
}, err error) {
	// Hier gehen wir davon aus, dass die Sicherheitsdaten in der devices-Tabelle enthalten sind.
	query := `SELECT security_mode, security_policy, certificate, key, username, password FROM devices WHERE id = ?`
	err = db.QueryRow(query, deviceID).Scan(
		&security.Mode, &security.Policy,
		&security.Cert, &security.Key,
		&security.Username, &security.Password,
	)
	if err != nil {
		return security, fmt.Errorf("DM: Error querying security options: %v", err)
	}
	return security, nil
}

// Hilfsfunktion: Lese alle OPC-UA-Knoten eines Gerätes
func readOPCUANodes(db *sql.DB, deviceID string) ([]opcua.DataNode, error) {
	nodeQuery := `SELECT name, node_identifier FROM opcua_datanodes WHERE device_id = ?`
	rows, err := db.Query(nodeQuery, deviceID)
	if err != nil {
		return nil, fmt.Errorf("DM: Error querying OPC-UA nodes: %v", err)
	}
	defer rows.Close()

	var nodes []opcua.DataNode
	for rows.Next() {
		var nodeName, nodeIdentifier string
		if err := rows.Scan(&nodeName, &nodeIdentifier); err != nil {
			return nil, fmt.Errorf("DM: Error scanning node data: %v", err)
		}
		nodes = append(nodes, opcua.DataNode{
			Name: nodeName,
			Node: nodeIdentifier,
		})
	}
	return nodes, nil
}

// Hilfsfunktion: Übernehme optionale Sicherheitswerte in die DeviceConfig
func applySecurityOptions(config *opcua.DeviceConfig, sec struct {
	Mode, Policy, Cert, Key, Username, Password sql.NullString
}) {
	if sec.Mode.Valid {
		config.SecurityMode = sec.Mode.String
	}
	if sec.Policy.Valid {
		config.SecurityPolicy = sec.Policy.String
	}
	if sec.Cert.Valid {
		config.CertFile = sec.Cert.String
	}
	if sec.Key.Valid {
		config.KeyFile = sec.Key.String
	}
	if sec.Username.Valid {
		config.Username = sec.Username.String
	}
	if sec.Password.Valid {
		config.Password = sec.Password.String
	}
}

// %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%% S7-Part %%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

// Liest die Hauptkonfiguration eines S7-Gerätes inklusive Rack/Slot-Konvertierung
func readS7DeviceConfig(db *sql.DB, deviceID string) (opcua.DeviceConfig, error) {
	var config opcua.DeviceConfig
	var rackStr, slotStr string
	query := `SELECT name, address, rack, slot, acquisition_time FROM devices WHERE id = ?`
	err := db.QueryRow(query, deviceID).Scan(&config.Name, &config.Address, &rackStr, &slotStr, &config.AcquisitionTime)
	if err != nil {
		return config, fmt.Errorf("DM: Error querying S7 device config: %v", err)
	}
	config.Rack, err = strconv.Atoi(rackStr)
	if err != nil {
		return config, fmt.Errorf("DM: Error converting rack value: %v", err)
	}
	config.Slot, err = strconv.Atoi(slotStr)
	if err != nil {
		return config, fmt.Errorf("DM: Error converting slot value: %v", err)
	}
	config.ID = deviceID
	return config, nil
}

// Liest die S7-Datenpunkte eines Gerätes aus der s7_datapoints-Tabelle
func readS7Datapoints(db *sql.DB, deviceID string) ([]opcua.Datapoint, error) {
	query := `SELECT name, datatype, address FROM s7_datapoints WHERE device_id = ?`
	rows, err := db.Query(query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("DM: Error querying S7 datapoints: %v", err)
	}
	defer rows.Close()

	var datapoints []opcua.Datapoint
	for rows.Next() {
		var dp opcua.Datapoint
		if err := rows.Scan(&dp.Name, &dp.Datatype, &dp.Address); err != nil {
			return nil, fmt.Errorf("DM: Error scanning S7 datapoint: %v", err)
		}
		datapoints = append(datapoints, dp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("DM: Error iterating S7 datapoints: %v", err)
	}
	return datapoints, nil
}
