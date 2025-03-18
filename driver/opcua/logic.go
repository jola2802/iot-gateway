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
	topic := "driver/states/" + deviceType + "/" + deviceID
	server.Publish(topic, []byte(status), false, 0)

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
				logrus.Errorf("OPC-UA: Error reading data from %v: %s", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "6 (connection lost)", db, lastStatus)
				return err
			}

			convData, err := convData(data, dataNodes)
			if err != nil {
				logrus.Errorf("OPC-UA: Error converting data from %v: %s", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "3 (error)", db, lastStatus)
				return err
			}

			if err = pubData(convData, device.Name, device.ID, server); err != nil {
				logrus.Errorf("OPC-UA: Error publishing data from %v: %s", device.Name, err)
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
	ip := strings.Split(deviceAddress, "://")[1]
	ip = strings.Split(ip, ":")[0]

	pinger, _ := ping.NewPinger(ip)
	pinger.Count = 1
	pinger.Timeout = 1000 * time.Millisecond
	err := pinger.Run()
	if err != nil {
		logrus.Errorf("OPC-UA: Connection test failed for device %v: %v", deviceAddress, err)
		return false
	}
	return true
}
