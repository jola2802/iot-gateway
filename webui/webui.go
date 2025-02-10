package webui

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/gorilla/websocket"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
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

	// Store the db connection and server in the context, so it can be accessed in route handlers
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

// Ändere die BrokerStatus-Struktur (falls noch nicht aktualisiert)
type BrokerStatus struct {
	Uptime            string `json:"uptime"`
	NumberMessages    int    `json:"numberMessages"`
	NumberDevices     int    `json:"numberDevices"`
	NodeRedConnection bool   `json:"nodeRedConnection"`
}

// Variable zum Speichern der Nachrichtenanzahl
var messageCount int
var brokerUptime string

// WebSocket-Endpunkt für den Broker-Status
func brokerStatusWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		logrus.Warn("No token provided for WebSocket connection")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Überprüfe den Token
	wsTokenStore.RLock()
	expriation, exists := wsTokenStore.tokens[token]
	wsTokenStore.RUnlock()
	if !exists || expriation.Before(time.Now()) {
		logrus.Warn("Invalid or expired token provided for WebSocket connection")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

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

	// get server from context
	server := c.MustGet("server").(*MQTT.Server)

	driverIDs := make(map[string]bool)

	// Starte eine Goroutine, um den Broker-Status regelmäßig zu lesen
	go func() {

		// Subscribe to a filter and handle any received messages via a callback function.
		callbackFn := func(cl *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
			topic := pk.TopicName
			payload := string(pk.Payload)

			switch topic {
			case "$SYS/broker/uptime":
				brokerUptime = payload
			case "$SYS/broker/messages/received":
				if count, err := strconv.Atoi(payload); err == nil {
					messageCount = count
				} else {
					logrus.Errorf("Error converting message count (%s): %v", payload, err)
				}
			default:
				// logrus.Infof("states Topic: %s mit Payload: %s", topic, payload)
				const prefix = "iot-gateway/driver/states/"
				if len(topic) >= len(prefix) && topic[:len(prefix)] == prefix {
					// Extrahiere die Driver-ID aus dem Topic
					driverID := topic[len(prefix):]
					driverIDs[driverID] = true
				} else {
					logrus.Infof("Unbehandelte Topic: %s mit Payload: %s", topic, payload)
				}
			}
		}

		_ = server.Subscribe("$SYS/broker/messages/received", 1, callbackFn)
		_ = server.Subscribe("iot-gateway/driver/states/#", 2, callbackFn)
		_ = server.Subscribe("$SYS/broker/uptime", 3, callbackFn)
	}()

	// Ticker, um den Status regelmäßig zu senden
	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	for range ticker.C {
		// Überprüfen, ob der Token noch gültig ist
		wsTokenStore.RLock()
		currentExp, exists := wsTokenStore.tokens[token]
		wsTokenStore.RUnlock()
		if !exists || currentExp.Before(time.Now()) {
			logrus.Warn("Invalid or expired token provided for WebSocket connection")
			closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Token expired")
			_ = conn.WriteMessage(websocket.CloseMessage, closeMessage)
			return
		}

		noderedconnection := false
		resp, err := httpClient.Get("http://127.0.0.1/nodered")
		if err == nil {
			if resp.StatusCode == 200 && resp.StatusCode < 300 {
				noderedconnection = true
			}
			resp.Body.Close()
		}

		// Hier wird der Status an den Client gesendet, angepasst an das alte Format:
		status := BrokerStatus{
			Uptime:            brokerUptime,
			NumberMessages:    messageCount,
			NumberDevices:     len(driverIDs),
			NodeRedConnection: noderedconnection, // Setze hier ggf. den gewünschten Standardwert
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
