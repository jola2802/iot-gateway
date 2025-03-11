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
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// clientOptsFromFlags erstellt die OPC UA Optionen und prüft, ob Zertifikat und Schlüssel vorhanden sind.
func clientOptsFromFlags(device DeviceConfig, db *sql.DB) ([]opcua.Option, error) {
	opts := []opcua.Option{}

	// Setze Basis-Sicherheitsoptionen
	opts = append(opts,
		opcua.SecurityMode(getSecurityMode(device.SecurityMode)),
		opcua.SecurityPolicy(getSecurityPolicy(device.SecurityPolicy)),
	)

	// Bei SecurityMode "None" nur anonyme Authentifizierung
	if getSecurityMode(device.SecurityMode) == ua.MessageSecurityModeNone {
		opts = append(opts, opcua.AuthAnonymous())
		return opts, nil
	}

	if device.Username != "" && device.Password != "" {
		opts = append(opts, opcua.AuthUsername(device.Username, device.Password))
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

// AddOpcuaClient adds an OPC-UA client to the map of clients.
func addOpcuaClient(deviceID string, client *opcua.Client) {
	opcuaClients[deviceID] = client
}
