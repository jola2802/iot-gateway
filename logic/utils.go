package logic

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

var cachedBrokerSettings *BrokerSettings

type BrokerSettings struct {
	Address  string
	Username string
	Password string
}

// <---------------------------------------->
// Utils for user_manager.go
// <---------------------------------------->

// genRandomPW generiert ein zufälliges Passwort
func genRandomPW() string {
	b := make([]byte, 10)
	_, err := rand.Read(b)
	if err != nil {
		logrus.Fatal(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

// LoadDevices liest die Gerätedaten aus der Datenbank
func LoadDevices(db *sql.DB) ([]map[string]interface{}, error) {
	query := `SELECT type, name FROM devices`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying devices from database: %v", err)
	}
	defer rows.Close()

	var deviceList []map[string]interface{}

	for rows.Next() {
		var deviceType, name string

		// Lese Daten aus der Datenbankzeile
		err := rows.Scan(&deviceType, &name)
		if err != nil {
			return nil, fmt.Errorf("error scanning device row: %v", err)
		}

		// Konvertiere die Daten in eine Map
		deviceMap := map[string]interface{}{
			"type": deviceType,
			"name": name,
		}
		deviceList = append(deviceList, deviceMap)
	}

	return deviceList, nil
}

// <---------------------------------------->
// Models for driver_manager.go
// <---------------------------------------->

// LoadMqttConfigFromDB lädt die MQTT-Broker-Daten aus der Datenbank
func LoadMqttConfigFromDB(db *sql.DB) (*MqttConfig, error) {
	var mqttConfig MqttConfig
	query := `
	SELECT bs.address AS broker, a.username, a.password
	FROM broker_settings bs
	JOIN auth a ON a.username = 'webui'
	LIMIT 1
	`
	row := db.QueryRow(query)
	err := row.Scan(&mqttConfig.Broker, &mqttConfig.Username, &mqttConfig.Password)
	if err != nil {
		logrus.Errorf("Driver_Manager: Error loading MQTT config from DB: %v", err)
		return nil, err
	}
	return &mqttConfig, nil
}

// Funktion, um einen MQTT-Client-Pool basierend auf Broker-Daten zu erstellen
func createMQTTClientPool(brokerAddress, username, password string, db *sql.DB) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return createMQTTClient(brokerAddress, username, password, db)
		},
	}
}

// Hole oder initialisiere den Pool dynamisch nach dem Abrufen der Broker-Daten
func getPooledMQTTClient(pool *sync.Pool, db ...*sql.DB) mqtt.Client {
	if pool == nil {
		logrus.Warn("MQTT client pool is not initialized, fetching broker settings from database")

		// Hole die Broker-Einstellungen aus der Datenbank, falls db übergeben wurde
		if len(db) > 0 && db[0] != nil {
			settings, err := getCachedBrokerSettings(db[0])
			if err != nil {
				logrus.Fatalf("Error fetching broker settings: %v", err)
			}

			// Erstelle den MQTT-Client-Pool mit den abgerufenen Einstellungen
			mqttClientPool = createMQTTClientPool(settings.Address, settings.Username, settings.Password, db[0])
		} else {
			logrus.Fatal("Database connection is required to fetch broker settings")
		}
	}
	return pool.Get().(mqtt.Client)
}

// getBrokerSettings fetches the broker settings from the database.
func getBrokerSettings(db *sql.DB) (BrokerSettings, error) {
	var settings BrokerSettings
	// Broker address will still come from broker_settings
	err := db.QueryRow("SELECT address FROM broker_settings WHERE id = 1").Scan(&settings.Address)
	if err != nil {
		return settings, fmt.Errorf("error fetching broker address: %v", err)
	}

	// Fetch the password for the username 'webui' from the auth table
	settings.Username = "webui"
	err = db.QueryRow("SELECT password FROM auth WHERE username = ?", settings.Username).Scan(&settings.Password)
	if err != nil {
		return settings, fmt.Errorf("error fetching password for user 'webui': %v", err)
	}

	return settings, nil
}

func getCachedBrokerSettings(db *sql.DB) (BrokerSettings, error) {
	if cachedBrokerSettings != nil {
		return *cachedBrokerSettings, nil
	}
	settings, err := getBrokerSettings(db)
	if err != nil {
		return settings, err
	}
	cachedBrokerSettings = &settings // Cache the settings
	return settings, nil
}

func releaseMQTTClient(pool *sync.Pool, client mqtt.Client) {
	pool.Put(client)
}

func createMQTTClient(brokerAddress, username, password string, db *sql.DB) mqtt.Client {
	opts := mqtt.NewClientOptions().AddBroker(brokerAddress).SetClientID(fmt.Sprintf("client_%d", time.Now().UnixNano()))
	opts.SetUsername(username).SetPassword(password).SetAutoReconnect(true).SetCleanSession(true)

	// // Setze den Default-Publish-Handler, der handleMqttDriverCommands aufruft
	// opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
	// 	handleMqttDriverCommands(msg, db)
	// })

	client := mqtt.NewClient(opts)
	for {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			logrus.Errorf("WEB-UI: Error connecting to MQTT broker: %v", token.Error())
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	return client
}

func parseTopic(topic string) (string, string) {
	parts := strings.Split(topic, "/")
	if len(parts) >= 5 {
		return parts[3], parts[4]
	}
	return "", ""
}

// Generische Funktion für das Starten eines Gerätezustands
func getOrCreateDeviceState(deviceName string, deviceStates map[string]*DeviceState) *DeviceState {
	state, exists := deviceStates[deviceName]
	if !exists {
		state = &DeviceState{}
		deviceStates[deviceName] = state
	}
	return state
}

var mqttClientPool *sync.Pool

// Clear all clients in the pool to ensure no clients are trying to connect to the MQTT broker
func clearAllMQTTClients(pool *sync.Pool) {
	if pool == nil {
		return
	}

	// Iterate through the entire pool and disconnect all MQTT clients
	for client := pool.Get(); client != nil; client = pool.Get() {
		if mqttClient, ok := client.(mqtt.Client); ok && mqttClient.IsConnected() {
			mqttClient.Disconnect(0) // Close the connection with a short wait time
		}
	}

	// Set the pool to `nil` to ensure it is recreated
	mqttClientPool = nil
	logrus.Info("All MQTT clients in the pool have been cleared")
}

var stateTopic string

func StartMqttListener(topic string, db *sql.DB) {
	stateTopic = topic
	settings, err := getBrokerSettings(db)
	if err != nil {
		logrus.Fatalf("Error fetching broker settings: %v", err)
	}
	mqttClientPool = createMQTTClientPool(settings.Address, settings.Username, settings.Password, db)
	// Hole einen fertigen Client aus dem Pool
	mqttClient := getPooledMQTTClient(mqttClientPool, db)
	defer releaseMQTTClient(mqttClientPool, mqttClient)

	// Verifiziere, dass der Client verbunden ist
	if !mqttClient.IsConnected() {
		logrus.Errorf("DM: MQTT client is not connected")
		return
	}

	// Abonniere das gewünschte Topic und setze den Handler für eingehende Nachrichten
	if token := mqttClient.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		// Verarbeite die eingehende Nachricht mit handleMqttDriverCommands
		handleMqttDriverCommands(msg, db)
	}); token.Wait() && token.Error() != nil {
		logrus.Errorf("DM: Error subscribing to topic: %v", token.Error())
		return
	}

	logrus.Infof("DM: Subscribed to topic %s", topic)
}

func StopMqttListener() {
	// Hole einen fertigen Client aus dem Pool, um ihn zu stoppen
	mqttClient := getPooledMQTTClient(mqttClientPool, nil)

	// Verifiziere, ob der Client noch verbunden ist
	if mqttClient.IsConnected() {
		// Trenne die MQTT-Verbindung
		mqttClient.Disconnect(0)
		logrus.Info("MQTT client disconnected")
	}
}

// func RestartMqttListener(db *sql.DB) {
// 	StopMqttListener()
// 	StartMqttListener(stateTopic, db)
// }

// MQTT-Publikation mit exponentiellem Backoff
func publishDeviceState(server *MQTT.Server, deviceType, deviceName string, status string) {
	topic := "iot-gateway/driver/states/" + deviceType + "/" + deviceName
	publishWithBackoff(server, topic, status, 5)
}

// Implementiere exponentiellen Backoff
func publishWithBackoff(server *MQTT.Server, topic string, payload string, maxRetries int) {
	backoff := 1000 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		err := server.Publish(topic, []byte(payload), true, 1)
		if err == nil {
			return
		}
		time.Sleep(backoff)
		backoff *= 2 // Exponentielles Wachstum der Wartezeit
	}
	logrus.Errorf("Failed to publish message after %d retries", maxRetries)
}
