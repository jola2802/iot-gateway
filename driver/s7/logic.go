// Package s7 provides functionality for reading data from S7 devices and publishing it to an MQTT broker.
package s7

import (
	"database/sql"
	"fmt"
	"iot-gateway/driver/opcua"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/go-ping/ping"
	MQTT "github.com/mochi-mqtt/server/v2"
	s7 "github.com/robinson/gos7"
	"github.com/sirupsen/logrus"
)

// Run startet die Datenerfassung und -verarbeitung für ein einzelnes S7-Gerät
func Run(device opcua.DeviceConfig, db *sql.DB, stopChan chan struct{}, server *MQTT.Server) error {
	var client s7.Client
	var handler *s7.TCPClientHandler
	var err error

	retryInterval := 5 * time.Second
	lastStatus := ""

	// Starten mit Initializing
	updateDeviceStatus(server, "s7", device.ID, "2 (initializing)", db, &lastStatus)

	for {
		select {
		case <-stopChan:
			if handler != nil {
				handler.Close()
			}
			logrus.Info("S7: Stopping data processing.")
			updateDeviceStatus(server, "s7", device.ID, "0 (stopped)", db, &lastStatus)
			return nil
		default:
			// Wenn kein Client existiert, erstelle einen neuen
			if client == nil || handler == nil {
				client, handler, err = createS7Client(device)
				if err != nil {
					logrus.Errorf("S7: Error creating client for device %s: %v", device.Name, err)
					updateDeviceStatus(server, "s7", device.ID, "5 (no connection)", db, &lastStatus)

					// Prüfe, ob ein Stop-Request empfangen wurde
					select {
					case <-stopChan:
						return fmt.Errorf("connection aborted for device %v", device.Name)
					case <-time.After(retryInterval):
						continue
					}
				}
			}

			// Versuche, die Verbindung herzustellen
			data, err := fetchS7Data(client, device)
			if err != nil {
				logrus.Errorf("S7: Error initializing client for device %s: %v", device.Name, err)
				updateDeviceStatus(server, "s7", device.ID, "5 (no connection)", db, &lastStatus)

				// Schließe den alten Handler sicher
				if handler != nil {
					handler.Close()
					handler = nil
				}
				client = nil

				// Prüfe, ob ein Stop-Request empfangen wurde
				select {
				case <-stopChan:
					return fmt.Errorf("connection aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}

			// Wenn die Verbindung erfolgreich war, verarbeite die Daten
			mqttData, err := convData(data, device.Name)
			if err != nil {
				logrus.Errorf("S7: Error converting data: %v", err)
				updateDeviceStatus(server, "s7", device.ID, "3 (error)", db, &lastStatus)

				select {
				case <-stopChan:
					return fmt.Errorf("processing aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}

			if err := pubData(mqttData, device.ID, server, db); err != nil {
				logrus.Errorf("S7: Error publishing data: %v", err)
				updateDeviceStatus(server, "s7", device.ID, "3 (error)", db, &lastStatus)

				select {
				case <-stopChan:
					return fmt.Errorf("publishing aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}

			// Wenn alles erfolgreich war
			if len(mqttData) > 0 {
				updateDeviceStatus(server, "s7", device.ID, "1 (running)", db, &lastStatus)
			} else {
				updateDeviceStatus(server, "s7", device.ID, "4 (no datapoints)", db, &lastStatus)
			}

			time.Sleep(time.Duration(device.AcquisitionTime) * time.Millisecond)
		}
	}
}

// MQTT-Publikation mit exponentiellem Backoff
func publishDeviceState(server *MQTT.Server, deviceType, deviceID string, status string, db *sql.DB) {
	topic := "iot-gateway/driver/states/" + deviceType + "/" + deviceID
	server.Publish(topic, []byte(status), false, 0)

	// publishWithBackoff(server, topic, status, 5)

	// Publish the state to the db
	_, err := db.Exec("UPDATE devices SET status = ? WHERE id = ?", status, deviceID)
	if err != nil {
		logrus.Errorf("Error updating device state in the database: %v", err)
	}
}

// Hilfs-Funktion für Status-Updates, die nur bei Änderungen veröffentlicht
func updateDeviceStatus(server *MQTT.Server, deviceType, deviceID, newStatus string, db *sql.DB, lastStatus *string) {
	if *lastStatus != newStatus {
		publishDeviceState(server, deviceType, deviceID, newStatus, db)
		*lastStatus = newStatus
		logrus.Debugf("%s: Device %s status changed to %s", deviceType, deviceID, newStatus)
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
func TestConnection(deviceAddress string) bool {
	// Erstelle einen neuen TCP Client Handler
	// handler := s7.NewTCPClientHandler(device.Address, device.Rack, device.Slot)
	// handler.Timeout = 3 * time.Second

	// // Versuche eine Verbindung herzustellen
	// if err := handler.Connect(); err != nil {
	// 	logrus.Errorf("S7: Verbindungstest fehlgeschlagen für Gerät %v: %v", device.Name, err)
	// 	return false
	// }

	// // Verbindung erfolgreich - wieder trennen
	// handler.Close()

	// Teste die Verbindung durch Anpingen der IP-Adresse
	// Extract the IP address from the device address (e.g. 192.168.1.1:102)
	ip := strings.Split(deviceAddress, ":")[0]
	pinger, _ := ping.NewPinger(ip)
	pinger.Count = 1
	pinger.Timeout = 1000 * time.Millisecond
	err := pinger.Run()
	if err != nil {
		logrus.Errorf("S7: Verbindungstest fehlgeschlagen für Gerät %v: %v", deviceAddress, err)
		return false
	}
	// logrus.Infof("S7: Verbindungstest erfolgreich für Gerät %v", deviceAddress)
	return true
}
