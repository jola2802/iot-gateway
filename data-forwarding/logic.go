package dataforwarding

import (
	"database/sql"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

var client MQTT.Client

// CacheMqttData stellt eine Verbindung zu einem MQTT-Broker her, abonniert ein Topic und speichert die empfangenen Nachrichten in der SQL-Datenbank
func CacheMqttData(db *sql.DB, cacheDuration time.Duration) {
	// Schritt 1: Hole die Zugangsdaten zum MQTT Broker aus der Datenbank
	var mqttConfig MqttConfig
	query := `
		SELECT bs.address AS broker, a.username, a.password
		FROM broker_settings bs
		JOIN auth a ON a.username = 'webui-admin'
		LIMIT 1
	`
	row := db.QueryRow(query)
	err := row.Scan(&mqttConfig.Broker, &mqttConfig.Username, &mqttConfig.Password)
	if err != nil {
		logrus.Printf("Error loading MQTT config from DB: %v", err)
		return
	}
	// Schritt 2: Überprüfen, ob ein funktionierender Client bereits existiert
	if client != nil && client.IsConnected() {
		logrus.Println("MQTT client already connected, disconnecting and reconnecting")
		// client.Disconnect(250) // Disconnect with a 250ms timeout
		// time.Sleep(500 * time.Millisecond)
	}

	// MQTT-Client konfigurieren
	opts := MQTT.NewClientOptions().
		AddBroker(mqttConfig.Broker).
		SetUsername(mqttConfig.Username).
		SetPassword(mqttConfig.Password).
		SetPingTimeout(10 * time.Second).
		SetAutoReconnect(true)

	client = MQTT.NewClient(opts)

	// Schritt 3: Mit dem MQTT-Broker verbinden
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logrus.Fatal(token.Error())
	}

	// Schritt 4: Topic abonnieren und Nachrichten verarbeiten
	client.Subscribe("data/#", 0, func(client MQTT.Client, msg MQTT.Message) {
		data := string(msg.Payload())
		timestamp := time.Now().Format("02.01.2006 15:04:05.000")

		// Parsen des Topics in verschiedene Teile
		topicParts := strings.Split(msg.Topic(), "/")
		if len(topicParts) < 4 {
			logrus.Errorf("Invalid topic format: %s", msg.Topic())
			return
		}

		// Der `device_name` ist der dritte Teil des Topics
		deviceName := topicParts[2]

		// Hole die GeräteID "id" und den Gerätetyp "type" aus der Tabelle "devices" anhand des Gerätenamens
		var deviceId string
		var deviceType string
		query = `
			SELECT id, type FROM devices WHERE name = ?
		`
		err = db.QueryRow(query, deviceName).Scan(&deviceId, &deviceType)
		if err != nil {
			if err == sql.ErrNoRows {
				logrus.Errorf("Device not found in 'devices' table: %s", deviceName)
			} else {
				deviceId = "999"
				logrus.Errorf("Error retrieving device ID and type for device %s: %v", deviceName, err)
			}
			return
		}

		// Der Datenpunktname ist der letzte Teil des Topics
		datapoint := topicParts[len(topicParts)-1]

		// Suche die ID des Datenpunkts basierend auf dem Gerätetyp
		var datapointId string
		if deviceType == "s7" {
			query = `
				SELECT datapointId FROM s7_datapoints WHERE name = ?
			`
			err = db.QueryRow(query, datapoint).Scan(&datapointId)
		} else if deviceType == "opcua" {
			query = `
				SELECT datapointId FROM opcua_datanodes WHERE name = ?
			`
			err = db.QueryRow(query, datapoint).Scan(&datapointId)
		} else {
			datapointId = "99"
			// logrus.Errorf("Unknown device type for device %s: %s", deviceName, deviceType)
			return
		}

		if datapointId == "" {
			datapointId = "99"
		}

		// Sammle die Daten im Batch und speichere sie nach 10 Werten
		SaveDeviceDataToBatch(db, deviceName, deviceId, datapoint, datapointId, data, timestamp, cacheDuration)
	})

	// Starten der konfigurierten Data Routes
	go func() {
		StartDataRoutes(db)
	}()

	// Halte die Funktion am Laufen, damit sie weiter auf neue Nachrichten wartet
	select {}
}

func StopCache(db *sql.DB) {
	// if client != nil && client.IsConnected() {
	// 	client.Disconnect(0)
	// }

	// Delete the data in the table "device_data"
	_, err := db.Exec("DELETE FROM device_data")
	if err != nil {
		logrus.Errorf("Error deleting data from 'device_data' table: %v", err)
	}
}

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
