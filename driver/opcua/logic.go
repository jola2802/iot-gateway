package opcua

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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

	// Retry-Logik: Versuche alle 5 Sekunden, die Verbindung aufzubauen
	retryInterval := 5 * time.Second
	for {
		publishDeviceState(server, "opc-ua", device.ID, "2 (initializing)", db)

		// Erstelle bei jedem Versuch einen neuen Client
		clientOpts, _ = clientOptsFromFlags(device, db)
		client, err = opcua.NewClient(device.Address, clientOpts...)
		if err != nil {
			// logrus.Errorf("OPC-UA: Error creating client for device %v: %v", device.Name, err)
			publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)
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
			publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)

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
		publishDeviceState(server, "opc-ua", device.ID, "6 (connection lost)", db)
		return err
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
			publishDeviceState(server, "opc-ua", device.ID, "1 (running)", db)

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

			time.Sleep(sleeptime)
		}
	}
}

// TestConnection versucht eine Verbindung zum OPC-UA Server herzustellen
func TestConnection(device DeviceConfig, db *sql.DB) bool {
	// Erstelle Client-Optionen mit den konfigurierten Einstellungen
	clientOpts, err := clientOptsFromFlags(device, db)
	if err != nil {
		logrus.Errorf("OPC-UA: Fehler beim Erstellen der Client-Optionen für Gerät %v: %v", device.Name, err)
		return false
	}

	// Erstelle neuen Client
	client, err := opcua.NewClient(device.Address, clientOpts...)
	if err != nil {
		logrus.Errorf("OPC-UA: Fehler beim Erstellen des Clients für Gerät %v: %v", device.Name, err)
		return false
	}

	// Versuche Verbindung herzustellen mit Timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		logrus.Errorf("OPC-UA: Verbindungstest fehlgeschlagen für Gerät %v: %v", device.Name, err)
		return false
	}

	// Verbindung erfolgreich - wieder trennen
	client.Close(ctx)
	return true
}
