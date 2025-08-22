package webui

import (
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
	// r.POST("/api/img-process", captureImage)

	// Files
	r.POST("/api/save-image", saveImage)

	r.GET("/api/get-influx-devices", getInfluxDevices)
	// Protected routes
	authorized := r.Group("/")
	authorized.Use(AuthRequired)
	{
		/// Broker Routes
		authorized.GET("/api/getBrokerUsers", getAllBrokerUsers)
		authorized.GET("/api/getBrokerUser/:username", getBrokerUser)
		authorized.GET("/api/getBrokerLogin", getBrokerLogin)
		authorized.POST("/api/add-broker-user", addBrokerUser)
		authorized.PUT("/api/update-broker-user/:username", addBrokerUser)
		authorized.DELETE("/api/delete-broker-user/:username", deleteBrokerUser)

		// Logs Routes
		authorized.GET("/api/logs", grabLogs)
		authorized.POST("/api/logs/clear", clearLogs)

		// Devices Routes
		authorized.GET("/api/getDevices", getDevices)
		authorized.GET("/api/getDevice/:device_id", getDevice)
		authorized.POST("/api/add-device", addDevice)
		authorized.POST("/api/update-device/:device_id", updateDevice)
		authorized.DELETE("/api/delete-device/:device_id", deleteDevice)
		authorized.GET("/api/ws-token", generateToken)
		authorized.POST("/api/restart-device/:device_id", restartDevice)
		authorized.GET("/api/browseNodes/:deviceID", browseNodes)

		// Historical Data Routes
		authorized.POST("/api/get-measurements", getMeasurements)
		authorized.POST("/api/query-data", queryDataHandler)

		// Data Forwarding Routes
		authorized.GET("/api/images", getImages)
		authorized.GET("/api/images/download", downloadImagesAsZip)

		// Image Capture Process Routes
		authorized.GET("/api/image-capture-processes", getImageCaptureProcesses)
		authorized.GET("/api/image-capture-processes/:id", getImageCaptureProcess)
		authorized.POST("/api/image-capture-processes", addImageCaptureProcess)
		authorized.PUT("/api/image-capture-processes/:id", updateImageCaptureProcess)
		authorized.DELETE("/api/image-capture-processes/:id", deleteImageCaptureProcess)
		authorized.POST("/api/image-capture-processes/:id/start", startImageCaptureProcess)
		authorized.POST("/api/image-capture-processes/:id/stop", stopImageCaptureProcess)
		authorized.POST("/api/image-capture-processes/:id/execute", executeImageCaptureProcess)

		// Public API Routes für externe Trigger (Node-RED)
		r.POST("/api/image-capture-processes/:id/trigger", executeImageCaptureProcess)
		r.POST("/api/image-capture-processes/:id/start-external", startImageCaptureProcess)
		r.POST("/api/image-capture-processes/:id/stop-external", stopImageCaptureProcess)

		// Profile Routes
		authorized.GET("/api/profile", getProfile)
		authorized.PUT("/api/profile", updateProfile)
		authorized.PUT("/api/changePassword", changePassword)

		// Browse Nodes
		authorized.GET("/api/browse-nodes/:deviceID", browseNodes)

		// Settings Routes
		authorized.POST("/api/restart", restartGatewayHandler)

		// Show pages Routes
		authorized.GET("/", showDashboard)
		authorized.GET("/home", showDashboard)
		authorized.GET("/devices", showDevicesPage)
		authorized.GET("/historical-data", showHistoricalDataPage)
		authorized.GET("/data-forwarding", showRoutingPage)
		authorized.GET("/image-capture", showImageCapturePage)
		authorized.GET("/broker", showBrokerPage)
		authorized.GET("/profile", showProfilePage)
		authorized.GET("/settings", showSettingsPage)
	}
}
