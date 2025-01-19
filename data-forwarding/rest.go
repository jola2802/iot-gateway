package dataforwarding

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DataReading enthält das Format für das JSON-Objekt, das gesendet wird
type DataReading struct {
	DatapointId string `json:"DatapointId"`
	Value       string `json:"Value"`
	Timestamp   string `json:"Timestamp"`
}

// forwardToREST startet einen Prozess, der Daten regelmäßig an eine REST API weiterleitet
func forwardToREST(db *sql.DB, destinationURL string, devices []string, interval int, headers []Header) {
	logrus.Infof("Starting REST forwarding to %s every %d seconds for devices: %v", destinationURL, interval, devices)

	// Setze den Interval-Mechanismus
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		currentTime := t // Das ist die aktuelle Zeit, wenn der Ticker ausgelöst wird
		lastSentTime := currentTime.Add(-time.Duration(interval) * time.Second)

		// Hole die Datenpunkte für die angegebenen Geräte
		dataPoints, err := GetDataPoints(db, devices, lastSentTime, currentTime)
		if err != nil {
			logrus.Errorf("Error getting data points: %v", err)
			continue
		}
		// logrus.Println(dataPoints)

		if len(dataPoints) == 0 {
			logrus.Infof("No data points to send for the interval %v to %v", lastSentTime, currentTime)
			continue
		}

		// Sende die Daten an die REST-API
		err = sendDataToREST(destinationURL, dataPoints, headers)
		currentTimeString := currentTime.Format("2006-01-02 15:04:05")
		if err != nil {
			logrus.Errorf("Error forwarding data to REST API: %v", err)
			// Add err to currentTimeString
			currentTimeString = "error sending request: " + currentTimeString
		}

		currentTimeString = fmt.Sprintf("%s\namount of datapoints sent: %d", currentTimeString, len(dataPoints))

		// Setze den Zeitstempel für die zuletzt gesendeten Datenpunkte
		err = SetLastSentTimestamp(db, destinationURL, currentTimeString)
	}
}

func SetLastSentTimestamp(db *sql.DB, destinationUrl string, timestamp string) error {
	query := `
		UPDATE data_routes
		SET created_at = $1
		WHERE destination_url = $2
	`
	_, err := db.Exec(query, timestamp, destinationUrl)
	if err != nil {
		return fmt.Errorf("error updating last_sent timestamp: %v", err)
	}
	return nil
}

// sendDataToREST sendet die gesammelten Datenpunkte an die REST API
func sendDataToREST(destinationURL string, dataPoints []DeviceData, headers []Header) error {
	// Baue die JSON-Daten auf
	var readings []DataReading

	for _, point := range dataPoints {
		readings = append(readings, DataReading{
			DatapointId: point.DatapointId,
			Value:       point.Value,
			Timestamp:   point.Timestamp,
		})
	}

	// JSON-Array in das erwartete Format konvertieren
	jsonData, err := json.Marshal(readings)
	if err != nil {
		return fmt.Errorf("error marshalling data: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Erstelle die HTTP-POST-Anfrage
	req, err := http.NewRequest("POST", destinationURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %v", err)
	}

	// Headers hinzufügen
	for _, header := range headers {
		req.Header.Add(header.Name, header.Value)
	}
	req.Header.Add("Content-Type", "application/json")

	// Sende die Anfrage
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("received non-OK HTTP status: %d", resp.StatusCode)
	}

	// logrus.Infof("Successfully sent data to REST API: %s", destinationURL)
	return nil
}
