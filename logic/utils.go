package logic

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
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
