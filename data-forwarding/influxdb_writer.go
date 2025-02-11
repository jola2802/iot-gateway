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

// GetInfluxConfig lädt die Konfiguration aus der SQLite-Datenbank.
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
	// Puffer mit Kapazität für 1000 Punkte
	influxBuffer = make([]*write.Point, 0, 1000)
	bufferMutex  sync.Mutex
	flushTimer   *time.Timer
	influxConfig *InfluxConfig
)

// flushBuffer schreibt alle im Puffer gesammelten Punkte an InfluxDB und leert den Puffer.
func flushBuffer(db *sql.DB) error {
	bufferMutex.Lock()
	points := influxBuffer
	// Puffer zurücksetzen (mit Kapazität 1000)
	influxBuffer = make([]*write.Point, 0, 1000)
	// Falls ein Timer aktiv war, stoppen wir ihn und setzen ihn zurück.
	if flushTimer != nil {
		flushTimer.Stop()
		flushTimer = nil
	}
	bufferMutex.Unlock()

	if len(points) == 0 {
		return nil
	}

	if influxConfig == nil {
		influxConfig, _ = GetInfluxConfig(db)
	}

	// InfluxDB-Client erstellen
	client := influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	defer client.Close()
	writeAPI := client.WriteAPIBlocking(influxConfig.Org, influxConfig.Bucket)
	ctx := context.Background()

	// Jeden Punkt im Puffer schreiben
	for _, p := range points {
		err := writeAPI.WritePoint(ctx, p)
		if err != nil {
			logrus.Errorf("Fehler beim Schreiben in InfluxDB: %v", err)
			return err
		}
	}
	logrus.Infof("Erfolgreich %d Punkte in InfluxDB geschrieben", len(points))
	return nil
}

// WriteDeviceDataToInflux konvertiert die eingehenden DeviceData in einen InfluxDB-Punkt und legt ihn in einem Puffer ab.
// Sobald der Puffer 1000 Punkte enthält oder nach 5 Sekunden automatisch geleert wird, werden alle Punkte an InfluxDB gesendet.
func writeDeviceDataToInflux(db *sql.DB, device DeviceData) error {
	// Messung wird hier als Kombination aus DeviceId und Datapoint aufgebaut.
	measurement := device.DeviceId + "_" + device.Datapoint
	point := influxdb2.NewPointWithMeasurement(measurement).
		// AddTag("deviceId", device.DeviceId).
		// AddTag("datapoint", device.Datapoint).
		AddTag("datapoint_Id", device.DatapointId).
		AddField("value", device.Value).
		SetTime(time.Now())

	bufferMutex.Lock()
	influxBuffer = append(influxBuffer, point)
	currentBufferSize := len(influxBuffer)
	// Wenn 1000 Nachrichten gesammelt wurden, sofort flushen.
	if currentBufferSize >= 1000 {
		bufferMutex.Unlock()
		return flushBuffer(db)
	}
	// Falls kein Timer aktiv ist: einen Timer starten, der nach 5 Sekunden den Puffer automatisch leert.
	if flushTimer == nil {
		flushTimer = time.AfterFunc(5*time.Second, func() {
			if err := flushBuffer(db); err != nil {
				logrus.Errorf("Fehler beim automatischen Flush: %v", err)
			}
		})
	}
	bufferMutex.Unlock()
	return nil
}

func StartInfluxDBWriter(db *sql.DB, server *MQTT.Server) {
	// Abonniere alle Topics, die mit "data/" beginnen, mit QoS 1
	server.Subscribe("data/#", 1, func(client *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
		// Beispiel: topic = "data/opcua/9/Hum1"
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
