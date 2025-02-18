package webui

import (
	"database/sql"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"time"

	dataforwarding "iot-gateway/data-forwarding"
	"iot-gateway/logic"
	"iot-gateway/mqtt_broker"

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
	NodeRedAddress    string `json:"nodeRedAddress"`
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

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Variable für den Node-RED-Status
	var noderedconnection bool
	var nodeRedAddress string

	// Starte eine Goroutine für die Node-RED-Überprüfung
	go func() {
		nodeRedTicker := time.NewTicker(2 * time.Second) // Prüfe alle 5 Sekunden
		defer nodeRedTicker.Stop()

		// get own ip address
		ip, err := getOwnIP()
		if err != nil {
			logrus.Errorf("Error getting own IP address: %v", err)
		}

		logrus.Infof("Own IP address: %s", ip)

		nodeRedUrls := []string{
			"http://" + ip + ":7777",
			"https://" + ip + "/nodered",
		}

		var currentUrl string
		var currentConnection bool
		for _, url := range nodeRedUrls {
			resp, err := httpClient.Get(url)
			if err == nil {
				if resp.StatusCode == 200 {
					currentConnection = true
					currentUrl = url
					resp.Body.Close()
					break
				}
				resp.Body.Close()
			}
		}

		// Nur aktualisieren wenn sich der Status geändert hat
		if currentConnection != noderedconnection {
			noderedconnection = currentConnection
			nodeRedAddress = currentUrl
		}
	}()

	// Ticker für den Broker-Status
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

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

		// Hier wird der Status an den Client gesendet, angepasst an das alte Format:
		status := BrokerStatus{
			Uptime:            brokerUptime,
			NumberMessages:    messageCount,
			NumberDevices:     len(driverIDs),
			NodeRedConnection: noderedconnection,
			NodeRedAddress:    nodeRedAddress,
		}

		if err := conn.WriteJSON(status); err != nil {
			logrus.Errorf("Error sending broker status: %v", err)
			return
		}
	}
}

func getOwnIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func restartGatewayHandler(c *gin.Context) {
	// restart the gateway
	RestartGateway(c)
	c.JSON(http.StatusOK, gin.H{"message": "Gateway restarted successfully"})
}

// RestartGateway accepts either *gin.Context or *sql.DB as argument
func RestartGateway(c *gin.Context) {
	var db *sql.DB
	var server *MQTT.Server
	// var context *gin.Context

	server = c.MustGet("server").(*MQTT.Server)
	db = c.MustGet("db").(*sql.DB)

	dataforwarding.StopInfluxDBWriter()

	// Restart MQTT Broker
	mqtt_broker.RestartBroker(db)

	// Restart All Drivers
	logic.RestartAllDrivers(db)

	dataforwarding.StartInfluxDBWriter(db, server)

	logrus.Info("Gateway restarted successfully")

	// Manual trigger to run Garbage Collector
	logrus.Info("Running garbage collector after restart.")
	runtime.GC()
}
