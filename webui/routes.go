package webui

import (
	"io"
	"net/http"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"
)

// setupRoutes sets up the routes for the web interface.
//
// This function configures the routes for serving static files, loading HTML templates,
// and handling requests for public and protected routes.
//
// Example:
//
//	r := gin.Default()
//	setupRoutes(r)
func setupRoutes(r *gin.Engine) {
	// Registriere die benutzerdefinierte Funktion
	r.SetFuncMap(template.FuncMap{
		"getPermissionText": getPermissionText,
	})

	// Serving static files (CSS and JS)
	r.Static("/assets/", "./webui/assets/")
	// r.StaticFS("/static", http.FS(staticFS))

	// Loading HTML templates
	r.LoadHTMLGlob("webui/templates/*")

	// Public routes
	r.GET("/login", showLoginPage)
	r.POST("/login", performLogin)
	r.GET("/logout", logout)

	// Protected routes
	authorized := r.Group("/")
	authorized.Use(AuthRequired)
	{
		/// #############
		// NEUE ROUTEN HIER
		authorized.GET("/api/getBrokerUsers", getBrokerUsers)
		authorized.GET("/api/getBrokerUser/:username", getBrokerUser)
		authorized.GET("/api/getBrokerLogin", getBrokerLogin)
		authorized.GET("/api/profile", getProfile)
		authorized.POST("/api/changePassword", changePassword)
		authorized.GET("/api/getDevices", getDevices)
		authorized.GET("/api/getDevice/:device_id", getDevice)
		authorized.GET("/api/influxdb", getInfluxDB)
		/// #############
		// Show the pages
		authorized.GET("/", showDashboard)
		authorized.GET("/home", showHomeContent)
		authorized.GET("/devices", showDevicesPage) // show devices page
		authorized.GET("/historical-data", showHistoricalDataPage)
		authorized.GET("/data-forwarding", showRoutingPage)
		authorized.GET("/broker", showBrokerPage)
		authorized.GET("/node-red", showNodeRedPage)
		authorized.GET("/profile", showProfilePage)
		authorized.GET("/settings", showSettingsPage)
		// ############

		authorized.GET("/ws/broker/status", brokerStatusWebSocket)
		authorized.POST("/restart", restartGatewayHandler)

		authorized.POST("/profile", updateProfile)

		// Data Routes
		authorized.GET("/routes", getRoutes)
		authorized.GET("/routes/:routeId", getRoutesById)
		authorized.DELETE("/routes/:routeId", deleteRoute)
		authorized.GET("/listdevices", getlistDevices)
		authorized.POST("/saveRouteConfig", SaveRouteConfig)

		//node-red

		// Device routes with RESTful methods
		authorized.GET("/devices/:deviceName", getDeviceStatus)          // Get a single device status
		authorized.PUT("/devices/:deviceName", updateDevice)             // Update a single device
		authorized.PUT("/devices/state/:deviceName", updateDeviceStatus) // Update device status
		authorized.POST("/devices", addDevice)                           // Add a new device
		authorized.DELETE("/devices/:deviceName", deleteDevice)          // Delete a device
		authorized.GET("/browseNodes/:deviceName", browseNodes)          // Get device attributes

		//websocket for device status and data
		authorized.GET("/ws/deviceStatus", deviceStatusWebSocket)
		authorized.GET("/ws/deviceData", deviceDataWebSocket)

		// Broker routes
		authorized.POST("/settings", updateBrokerSettings)
		authorized.POST("/updateUser", updateBrokerUser)
		authorized.DELETE("/users/:username", deleteUserHandler)
		authorized.PUT("/users/:username", updateUserHandler)
		authorized.POST("/users", createUserHandler)

		// Features
		authorized.GET("/listopcuadevices", listDevices) // list devices
		authorized.GET("/latest-image/:deviceName", latestImage)
		authorized.GET("/listprocesses", listImgCapProcesses)
		authorized.POST("/start-image-process/:id", startImageProcess)
		authorized.GET("/stop-image-process/:id", stopImageProcess)
		authorized.POST("/delete-image-process/:id", deleteImageProcess)
		authorized.GET("/add-image-process", addImageProcess)

		authorized.GET("/browse-nodes/:deviceName", browseNodes)
		authorized.GET("/node-red-url", getNodeRedURL)
	}
}

// getPermissionText wandelt eine Permission-Zahl in einen Text um
func getPermissionText(permission int) string {
	switch permission {
	case 0:
		return "NA"
	case 1:
		return "R"
	case 2:
		return "W"
	case 3:
		return "R/W"
	default:
		return "unknown"
	}
}

func showNodeRedPage(c *gin.Context) {
	c.HTML(http.StatusOK, "node-red.html", nil)
}

func showBrokerPage(c *gin.Context) {
	c.HTML(http.StatusOK, "broker.html", nil)
}

func getInfluxDB(c *gin.Context) {
	resp, err := http.Get("http://127.0.0.1:8086")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch InfluxDB UI"})
		return
	}
	defer resp.Body.Close()

	// HTML-Inhalt aus der Antwort lesen
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from InfluxDB"})
		return
	}

	// Manipuliere das <base href> im HTML
	modifiedBody := strings.ReplaceAll(string(body), `<base href="/">`, `<base href="/api/influxdb/">`)

	// Geänderten HTML-Inhalt zurückgeben
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(modifiedBody))
}
