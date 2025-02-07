package webui

import (
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

type WebUIConfig struct {
	HTTPPort  string `json:"http_port"`
	HTTPSPort string `json:"https_port"`
	UseHTTPS  bool   `json:"use_https"`
	TLSCert   string `json:"tls_cert"`
	TLSKey    string `json:"tls_key"`
}

type Config struct {
	WebUI WebUIConfig `json:"webui"`
}

var server *MQTT.Server

// Main function to start the web server
func Main(db *sql.DB, serverF *MQTT.Server) {
	server = serverF

	if db == nil {
		logrus.Fatal("Database connection is not initalized.")
	}

	// Lade die Konfiguration aus der config.json
	config, err := loadConfig("config.json")
	if err != nil {
		logrus.Fatal("Failed to load config: ", err)
	}

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))

	// Store the db connection in the context, so it can be accessed in route handlers
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("server", server)
		c.Next()
	})

	// Define routes
	setupRoutes(r)

	// Starte den Server basierend auf der Konfiguration
	if config.WebUI.UseHTTPS {
		// Starte den HTTPS-Server
		if config.WebUI.TLSCert == "" || config.WebUI.TLSKey == "" {
			logrus.Fatal("TLS certificate and key must be specified for HTTPS.")
		}
		logrus.Infof("Starting HTTPS server on port %s", config.WebUI.HTTPSPort)
		err = r.RunTLS(":"+config.WebUI.HTTPSPort, config.WebUI.TLSCert, config.WebUI.TLSKey)
		if err != nil {
			logrus.Fatal("Failed to start HTTPS server: ", err)
		}
	} else {
		// Starte den HTTP-Server
		port := config.WebUI.HTTPPort
		if port == "" {
			port = "8080" // Fallback auf den Standardport
		}
		logrus.Infof("Starting HTTP server on port %s", port)
		err = r.Run(":" + port)
		if err != nil {
			logrus.Fatal("Failed to start HTTP server: ", err)
		}
	}
}

// var templatesFS embed.FS
// var staticFS embed.FS
// var tmpl *template.Template

// Struktur für die MQTT-Metriken
type BrokerStatus struct {
	BrokerUptime    string   `json:"brokerUptime"`
	MessageCount    int      `json:"messageCount"`
	ClientCount     int      `json:"clientCount"`
	LastMessageTime string   `json:"lastMessageTime"`
	Devices         []Device `json:"devices"`
}

// Variable zum Speichern der Nachrichtenanzahl
var messageCount int
var clientCount int
var brokerUptime string
var lastMessageTime time.Time

// MQTT-Handler für das Empfangen von System-Metriken
var sysTopicHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	switch topic {
	case "$SYS/broker/messages/received":
		messageCount = parseInt(payload)
		lastMessageTime = time.Now()
	case "$SYS/broker/clients/connected":
		clientCount = parseInt(payload)
	case "$SYS/broker/uptime":
		brokerUptime = payload
	}
}

var devicesMap = make(map[string]Device) // Key: "deviceType/deviceName"
var devicesMapMutex sync.Mutex

var deviceTopicHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// Extrahiere deviceType und deviceName aus dem Topic
	parts := strings.Split(topic, "/")
	if len(parts) >= 4 {
		deviceType := parts[2]
		deviceName := parts[3]

		// Bestimme, ob das Gerät verbunden ist ("running" bedeutet verbunden)
		connected := (payload == "1 (running)")

		// Erstelle oder aktualisiere das Gerät in der devicesMap
		deviceKey := deviceType + "/" + deviceName
		devicesMapMutex.Lock()
		devicesMap[deviceKey] = Device{
			DeviceType: deviceType,
			DeviceName: deviceName,
			Connected:  connected,
		}
		devicesMapMutex.Unlock()
	}
}

// WebSocket-Endpunkt für den Broker-Status
func brokerStatusWebSocket(c *gin.Context) {
	// WebSocket-Verbindung herstellen
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("Error upgrading to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	// Starte eine Goroutine, um Verbindungsabbrüche zu überwachen
	go func() {
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				logrus.Warnf("WebSocket disconnected: %v", err)
				gracefulShutdown(conn)
				return
			}
		}
	}()

	// Hole die Datenbankverbindung aus dem Context
	db, exists := c.Get("db")
	if !exists {
		logrus.Println("Error retrieving database connection from context")
		return
	}

	// Hole einen MQTT-Client aus dem Pool
	mqttClient := getPooledMQTTClient(mqttClientPool, db.(*sql.DB))
	defer releaseMQTTClient(mqttClientPool, mqttClient) // Gib den Client nach der Nutzung zurück

	// Abonniere die relevanten $SYS-Topics für Metriken
	sysTopics := []string{
		"$SYS/broker/uptime",
		"$SYS/broker/messages/received",
		"$SYS/broker/clients/connected",
	}

	for _, topic := range sysTopics {
		mqttClient.Subscribe(topic, 1, sysTopicHandler)
	}

	// Abonniere das Wildcard-Topic für Geräte
	mqttClient.Subscribe("iot-gateway/driver/#", 1, deviceTopicHandler)

	// Ticker, um den Status regelmäßig zu senden
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// Konvertiere die devicesMap in ein Slice, um es zu serialisieren
		devices := make([]Device, 0, len(devicesMap))
		for _, device := range devicesMap {
			devices = append(devices, device)
		}

		// Sende den Status an den Client
		status := BrokerStatus{
			BrokerUptime:    brokerUptime,
			MessageCount:    messageCount,
			ClientCount:     clientCount,
			LastMessageTime: lastMessageTime.Format("2006-01-02 15:04:05"),
			Devices:         devices,
		}

		if err := conn.WriteJSON(status); err != nil {
			logrus.Errorf("Error sending broker status: %v", err)
			return
		}
	}
}

// showDashboard shows the dashboard page
func showDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)

	dbConn, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	// Hole die Broker-Einstellungen aus der Datenbank
	settings, err := getCachedBrokerSettings(dbConn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching broker settings"})
		return
	}

	// Erstelle den MQTT-Client-Pool mit den Broker-Einstellungen
	mqttClientPool = createMQTTClientPool(settings.Address, settings.Username, settings.Password)
}

// showRoutingPage shows the routing page
func showRoutingPage(c *gin.Context) {
	c.HTML(http.StatusOK, "data-forwarding.html", nil)
}
