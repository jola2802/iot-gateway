package dataforwarding

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// forwardToFile startet einen Prozess, der Daten regelmäßig in eine Datei schreibt
func forwardToFile(db *sql.DB, filePath string, devices []string, interval int) {
	logrus.Infof("Starting file forwarding to %s every %d seconds for devices: %v", filePath, interval, devices)

	// Setze den Interval-Mechanismus
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lastSentTime := time.Now().Add(-time.Duration(interval) * time.Second)
			currentTime := time.Now()

			// Hole die Datenpunkte für die angegebenen Geräte
			dataPoints, err := GetDataPoints(db, devices, currentTime, lastSentTime)
			if err != nil {
				logrus.Errorf("Error getting data points: %v", err)
				continue
			}

			// Schreibe die Daten in die Datei
			err = writeDataToFile(filePath, dataPoints)
			if err != nil {
				logrus.Errorf("Error writing data to file: %v", err)
			}
		}
	}
}

// writeDataToFile schreibt die Daten der Geräte in die angegebene Datei
func writeDataToFile(filePath string, dataPoints []DeviceData) error {
	// Öffne die Datei zum Anhängen (oder erstelle sie, falls sie nicht existiert)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Schreibe die Daten für jedes Gerät in die Datei
	for _, data := range dataPoints {
		// Formatiere die Zeile für die Datei
		dataLine := fmt.Sprintf("%s - Device: %s, Datapoint: %s, Value: %s, Timestamp: %s\n",
			time.Now().Format(time.RFC3339), data.DeviceName, data.Datapoint, data.Value, data.Timestamp)

		// Schreibe die Zeile in die Datei
		_, err := file.WriteString(dataLine)
		if err != nil {
			return fmt.Errorf("failed to write to file: %v", err)
		}
	}

	logrus.Infof("Successfully wrote %d data points to file %s", len(dataPoints), filePath)
	return nil
}
