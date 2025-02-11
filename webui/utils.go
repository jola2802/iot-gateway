package webui

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var mqttClientPool *sync.Pool

// Funktion, um einen MQTT-Client-Pool basierend auf Broker-Daten zu erstellen
func createMQTTClientPool(brokerAddress, username, password string) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return createMQTTClient(brokerAddress, username, password)
		},
	}
}

// Hole oder initialisiere den Pool dynamisch nach dem Abrufen der Broker-Daten
func getPooledMQTTClient(pool *sync.Pool, db *sql.DB) mqtt.Client {
	if mqttClientPool == nil {
		logrus.Warn("MQTT client pool is not initialized, fetching broker settings from database")

		// Hole die Broker-Einstellungen aus der Datenbank
		settings, err := getCachedBrokerSettings(db)
		if err != nil {
			logrus.Fatalf("Error fetching broker settings: %v", err)
		}

		// Erstelle den MQTT-Client-Pool mit den abgerufenen Einstellungen
		mqttClientPool = createMQTTClientPool(settings.Address, settings.Username, settings.Password)
	}
	client := pool.Get().(mqtt.Client)
	if !client.IsConnected() {
		logrus.Warn("MQTT client is not connected | should reconnect |")
		// client = createMQTTClientPool()
	}

	return client
}

func releaseMQTTClient(pool *sync.Pool, client mqtt.Client) {
	pool.Put(client)
}

func createMQTTClient(brokerAddress, username, password string) mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerAddress).
		SetClientID(fmt.Sprintf("client_%d", time.Now().UnixNano())).
		SetUsername(username).
		SetPassword(password).
		// SetAutoReconnect(true).
		// SetConnectRetryInterval(2 * time.Second).
		SetConnectRetry(true)

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		// try to reconnect
		logrus.Errorf("WEB-UI: Connection lost to MQTT broker: %v", err)
	}

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

// Hilfsfunktion zum Parsen von Strings in Int
func parseInt(str string) int {
	val, err := strconv.Atoi(str)
	if err != nil {
		logrus.Errorf("Error parsing integer: %v", err)
		return 0
	}
	return val
}

func loadConfig(filename string) (*Config, error) {
	configBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func generateToken(c *gin.Context) {
	token, _ := generateRandomToken()
	expiration := time.Now().Add(10 * time.Minute)

	wsTokenStore.Lock()
	wsTokenStore.tokens[token] = expiration
	wsTokenStore.Unlock()

	c.JSON(http.StatusOK, gin.H{"token": token, "expiration": expiration})
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32) // 256 Bit Token
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// monitorWebSocket überwacht die Verbindung und schließt sie bei Fehlern.
func monitorWebSocket(conn *websocket.Conn) {
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			logrus.Warnf("WebSocket disconnected: %v", err)
			gracefulShutdown(conn)
			return
		}
	}
}
