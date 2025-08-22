package webui

import (
	"crypto/tls"
	"database/sql"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/gorilla/websocket"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/rand"
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
var nodeRedUrl string

// showDashboard shows the dashboard page
func showDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

// WebSocket-Endpunkt für den Broker-Status auf dem Dashboard
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
				const prefix = "driver/states/"
				if len(topic) >= len(prefix) && topic[:len(prefix)] == prefix {
					// Hier den Mutex verwenden beim Schreiben in die Map
					// driverIDsMutex.Lock()
					// driverID := topic[len(prefix):]
					// driverIDs[driverID] = true
					// driverIDsMutex.Unlock()
				} else {
					logrus.Infof("Unbehandelte Topic: %s mit Payload: %s", topic, payload)
				}
			}
		}

		_ = server.Subscribe("$SYS/broker/messages/received", rand.Intn(100), callbackFn)
		// _ = server.Subscribe("driver/states/#", rand.Intn(100), callbackFn)
		_ = server.Subscribe("$SYS/broker/uptime", rand.Intn(100), callbackFn)
	}()

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// format node-red url
	nodeRedPort := os.Getenv("NODE_RED_HTTP_PORT")
	if nodeRedPort == "" {
		if os.Getenv("NODE_RED_URL") != "" {
			nodeRedUrl = os.Getenv("NODE_RED_URL")
		} else {
			nodeRedUrl = "http://node-red:1880"
		}
	} else {
		nodeRedUrl = "http://node-red:" + nodeRedPort
		if os.Getenv("NODE_RED_URL") != "" {
			nodeRedUrl = os.Getenv("NODE_RED_URL")
		}
	}

	if nodeRedUrl == "" {
		logrus.Warn("Node-RED URL is not set, skipping node red check")
		stateConnection = false
		return
	}

	// Starte eine Goroutine für die Node-RED-Überprüfung
	go func() {
		nodeRedTicker := time.NewTicker(20 * time.Second) // Prüfe alle 10 Sekunden
		defer nodeRedTicker.Stop()

		// Einmal zu Beginn die Verbindung prüfen
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext:     (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		}
		resp, err := httpClient.Get(nodeRedUrl)
		if err != nil {
		} else {
			stateConnection = true
			defer resp.Body.Close()
		}

		for range nodeRedTicker.C {
			// Zertifikatsüberprüfung überspringen
			httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				DialContext:     (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			}
			resp, err := httpClient.Get(nodeRedUrl)
			if err != nil {
				logrus.Errorf("HTTP-Anfrage an Node-RED fehlgeschlagen: %v", err)
				stateConnection = false
				continue
			}

			// Überprüfe den Statuscode der Antwort
			if resp.StatusCode == 200 || resp.StatusCode == 302 {
				stateConnection = true
				// logrus.Infof("Node-RED URL: %s, Status: %d", nodeRedUrl, resp.StatusCode)
			} else {
				stateConnection = false
				logrus.Infof("Node-RED URL: %s, Status: %d", nodeRedUrl, resp.StatusCode)
			}
		}
	}()

	// Ticker für den Broker-Status
	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	// get db connection
	db, _ := getDBConnection(c)

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
		// get device count from db table devices
		deviceCount := getDeviceCount(db)

		status := BrokerStatus{
			Uptime:            brokerUptime,
			NumberMessages:    messageCount,
			NumberDevices:     deviceCount,
			NodeRedConnection: stateConnection,
		}

		if err := conn.WriteJSON(status); err != nil {
			logrus.Errorf("Error sending broker status: %v", err)
			return
		}
	}
}

func getDeviceCount(db *sql.DB) int {
	rows, err := db.Query("SELECT COUNT(*) FROM devices")
	if err != nil {
		logrus.Errorf("Error getting device count: %v", err)
		return 0
	}
	defer rows.Close()

	var count int
	rows.Next()
	err = rows.Scan(&count)
	if err != nil {
		logrus.Errorf("Error getting device count: %v", err)
		return 0
	}

	return count
}
