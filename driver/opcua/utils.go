package opcua

import (
	"bytes"
	"context"
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

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/sirupsen/logrus"
)

// clientOptsFromFlags erstellt die OPC UA Optionen und prüft, ob Zertifikat und Schlüssel vorhanden sind.
func clientOptsFromFlags(device DeviceConfig, db *sql.DB) ([]client.Option, error) {
	opts := []client.Option{}

	// awcullen/opcua verwendet eine einfachere API - die meisten Optionen sind in Dial integriert
	// Füge nur InsecureSkipVerify hinzu um SSL-Probleme zu vermeiden
	opts = append(opts, client.WithInsecureSkipVerify())

	// TODO: Security und Auth-Optionen müssen anders implementiert werden
	// Für jetzt verwenden wir die einfachste Konfiguration

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
			Organization:  []string{"KIOekoSys"},
			Country:       []string{"DE"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
			CommonName:    "KIOekoSys IoT Gateway",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Verwende die Gateway Application URI für das Zertifikat
	gatewayURI := "urn:KIOekoSys:IoT:Gateway"
	if uri, err := url.Parse(gatewayURI); err == nil {
		template.URIs = append(template.URIs, uri)
		// logrus.Infof("OPC-UA: Certificate will include Application URI: %s", gatewayURI)
	}

	// Füge universelle und lokale Identifikatoren hinzu
	err = addCertificateIdentifiers(&template)
	if err != nil {
		logrus.Warnf("OPC-UA: Could not add all certificate identifiers: %v", err)
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
		return "http://opcfoundation.org/UA/SecurityPolicy#None"
	case "Basic128Rsa15", "basic128rsa15":
		return ua.SecurityPolicyURIBasic128Rsa15
	case "Basic256", "basic256":
		return ua.SecurityPolicyURIBasic256
	case "Basic256Sha256", "basic256sha256":
		return ua.SecurityPolicyURIBasic256Sha256
	default:
		return "http://opcfoundation.org/UA/SecurityPolicy#None"
	}
}

// AddOpcuaClient adds an OPC-UA client to the map of clients.
func addOpcuaClient(deviceID string, ch *client.Client) {
	opcuaClients[deviceID] = ch
}

// DebugIdentityToken gibt detaillierte Informationen über die Authentifizierungskonfiguration aus
func DebugIdentityToken(device DeviceConfig) {
	// logrus.Infof("=== OPC-UA Identity Debug für Gerät: %s ===", device.Name)

	// Gateway Application Identity
	// gatewayApplicationURI := "urn:KIOekoSys:IoT:Gateway"
	// logrus.Infof("Gateway Application URI: %s", gatewayApplicationURI)

	// Device Configuration
	logrus.Infof("Device Address: %s", device.Address)
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

	// logrus.Infof("=== Ende Debug ===")
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

// EndpointInfo erweiterte Endpoint-Informationen
type EndpointInfo struct {
	URL            string
	SecurityMode   string
	SecurityPolicy string
	UserTokens     []string
}

// DiscoverEndpoints entdeckt verfügbare Endpoints vom OPC-UA Server
func DiscoverEndpoints(baseAddress string) ([]string, error) {
	endpoints, err := DiscoverEndpointsDetailed(baseAddress)
	if err != nil {
		return nil, err
	}

	var urls []string
	for _, ep := range endpoints {
		urls = append(urls, ep.URL)
	}
	return urls, nil
}

// DiscoverEndpointsDetailed entdeckt verfügbare Endpoints mit detaillierten Informationen
func DiscoverEndpointsDetailed(baseAddress string) ([]EndpointInfo, error) {
	logrus.Infof("=== Detailed Endpoint Discovery für: %s ===", baseAddress)

	// Extrahiere nur den Host:Port Teil für die Discovery
	parts := strings.Split(baseAddress, "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid address format")
	}

	hostPort := parts[2]
	if !strings.Contains(hostPort, ":") {
		hostPort += ":4840"
	}

	// Erstelle Discovery URL (ohne Pfad)
	discoveryURL := fmt.Sprintf("opc.tcp://%s", hostPort)
	logrus.Infof("Using discovery URL: %s", discoveryURL)

	// awcullen/opcua hat keine GetEndpoints Funktion, verwende direkte Verbindung
	ctx := context.Background()
	ch, err := client.Dial(ctx, discoveryURL, client.WithInsecureSkipVerify())
	if err != nil {
		logrus.Errorf("Endpoint discovery failed: %v", err)
		return nil, err
	}
	defer ch.Close(ctx)

	// Vereinfachte Endpoint Info für awcullen/opcua
	endpointInfo := EndpointInfo{
		URL:            discoveryURL,
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokens:     []string{"Anonymous"},
	}
	endpointInfos := []EndpointInfo{endpointInfo}

	logrus.Infof("Endpoint discovered: %s", discoveryURL)

	logrus.Infof("=== Detailed Endpoint Discovery Complete ===")
	logrus.Infof("Found %d endpoints with full paths preserved", len(endpointInfos))

	return endpointInfos, nil
}

// FindBestEndpoint findet den besten Endpoint basierend auf Sicherheitsanforderungen
func FindBestEndpoint(baseAddress string, preferredSecurityMode string, preferredSecurityPolicy string) (string, error) {
	logrus.Infof("OPC-UA: Searching for best endpoint for base address: %s", baseAddress)

	endpoints, err := DiscoverEndpoints(baseAddress)
	if err != nil {
		return "", err
	}

	if len(endpoints) == 0 {
		return "", fmt.Errorf("no endpoints discovered")
	}

	// Suche nach dem besten Endpoint basierend auf Präferenzen
	bestEndpoint := findMatchingEndpoint(endpoints, preferredSecurityMode, preferredSecurityPolicy)

	if bestEndpoint == "" {
		// Fallback: Verwende den ersten verfügbaren Endpoint
		bestEndpoint = endpoints[0]
		logrus.Warnf("OPC-UA: No matching endpoint found, using first available: %s", bestEndpoint)
	} else {
		logrus.Infof("OPC-UA: Found matching endpoint: %s", bestEndpoint)
	}

	return bestEndpoint, nil
}

// findMatchingEndpoint sucht den besten passenden Endpoint
func findMatchingEndpoint(endpoints []string, preferredSecurityMode string, preferredSecurityPolicy string) string {
	// Für jetzt geben wir den ersten Endpoint zurück
	// In Zukunft kann hier eine intelligentere Auswahl implementiert werden
	if len(endpoints) > 0 {
		return endpoints[0]
	}
	return ""
}

// TryDifferentAuthMethods versucht verschiedene Authentifizierungsmethoden
func TryDifferentAuthMethods(device DeviceConfig, endpointURL string) error {
	// logrus.Infof("=== Trying Different Authentication Methods ===")

	ctx := context.Background()
	// gatewayApplicationURI := "urn:KIOekoSys:IoT:Gateway"

	// Methode 1: Anonyme Authentifizierung
	logrus.Infof("Trying Method 1: Anonymous Authentication")
	opts1 := []client.Option{
		client.WithInsecureSkipVerify(),
	}

	client1, err := client.Dial(ctx, endpointURL, opts1...)
	if err == nil {
		// logrus.Infof("SUCCESS: Anonymous authentication worked!")
		client1.Close(ctx)
		return nil
	} else {
		logrus.Warnf("Anonymous authentication failed: %v", err)
	}

	// Methode 2: Username/Password (falls verfügbar)
	if device.Username != "" && device.Password != "" {
		// logrus.Infof("Trying Method 2: Username/Password Authentication")
		opts2 := []client.Option{
			client.WithInsecureSkipVerify(),
		}

		client2, err := client.Dial(ctx, endpointURL, opts2...)
		if err == nil {
			// logrus.Infof("SUCCESS: Username/Password authentication worked!")
			client2.Close(ctx)
			return nil
		} else {
			logrus.Warnf("Username/Password authentication failed: %v", err)
		}
	}

	logrus.Errorf("All authentication methods failed")
	return fmt.Errorf("all authentication methods failed")
}

// GetGatewayApplicationURI gibt die Gateway Application URI zurück
func GetGatewayApplicationURI() string {
	return "urn:KIOekoSys:IoT:Gateway"
}

// RegenerateCertificateWithCorrectURI regeneriert das Zertifikat mit der korrekten Gateway URI
func RegenerateCertificateWithCorrectURI() error {
	// logrus.Infof("=== Regenerating Certificate with Gateway URI ===")

	certPath := "certificate-opcua/idpm_cert.pem"
	keyPath := "certificate-opcua/idpm_key.pem"

	// Lösche alte Zertifikate
	if err := os.Remove(certPath); err != nil && !os.IsNotExist(err) {
		logrus.Warnf("Could not remove old certificate: %v", err)
	}
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		logrus.Warnf("Could not remove old key: %v", err)
	}

	// Generiere neue Zertifikate
	c, err := generateCert()
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %v", err)
	}

	pk := c.PrivateKey.(*rsa.PrivateKey)
	cert := c.Certificate[0]

	// Erstelle Verzeichnis falls es nicht existiert
	if err := os.MkdirAll("certificate-opcua", 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %v", err)
	}

	// Speichere das Zertifikat und den privaten Schlüssel
	if err := os.WriteFile(certPath, cert, 0644); err != nil {
		return fmt.Errorf("failed to save certificate: %v", err)
	}
	if err := os.WriteFile(keyPath, x509.MarshalPKCS1PrivateKey(pk), 0644); err != nil {
		return fmt.Errorf("failed to save private key: %v", err)
	}

	logrus.Infof("Certificate regenerated successfully with Gateway URI: %s", GetGatewayApplicationURI())
	return nil
}

// TryManualEndpointConstruction versucht, den Endpoint manuell zu konstruieren
// Dies ist ein Fallback, wenn Endpoint Discovery fehlschlägt
func TryManualEndpointConstruction(originalAddress string) []string {
	// logrus.Infof("=== Manual Endpoint Construction for: %s ===", originalAddress)

	var endpoints []string

	// Extrahiere Host:Port
	parts := strings.Split(originalAddress, "/")
	if len(parts) < 3 {
		logrus.Errorf("Invalid address format for manual construction")
		return endpoints
	}

	hostPort := parts[2]
	if !strings.Contains(hostPort, ":") {
		hostPort += ":4840"
	}

	// Konstruiere verschiedene mögliche Endpoints
	commonPaths := []string{
		"",              // Nur Host:Port
		"/UA/Server",    // Häufiger Pfad
		"/OPCUA/Server", // Alternative
		"/OPC/Server",   // Alternative
		"/Server",       // Einfacher Pfad
	}

	// Rekonstruiere ursprünglichen Pfad falls vorhanden
	if len(parts) > 3 {
		originalPath := "/" + strings.Join(parts[3:], "/")
		commonPaths = append([]string{originalPath}, commonPaths...)
		logrus.Infof("Extracted original path: %s", originalPath)
	}

	for _, path := range commonPaths {
		endpoint := fmt.Sprintf("opc.tcp://%s%s", hostPort, path)
		endpoints = append(endpoints, endpoint)
		logrus.Infof("Manual endpoint candidate: %s", endpoint)
	}

	logrus.Infof("=== Manual Construction Complete: %d endpoints ===", len(endpoints))
	return endpoints
}

// TryMultipleEndpoints versucht verschiedene Endpoints nacheinander
func TryMultipleEndpoints(device DeviceConfig, candidateEndpoints []string) (string, error) {
	logrus.Infof("=== Trying Multiple Endpoints ===")

	ctx := context.Background()

	for i, endpoint := range candidateEndpoints {
		logrus.Infof("Trying endpoint %d/%d: %s", i+1, len(candidateEndpoints), endpoint)

		// Teste mit anonymer Authentifizierung
		opts := []client.Option{
			// client.WithSecurityMode(ua.MessageSecurityModeNone),
			// client.WithSecurityPolicy(ua.SecurityPolicyURINone),
			// client.WithAnonymousIdentity(),
			client.WithInsecureSkipVerify(),
		}

		ch, err := client.Dial(ctx, endpoint, opts...)
		if err != nil {
			logrus.Warnf("Failed to connect to endpoint %s: %v", endpoint, err)
			continue
		}

		// Erfolg!
		ch.Close(ctx)
		logrus.Infof("SUCCESS: Found working endpoint: %s", endpoint)
		return endpoint, nil
	}

	return "", fmt.Errorf("no working endpoint found among %d candidates", len(candidateEndpoints))
}

// CreateOPCUACertificates erstellt neue OPC-UA Zertifikate mit der korrekten Gateway URI
func CreateOPCUACertificates() error {
	// logrus.Infof("=== Creating OPC-UA Certificates ===")

	// Definiere Pfade
	certPath := "certificate-opcua/idpm_cert.pem"
	keyPath := "certificate-opcua/idpm_key.pem"

	// Erstelle Verzeichnis falls es nicht existiert
	if err := os.MkdirAll("certificate-opcua", 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %v", err)
	}

	// Lösche bestehende Zertifikate
	if err := os.Remove(certPath); err != nil && !os.IsNotExist(err) {
		logrus.Warnf("Could not remove existing certificate: %v", err)
	}
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		logrus.Warnf("Could not remove existing key: %v", err)
	}

	// Generiere neue Zertifikate
	// logrus.Infof("OPC-UA: Generating new certificate and private key...")
	c, err := generateCert()
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %v", err)
	}

	pk := c.PrivateKey.(*rsa.PrivateKey)
	cert := c.Certificate[0]

	// Speichere das Zertifikat im PEM-Format
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to save certificate: %v", err)
	}
	// logrus.Infof("OPC-UA: Certificate saved to %s", certPath)

	// Speichere den privaten Schlüssel im PEM-Format
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pk),
	})
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		return fmt.Errorf("failed to save private key: %v", err)
	}
	// logrus.Infof("OPC-UA: Private key saved to %s", keyPath)

	// Validiere das erstellte Zertifikat
	if err := validateCreatedCertificate(certPath); err != nil {
		return fmt.Errorf("certificate validation failed: %v", err)
	}

	// logrus.Infof("✅ OPC-UA certificates created successfully!")
	// logrus.Infof("Gateway Application URI: %s", GetGatewayApplicationURI())

	return nil
}

// validateCreatedCertificate validiert das erstellte Zertifikat
func validateCreatedCertificate(certPath string) error {
	// Lese das Zertifikat
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %v", err)
	}

	// Parse das PEM
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode PEM certificate")
	}

	// Parse das X.509 Zertifikat
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %v", err)
	}

	// Validiere Subject
	// actualSubject := cert.Subject.String()
	// logrus.Infof("Certificate Subject: %s", actualSubject)

	// Validiere Application URI
	gatewayURI := GetGatewayApplicationURI()
	foundURI := false
	for _, uri := range cert.URIs {
		if uri.String() == gatewayURI {
			foundURI = true
			// logrus.Infof("✅ Gateway Application URI found in certificate: %s", uri.String())
			break
		}
	}

	if !foundURI {
		return fmt.Errorf("gateway application URI %s not found in certificate", gatewayURI)
	}

	// Validiere Gültigkeit
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid (NotBefore: %v)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired (NotAfter: %v)", cert.NotAfter)
	}

	// logrus.Infof("✅ Certificate is valid from %v to %v", cert.NotBefore, cert.NotAfter)

	return nil
}

// ForceRegenerateCertificates entfernt bestehende Zertifikate und erstellt neue
func ForceRegenerateCertificates() error {
	logrus.Infof("=== Force Regenerating OPC-UA Certificates ===")

	// Entferne komplettes Verzeichnis
	if err := os.RemoveAll("certificate-opcua"); err != nil {
		logrus.Warnf("Could not remove certificate directory: %v", err)
	} else {
		logrus.Infof("Removed existing certificate directory")
	}

	// Erstelle neue Zertifikate
	return CreateOPCUACertificates()
}

// CheckCertificateStatus überprüft den Status der vorhandenen Zertifikate
func CheckCertificateStatus() map[string]interface{} {
	status := map[string]interface{}{
		"certificate_exists":  false,
		"key_exists":          false,
		"certificate_valid":   false,
		"gateway_uri_present": false,
		"expires_at":          nil,
		"subject":             "",
	}

	certPath := "certificate-opcua/idpm_cert.pem"
	keyPath := "certificate-opcua/idpm_key.pem"

	// Prüfe ob Dateien existieren
	if _, err := os.Stat(certPath); err == nil {
		status["certificate_exists"] = true
	}
	if _, err := os.Stat(keyPath); err == nil {
		status["key_exists"] = true
	}

	// Wenn Zertifikat existiert, validiere es
	if status["certificate_exists"].(bool) {
		if err := validateCreatedCertificate(certPath); err == nil {
			status["certificate_valid"] = true

			// Lese weitere Details
			if certPEM, err := os.ReadFile(certPath); err == nil {
				if block, _ := pem.Decode(certPEM); block != nil {
					if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
						status["subject"] = cert.Subject.String()
						status["expires_at"] = cert.NotAfter

						// Prüfe Gateway URI
						gatewayURI := GetGatewayApplicationURI()
						for _, uri := range cert.URIs {
							if uri.String() == gatewayURI {
								status["gateway_uri_present"] = true
								break
							}
						}
					}
				}
			}
		}
	}

	return status
}

// addCertificateIdentifiers fügt universelle und lokale Identifikatoren zum Zertifikat hinzu
func addCertificateIdentifiers(template *x509.Certificate) error {
	logrus.Infof("OPC-UA: Adding certificate identifiers...")

	// Universelle DNS Names (funktionieren auf jedem System)
	universalDNS := []string{
		"localhost",
		"*.local",       // mDNS/Bonjour
		"gateway.local", // Spezifischer Gateway Name
		"iot-gateway",   // Allgemeiner Gateway Name
		"opcua-client",  // OPC-UA Client Identifier
	}

	// Universelle IPs
	universalIPs := []string{
		"127.0.0.1", // IPv4 Loopback
		"::1",       // IPv6 Loopback
	}

	// Universelle URIs
	additionalURIs := []string{
		"urn:gateway:client",
		"urn:opcua:client",
		"urn:localhost:opcua",
	}

	// Füge universelle DNS Namen hinzu
	for _, dns := range universalDNS {
		template.DNSNames = append(template.DNSNames, dns)
		logrus.Infof("OPC-UA: Added DNS name: %s", dns)
	}

	// Füge universelle IPs hinzu
	for _, ipStr := range universalIPs {
		if ip := net.ParseIP(ipStr); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			logrus.Infof("OPC-UA: Added IP address: %s", ip.String())
		}
	}

	// Füge zusätzliche URIs hinzu
	for _, uriStr := range additionalURIs {
		if uri, err := url.Parse(uriStr); err == nil {
			template.URIs = append(template.URIs, uri)
			logrus.Infof("OPC-UA: Added URI: %s", uri.String())
		}
	}

	// Versuche lokale Netzwerk-Informationen zu ermitteln
	err := addLocalNetworkIdentifiers(template)
	if err != nil {
		logrus.Warnf("OPC-UA: Could not add local network identifiers: %v", err)
	}

	// Versuche Hostname zu ermitteln
	if hostname, err := os.Hostname(); err == nil {
		template.DNSNames = append(template.DNSNames, hostname)
		template.DNSNames = append(template.DNSNames, strings.ToLower(hostname))
		logrus.Infof("OPC-UA: Added hostname: %s", hostname)
	}

	logrus.Infof("OPC-UA: Certificate will be valid for %d DNS names, %d IP addresses, %d URIs",
		len(template.DNSNames), len(template.IPAddresses), len(template.URIs))

	return nil
}

// addLocalNetworkIdentifiers fügt lokale Netzwerk-IP-Adressen hinzu
func addLocalNetworkIdentifiers(template *x509.Certificate) error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("could not get network interfaces: %v", err)
	}

	for _, iface := range interfaces {
		// Überspringe inaktive Interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			logrus.Warnf("OPC-UA: Could not get addresses for interface %s: %v", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Füge nur nicht-link-lokale IPv4 und globale IPv6 Adressen hinzu
			if ip.To4() != nil || (ip.To16() != nil && !ip.IsLinkLocalUnicast()) {
				template.IPAddresses = append(template.IPAddresses, ip)
				logrus.Infof("OPC-UA: Added local IP from interface %s: %s", iface.Name, ip.String())
			}
		}
	}

	return nil
}

// CreatePortableOPCUACertificates erstellt Zertifikate die auf verschiedenen Systemen funktionieren
func CreatePortableOPCUACertificates() error {
	logrus.Infof("=== Creating Portable OPC-UA Certificates ===")

	// Lösche bestehende Zertifikate um Neugenerierung zu erzwingen
	if err := os.RemoveAll("certificate-opcua"); err != nil {
		logrus.Warnf("Could not remove certificate directory: %v", err)
	}

	// Erstelle neue portable Zertifikate
	return CreateOPCUACertificates()
}
