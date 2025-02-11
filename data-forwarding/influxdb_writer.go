package dataforwarding

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	MQTT "github.com/mochi-mqtt/server/v2"
	packets "github.com/mochi-mqtt/server/v2/packets"
	"github.com/sirupsen/logrus"
)

// InfluxConfig speichert die InfluxDB-Konfiguration, die aus der SQLite-Tabelle influxdb geladen wird.
type InfluxConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// GetInfluxConfig lädt die korrekte InfluxDB-Konfiguration aus der SQLite-Datenbank.
func GetInfluxConfig(db *sql.DB) (*InfluxConfig, error) {
	query := "SELECT url, token, org, bucket FROM influxdb"
	rows, err := db.Query(query)
	if err != nil {
		logrus.Errorf("Fehler beim Laden der InfluxDB-Konfigurationen: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var config InfluxConfig
		if err := rows.Scan(&config.URL, &config.Token, &config.Org, &config.Bucket); err != nil {
			logrus.Errorf("Fehler beim Scannen der InfluxDB-Konfiguration: %v", err)
			continue
		}

		// Prüfe die Konfiguration, indem eine Health-Abfrage an den InfluxDB-Client gesendet wird
		client := influxdb2.NewClient(config.URL, config.Token)
		health, err := client.Health(context.Background())
		client.Close()
		if err != nil || health.Status != "pass" {
			logrus.Warnf("InfluxDB Konfiguration %s scheint nicht funktionsfähig zu sein: %v", config.URL, err)
			continue
		}

		logrus.Infof("Verwende InfluxDB Konfiguration: %s", config.URL)
		return &config, nil
	}

	return nil, fmt.Errorf("Keine funktionsfähige InfluxDB-Konfiguration gefunden")
}

// Globale Variablen für das Buffering
var (
	// Puffer mit Kapazität für 1100 Punkte
	influxBuffer = make([]*write.Point, 0, 1100)
	bufferMutex  sync.Mutex
	flushTimer   *time.Timer
	influxConfig *InfluxConfig
)

var client influxdb2.Client
var writeAPI api.WriteAPI

// initializeClient erstellt einen neuen InfluxDB-Client, falls noch keiner existiert
func initializeClient() error {
	if client == nil {
		client = influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
		writeAPI = client.WriteAPI(influxConfig.Org, influxConfig.Bucket)
	}
	return nil
}

// flushBuffer schreibt alle im Puffer gesammelten Punkte an InfluxDB und leert den Puffer.
func flushBuffer(db *sql.DB) error {
	logrus.Infof("Flushen des Puffers")
	bufferMutex.Lock()
	points := influxBuffer
	// Puffer zurücksetzen (mit Kapazität 1100)
	influxBuffer = make([]*write.Point, 0, 1100)
	// Falls ein Timer aktiv war, stoppen wir ihn und setzen ihn zurück.
	if flushTimer != nil {
		logrus.Infof("Timer aktiv, stoppen und zurücksetzen")
		flushTimer.Stop()
		flushTimer = nil
	}
	bufferMutex.Unlock()

	if len(points) == 0 {
		logrus.Infof("Keine Punkte zum Flushen vorhanden")
		return nil
	}

	if influxConfig == nil {
		logrus.Infof("Keine InfluxDB-Konfiguration vorhanden, versuche sie zu laden")
		influxConfig, _ = GetInfluxConfig(db)
	}

	// Stelle sicher, dass ein Client existiert und funktionsfähig ist
	if client == nil {
		if err := initializeClient(); err != nil {
			logrus.Errorf("Fehler beim Initialisieren des Clients: %v", err)
			return err
		}
	} else {
		health, err := client.Health(context.Background())
		if err != nil || health.Status != "pass" {
			if err := initializeClient(); err != nil {
				logrus.Errorf("Fehler beim Initialisieren des Clients: %v", err)
				return err
			}
		}
	}

	// Jeden Punkt im Puffer schreiben
	for _, p := range points {
		writeAPI.WritePoint(p)
	}
	writeAPI.Flush()
	logrus.Infof("Erfolgreich %d Punkte in InfluxDB geschrieben", len(points))
	return nil
}

// WriteDeviceDataToInflux konvertiert die eingehenden DeviceData in einen InfluxDB-Punkt und legt ihn in einem Puffer ab.
// Sobald der Puffer 1100 Punkte enthält oder nach 5 Sekunden automatisch geleert wird, werden alle Punkte an InfluxDB gesendet.
func writeDeviceDataToInflux(db *sql.DB, device DeviceData) error {
	logrus.Infof("Schreibe DeviceData in InfluxDB: %v", device)
	// Verwende nur den Datapoint als Measurement und speichere DeviceId als zusätzlichen Tag.
	measurement := device.Datapoint
	point := influxdb2.NewPointWithMeasurement(measurement).
		AddTag("deviceId", device.DeviceId).
		AddTag("datapoint_Id", device.DatapointId).
		AddField("value", device.Value).
		SetTime(time.Now())

	// Füge den Punkt in den globalen Buffer ein
	bufferMutex.Lock()
	influxBuffer = append(influxBuffer, point)
	currentBufferSize := len(influxBuffer)
	// Wenn 1100 Punkte erreicht sind, sofort flushen
	if currentBufferSize >= 1100 {
		logrus.Infof("Buffer reached %d points, starting immediate flush", currentBufferSize)
		bufferMutex.Unlock()
		return flushBuffer(db)
	}
	// Falls kein Timer aktiv ist, starte einen Timer für 5 Sekunden
	if flushTimer == nil {
		logrus.Infof("No flush timer active. Setting up flush timer for 5 seconds")
		flushTimer = time.AfterFunc(5*time.Second, func() {
			logrus.Infof("Flush timer triggered")
			if err := flushBuffer(db); err != nil {
				logrus.Errorf("Fehler beim automatischen Flush: %v", err)
			}
		})
	}
	bufferMutex.Unlock()
	return nil
}

func StartInfluxDBWriter(db *sql.DB, server *MQTT.Server) {
	logrus.Infof("Starte InfluxDB-Writer")
	// Abonniere alle Topics, die mit "data/" beginnen, mit QoS 1
	server.Subscribe("data/#", 1, func(client *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
		parts := strings.Split(pk.TopicName, "/")
		if len(parts) < 4 {
			logrus.Errorf("Ungültiges Topic-Format: %s", pk.TopicName)
			return
		}

		// parts[0] = "data", parts[1] = Quelle, parts[2] = Geräte-ID, parts[3] = Messwertname
		deviceId := parts[2]
		measurement := parts[3]

		// Verarbeite den Payload so, dass als int, float oder string zurückgegeben wird
		fieldValue := processPayload(pk.Payload)

		dd := DeviceData{
			DeviceId:    deviceId,
			DeviceName:  "", // deviceName nicht benötigt
			Datapoint:   measurement,
			DatapointId: "",
			Value:       fieldValue,
		}

		logrus.Infof("Schreibe DeviceData in InfluxDB: %v", dd)
		if err := writeDeviceDataToInflux(db, dd); err != nil {
			logrus.Errorf("Fehler beim Schreiben in InfluxDB: %v", err)
		}
	})
}

// processPayload verarbeitet den Payload und gibt ihn als int, float oder als string zurück.
func processPayload(payload []byte) interface{} {
	// Payload in einen String umwandeln und Leerzeichen entfernen
	payloadStr := strings.TrimSpace(string(payload))

	// Versuche, den Payload als int zu parsen und in float64 umzuwandeln
	if intVal, err := strconv.ParseInt(payloadStr, 10, 64); err == nil {
		return float64(intVal)
	}

	// Versuche, den Payload als float zu parsen
	if floatVal, err := strconv.ParseFloat(payloadStr, 64); err == nil {
		return floatVal
	}

	// Falls weder int noch float möglich sind, gebe den String zurück
	return payloadStr
}

// StopInfluxDBWriter beendet den Writer und schließt den Client
func StopInfluxDBWriter() {
	if flushTimer != nil {
		flushTimer.Stop()
		flushTimer = nil
	}
	if client != nil {
		client.Close()
		client = nil
		writeAPI = nil
	}
}
