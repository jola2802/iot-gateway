// Package s7 provides functionality for reading data from S7 devices and publishing it to an MQTT broker.
package s7

import (
	"database/sql"
	"fmt"
	"iot-gateway/driver/opcua"
	"time"

	_ "github.com/glebarez/go-sqlite"
	s7 "github.com/robinson/gos7"
	"github.com/sirupsen/logrus"
)

// Konfigurationsstruktur
type Config struct {
	Devices    []DeviceConfig `json:"devices"`
	MqttBroker MqttConfig     `json:"mqttBroker"`
}

// MQTT-Konfigurationsstruktur
type MqttConfig struct {
	Broker     string `json:"broker"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	CACert     string `json:"caCert,omitempty"`
	ClientCert string `json:"clientCert,omitempty"`
	ClientKey  string `json:"clientKey,omitempty"`
}

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

// LoadMqttConfigFromDB loads the MQTT configuration from the database.
//
// Args:
//   - db: The database connection
//
// Returns:
//   - *MqttConfig: The MQTT configuration loaded from the database
//   - error: An error if the configuration could not be loaded
//
// Example:
//
//	db, err := sql.Open("sqlite3", "./example.db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	mqttConfig, err := LoadMqttConfigFromDB
func loadMqttConfigFromDB(db *sql.DB, username string) (*MqttConfig, error) {
	var mqttConfig MqttConfig

	// Abfrage, um die Brokeradresse aus `broker_settings` und das Passwort aus `auth` zu holen
	query := `
		SELECT 
			bs.address AS broker, 
			a.password 
		FROM 
			broker_settings bs
		JOIN 
			auth a 
		ON 
			a.username = ?
		LIMIT 1
	`

	row := db.QueryRow(query, username)
	err := row.Scan(&mqttConfig.Broker, &mqttConfig.Password)
	if err != nil {
		logrus.Errorf("S7: could not load MQTT config from DB: %v", err)
		return nil, err
	}

	// Setze den Username explizit, da er bereits als Parameter übergeben wurde
	mqttConfig.Username = username

	// logrus.Errorf("S7: logged in at %s with %s and %s", mqttConfig.Broker, mqttConfig.Username, mqttConfig.Password)

	return &mqttConfig, nil
}

// Run startet die Datenerfassung und -verarbeitung für ein einzelnes S7-Gerät
func Run(device opcua.DeviceConfig, db *sql.DB, stopChan chan struct{}) error {
	// MQTT-Client initialisieren
	mqttConfig, err := loadMqttConfigFromDB(db, device.Name)
	if err != nil {
		mqttConfig, err = loadMqttConfigFromDB(db, "admin")
		if err != nil {
			logrus.Errorf("S7: Error loading MQTT config from DB: %v", err)
			return err
		}
	}
	initMqttClient(*mqttConfig)

	// Start MQTT data updater listener
	StartMqttDataUpdateListener("data/s7/update", db)

	dataChannel := make(chan []map[string]interface{})

	go func(device opcua.DeviceConfig, ch chan []map[string]interface{}) {
		failCount := 0 // Zähler für fehlgeschlagene Verbindungsversuche
		for {
			select {
			case <-stopChan:
				return
			default:
				if failCount >= 5 {
					logrus.Errorf("S7: Aborting connection to device %s after %d failed attempts", device.Name, failCount)
					ch <- nil
					return
				}

				data, err := InitClient(device)
				if err != nil {
					logrus.Errorf("S7: Error initializing client for device %s: %v", device.Name, err)
					failCount++
					time.Sleep(time.Duration(device.AcquisitionTime) * time.Millisecond)
				}

				ch <- data
				failCount = 0 // Zähler zurücksetzen bei erfolgreicher Verbindung
				time.Sleep(time.Duration(device.AcquisitionTime) * time.Millisecond)
			}
		}
	}(device, dataChannel)

	// Sammle und verarbeite die Daten von dem Gerät
	for {
		select {
		case <-stopChan:
			// logrus.Info("S7: Stopping data processing.")
			return nil
		case data := <-dataChannel:
			if err != nil {
				return fmt.Errorf("failed to connect to device %s: %v", device.Name, err)
			}
			if data == nil {
				return fmt.Errorf("S7: Aborted connection to device %s due to repeated failures", device.Name)
			}

			mqttData, err := ConvData(data, device.Name)
			if err != nil {
				logrus.Errorf("S7: Error converting data: %v", err)
				return err
			}
			if err := PubData(mqttData, device.Name); err != nil {
				logrus.Errorf("S7: Error publishing data: %v", err)
				return err
			}
		}
	}
}
