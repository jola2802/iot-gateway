package dataforwarding

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
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
	influxBuffer   = make([]*write.Point, 0, 1100)
	bufferMutex    sync.Mutex
	flushTimer     *time.Timer
	influxConfig   *InfluxConfig
	subscriptionID = rand.Intn(100)

	client   influxdb2.Client
	writeAPI api.WriteAPI

	// Neu: Zeitpunkt der letzten empfangenen Nachricht
	lastMessageReceived time.Time
)

func setInfluxDBConfig() {
	influxdbURL := os.Getenv("INFLUXDB_URL")
	influxdbToken := os.Getenv("INFLUXDB_TOKEN")
	influxdbOrg := os.Getenv("INFLUXDB_ORG")
	influxdbBucket := os.Getenv("INFLUXDB_BUCKET")

	// Prüfe, ob alle erforderlichen Umgebungsvariablen gesetzt sind
	if influxdbURL == "" || influxdbToken == "" || influxdbOrg == "" || influxdbBucket == "" {
		logrus.Warn("Mindestens eine InfluxDB-Umgebungsvariable ist nicht gesetzt. Verwende Standardwerte.")
		influxdbURL = "http://influxdb:8086"
		influxdbToken = "secret-token"
		influxdbOrg = "idpm"
		influxdbBucket = "iot-data"
	}

	influxConfig = &InfluxConfig{
		URL:    influxdbURL,
		Token:  influxdbToken,
		Org:    influxdbOrg,
		Bucket: influxdbBucket,
	}
}

// GetInfluxConfig gibt die aktuelle InfluxDB-Konfiguration zurück.
// Die Konfiguration wird nun direkt aus der globalen Variable geladen und nicht mehr aus der Datenbank.
func GetInfluxConfig(db *sql.DB) (*InfluxConfig, error) {
	// Wenn noch keine Konfiguration gesetzt wurde, versuche sie zu setzen
	if influxConfig == nil {
		setInfluxDBConfig()
	}

	// Wenn die Konfiguration immer noch nil ist, gib einen Fehler zurück
	if influxConfig == nil || influxConfig.URL == "" {
		return nil, fmt.Errorf("Keine InfluxDB-Konfiguration gefunden. Bitte setze die Umgebungsvariablen.")
	}

	// Prüfe, ob die Verbindung funktioniert
	client := influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	defer client.Close()

	health, err := client.Health(context.Background())
	if err != nil || health.Status != "pass" {
		return nil, fmt.Errorf("InfluxDB nicht erreichbar: %v", err)
	}

	return influxConfig, nil
}

// initializeClient erstellt einen neuen InfluxDB-Client, falls noch keiner existiert.
// Falls noch keine gültige Konfiguration vorhanden ist, wird alle 10 Sekunden versucht, diese zu laden.
func initializeClient() error {
	// Warte auf eine gültige InfluxDB-Konfiguration, falls nicht vorhanden
	for influxConfig == nil || influxConfig.URL == "" {
		// Wenn keine Konfiguration vorhanden ist, versuche sie zu laden
		if influxConfig == nil {
			setInfluxDBConfig()
		}

		// Wenn immer noch keine Konfiguration vorhanden ist, warte und versuche es erneut
		if influxConfig == nil || influxConfig.URL == "" {
			logrus.Warn("Keine gültige InfluxDB-Konfiguration gefunden, versuche in 10 Sekunden erneut...")
			time.Sleep(10 * time.Second)
			continue
		}
	}

	client = influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	writeAPI = client.WriteAPI(influxConfig.Org, influxConfig.Bucket)
	// logrus.Infof("InfluxDB-Client initialisiert mit URL: %s", influxConfig.URL)
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
	// Setze die InfluxDB-Konfiguration aus den Umgebungsvariablen
	setInfluxDBConfig()

	// Initialisiere den InfluxDB-Client
	if err := initializeClient(); err != nil {
		logrus.Errorf("Fehler beim Initialisieren des InfluxDB-Clients: %v", err)
	}

	// Verwende den neuen Callback
	server.Subscribe("data/#", rand.Intn(100), influxMessageCallback(db))
	// Setze den initialen Zeitpunkt auf jetzt
	lastMessageReceived = time.Now()
	// go monitorServer(db, server) // erstmal nicht, weil sonst bei jedem Re-Subscribe eine neue, zusätzliche Verbindung hergestellt wird
}

// monitorServer überwacht die MQTT-Verbindung und führt bei Verbindungsverlust einen Re-Subscribe durch.
func monitorServer(db *sql.DB, server *MQTT.Server) {
	ticker := time.NewTicker(5000 * time.Millisecond) // alle 1000 millisekunden wird der server überwacht
	defer ticker.Stop()

	for {
		<-ticker.C
		// Prüfe, ob in den letzten 30 Sekunden Nachrichten empfangen wurden
		if time.Since(lastMessageReceived) > 30000*time.Millisecond {
			logrus.Warn("InfluxDB-Writer: Keine Nachrichten in den letzten 15 Sekunden empfangen. Versuche, die MQTT-Verbindung wiederherzustellen (Re-Subscribe)...")

			if server == nil {
				logrus.Warn("InfluxDB-Writer: MQTT-Server nicht mehr aktiv. Beende den Writer...")

				// Altes Abonnement entfernen
				if err := server.Unsubscribe("data/#", subscriptionID); err != nil {
					logrus.Warnf("Fehler beim Entfernen des alten Abonnements: %v", err)
				}

				// Neue Abonnement-ID generieren
				subscriptionID = rand.Intn(100)

				// Re-Subscribe: Die Verwendung unserer Callback-Hilfsfunktion stellt sicher,
				// dass auch der Zeitstempel neu gesetzt wird, sobald eine Nachricht ankommt.
				if err := server.Subscribe("data/#", subscriptionID, influxMessageCallback(db)); err != nil {
					logrus.Errorf("Fehler beim erneuten Subscriben: %v", err)
				} else {
					logrus.Infof("Re-Subscribe erfolgreich")
					// Aktualisiere lastMessageReceived, um wieder holpernde Reconnects zu vermeiden.
					lastMessageReceived = time.Now()
				}
				return
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
