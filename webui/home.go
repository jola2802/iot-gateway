package webui

import (
	"database/sql"
	"net/http"
	"runtime"

	"iot-gateway/logic"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/sirupsen/logrus"
)

// showDashboard shows the dashboard page (main-page)
func showHomeContent(c *gin.Context) {
	//Call generate token function
	token := generateToken()

	// c.HTML(http.StatusOK, "index.html", nil)
	c.HTML(http.StatusOK, "index.html", gin.H{
		"token": token,
	})
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
