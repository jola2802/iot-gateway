package dataforwarding

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
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

// Globale Variablen für das Buffering
var (
	// Puffer mit Kapazität für 1100 Punkte
	influxBuffer = make([]*write.Point, 0, 1100)
	bufferMutex  sync.Mutex
	flushTimer   *time.Timer
	influxConfig *InfluxConfig

	client   influxdb2.Client
	writeAPI api.WriteAPI

	// Neu: Zeitpunkt der letzten empfangenen Nachricht
	lastMessageReceived time.Time
)

func SetInfluxDBConfig(influxdbURL string, influxdbToken string, influxdbOrg string, influxdbBucket string) {
	influxConfig = &InfluxConfig{
		URL:    influxdbURL,
		Token:  influxdbToken,
		Org:    influxdbOrg,
		Bucket: influxdbBucket,
	}

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

		// logrus.Infof("Verwende InfluxDB Konfiguration: %s", config.URL)
		return &config, nil
	}

	return nil, fmt.Errorf("Keine funktionsfähige InfluxDB-Konfiguration gefunden")
}

// initializeClient erstellt einen neuen InfluxDB-Client, falls noch keiner existiert.
// Falls noch keine gültige Konfiguration vorhanden ist, wird alle 10 Sekunden versucht, diese zu laden.
func initializeClient(db *sql.DB) error {
	// Warte auf eine gültige InfluxDB-Konfiguration, falls nicht vorhanden
	for influxConfig == nil {
		config, err := GetInfluxConfig(db)
		if err == nil {
			influxConfig = config
			break
		}
		logrus.Warn("Keine InfluxDB-Konfiguration gefunden, versuche in 10 Sekunden erneut...")
		time.Sleep(10 * time.Second)
	}
	client = influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	writeAPI = client.WriteAPI(influxConfig.Org, influxConfig.Bucket)
	return nil
}

// flushBuffer schreibt alle im Puffer gesammelten Punkte an InfluxDB und leert den Puffer.
func flushBuffer(db *sql.DB) error {
	// logrus.Infof("Flushen des Puffers")
	bufferMutex.Lock()
	points := influxBuffer
	// Puffer zurücksetzen (mit Kapazität 1100)
	influxBuffer = make([]*write.Point, 0, 1100)
	// Falls ein Timer aktiv war, stoppen wir ihn und setzen ihn zurück.
	if flushTimer != nil {
		// logrus.Infof("Timer aktiv, stoppen und zurücksetzen")
		flushTimer.Stop()
		flushTimer = nil
	}
	bufferMutex.Unlock()

	if len(points) == 0 {
		// logrus.Infof("Keine Punkte zum Flushen vorhanden")
		return nil
	}

	// Stelle sicher, dass ein Client existiert und funktionsfähig ist
	if client == nil {
		if err := initializeClient(db); err != nil {
			logrus.Errorf("Fehler beim Initialisieren des Clients: %v", err)
			return err
		}
	} else {
		health, err := client.Health(context.Background())
		if err != nil || health.Status != "pass" {
			if err := initializeClient(db); err != nil {
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
	// logrus.Infof("Erfolgreich %d Punkte in InfluxDB geschrieben", len(points))
	return nil
}

// WriteDeviceDataToInflux konvertiert die eingehenden DeviceData in einen InfluxDB-Punkt und legt ihn in einem Puffer ab.
// Sobald der Puffer 1100 Punkte enthält oder nach 5 Sekunden automatisch geleert wird, werden alle Punkte an InfluxDB gesendet.
func writeDeviceDataToInflux(db *sql.DB, device DeviceData) error {
	// logrus.Infof("Schreibe DeviceData in InfluxDB: %v", device)
	// Verwende nur den Datapoint als Measurement und speichere DeviceId als zusätzlichen Tag.
	measurement := device.Datapoint
	point := influxdb2.NewPointWithMeasurement(measurement).
		AddTag("deviceId", device.DeviceId).
		AddTag("datapointId", device.DatapointId).
		AddField("value", device.Value).
		SetTime(time.Now())

	// Füge den Punkt in den globalen Buffer ein
	bufferMutex.Lock()
	influxBuffer = append(influxBuffer, point)
	currentBufferSize := len(influxBuffer)
	// Wenn 1100 Punkte erreicht sind, sofort flushen
	if currentBufferSize >= 1000 {
		// logrus.Infof("Buffer reached %d points, starting immediate flush", currentBufferSize)
		bufferMutex.Unlock()
		return flushBuffer(db)
	}
	// Falls kein Timer aktiv ist, starte einen Timer für 1000 milliseconds
	if flushTimer == nil {
		// logrus.Infof("No flush timer active. Setting up flush timer for 1000 milliseconds")
		flushTimer = time.AfterFunc(1000*time.Millisecond, func() {
			// logrus.Infof("Flush timer triggered")
			if err := flushBuffer(db); err != nil {
				logrus.Errorf("Fehler beim automatischen Flush: %v", err)
			}
		})
	}
	bufferMutex.Unlock()
	return nil
}

// influxMessageCallback kapselt den Callback-Code so, dass lastMessageReceived immer aktualisiert wird.
func influxMessageCallback(db *sql.DB) func(client *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
	return func(client *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
		// Aktualisiere den Zeitstempel der letzten Nachricht.
		lastMessageReceived = time.Now()

		parts := strings.Split(pk.TopicName, "/")
		if len(parts) < 4 {
			logrus.Errorf("Ungültiges Topic-Format: %s", pk.TopicName)
			return
		}
		deviceId := parts[2]
		measurement := parts[3]

		// measurement ist der Name bestehend aus [DatapointId]_[DatapointName]
		// wir müssen also DatapointId und DatapointName trennen --> DatapointId in der ersten Klammer, DatapointName in der zweiten Klammer
		datapointId := strings.Split(measurement, "[")[1]
		datapointId = strings.Split(datapointId, "]")[0]
		fieldValue := processPayload(pk.Payload)

		dd := DeviceData{
			DeviceId:    deviceId,
			DeviceName:  "", // nicht benötigt
			Datapoint:   measurement,
			DatapointId: datapointId,
			Value:       fieldValue,
		}

		// logrus.Infof("Schreibe DeviceData in InfluxDB: %v", dd)
		if err := writeDeviceDataToInflux(db, dd); err != nil {
			logrus.Errorf("Fehler beim Schreiben in InfluxDB: %v", err)
		}
	}
}

func StartInfluxDBWriter(db *sql.DB, server *MQTT.Server) {
	// logrus.Infof("Starte InfluxDB-Writer")
	// Verwende den neuen Callback
	server.Subscribe("data/#", rand.Intn(100), influxMessageCallback(db))
	// Setze den initialen Zeitpunkt auf jetzt
	lastMessageReceived = time.Now()
	go monitorServer(db, server)
}

// monitorServer überwacht die MQTT-Verbindung und führt bei Verbindungsverlust einen Re-Subscribe durch.
func monitorServer(db *sql.DB, server *MQTT.Server) {
	ticker := time.NewTicker(1000 * time.Millisecond) // alle 1000 millisekunden wird der server überwacht
	defer ticker.Stop()

	for {
		<-ticker.C
		// Prüfe, ob in den letzten 30 Sekunden Nachrichten empfangen wurden
		if time.Since(lastMessageReceived) > 5000*time.Millisecond {
			logrus.Warn("Keine Nachrichten in den letzten 5000 Millisekunden empfangen. Versuche, die MQTT-Verbindung wiederherzustellen (Re-Subscribe)...")
			// Re-Subscribe: Die Verwendung unserer Callback-Hilfsfunktion stellt sicher,
			// dass auch der Zeitstempel neu gesetzt wird, sobald eine Nachricht ankommt.
			if err := server.Subscribe("data/#", rand.Intn(100), influxMessageCallback(db)); err != nil {
				logrus.Errorf("Fehler beim erneuten Subscriben: %v", err)
			} else {
				logrus.Infof("Re-Subscribe erfolgreich")
				// Aktualisiere lastMessageReceived, um wieder holpernde Reconnects zu vermeiden.
				lastMessageReceived = time.Now()
			}
		}
	}
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

// StartDataForwarding startet die Datenweiterleitung
func StartDataForwarding(db *sql.DB, server *MQTT.Server) {
	// Hole alle data Routes aus der Tabelle data_routes
	query := "SELECT * FROM data_routes"
	rows, err := db.Query(query)
	if err != nil {
		logrus.Errorf("Fehler beim Laden der Datenrouten: %v", err)
		return
	}
	defer rows.Close()

	var routes []DataRoute
	for rows.Next() {
		var route DataRoute
		var interval int
		err := rows.Scan(&route.ID, &route.DestinationType, &route.DataFormat, &route.Devices, &route.DestinationURL, &route.Headers, &route.FilePath, &interval, &route.Status)
		if err != nil {
			logrus.Errorf("Fehler beim Scannen der Datenroute: %v", err)
			continue
		}
		route.Interval = strconv.Itoa(interval)
		logrus.Infof("Datenroute geladen: %v", route)
	}

	logrus.Infof("Alle Datenrouten geladen: %d", len(routes))
}

func StopDataForwarding() {
	StopInfluxDBWriter()
}
