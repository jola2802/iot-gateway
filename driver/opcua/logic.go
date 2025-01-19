package opcua

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"database/sql"
	"fmt"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/gopcua/opcua/uatest"
	"github.com/sirupsen/logrus"
)

// Konfigurationsstruktur
type Config struct {
	Devices []DeviceConfig `json:"devices"`
	// MqttBroker MqttConfig     `json:"mqttBroker"`
}

// MQTT-Konfigurationsstruktur
type MqttConfig struct {
	Broker     string `json:"broker"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	CACert     string `json:"caCert,omitempty"`
	ClientCert string `json:"clientCert,omitempty"`
	ClientKey  string `json:"clientKey,omitempty"`
}

// Gerätekonfigurationsstruktur

type DeviceConfig struct {
	Type            string      `json:"type"`
	Name            string      `json:"name"`
	Address         string      `json:"address"`
	SecurityMode    string      `json:"securityMode,omitempty"`   // Only for OPC UA
	SecurityPolicy  string      `json:"securityPolicy,omitempty"` // Only for OPC UA
	Datapoint       []Datapoint `json:"datapoints,omitempty"`     // Only for S7
	DataNode        []string    `json:"dataNodes,omitempty"`      // Only for OPC UA
	AcquisitionTime int         `json:"acquisitionTime"`
	CertFile        string      `json:"certificate,omitempty"` // Only for OPC UA
	KeyFile         string      `json:"key,omitempty"`         // Only for OPC UA
	Username        string      `json:"username,omitempty"`    // Only for OPC UA
	Password        string      `json:"password,omitempty"`    // Only for OPC UA
	Rack            int         `json:"rack,omitempty"`        // Only for S7
	Slot            int         `json:"slot,omitempty"`        // Only for S7
}

type Datapoint struct {
	Name     string `json:"name"`
	Datatype string `json:"datatype"`
	Address  string `json:"address"`
}

// GetSecurityMode converts a security mode string to a ua.MessageSecurityMode type.
//
// Args:
//
//	mode (string): The security mode string.
//
// Returns:
//
//	ua.MessageSecurityMode: The corresponding ua.MessageSecurityMode type.
func GetSecurityMode(mode string) ua.MessageSecurityMode {
	switch mode {
	case "None":
		return ua.MessageSecurityModeNone
	case "Sign":
		return ua.MessageSecurityModeSign
	case "SignAndEncrypt":
		return ua.MessageSecurityModeSignAndEncrypt
	default:
		return ua.MessageSecurityModeNone
	}
}

// collectAndPublishData collects and publishes data from an OPC-UA client to an MQTT broker.
//
// Args:
//
//	device (DeviceConfig): The device configuration.
//	client (*opcua.Client): The OPC-UA client.
//	stopChan (chan struct{}): The stop channel.
//
// Example usage of collectAndPublishData:
//
//	device := DeviceConfig{Name: "MyDevice", DataNodes: []string{"node1", "node2"}}
//	client, _ := InitClient("opc.tcp://localhost:4840", ua.MessageSecurityModeNone, "MySecurityPolicy")
//	stopChan := make(chan struct{})
//	collectAndPublishData(device, client, stopChan)
func collectAndPublishData(device DeviceConfig, client *opcua.Client, stopChan chan struct{}) error {
	nodes := device.DataNode
	sleeptime := time.Duration(time.Duration(device.AcquisitionTime) * time.Millisecond)
	maxAttempts := 10 // Maximale Anzahl von Versuchen
	attempts := 0     // Zählt die Fehlversuche

	for {
		select {
		case <-stopChan:
			return nil
		default:
			data, err := ReadData(client, nodes)
			if err != nil {
				logrus.Errorf("OPC-UA: Error reading data from %v: %s", device.Name, err)
				attempts++
				if attempts >= maxAttempts {
					logrus.Errorf("OPC-UA: Max attempts reached for reading data from %v. Stopping process.", device.Name)
					return fmt.Errorf("max attempts reached for reading data from %v", device.Name)
				}
				time.Sleep(5 * time.Second)
				continue
			}

			convData, err := ConvData(client, data, nodes)
			if err != nil {
				logrus.Errorf("OPC-UA: Error converting data from %v: %s", device.Name, err)
				attempts++
				if attempts >= maxAttempts {
					logrus.Errorf("OPC-UA: Max attempts reached for converting data from %v. Stopping process.", device.Name)
					return fmt.Errorf("max attempts reached for converting data from %v", device.Name)
				}
				time.Sleep(5 * time.Second)
				continue
			}

			err = pubData(convData, device.Name)
			if err != nil {
				logrus.Errorf("OPC-UA: Error publishing data from %v: %s", device.Name, err)
				attempts++
				if attempts >= maxAttempts {
					logrus.Errorf("OPC-UA: Max attempts reached for publishing data from %v. Stopping process.", device.Name)
					return fmt.Errorf("max attempts reached for publishing data from %v", device.Name)
				}
				time.Sleep(5 * time.Second)
				continue
			}

			// Wenn alles erfolgreich ist, setze die Anzahl der Versuche zurück
			attempts = 0
			time.Sleep(sleeptime)
		}
	}
}

// loadMqttConfigFromDB loads the MQTT configuration from the database.
//
// Args:
//
//	db (*sql.DB): The database connection.
//
// Returns:
//
//	*MqttConfig: The MQTT configuration.
//	error: Any error that occurred while loading the configuration.
//
// Example usage of loadMqttConfigFromDB:
//
//	db, _ := sql.Open("mysql", "user:password@tcp(localhost:3306)/database")
//	mqttConfig, _ := loadMqttConfigFromDB(db)
//	log.Println(mqttConfig)
func LoadMqttConfigFromDB(db *sql.DB, username string) (*MqttConfig, error) {
	var mqttConfig MqttConfig
	query := `
	SELECT bs.address AS broker, a.username, a.password
	FROM broker_settings bs
	JOIN auth a ON a.username = ?
	LIMIT 1
`
	row := db.QueryRow(query, username)
	err := row.Scan(&mqttConfig.Broker, &mqttConfig.Username, &mqttConfig.Password)
	if err != nil {
		logrus.Warnf("OPC-UA: could not load MQTT config from DB: %v", err)
		return nil, err
	}
	return &mqttConfig, nil
}

// Run runs the OPC-UA client and MQTT publisher.
//
// Args:
//
//	device (DeviceConfig): The device configuration.
//	db (*sql.DB): The database connection.
//	stopChan (chan struct{}): The stop channel.
func Run(device DeviceConfig, db *sql.DB, stopChan chan struct{}) error {
	// MQTT-Client initialisieren
	mqttConfig, err := LoadMqttConfigFromDB(db, device.Name)
	if err != nil {
		mqttConfig, err = LoadMqttConfigFromDB(db, "admin")
		if err != nil {
			logrus.Errorf("OPC-UA: Error loading MQTT config from DB: %v", err)
			return fmt.Errorf("failed to load MQTT config for device %v: %v", device.Name, err)
		}
	}
	initMqttClient(*mqttConfig)

	// OPC-UA-Client initialisieren
	// Zertifikate und Schlüssel für den OPC-UA-Client setzen
	if device.SecurityPolicy != "None" {
		device.CertFile = "server.crt" // Setze den Pfad zu deinem Zertifikat
		device.KeyFile = "server.key"  // Setze den Pfad zu deinem Schlüssel
	}

	// Optionen für den OPC-UA Client festlegen
	clientOpts, err := clientOptsFromFlags(device)
	if err != nil {
		logrus.Errorf("OPC-UA: Error creating client options for device %v: %v", device.Name, err)
		return fmt.Errorf("failed to create client options for device %v: %v", device.Name, err)
	}

	client, err := opcua.NewClient(device.Address, clientOpts...)
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		logrus.Errorf("OPC-UA: Error initializing OPC-UA client for device %v: %s", device.Name, err)
		return fmt.Errorf("failed to initialize OPC-UA client for device %v: %v", device.Name, err)
	}

	// Device zu OPC-UA Device Map hinzufügen
	addOpcuaClient(device.Name, client)

	// Start MQTT data update listener
	startMqttDataUpdateListener("data/opcua/update/#")

	// Daten vom OPC-UA-Client sammeln und veröffentlichen
	err = collectAndPublishData(device, client, stopChan)
	if err != nil {
		return err
	}
	defer client.Close(ctx)
	return nil
}

func clientOptsFromFlags(device DeviceConfig) ([]opcua.Option, error) {
	opts := []opcua.Option{}

	// Setze Sicherheitsmodus und -richtlinie
	securityMode := GetSecurityMode(device.SecurityMode)
	securityPolicy := device.SecurityPolicy
	opts = append(opts, opcua.SecurityMode(securityMode))
	opts = append(opts, opcua.SecurityPolicy(securityPolicy))

	// Lade Zertifikate, falls erforderlich
	if securityMode != ua.MessageSecurityModeNone && securityPolicy != "None" {
		if device.CertFile != "" && device.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(device.CertFile, device.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate and key: %v", err)
			}

			// Typüberprüfung des PrivateKeys, um sicherzustellen, dass es ein *rsa.PrivateKey ist
			privateKey, ok := cert.PrivateKey.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("unexpected type of private key, expected *rsa.PrivateKey")
			}

			opts = append(opts, opcua.PrivateKey(privateKey), opcua.Certificate(cert.Certificate[0]))
		} else {
			// Optional: Generiere ein Zertifikat, wenn keines bereitgestellt wird
			certPEM, keyPEM, err := uatest.GenerateCert(device.Address, 2048, 24*time.Hour)
			if err != nil {
				return nil, fmt.Errorf("failed to generate cert: %v", err)
			}

			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("failed to parse generated cert: %v", err)
			}

			privateKey, ok := cert.PrivateKey.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("unexpected type of generated private key, expected *rsa.PrivateKey")
			}

			opts = append(opts, opcua.PrivateKey(privateKey), opcua.Certificate(cert.Certificate[0]))
		}
	}

	// Authentifizierungsmethode wählen
	if device.Username == "" || device.Password == "" {
		opts = append(opts, opcua.AuthAnonymous())
	} else {
		opts = append(opts, opcua.AuthUsername(device.Username, device.Password))
	}

	return opts, nil
}
