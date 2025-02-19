package mqtt_broker

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"iot-gateway/logic"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/glebarez/go-sqlite" // Import für SQLite
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

var server *MQTT.Server
var once sync.Once

type ListenerConfig struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Type    string `json:"type"`
	TLS     bool   `json:"tls"`
}

type Config struct {
	Listeners []ListenerConfig `json:"listeners"`
}

// StartBroker initialisiert den Broker synchron und startet den blockierenden Serve-Loop asynchron.
func StartBroker(db *sql.DB) *MQTT.Server {
	once.Do(func() {
		// Synchronously initialisieren und das Serverobjekt zuweisen.
		server = startBrokerInstance(db)

		// Starten des blockierenden Serve-Loops in einer eigenen Goroutine.
		go func() {
			if err := server.Serve(); err != nil {
				logrus.Fatalf("MQTT-Broker: Serve error: %v", err)
			}
		}()
	})
	return server
}

// startBrokerInstance erstellt und konfiguriert den MQTT Broker und gibt ihn synchron zurück.
func startBrokerInstance(db *sql.DB) *MQTT.Server {
	// Verwaltung des Admin-Zugangs und der Benutzer-/Driver-Zugriffe.
	if err := logic.AddAdminUser(db); err != nil {
		logrus.Fatal("Failed to manage Admin access: ", err)
	}
	if err := logic.WebUIAccessManagement(db); err != nil {
		logrus.Fatal("Failed to manage Web-UI access: ", err)
	}
	// if err := logic.DriverAccessManagement(db); err != nil {
	// 	logrus.Fatal("Failed to manage driver access: ", err)
	// }

	// Authentifizierungsdaten aus der Datenbank laden.
	authData, err := loadAuthDataFromDB(db)
	if err != nil {
		logrus.Fatal("Failed to load auth data from the database: ", err)
	}

	// Generierung eines selbstsignierten Zertifikats für TLS.
	cert, err := logic.GenerateSelfSignedCert()
	if err != nil {
		logrus.Fatalf("MQTT-Broker: Failed to generate self-signed certificate: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Erzeugen des neuen MQTT-Servers.
	s := MQTT.New(&MQTT.Options{
		InlineClient: true,
	})

	// Hinzufügen des Authentifizierungs-Hooks mit den geladenen Daten.
	if err := s.AddHook(new(auth.Hook), &auth.Options{Data: authData}); err != nil {
		logrus.Fatal("MQTT-Broker: Failed to add auth hook: ", err)
	}

	// Listener anhand der Konfiguration hinzufügen.
	if err := createListeners(s, tlsConfig); err != nil {
		logrus.Fatal("MQTT-Broker: Error adding listeners: ", err)
	}

	// Signal-Handler in einer separaten Goroutine, um bei SIGINT/SIGTERM den Broker (und die DB) sauber zu schließen.
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		s.Close()
		db.Close()
		logrus.Info("MQTT Broker shutdown triggered by signal")
	}()

	return s
}

func loadConfig(filename string) (*Config, error) {
	configBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func createListeners(server *MQTT.Server, tlsConfig *tls.Config) error {
	config, err := loadConfig("config.json")
	if err != nil {
		return err
	}

	for _, listener := range config.Listeners {
		var l listeners.Listener

		switch listener.Type {
		case "tcp":
			l = listeners.NewTCP(listeners.Config{
				ID:        listener.ID,
				Address:   listener.Address,
				TLSConfig: getTLSConfig(listener.TLS, tlsConfig),
			})
		case "websocket":
			l = listeners.NewWebsocket(listeners.Config{
				ID:        listener.ID,
				Address:   listener.Address,
				TLSConfig: getTLSConfig(listener.TLS, tlsConfig),
			})
		case "http":
			l = listeners.NewHTTPStats(
				listeners.Config{
					ID:        listener.ID,
					Address:   listener.Address,
					TLSConfig: getTLSConfig(listener.TLS, tlsConfig),
				}, server.Info,
			)
		default:
			logrus.Warn("Unknown listener type: ", listener.Type)
			continue
		}

		err := server.AddListener(l)
		if err != nil {
			logrus.Fatal("Error adding listener: ", err)
		}
	}

	return nil
}

func getTLSConfig(tlsRequired bool, tlsConfig *tls.Config) *tls.Config {
	if tlsRequired {
		return tlsConfig
	}
	return nil
}

// loadAuthDataFromDB lädt Authentifizierungsdaten aus der SQLite-Datenbank
func loadAuthDataFromDB(db *sql.DB) ([]byte, error) {
	// Hole alle Benutzerinformationen
	rows, err := db.Query("SELECT username, password, allow FROM auth")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var auth []map[string]interface{}
	var acl []map[string]interface{}

	for rows.Next() {
		var username, password string
		var allow bool
		if err := rows.Scan(&username, &password, &allow); err != nil {
			return nil, err
		}

		// Auth-Daten sammeln
		auth = append(auth, map[string]interface{}{
			"username": username,
			"password": password,
			"allow":    allow,
		})

		// Jetzt Zugriffsrechte (ACL) für diesen Benutzer holen
		aclRows, err := db.Query("SELECT topic, permission FROM acl WHERE username = ?", username)
		if err != nil {
			return nil, err
		}
		defer aclRows.Close()

		filters := make(map[string]int)
		for aclRows.Next() {
			var topic string
			var permission int
			if err := aclRows.Scan(&topic, &permission); err != nil {
				return nil, err
			}
			filters[topic] = permission
		}

		// ACL-Daten sammeln
		acl = append(acl, map[string]interface{}{
			"username": username,
			"filters":  filters,
		})
	}

	// Erstelle die komplette Datenstruktur
	data := map[string]interface{}{
		"auth": auth,
		"acl":  acl,
	}

	// Konvertiere die Daten in YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}

	return yamlData, nil
}

// StopBroker stoppt den MQTT Broker
func StopBroker() {
	if server != nil {
		server.Close()
		logrus.Info("MQTT Broker stopped successfully.")
	} else {
		logrus.Info("MQTT Broker is not running.")
	}
}

// RestartBroker startet den MQTT-Broker neu
func RestartBroker(db *sql.DB) {
	StopBroker()                        // Stoppt den Broker
	once = sync.Once{}                  // Setzt die `once` Variable zurück, um erneut starten zu können
	time.Sleep(1000 * time.Millisecond) // Wartet 2 Sekunden
	StartBroker(db)                     // Startet den Broker neu
	logrus.Info("MQTT Broker restarted successfully.")
}
