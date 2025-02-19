package dataforwarding

import (
	"database/sql"
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
		err := rows.Scan(&id, &destinationType, &dataFormat, &interval, pq.Array(&devices), &destinationURL, &headers, &filePath)
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
	return nil, nil
}
