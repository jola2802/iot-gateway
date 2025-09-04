package webui

import (
	"crypto/rand"
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"time"

	"net/http/pprof"

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
var stateConnection bool
var num_images_db int

// Main function to start the web server
func Main(db *sql.DB, serverF *MQTT.Server) {
	// 0) set num_images_db from env
	num_images_db, _ = strconv.Atoi(os.Getenv("NUM_IMAGES_DB"))
	if num_images_db == 0 {
		num_images_db = 100 //default value
	}
	server = serverF

	if db == nil {
		logrus.Fatal("Database connection is not initalized.")
	}

	// Lade die Konfiguration
	config, err := loadConfigFromEnv()
	if err != nil {
		logrus.Fatal("Failed to load config: ", err)
	}

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	randomKey := make([]byte, 32)
	_, err = rand.Read(randomKey)
	if err != nil {
		logrus.Fatal("Failed to generate random key: ", err)
	}

	store := cookie.NewStore(randomKey)

	// Cookie-Optionen für bessere Kompatibilität zwischen localhost und IP-Adressen
	config, err = loadConfigFromEnv()
	if err != nil {
		logrus.Warnf("Fehler beim Laden der Konfiguration: %v", err)
	}
	isSecure := config != nil && config.WebUI.UseHTTPS

	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 Tage
		HttpOnly: true,
		Secure:   isSecure, // Automatisch basierend auf HTTPS-Konfiguration
		SameSite: http.SameSiteLaxMode,
		// Domain wird absichtlich NICHT gesetzt, damit es mit localhost UND IP funktioniert
	})

	sessionName := "idpm-gateway-session"
	r.Use(sessions.Sessions(sessionName, store))

	// Store the db connection and mqtt server in the context, so it can be accessed in route handlers
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("server", server)
		c.Next()
	})

	// Define routes
	setupRoutes(r)

	// Starte zuerst den pprof Server
	go func() {
		mux := http.NewServeMux()
		// Registriere alle pprof-Endpunkte
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		logrus.Info("Starting pprof server on port 6060")
		if err := http.ListenAndServe(":6060", mux); err != nil {
			logrus.Error("pprof server failed: ", err)
		}
	}()

	// Warte kurz, damit der pprof-Server Zeit hat zu starten
	time.Sleep(time.Second)

	// Starte dann den Hauptserver
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

func StopWebUI() {
	server.Close()
}
