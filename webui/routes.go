package webui

import (
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
)

var wsTokenStore = struct {
	sync.RWMutex
	tokens map[string]time.Time
}{
	tokens: make(map[string]time.Time),
}

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
	r.GET("/api/ws-broker-status", brokerStatusWebSocket)
	r.GET("/api/ws-device-data", deviceDataWebSocket)

	// Protected routes
	authorized := r.Group("/")
	authorized.Use(authRequired)
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
		authorized.POST("/api/query-data", queryDataHandler)
		authorized.POST("/api/get-measurements", getMeasurements)
		authorized.POST("/api/add-device", addDevice)
		authorized.PUT("/api/update-device/:device_id", updateDevice)
		authorized.DELETE("/api/delete-device/:device_id", deleteDevice)
		authorized.GET("/api/ws-token", generateToken)

		// WSS for dashboard data
		// authorized.GET("/api/ws-broker-status", brokerStatusWebSocket)
		// WSS for device data
		// authorized.GET("/api/ws-device-data", deviceDataWebSocket)

		/// #############
		// Show the pages
		authorized.GET("/", showDashboard)
		authorized.GET("/home", showDashboard)
		authorized.GET("/devices", showDevicesPage) // show devices page
		authorized.GET("/historical-data", showHistoricalDataPage)
		authorized.GET("/data-forwarding", showRoutingPage)
		authorized.GET("/broker", showBrokerPage)
		authorized.GET("/node-red", showNodeRedPage)
		authorized.GET("/profile", showProfilePage)
		authorized.GET("/settings", showSettingsPage)
		// ############

		authorized.POST("/restart", restartGatewayHandler)

		authorized.POST("/profile", updateProfile)

		// Data Routes
		authorized.GET("/routes", getRoutes)
		authorized.GET("/routes/:routeId", getRoutesById)
		authorized.DELETE("/routes/:routeId", deleteRoute)
		authorized.GET("/listdevices", getlistDevices)
		authorized.POST("/saveRouteConfig", SaveRouteConfig)

		//node-red

		// Device routes
		// authorized.GET("/devices/:deviceName", getDeviceStatus)          // Get a single device status
		// authorized.PUT("/devices/:deviceName", updateDevice)             // Update a single device
		// authorized.PUT("/devices/state/:deviceName", updateDeviceStatus) // Update device status
		// authorized.POST("/devices", addDevice)                           // Add a new device
		// authorized.DELETE("/devices/:deviceName", deleteDevice) // Delete a device
		authorized.GET("/browseNodes/:deviceName", browseNodes) // Get device attributes

		//websocket for device status and data
		// authorized.GET("/ws/deviceStatus", deviceStatusWebSocket)
		// authorized.GET("/ws/deviceData", deviceDataWebSocket)

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
