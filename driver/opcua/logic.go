package opcua

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/gopcua/opcua/uatest"
	MQTT "github.com/mochi-mqtt/server/v2"
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
	ID              string      `json:"id"`
	Type            string      `json:"type"`
	Name            string      `json:"name"`
	Address         string      `json:"address"`
	SecurityMode    string      `json:"securityMode,omitempty"`   // Only for OPC UA
	SecurityPolicy  string      `json:"securityPolicy,omitempty"` // Only for OPC UA
	Datapoint       []Datapoint `json:"datapoints,omitempty"`     // Only for S7
	DataNode        []DataNode  `json:"dataNodes,omitempty"`      // Only for OPC UA
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

type DataNode struct {
	Name string `json:"name"`
	Node string `json:"node"`
}

// Run runs the OPC-UA client and MQTT publisher.
func Run(device DeviceConfig, db *sql.DB, stopChan chan struct{}, server *MQTT.Server) error {

	// Zertifikate und Schlüssel für den OPC-UA-Client setzen
	if device.SecurityPolicy != "None" && device.SecurityPolicy != "none" {
		device.CertFile = "./server.crt" // Setze den Pfad zu deinem Zertifikat
		device.KeyFile = "./server.key"  // Setze den Pfad zu deinem Schlüssel
	}

	// Optionen für den OPC-UA Client festlegen
	clientOpts, err := clientOptsFromFlags(device)
	if err != nil {
		logrus.Errorf("OPC-UA: Error creating client options for device %v: %v", device.Name, err)
		return fmt.Errorf("failed to create client options for device %v: %v", device.Name, err)
	}

	client, _ := opcua.NewClient(device.Address, clientOpts...)
	ctx := context.Background()

	// Retry-Logik: Versuche alle 10 Sekunden, die Verbindung aufzubauen
	retryInterval := 10 * time.Second
	for {
		err = client.Connect(ctx)
		if err != nil {
			logrus.Errorf("OPC-UA: Fehler beim Verbinden mit Gerät %v: %v. Versuche in %v erneut...", device.Name, err, retryInterval)
			// Prüfe, ob ein Stop-Request empfangen wurde
			select {
			case <-stopChan:
				logrus.Infof("OPC-UA: Stop-Request erhalten. Verbindungsversuch für Gerät %v abgebrochen.", device.Name)
				return fmt.Errorf("connection aborted for device %v", device.Name)
			case <-time.After(retryInterval):
				continue
			}
		}
		// Verbindung erfolgreich!
		logrus.Infof("OPC-UA: Erfolgreich mit Gerät %v verbunden.", device.Name)
		break
	}

	// Device zu OPC-UA Device Map hinzufügen
	addOpcuaClient(device.ID, client)

	// Daten vom OPC-UA-Client sammeln und veröffentlichen
	err = collectAndPublishData(device, client, stopChan, server)
	if err != nil {
		return err
	}
	defer client.Close(ctx)
	return nil
}

// getSecurityMode converts a security mode string to a ua.MessageSecurityMode type.
func getSecurityMode(mode string) ua.MessageSecurityMode {
	switch mode {
	case "None", "none":
		return ua.MessageSecurityModeNone
	case "Sign", "sign":
		return ua.MessageSecurityModeSign
	case "SignAndEncrypt", "signandencrypt":
		return ua.MessageSecurityModeSignAndEncrypt
	default:
		return ua.MessageSecurityModeNone
	}
}

// collectAndPublishData collects and publishes data from an OPC-UA client to an MQTT broker.
func collectAndPublishData(device DeviceConfig, client *opcua.Client, stopChan chan struct{}, server *MQTT.Server) error {
	dataNodes := device.DataNode

	sleeptime := time.Duration(time.Duration(device.AcquisitionTime) * time.Millisecond)
	maxAttempts := 1000 // Maximale Anzahl von Versuchen
	attempts := 0       // Zählt die Fehlversuche

	for {
		select {
		case <-stopChan:
			return nil
		default:
			data, err := ReadData(client, dataNodes)
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

			convData, err := ConvData(client, data, dataNodes)
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

			err = pubData(convData, device.Name, device.ID, server)
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

func clientOptsFromFlags(device DeviceConfig) ([]opcua.Option, error) {
	opts := []opcua.Option{}

	// Sicherheitsmodus und -richtlinie setzen
	securityMode := getSecurityMode(device.SecurityMode)
	securityPolicy := getSecurityPolicy(device.SecurityPolicy)
	opts = append(opts, opcua.SecurityMode(securityMode))
	opts = append(opts, opcua.SecurityPolicy(securityPolicy))

	// Falls ein Zertifikat benötigt wird (Sicherheitsmodus ungleich None):
	if securityMode != ua.MessageSecurityModeNone {
		var cert tls.Certificate
		var err error
		// Falls in der Konfiguration bereits Zertifikatspfad und Schlüsselpfad hinterlegt sind,
		// versuchen wir, diese zu laden. Andernfalls wird automatisch ein neues Zertifikat generiert.
		if device.CertFile != "" && device.KeyFile != "" {
			cert, err = tls.LoadX509KeyPair(device.CertFile, device.KeyFile)
			if err == nil {
				logrus.Warnf("failed to load provided certificate and key for device %v: %v; generating new certificate", device.Name, err)
				cert, err = generateNewCertificateForDevice(device)
				if err != nil {
					return nil, fmt.Errorf("failed to generate new RSA certificate for device %v: %v", device.Name, err)
				}
			}
		} else {
			cert, err = generateNewCertificateForDevice(device)
			if err != nil {
				return nil, fmt.Errorf("failed to generate new RSA certificate for device %v: %v", device.Name, err)
			}
		}

		// Prüfe, ob das Zertifikat gültig im PEM-Format vorliegt.
		// block, _ := pem.Decode(cert.Certificate[0])
		// if block == nil || block.Type != "CERTIFICATE" {
		// 	return nil, fmt.Errorf("malformed certificate: PEM block invalid")
		// }

		// Überprüfe, ob der private Schlüssel vom Typ *rsa.PrivateKey ist.
		privateKey, ok := cert.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			logrus.Warnf("unexpected private key type for device %v; generating new RSA certificate", device.Name)
			cert, err = generateNewCertificateForDevice(device)
			if err != nil {
				return nil, fmt.Errorf("failed to generate new RSA certificate for device %v: %v", device.Name, err)
			}
			privateKey, ok = cert.PrivateKey.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("unexpected type of generated private key for device %v, expected *rsa.PrivateKey", device.Name)
			}
		}

		opts = append(opts, opcua.PrivateKey(privateKey), opcua.Certificate(cert.Certificate[0]))
	}

	// Authentifizierungsmethode wählen
	if device.Username == "" && device.Password == "" {
		opts = append(opts, opcua.AuthAnonymous())
	} else {
		opts = append(opts, opcua.AuthUsername(device.Username, device.Password))
	}

	return opts, nil
}

func getSecurityPolicy(policy string) string {
	switch policy {
	case "None", "none":
		return ua.SecurityPolicyURINone
	case "Basic128Rsa15", "basic128rsa15":
		return ua.SecurityPolicyURIBasic128Rsa15
	case "Basic256", "basic256":
		return ua.SecurityPolicyURIBasic256
	case "Basic256Sha256", "basic256sha256":
		return ua.SecurityPolicyURIBasic256Sha256
	default:
		return ua.SecurityPolicyURINone
	}
}

// getLocalIP ermittelt die erste nicht-Loopback IPv4-Adresse des Hosts.
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		// Prüfen, ob es sich um eine IP-Adresse handelt und ob sie nicht Loopback ist
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ip4.String(), nil
			}
		}
	}
	return "", fmt.Errorf("keine nicht-Loopback IP-Adresse gefunden")
}

// generateNewCertificateForDevice generiert ein neues RSA‑Zertifikat basierend auf der lokalen IP des Hosts.
func generateNewCertificateForDevice(device DeviceConfig) (tls.Certificate, error) {
	localIP, err := getLocalIP()
	if err != nil || localIP == "" {
		localIP = "localhost"
	}
	certPEM, keyPEM, err := uatest.GenerateCert(localIP, 2048, 24*time.Hour)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate RSA certificate: %v", err)
	}
	// Optional: Ausgabe prüfen (zum Debuggen)
	fmt.Println(string(certPEM))
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to parse generated RSA certificate: %v", err)
	}
	return cert, nil
}
