package opcua

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

// clientOptsFromFlags erstellt die OPC UA Optionen und prüft, ob Zertifikat und Schlüssel vorhanden sind.
func clientOptsFromFlags(device DeviceConfig, db *sql.DB) ([]opcua.Option, error) {
	opts := []opcua.Option{}

	// Setze Basis-Sicherheitsoptionen
	opts = append(opts,
		opcua.SecurityMode(getSecurityMode(device.SecurityMode)),
		opcua.SecurityPolicy(getSecurityPolicy(device.SecurityPolicy)),
	)

	// Authentifizierung konfigurieren
	// Priorität: Username/Password > Anonyme Authentifizierung
	if device.Username != "" && device.Password != "" {
		opts = append(opts, opcua.AuthUsername(device.Username, device.Password))
		logrus.Infof("OPC-UA: Using username authentication for device %s (user: %s)", device.Name, device.Username)
	} else {
		opts = append(opts, opcua.AuthAnonymous())
		logrus.Infof("OPC-UA: Using anonymous authentication for device %s", device.Name)
	}

	// Bei SecurityMode "None" keine Zertifikate benötigt
	if getSecurityMode(device.SecurityMode) == ua.MessageSecurityModeNone {
		return opts, nil
	}

	// Definiere Pfade für Zertifikat und privaten Schlüssel
	certPath := "certificate-opcua/idpm_cert.pem" // Zertifikat im PEM-Format
	keyPath := "certificate-opcua/idpm_key.pem"   // Privater Schlüssel im PEM-Format

	var cert []byte
	var pk *rsa.PrivateKey

	// Falls eine der Dateien fehlt, Zertifikat und Schlüssel generieren und speichern
	if _, err := os.Stat(certPath); os.IsNotExist(err) || fileNotExists(keyPath) {

		c, err := generateCert()
		if err != nil {
			return nil, fmt.Errorf("failed to generate certificate: %v", err)
		}

		pk = c.PrivateKey.(*rsa.PrivateKey)

		cert = c.Certificate[0]

		// Speichere das Zertifikat und den privaten Schlüssel
		if err := os.WriteFile(certPath, cert, 0644); err != nil {
			return nil, fmt.Errorf("failed to save certificate: %v", err)
		}
		if err := os.WriteFile(keyPath, x509.MarshalPKCS1PrivateKey(pk), 0644); err != nil {
			return nil, fmt.Errorf("failed to save private key: %v", err)
		}
	}

	// Füge den privaten Schlüssel und das Zertifikat zu den OPC UA Optionen hinzu
	opts = append(opts,
		opcua.PrivateKeyFile(keyPath),
		opcua.CertificateFile(certPath),
	)

	return opts, nil
}

// fileNotExists prüft, ob eine Datei nicht vorhanden ist.
func fileNotExists(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// GenerateCert erzeugt ein neues selbstsigniertes Zertifikat sowie den privaten Schlüssel.
func generateCert() (*tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %s", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(5 * 365 * 24 * time.Hour) // 5 years

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Gateway Client"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	host := "urn:gateway:client"
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}
	if uri, err := url.Parse(host); err == nil {
		template.URIs = append(template.URIs, uri)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	certBuf := bytes.NewBuffer(nil)
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, fmt.Errorf("failed to encode certificate: %s", err)
	}

	keyBuf := bytes.NewBuffer(nil)
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return nil, fmt.Errorf("failed to encode key: %s", err)
	}

	cert, err := tls.X509KeyPair(certBuf.Bytes(), keyBuf.Bytes())
	return &cert, err
}

// getSecurityMode converts a security mode string to a ua.MessageSecurityMode type.
func getSecurityMode(mode string) ua.MessageSecurityMode {
	switch mode {
	case "None", "none":
		return ua.MessageSecurityModeNone
	case "Sign", "sign":
		return ua.MessageSecurityModeSign
	case "Sign&Encrypt", "sign&encrypt":
		return ua.MessageSecurityModeSignAndEncrypt
	default:
		return ua.MessageSecurityModeNone
	}
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

// AddOpcuaClient adds an OPC-UA client to the map of clients.
func addOpcuaClient(deviceID string, client *opcua.Client) {
	opcuaClients[deviceID] = client
}

// DebugIdentityToken gibt detaillierte Informationen über die Authentifizierungskonfiguration aus
func DebugIdentityToken(device DeviceConfig) {
	logrus.Infof("=== OPC-UA Identity Token Debug für Gerät: %s ===", device.Name)
	logrus.Infof("Security Mode: %s", device.SecurityMode)
	logrus.Infof("Security Policy: %s", device.SecurityPolicy)
	logrus.Infof("Username gesetzt: %t", device.Username != "")
	logrus.Infof("Password gesetzt: %t", device.Password != "")

	if device.Username != "" {
		logrus.Infof("Username: %s", device.Username)
		logrus.Infof("Password: %s", strings.Repeat("*", len(device.Password)))
	}

	// Zeige die tatsächlichen OPC-UA Werte
	securityMode := getSecurityMode(device.SecurityMode)
	securityPolicy := getSecurityPolicy(device.SecurityPolicy)

	logrus.Infof("OPC-UA Security Mode: %v", securityMode)
	logrus.Infof("OPC-UA Security Policy: %s", securityPolicy)

	// Zeige welche Authentifizierung verwendet wird
	if device.Username != "" && device.Password != "" {
		logrus.Infof("Verwendete Authentifizierung: Username/Password")
	} else {
		logrus.Infof("Verwendete Authentifizierung: Anonymous")
	}

	logrus.Infof("=== Ende Debug ===")
}

// TestAuthenticationOptions testet verschiedene Authentifizierungsoptionen
func TestAuthenticationOptions(device DeviceConfig) {
	logrus.Infof("=== Testing Authentication Options für %s ===", device.Name)

	// Test 1: Anonyme Authentifizierung
	logrus.Infof("Test 1: Anonyme Authentifizierung")
	device.Username = ""
	device.Password = ""
	opts, err := clientOptsFromFlags(device, nil)
	if err != nil {
		logrus.Errorf("Fehler bei Test 1: %v", err)
	} else {
		logrus.Infof("Test 1 erfolgreich - %d Optionen erstellt", len(opts))
	}

	// Test 2: Username/Password Authentifizierung (falls verfügbar)
	if device.Username != "" && device.Password != "" {
		logrus.Infof("Test 2: Username/Password Authentifizierung")
		opts, err := clientOptsFromFlags(device, nil)
		if err != nil {
			logrus.Errorf("Fehler bei Test 2: %v", err)
		} else {
			logrus.Infof("Test 2 erfolgreich - %d Optionen erstellt", len(opts))
		}
	}

	logrus.Infof("=== Ende Authentication Tests ===")
}

// ValidateAndFixOPCUAAddress validiert und korrigiert OPC-UA Adressen
func ValidateAndFixOPCUAAddress(address string) (string, error) {
	logrus.Infof("OPC-UA: Validating address: %s", address)

	// Prüfe ob es eine gültige OPC-UA URL ist
	if !strings.HasPrefix(address, "opc.tcp://") {
		return "", fmt.Errorf("invalid OPC-UA address format: must start with 'opc.tcp://'")
	}

	// Teile die URL auf, um die Host-Adresse zu extrahieren
	parts := strings.Split(address, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid OPC-UA address format: missing host part")
	}

	// Extrahiere Host:Port Teil
	hostPort := parts[2]

	// Prüfe ob Port angegeben ist
	if !strings.Contains(hostPort, ":") {
		hostPort += ":4840"
		logrus.Infof("OPC-UA: Added default port 4840 to address")
	}

	// Versuche die Adresse aufzulösen
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", fmt.Errorf("invalid host:port format: %v", err)
	}

	// Prüfe ob es eine IP-Adresse ist
	if net.ParseIP(host) != nil {
		logrus.Infof("OPC-UA: Address contains valid IP: %s", host)
		return address, nil
	}

	// Versuche Hostname aufzulösen
	ips, err := net.LookupHost(host)
	if err != nil {
		logrus.Warnf("OPC-UA: Could not resolve hostname '%s': %v", host, err)
		logrus.Warnf("OPC-UA: This might cause connection issues")
		return address, nil // Gib die ursprüngliche Adresse zurück
	}

	logrus.Infof("OPC-UA: Successfully resolved hostname '%s' to IPs: %v", host, ips)
	return address, nil
}

// GetResolvedAddress gibt die aufgelöste IP-Adresse für eine OPC-UA URL zurück
func GetResolvedAddress(address string) (string, error) {
	if !strings.HasPrefix(address, "opc.tcp://") {
		return "", fmt.Errorf("invalid OPC-UA address format")
	}

	parts := strings.Split(address, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid OPC-UA address format")
	}

	hostPort := parts[2]
	if !strings.Contains(hostPort, ":") {
		hostPort += ":4840"
	}

	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", err
	}

	// Wenn es bereits eine IP ist, gib sie zurück
	if ip := net.ParseIP(host); ip != nil {
		return fmt.Sprintf("%s:%s", ip.String(), port), nil
	}

	// Versuche Hostname aufzulösen
	ips, err := net.LookupHost(host)
	if err != nil {
		return "", fmt.Errorf("could not resolve hostname '%s': %v", host, err)
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found for hostname '%s'", host)
	}

	// Verwende die erste IP
	return fmt.Sprintf("%s:%s", ips[0], port), nil
}

// DiagnoseDNSProblem diagnostiziert DNS-Auflösungsprobleme für OPC-UA Adressen
func DiagnoseDNSProblem(address string) {
	logrus.Infof("=== DNS Problem Diagnose für: %s ===", address)

	if !strings.HasPrefix(address, "opc.tcp://") {
		logrus.Errorf("Keine gültige OPC-UA Adresse")
		return
	}

	parts := strings.Split(address, "/")
	if len(parts) < 3 {
		logrus.Errorf("Ungültiges Adressformat")
		return
	}

	hostPort := parts[2]
	if !strings.Contains(hostPort, ":") {
		hostPort += ":4840"
	}

	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen von Host:Port: %v", err)
		return
	}

	logrus.Infof("Host: %s", host)
	logrus.Infof("Port: %s", port)

	// Prüfe ob es eine IP-Adresse ist
	if ip := net.ParseIP(host); ip != nil {
		logrus.Infof("Host ist eine gültige IP-Adresse: %s", ip.String())
		return
	}

	// Versuche DNS-Auflösung
	logrus.Infof("Versuche DNS-Auflösung für Hostname: %s", host)
	ips, err := net.LookupHost(host)
	if err != nil {
		logrus.Errorf("DNS-Auflösung fehlgeschlagen: %v", err)
		logrus.Infof("Mögliche Lösungen:")
		logrus.Infof("1. Überprüfe die Schreibweise des Hostnamens")
		logrus.Infof("2. Füge den Hostnamen in /etc/hosts ein")
		logrus.Infof("3. Überprüfe die DNS-Konfiguration")
		logrus.Infof("4. Verwende die IP-Adresse direkt")
		return
	}

	logrus.Infof("DNS-Auflösung erfolgreich:")
	for i, ip := range ips {
		logrus.Infof("  IP %d: %s", i+1, ip)
	}

	logrus.Infof("=== Ende DNS Diagnose ===")
}
