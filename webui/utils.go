package webui

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	dataforwarding "iot-gateway/data-forwarding"
	"iot-gateway/logic"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

// User defines a user with their associated ACL entries.
type User struct {
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	AclEntries []ACLEntry `json:"aclEntries"`
}

// ACLEntry defines the ACL topic and its associated permission for a user.
type ACLEntry struct {
	Topic      string `json:"topic"`
	Permission int    `json:"permission"` // Als int definiert
}

// getDBConnection retrieves the database connection from the gin.Context.
func getDBConnection(c *gin.Context) (*sql.DB, error) {
	db, exists := c.Get("db")
	if !exists {
		return nil, errors.New("database connection not found")
	}
	return db.(*sql.DB), nil
}

// getDBConnection retrieves the database connection from the gin.Context.
func getMQTTServer(c *gin.Context) (*MQTT.Server, error) {
	server, exists := c.Get("server")
	if !exists {
		return nil, errors.New("database connection not found")
	}
	return server.(*MQTT.Server), nil
}

func loadConfigFromEnv() (*Config, error) {
	config := &Config{}

	config.WebUI.HTTPPort = os.Getenv("WEBUI_HTTP_PORT")
	config.WebUI.HTTPSPort = os.Getenv("WEBUI_HTTPS_PORT")
	config.WebUI.TLSCert = os.Getenv("WEBUI_TLS_CERT")
	config.WebUI.TLSKey = os.Getenv("WEBUI_TLS_KEY")

	return config, nil
}

func generateToken(c *gin.Context) {
	token, _ := generateRandomToken()
	expiration := time.Now().Add(30 * time.Minute)

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

func restartGatewayHandler(c *gin.Context) {
	// restart the gateway
	RestartGateway(c)
	c.JSON(http.StatusOK, gin.H{"message": "Gateway restarted successfully"})
}

// RestartGateway accepts either *gin.Context or *sql.DB as argument
func RestartGateway(c *gin.Context) {
	var db *sql.DB
	var server *MQTT.Server

	db = c.MustGet("db").(*sql.DB)
	server = c.MustGet("server").(*MQTT.Server)

	// Stop InfluxDB Writer
	dataforwarding.StopInfluxDBWriter()

	// Restart MQTT Broker
	// server = mqtt_broker.RestartBroker(db)

	// update server in context
	// c.Set("server", server)

	// Restart All Drivers
	logic.RestartAllDrivers(db, server)

	dataforwarding.StartInfluxDBWriter(db, server)

	logrus.Info("Gateway restarted successfully")

	// Manual trigger to run Garbage Collector
	logrus.Info("Running garbage collector after restart.")
	runtime.GC()
}

func RestartDriver(c *gin.Context) {
	// get driver id from context
	driverID := c.Param("device_id")

	// get db from context
	db := c.MustGet("db").(*sql.DB)

	// restart driver
	logic.RestartDevice(db, driverID)

	c.JSON(http.StatusOK, gin.H{"message": "Driver restarted successfully"})
}

// join fügt den aktuellen Pfad mit dem neuen Knoten zusammen
func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}
