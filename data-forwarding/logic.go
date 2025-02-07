package dataforwarding

import (
	"database/sql"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

func StartDataRoutes(db *sql.DB) {
	// SQL-Abfrage, um alle Routen aus der Tabelle 'data_routes' zu laden
	query := `
		SELECT id, destination_type, data_format, interval, devices, destination_url, headers, file_path
		FROM data_routes
	`
	rows, err := db.Query(query)
	if err != nil {
		logrus.Errorf("Error fetching data routes: %v", err)
		return
	}
	defer rows.Close()

	// Iteriere über die abgerufenen Routen und starte die Weiterleitungsprozesse
	for rows.Next() {
		var id, interval int
		var destinationType, dataFormat, destinationURL, filePath string
		var devices []string
		var headers []Header

		// Scan die Zeilen in lokale Variablen
		err := rows.Scan(&id, &destinationType, &dataFormat, &interval, pq.Array(&devices), &destinationURL, pq.Array(&headers), &filePath)
		if err != nil {
			logrus.Errorf("Error scanning route: %v", err)
			continue
		}

		// Starte den Prozess basierend auf dem Zieltyp
		switch destinationType {
		case "REST":
			// Starten eines REST API-Prozesses
			go forwardToREST(db, destinationURL, devices, interval, headers)
		case "File":
			// Starten eines Datei-basierten Prozesses
			go forwardToFile(db, filePath, devices, interval)
		case "MQTT":
			go forwardToMqtt()
		default:
			logrus.Warnf("Unsupported destination type: %s", destinationType)
		}
	}
}

// GetDataPointsSinceLastSent holt die Datenpunkte für die angegebenen Geräte, die seit dem letzten Senden
// bis zum aktuellen Zeitpunkt angefallen sind.
func GetDataPoints(db *sql.DB, devices []string, lastSendTime time.Time, currentTime time.Time) ([]DeviceData, error) {
	var dataPoints []DeviceData
	// Format der Zeitstempel in der DB (z.B. 04.10.2024 10:51:32.587)
	timeFormat := "02.01.2006 15:04:05.000"

	// Zeitstempel in das richtige Format für den SQL-Vergleich umwandeln
	lastSendTimeStr := lastSendTime.Format(timeFormat)
	currentTimeStr := currentTime.Format(timeFormat)

	// Erstelle die Platzhalter für die IN-Klausel
	placeholders := make([]string, len(devices))
	args := make([]interface{}, len(devices)+2) // +2 für die Zeitstempel (lastSendTime, currentTime)
	for i, device := range devices {
		placeholders[i] = "?" // Platzhalter für Prepared Statement
		args[i] = device      // Gerätename als Argument
	}

	// Füge die Zeitstempel zu den SQL-Argumenten hinzu
	args[len(devices)] = lastSendTimeStr
	args[len(devices)+1] = currentTimeStr

	// SQL-Abfrage für die Datenpunkte zwischen lastSendTime und currentTime für die angegebenen Geräte
	query := `
		SELECT device_name, deviceId, datapoint, datapointId, value, timestamp 
		FROM device_data 
		WHERE device_name IN (` + strings.Join(placeholders, ",") + `)
		AND timestamp BETWEEN ? AND ?
	`
	// Bereite die Abfrage vor und führe sie aus
	rows, err := db.Query(query, args...)
	if err != nil {
		logrus.Errorf("Error executing query: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Datenpunkte auslesen und in die DeviceData-Struktur speichern
	for rows.Next() {
		var data DeviceData
		var timestampStr string
		err := rows.Scan(&data.DeviceName, &data.DeviceId, &data.Datapoint, &data.DatapointId, &data.Value, &timestampStr)
		if err != nil {
			logrus.Errorf("Error scanning row: %v", err)
			return nil, err
		}

		// Timestamp in das passende Format umwandeln (bereits String, also kein weiteres Formatieren nötig)
		data.Timestamp = timestampStr

		// Füge den Datenpunkt zur Liste hinzu
		dataPoints = append(dataPoints, data)
	}

	// Fehlerbehandlung für das Iterieren durch die Zeilen
	if err = rows.Err(); err != nil {
		logrus.Errorf("Error iterating through rows: %v", err)
		return nil, err
	}

	return dataPoints, nil
}
