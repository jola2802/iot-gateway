package logic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"
)

func GenerateSelfSignedCert() (tls.Certificate, error) {
	// Zertifikatspfade aus Umgebungsvariablen holen
	certPath := os.Getenv("NGINX_TLS_CERT")
	keyPath := os.Getenv("NGINX_TLS_KEY")

	// Fallback auf Standardpfade, wenn Umgebungsvariablen nicht gesetzt sind
	if certPath == "" {
		certPath = "server.crt"
	}
	if keyPath == "" {
		keyPath = "server.key"
	}

	// Prüfen, ob Zertifikatsdateien bereits vorhanden sind
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			// Zertifikate laden, wenn sie existieren
			return tls.LoadX509KeyPair(certPath, keyPath)
		}
	}

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(5 * 365 * 24 * time.Hour) // für 5 Jahre gültig

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"DE"},
			Province:           []string{"Bayern"},
			Locality:           []string{"Ansbach"},
			Organization:       []string{"HS Ansbach"},
			OrganizationalUnit: []string{"IDPM"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	defer certOut.Close()
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return tls.LoadX509KeyPair(certPath, keyPath)
}
