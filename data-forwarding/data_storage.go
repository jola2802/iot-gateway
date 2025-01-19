package dataforwarding

import (
	"database/sql"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type DeviceData struct {
	DeviceName  string
	DeviceId    string
	Datapoint   string
	DatapointId string
	Value       string
	Timestamp   string
}

// MqttConfig definiert die Konfigurationsstruktur für die Verbindung zum MQTT-Broker
type MqttConfig struct {
	Broker   string
	Username string
	Password string
}

var dataBatch = make([]DeviceData, 0, 10) // Puffer für 10 Datenpunkte
var batchMu sync.Mutex                    // Mutex für die Synchronisation

// SaveDeviceDataToBatch sammelt die Daten und speichert sie in Batches
func SaveDeviceDataToBatch(db *sql.DB, deviceName string, deviceId string, datapoint string, datapointId string, value string, timestamp string, cacheDuration time.Duration) {
	batchMu.Lock()
	defer batchMu.Unlock()

	// Füge den neuen Datenpunkt zum Puffer hinzu
	deviceData := DeviceData{
		DeviceName:  deviceName,
		DeviceId:    deviceId,
		Datapoint:   datapoint,
		DatapointId: datapointId,
		Value:       value,
		Timestamp:   timestamp,
	}
	dataBatch = append(dataBatch, deviceData)

	// Wenn der Puffer 10 Daten erreicht hat, schreibe sie in die Datenbank
	if len(dataBatch) >= 5 {
		err := writeBatchToDB(db, dataBatch, cacheDuration)
		if err != nil {
			logrus.Errorf("Error writing batch to DB: %v", err)
			return
		}
		// Puffer leeren, nachdem die Daten gespeichert wurden
		dataBatch = dataBatch[:0]
	}
}

// writeBatchToDB führt eine Batch-Schreiboperation in die Datenbank durch und löscht alte Daten
func writeBatchToDB(db *sql.DB, batch []DeviceData, cacheDuration time.Duration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO device_data (device_name, deviceId, datapoint, datapointId, value, timestamp) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, data := range batch {
		_, err := stmt.Exec(data.DeviceName, data.DeviceId, data.Datapoint, data.DatapointId, data.Value, data.Timestamp)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// Lösche alte Daten, die älter als 5 Minuten sind
	deleteQuery := `DELETE FROM device_data WHERE timestamp < ?`
	tenMinutesAgo := time.Now().Add(-cacheDuration).Format("02.01.2006 15:04:05.000")
	_, err = tx.Exec(deleteQuery, tenMinutesAgo)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Transaktion abschließen
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
