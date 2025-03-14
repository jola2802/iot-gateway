package webui

import (
	"net/http"
	"sync"
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
	// Serving static files (CSS and JS)
	r.Static("/assets/", "./webui/assets/")

	// Loading HTML templates
	r.LoadHTMLGlob("webui/templates/*")

	// Public routes
	r.GET("/login", showLoginPage)
	r.POST("/login", performLogin)
	r.GET("/logout", logout)
	r.GET("/api/ws-broker-status", brokerStatusWebSocket)
	r.GET("/api/ws-device-data", deviceDataWebSocket)
	r.POST("/api/img-process", captureImage)

	// Files
	r.POST("/api/save-image", saveImage)

	r.GET("/api/get-influx-devices", getInfluxDevices)
	// Protected routes
	authorized := r.Group("/")
	authorized.Use(AuthRequired)
	{
		/// #############
		// NEUE ROUTEN HIER
		authorized.GET("/api/getBrokerUsers", getAllBrokerUsers)
		authorized.GET("/api/getBrokerUser/:username", getBrokerUser)
		authorized.GET("/api/getBrokerLogin", getBrokerLogin)
		authorized.POST("/api/add-broker-user", addBrokerUser)
		authorized.PUT("/api/update-broker-user/:username", addBrokerUser)
		authorized.DELETE("/api/delete-broker-user/:username", deleteBrokerUser)

		// Logs - Neue Route für das Abrufen der Logs
		authorized.GET("/api/logs", grabLogs)
		authorized.POST("/api/logs/clear", clearLogs)

		// Devices
		authorized.GET("/api/getDevices", getDevices)
		authorized.GET("/api/getDevice/:device_id", getDevice)
		authorized.POST("/api/add-device", addDevice)
		authorized.PUT("/api/update-device/:device_id", updateDevice)
		authorized.DELETE("/api/delete-device/:device_id", deleteDevice)
		authorized.GET("/api/ws-token", generateToken)
		authorized.POST("/api/restart-device/:device_id", restartDevice)
		authorized.GET("/api/browseNodes/:deviceID", browseNodes)

		// Historical Data
		authorized.POST("/api/get-measurements", getMeasurements)
		authorized.POST("/api/query-data", queryDataHandler)

		// Data Forwarding
		authorized.GET("/api/get-node-red-url", getNodeRedURL)
		authorized.GET("/api/images", getImages)
		authorized.GET("/api/images/download", downloadImagesAsZip)

		// Profile
		authorized.GET("/api/profile", getProfile)
		authorized.PUT("/api/profile", updateProfile)
		authorized.PUT("/api/changePassword", changePassword)

		// Settings

		authorized.POST("/api/restart", restartGatewayHandler)

		/// #############
		// Show pages
		authorized.GET("/", showDashboard)
		authorized.GET("/home", showDashboard)
		authorized.GET("/devices", showDevicesPage) // show devices page
		authorized.GET("/historical-data", showHistoricalDataPage)
		authorized.GET("/data-forwarding", showRoutingPage)
		authorized.GET("/broker", showBrokerPage)
		authorized.GET("/node-red", showNodeRedPage)
		authorized.GET("/profile", showProfilePage)
		authorized.GET("/settings", showSettingsPage)

		// authorized.GET("/files", showFilesPage)
		// ############

		// Data Routes
		authorized.GET("/listdevices", getlistDevices)

		//node-red

		// Device routes
		// authorized.GET("/devices/:deviceName", getDeviceStatus)          // Get a single device status
		// authorized.PUT("/devices/:deviceName", updateDevice)             // Update a single device
		// authorized.PUT("/devices/state/:deviceName", updateDeviceStatus) // Update device status
		// authorized.POST("/devices", addDevice)                           // Add a new device
		// authorized.DELETE("/devices/:deviceName", deleteDevice) // Delete a device

		//websocket for device status and data
		// authorized.GET("/ws/deviceStatus", deviceStatusWebSocket)
		// authorized.GET("/ws/deviceData", deviceDataWebSocket)

		// Broker routes
		// authorized.POST("/settings", updateBrokerSettings)
		// authorized.POST("/updateUser", updateBrokerUser)
		// authorized.DELETE("/users/:username", deleteUserHandler)
		// authorized.PUT("/users/:username", updateUserHandler)
		// authorized.POST("/users", createUserHandler)

		// Features
		// authorized.GET("/latest-image/:deviceName", latestImage)
		// authorized.GET("/add-image-process", addImageProcess)

		authorized.GET("/browse-nodes/:deviceID", browseNodes)

	}
}

func showNodeRedPage(c *gin.Context) {
	c.HTML(http.StatusOK, "node-red.html", nil)
}
