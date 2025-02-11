package webui

import (
	"database/sql"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"iot-gateway/logic"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/gorilla/websocket"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/sirupsen/logrus"
)

type BrokerStatus struct {
	Uptime            string `json:"uptime"`
	NumberMessages    int    `json:"numberMessages"`
	NumberDevices     int    `json:"numberDevices"`
	NodeRedConnection bool   `json:"nodeRedConnection"`
}

// Variable zum Speichern der Nachrichtenanzahl
var messageCount int
var brokerUptime string

// showDashboard shows the dashboard page
func showDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

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

	// Starte eine Goroutine, um Verbindungsabbrüche zu überwachen.
	go monitorWebSocket(conn)

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

func restartGatewayHandler(c *gin.Context) {
	// restart the gateway
	RestartGateway(c)
}

// RestartGateway accepts either *gin.Context or *sql.DB as argument
func RestartGateway(input interface{}) {
	var db *sql.DB
	var context *gin.Context

	switch v := input.(type) {
	case *gin.Context:
		// If input is *gin.Context, extract the database connection from the context
		context = v
		dbConn, exists := context.Get("db")
		if !exists {
			context.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
			return
		}

		// Ensure the dbConn is of type *sql.DB
		var ok bool
		db, ok = dbConn.(*sql.DB)
		if !ok {
			context.JSON(http.StatusInternalServerError, gin.H{"message": "Invalid database connection"})
			return
		}

	case *sql.DB:
		// If input is directly *sql.DB, assign it to the db variable
		db = v

	default:
		// Handle unsupported types
		context.JSON(http.StatusInternalServerError, gin.H{"message": "Invalid input type"})
		return
	}

	// Restart MQTT Broker
	// mqtt_broker.RestartBroker(db)

	// Restart All Drivers
	logic.RestartAllDrivers(db)

	logrus.Info("Gateway restarted successfully")

	// Manual trigger to run Garbage Collector
	logrus.Info("Running garbage collector after restart.")
	runtime.GC()

	// If input was a *gin.Context, send a success response
	if context != nil {
		context.JSON(http.StatusOK, gin.H{"message": "Gateway restarted successfully"})
	}
}

// showDashboard gives the data to the dashboard page
func dashboardWS(c *gin.Context) {

}
