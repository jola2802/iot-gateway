package webui

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	MQTT "github.com/mochi-mqtt/server/v2"
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
	store.Options(sessions.Options{
		MaxAge: int(1 * time.Minute),
		Secure: true,
	})
	r.Use(sessions.Sessions("mysession", store))

	// Store the db connection and mqtt server in the context, so it can be accessed in route handlers
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

// showRoutingPage shows the routing page
func showRoutingPage(c *gin.Context) {
	c.HTML(http.StatusOK, "data-forwarding.html", nil)
}

func StopWebUI() {
	server.Close()
}
