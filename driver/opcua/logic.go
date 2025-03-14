package opcua

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-ping/ping"
	"github.com/gopcua/opcua"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

// Run runs the OPC-UA client and MQTT publisher.
func Run(device DeviceConfig, db *sql.DB, stopChan chan struct{}, server *MQTT.Server) error {
	var clientOpts []opcua.Option
	var err error

	// Erstelle Context außerhalb der Schleife
	ctx := context.Background()
	var client *opcua.Client

	lastStatus := ""

	// Starten mit Initializing
	updateDeviceStatus(server, "opc-ua", device.ID, "2 (initializing)", db, &lastStatus)

	// Retry-Logik: Versuche alle 5 Sekunden, die Verbindung aufzubauen
	retryInterval := 5 * time.Second
	for {
		select {
		case <-stopChan:
			if client != nil {
				client.Close(ctx)
			}
			updateDeviceStatus(server, "opc-ua", device.ID, "0 (stopped)", db, &lastStatus)
			return nil
		default:
			// Erstelle bei jedem Versuch einen neuen Client
			clientOpts, err = clientOptsFromFlags(device, db)
			if err != nil {
				logrus.Errorf("OPC-UA: Error creating client options for device %v: %v", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "3 (error)", db, &lastStatus)
				select {
				case <-stopChan:
					return fmt.Errorf("configuration aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}

			client, err = opcua.NewClient(device.Address, clientOpts...)
			if err != nil {
				logrus.Errorf("OPC-UA: Error creating client for device %v: %v", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "6 (connection lost)", db, &lastStatus)
				select {
				case <-stopChan:
					return fmt.Errorf("client creation aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}

			err = client.Connect(ctx)
			if err != nil {
				// Schließe den fehlgeschlagenen Client
				if client != nil {
					client.Close(ctx)
					client = nil
				}

				logrus.Errorf("OPC-UA: Error connecting to device %v: %v. Trying again in %v...", device.Name, err, retryInterval)
				updateDeviceStatus(server, "opc-ua", device.ID, "6 (connection lost)", db, &lastStatus)

				// Prüfe, ob ein Stop-Request empfangen wurde
				select {
				case <-stopChan:
					return fmt.Errorf("connection aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}

			// Device zu OPC-UA Device Map hinzufügen
			addOpcuaClient(device.ID, client)

			// Wenn wir hierher kommen, war die Verbindung erfolgreich
			updateDeviceStatus(server, "opc-ua", device.ID, "2 (initializing)", db, &lastStatus)

			// Daten vom OPC-UA-Client sammeln und veröffentlichen
			err = collectAndPublishData(device, client, stopChan, server, db, &lastStatus)

			// Bei Fehler wird der Client geschlossen und ein neuer Verbindungsversuch gestartet
			if err != nil {
				logrus.Errorf("OPC-UA: Error collecting data from device %v: %v", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "6 (connection lost)", db, &lastStatus)

				if client != nil {
					client.Close(ctx)
					client = nil
				}

				// Prüfe erneut für Stop-Request
				select {
				case <-stopChan:
					return fmt.Errorf("data collection aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}
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

// publishWithBackoff versucht, eine Nachricht mit exponentiellem Backoff zu veröffentlichen
func publishWithBackoff(server *MQTT.Server, topic string, payload string, maxRetries int) {
	backoff := 200 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		err := server.Publish(topic, []byte(payload), false, 0)
		if err == nil {
			return
		}
		time.Sleep(backoff)
		backoff *= 2 // Exponentielles Wachstum der Wartezeit
	}
	logrus.Errorf("Failed to publish message after %d retries", maxRetries)
}

// collectAndPublishData collects and publishes data from an OPC-UA client to an MQTT broker.
func collectAndPublishData(device DeviceConfig, client *opcua.Client, stopChan chan struct{}, server *MQTT.Server, db *sql.DB, lastStatus *string) error {
	dataNodes := device.DataNode

	sleeptime := time.Duration(device.AcquisitionTime) * time.Millisecond

	for {
		select {
		case <-stopChan:
			return nil
		default:
			data, err := readData(client, dataNodes)
			if err != nil {
				logrus.Errorf("OPC-UA: Fehler beim Lesen der Daten von %v: %s", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "6 (connection lost)", db, lastStatus)
				return err
			}

			convData, err := convData(client, data, dataNodes)
			if err != nil {
				logrus.Errorf("OPC-UA: Fehler beim Konvertieren der Daten von %v: %s", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "3 (error)", db, lastStatus)
				return err
			}

			if err = pubData(convData, device.Name, device.ID, server); err != nil {
				logrus.Errorf("OPC-UA: Fehler beim Veröffentlichen der Daten von %v: %s", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "3 (error)", db, lastStatus)
				return err
			}

			// If convData is not empty, update the device state
			if len(convData) > 0 {
				updateDeviceStatus(server, "opc-ua", device.ID, "1 (running)", db, lastStatus)
			} else {
				updateDeviceStatus(server, "opc-ua", device.ID, "4 (no datapoints)", db, lastStatus)
			}
			time.Sleep(sleeptime)
		}
	}
}

// TestConnection versucht eine Verbindung zum OPC-UA Server herzustellen
func TestConnection(deviceAddress string) bool {
	// Erstelle Client-Optionen mit den konfigurierten Einstellungen
	// clientOpts, err := clientOptsFromFlags(device, db)
	// if err != nil {
	// 	logrus.Errorf("OPC-UA: Fehler beim Erstellen der Client-Optionen für Gerät %v: %v", device.Name, err)
	// 	return false
	// }

	// // Erstelle neuen Client
	// client, err := opcua.NewClient(device.Address, clientOpts...)
	// if err != nil {
	// 	logrus.Errorf("OPC-UA: Fehler beim Erstellen des Clients für Gerät %v: %v", device.Name, err)
	// 	return false
	// }

	// // Versuche Verbindung herzustellen mit Timeout
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	// if err := client.Connect(ctx); err != nil {
	// 	logrus.Errorf("OPC-UA: Verbindungstest fehlgeschlagen für Gerät %v: %v", device.Name, err)
	// 	return false
	// }

	// // Verbindung erfolgreich - wieder trennen
	// client.Close(ctx)

	// Teste die Verbindung durch Anpingen der IP-Adresse
	// Extract the IP address from the device address (e.g. opc.tcp://192.168.1.1:4840)
	ip := strings.Split(deviceAddress, "://")[1]
	ip = strings.Split(ip, ":")[0]
	// logrus.Infof("OPC-UA: IP-Adresse: %v", ip)

	pinger, _ := ping.NewPinger(ip)
	pinger.Count = 1
	pinger.Timeout = 1000 * time.Millisecond
	err := pinger.Run()
	if err != nil {
		logrus.Errorf("OPC-UA: Verbindungstest fehlgeschlagen für Gerät %v: %v", deviceAddress, err)
		return false
	}
	// logrus.Infof("OPC-UA: Verbindungstest erfolgreich für Gerät %v", deviceAddress)
	return true
}
