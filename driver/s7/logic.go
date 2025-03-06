// Package s7 provides functionality for reading data from S7 devices and publishing it to an MQTT broker.
package s7

import (
	"database/sql"
	"iot-gateway/driver/opcua"
	"time"

	_ "github.com/glebarez/go-sqlite"
	MQTT "github.com/mochi-mqtt/server/v2"
	s7 "github.com/robinson/gos7"
	"github.com/sirupsen/logrus"
)

// Run startet die Datenerfassung und -verarbeitung für ein einzelnes S7-Gerät
func Run(device opcua.DeviceConfig, db *sql.DB, stopChan chan struct{}, server *MQTT.Server) error {
	for {
		select {
		case <-stopChan:
			logrus.Info("S7: Stopping data processing.")
			return nil
		default:
		}

		// Versuche, die Verbindung herzustellen
		data, err := initClient(device)
		if err != nil {
			logrus.Errorf("S7: Error initializing client for device %s: %v", device.Name, err)
			publishDeviceState(server, "s7", device.ID, "3 (error)", db)
			// Warte 5 Sekunden vor dem nächsten Versuch
			time.Sleep(5 * time.Second)
			continue
		}

		// Wenn die Verbindung erfolgreich war, verarbeite die Daten
		mqttData, err := convData(data, device.Name)
		if err != nil {
			logrus.Errorf("S7: Error converting data: %v", err)
			publishDeviceState(server, "s7", device.ID, "3 (error)", db)
			return err
		}
		if err := pubData(mqttData, device.ID, server, db); err != nil {
			logrus.Errorf("S7: Error publishing data: %v", err)
			publishDeviceState(server, "s7", device.ID, "3 (error)", db)
			return err
		}

		time.Sleep(time.Duration(device.AcquisitionTime) * time.Millisecond)
	}
}

// MQTT-Publikation mit exponentiellem Backoff
func publishDeviceState(server *MQTT.Server, deviceType, deviceID string, status string, db *sql.DB) {
	topic := "iot-gateway/driver/states/" + deviceType + "/" + deviceID
	publishWithBackoff(server, topic, status, 5)

	// Publish the state to the db
	_, err := db.Exec("UPDATE devices SET status = ? WHERE id = ?", status, deviceID)
	if err != nil {
		logrus.Errorf("Error updating device state in the database: %v", err)
	}
}

func publishWithBackoff(server *MQTT.Server, topic string, payload string, maxRetries int) {
	backoff := 200 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		err := server.Publish(topic, []byte(payload), true, 2)
		if err == nil {
			return
		}
		time.Sleep(backoff)
		backoff *= 2 // Exponentielles Wachstum der Wartezeit
	}
	logrus.Errorf("Failed to publish message after %d retries", maxRetries)
}

// TestConnection versucht eine Verbindung zur S7-SPS herzustellen
func TestConnection(device opcua.DeviceConfig) bool {
	// Erstelle einen neuen TCP Client Handler
	handler := s7.NewTCPClientHandler(device.Address, device.Rack, device.Slot)
	handler.Timeout = 3 * time.Second

	// Versuche eine Verbindung herzustellen
	if err := handler.Connect(); err != nil {
		logrus.Errorf("S7: Verbindungstest fehlgeschlagen für Gerät %v: %v", device.Name, err)
		return false
	}

	// Verbindung erfolgreich - wieder trennen
	handler.Close()
	return true
}
