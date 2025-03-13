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

	var lastState, currentState string
	var count int

	// Retry-Logik: Versuche alle 5 Sekunden, die Verbindung aufzubauen
	retryInterval := 5 * time.Second
	for {
		currentState = "2 (initializing)"

		// Erstelle bei jedem Versuch einen neuen Client
		clientOpts, _ = clientOptsFromFlags(device, db)
		client, err = opcua.NewClient(device.Address, clientOpts...)
		if err != nil {
			// logrus.Errorf("OPC-UA: Error creating client for device %v: %v", device.Name, err)
			currentState = "6 (connection lost)"
			time.Sleep(retryInterval)
			continue
		}

		err = client.Connect(ctx)
		if err != nil {
			// Schließe den fehlgeschlagenen Client
			client.Close(ctx)
			// Setze Client auf nil um sicherzustellen, dass er vom Garbage Collector aufgeräumt wird
			client = nil

			logrus.Errorf("OPC-UA: Error connecting to device %v: %v. Trying again in %v...", device.Name, err, retryInterval)
			// publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)
			currentState = "6 (connection lost)"

			// Prüfe, ob ein Stop-Request empfangen wurde
			select {
			case <-stopChan:
				// logrus.Infof("OPC-UA: Stop-Request received. Connection attempt for device %v aborted.", device.Name)
				return fmt.Errorf("connection aborted for device %v", device.Name)
			case <-time.After(retryInterval):
				continue
			}
		}

		break
	}

	// Device zu OPC-UA Device Map hinzufügen
	addOpcuaClient(device.ID, client)

	// Daten vom OPC-UA-Client sammeln und veröffentlichen
	if err := collectAndPublishData(device, client, stopChan, server, db); err != nil {
		// publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)
		currentState = "6 (connection lost)"
		return err
	} else {
		// publishDeviceState(server, "opc-ua", device.ID, "1 (running)", db)
		currentState = "1 (running)"
	}

	if currentState != lastState {
		publishDeviceState(server, "opc-ua", device.ID, currentState, db)
		lastState = currentState
		count = 0
	}

	count++

	if count > 2 {
		publishDeviceState(server, "opc-ua", device.ID, currentState, db)
		lastState = currentState
		count = 0
	}

	defer client.Close(ctx)
	return nil
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
func collectAndPublishData(device DeviceConfig, client *opcua.Client, stopChan chan struct{}, server *MQTT.Server, db *sql.DB) error {
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
				publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)
				time.Sleep(5 * time.Second)
				continue
			}

			convData, err := convData(client, data, dataNodes)
			if err != nil {
				logrus.Errorf("OPC-UA: Fehler beim Konvertieren der Daten von %v: %s", device.Name, err)
				publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)
				time.Sleep(5 * time.Second)
				continue
			}

			if err = pubData(convData, device.Name, device.ID, server); err != nil {
				logrus.Errorf("OPC-UA: Fehler beim Veröffentlichen der Daten von %v: %s", device.Name, err)
				publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)
				time.Sleep(5 * time.Second)
				continue
			}

			// If convData is not empty, update the device state
			if len(convData) > 0 {
				publishDeviceState(server, "opc-ua", device.ID, "1 (running)", db)
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
