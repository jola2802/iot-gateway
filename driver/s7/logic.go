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

// Gerätekonfigurationsstruktur
type DeviceConfig struct {
	Type            string               `json:"type"`
	Name            string               `json:"name"`
	Address         string               `json:"address"`
	AcquisitionTime int                  `json:"acquisitionTime"`
	Rack            int                  `json:"rack"`
	Slot            int                  `json:"slot"`
	Datapoint       []Datapoint          `json:"datapoints"`
	PLCClient       *s7.Client           `json:"-"`
	PLCHandler      *s7.TCPClientHandler `json:"-"`
	ID              string               `json:"id"` // ID des Geräts

}

// Datapoint defines a single datapoint to be read from the PLC
type Datapoint struct {
	Name     string `json:"name"`
	DataType string `json:"datatype"`
	Address  string `json:"address"`
}

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
			// Warte 10 Sekunden vor dem nächsten Versuch
			time.Sleep(10 * time.Second)
			continue
		}

		// Wenn die Verbindung erfolgreich war, verarbeite die Daten
		mqttData, err := convData(data, device.Name)
		if err != nil {
			logrus.Errorf("S7: Error converting data: %v", err)
			return err
		}
		if err := pubData(mqttData, device.ID, server); err != nil {
			logrus.Errorf("S7: Error publishing data: %v", err)
			return err
		}

		time.Sleep(time.Duration(device.AcquisitionTime) * time.Millisecond)
	}
}
