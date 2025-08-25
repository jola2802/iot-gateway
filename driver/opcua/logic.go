package opcua

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/go-ping/ping"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

// Run runs the OPC-UA client and MQTT publisher.
func Run(device DeviceConfig, db *sql.DB, stopChan chan struct{}, server *MQTT.Server) error {

	// Teste verschiedene Authentifizierungsmethoden
	logrus.Infof("OPC-UA: Testing authentication methods for device %s", device.Name)
	err := TryDifferentAuthMethods(device, device.Address)
	if err != nil {
		logrus.Errorf("OPC-UA: All authentication methods failed for device %s: %v", device.Name, err)
		// Trotzdem weiter versuchen mit der Standard-Konfiguration
	}

	var clientOpts []client.Option
	var ch *client.Client
	var connectionEstablished bool = false

	// Erstelle Context außerhalb der Schleife
	ctx := context.Background()
	lastStatus := ""

	// Starten mit Initializing
	updateDeviceStatus(server, "opc-ua", device.ID, "2 (initializing)", db, &lastStatus)

	// Client-Optionen einmalig erstellen
	clientOpts, err = clientOptsFromFlags(device, db)
	if err != nil {
		logrus.Errorf("OPC-UA: Error creating client options for device %v: %v", device.Name, err)
		updateDeviceStatus(server, "opc-ua", device.ID, "3 (error)", db, &lastStatus)
		return fmt.Errorf("configuration failed for device %v: %v", device.Name, err)
	}

	// Connection und Retry-Logik
	retryInterval := 5 * time.Second
	for {
		select {
		case <-stopChan:
			if ch != nil {
				ch.Close(ctx)
			}
			updateDeviceStatus(server, "opc-ua", device.ID, "0 (stopped)", db, &lastStatus)
			return nil
		default:
			// Versuche Verbindung aufzubauen, falls noch nicht vorhanden
			if !connectionEstablished {
				logrus.Infof("OPC-UA: Establishing connection to device %v at %s", device.Name, device.Address)
				ch, err = client.Dial(ctx, device.Address, clientOpts...)
				if err != nil {
					logrus.Errorf("OPC-UA: Failed to connect to device %v: %v. Retrying in %v...", device.Name, err, retryInterval)
					updateDeviceStatus(server, "opc-ua", device.ID, "6 (connection lost)", db, &lastStatus)

					select {
					case <-stopChan:
						return fmt.Errorf("connection aborted for device %v", device.Name)
					case <-time.After(retryInterval):
						continue
					}
				}

				// Verbindung erfolgreich
				connectionEstablished = true
				addOpcuaClient(device.ID, ch)
				logrus.Infof("OPC-UA: Successfully connected to device %v", device.Name)
				updateDeviceStatus(server, "opc-ua", device.ID, "2 (initializing)", db, &lastStatus)
			}

			// Daten sammeln und veröffentlichen mit persistenter Verbindung
			err = collectAndPublishDataPersistent(device, ch, stopChan, server, db, &lastStatus, &connectionEstablished)

			// Bei Verbindungsverlust: Markiere Verbindung als getrennt und versuche erneut
			if err != nil {
				logrus.Warnf("OPC-UA: Connection issue with device %v: %v", device.Name, err)
				updateDeviceStatus(server, "opc-ua", device.ID, "6 (connection lost)", db, &lastStatus)

				if ch != nil {
					ch.Close(ctx)
					ch = nil
				}
				connectionEstablished = false

				// Warte vor erneutem Verbindungsversuch
				select {
				case <-stopChan:
					return fmt.Errorf("connection aborted for device %v", device.Name)
				case <-time.After(retryInterval):
					continue
				}
			}
		}
	}
}

// MQTT-Publikation mit exponentiellem Backoff
func publishDeviceState(server *MQTT.Server, deviceType, deviceID string, status string, db *sql.DB) {
	topic := fmt.Sprintf("driver/states/%s/%s", deviceType, deviceID)
	logrus.Debugf("OPC-UA: Publishing device state to %s: %s", topic, status)
	server.Publish(topic, []byte(status), true, 2)

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

// collectAndPublishDataPersistent sammelt und veröffentlicht Daten mit persistenter Verbindung
func collectAndPublishDataPersistent(device DeviceConfig, ch *client.Client, stopChan chan struct{}, server *MQTT.Server, db *sql.DB, lastStatus *string, connectionEstablished *bool) error {
	dataNodes := device.DataNode
	sleeptime := time.Duration(device.AcquisitionTime) * time.Millisecond

	// Mehrere Zyklen mit derselben Verbindung ausführen
	maxCyclesPerConnection := 100 // Nach 100 Zyklen kurz prüfen
	cycleCount := 0

	for cycleCount < maxCyclesPerConnection {
		select {
		case <-stopChan:
			return nil
		default:
			cycleStart := time.Now()

			// Lese Daten mit Fehlerbehandlung
			data, err := readDataWithRetry(ch, dataNodes, 3) // 3 Retry-Versuche
			if err != nil {
				logrus.Errorf("OPC-UA: Persistent connection failed for device %v: %v", device.Name, err)
				*connectionEstablished = false
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

			// Status aktualisieren
			if len(convData) > 0 {
				updateDeviceStatus(server, "opc-ua", device.ID, "1 (running)", db, lastStatus)
			} else {
				updateDeviceStatus(server, "opc-ua", device.ID, "4 (no datapoints)", db, lastStatus)
			}

			// Cycle-Timing
			cycleDuration := time.Since(cycleStart)
			remainingTime := sleeptime - cycleDuration
			if remainingTime > 0 {
				time.Sleep(remainingTime)
			}

			cycleCount++
		}
	}

	// Nach maxCyclesPerConnection Zyklen zurückkehren für Connection-Health-Check
	return nil
}

// collectAndPublishData collects and publishes data from an OPC-UA client to an MQTT broker.
func collectAndPublishData(device DeviceConfig, ch *client.Client, stopChan chan struct{}, server *MQTT.Server, db *sql.DB, lastStatus *string) error {
	dataNodes := device.DataNode

	sleeptime := time.Duration(device.AcquisitionTime) * time.Millisecond

	for {
		select {
		case <-stopChan:
			return nil
		default:
			// Startzeit ermitteln
			cycleStart := time.Now()

			data, err := readData(ch, dataNodes)
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

			// Berechnung der Dauer der Cycle
			cycleDuration := time.Since(cycleStart)

			// Cycle Time von sleeptime abziehen
			remainingTime := sleeptime - cycleDuration
			if remainingTime > 0 {
				time.Sleep(remainingTime)
			}
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
