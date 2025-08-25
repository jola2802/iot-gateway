package main

import (
	"github.com/sirupsen/logrus"

	dataforwarding "iot-gateway/data-forwarding"
	opcua_driver "iot-gateway/driver/opcua"
	logic "iot-gateway/logic"
	mqtt_broker "iot-gateway/mqtt_broker"
	webui "iot-gateway/webui"
)

var (
	dbPath = "./iot_gateway.db"
)

// initializeOPCUACertificates stellt sicher, dass OPC-UA Zertifikate beim Gateway-Start verfügbar sind
func initializeOPCUACertificates() {
	logrus.Info("MAIN: Initializing OPC-UA certificates...")

	// Prüfe ob Zertifikate bereits existieren
	status := opcua_driver.CheckCertificateStatus()

	if status["certificate_exists"].(bool) && status["key_exists"].(bool) && status["certificate_valid"].(bool) {
		logrus.Info("MAIN: Valid OPC-UA certificates already exist")
		logrus.Infof("MAIN: Certificate expires at: %v", status["expires_at"])
		return
	}

	// Erstelle neue portable Zertifikate
	logrus.Info("MAIN: Creating new portable OPC-UA certificates...")
	err := opcua_driver.CreatePortableOPCUACertificates()
	if err != nil {
		logrus.Errorf("MAIN: Failed to create OPC-UA certificates: %v", err)
		logrus.Warn("MAIN: OPC-UA connections may fail until certificates are manually created")
		return
	}

	logrus.Info("MAIN: ✅ OPC-UA certificates successfully initialized")
}

func main() {
	// Log-System initialisieren
	go logic.GatewayLogs()

	// Initialisiere die SQLite-Datenbank mit dem übergebenen Pfad
	db, _ := logic.InitDB(dbPath)
	defer db.Close()

	// Initialisiere OPC-UA Zertifikate (proaktiv erstellen)
	initializeOPCUACertificates()

	// Start MQTT-Broker
	server := mqtt_broker.StartBroker(db)
	logrus.Info("MAIN: Broker started.")

	// Web-UI
	go webui.Main(db, server)
	defer webui.StopWebUI()
	logrus.Info("MAIN: Web-UI-server started.")

	// InfluxDB-Writer
	go dataforwarding.StartInfluxDBWriter(db, server)
	defer dataforwarding.StopInfluxDBWriter()

	// Start Driver
	go logic.StartAllDrivers(db, server)
	defer logic.StopAllDrivers()

	// Image Capture Prozesse initialisieren
	webui.InitImageCaptureProcesses(db)
	logrus.Info("MAIN: Image Capture Prozesse initialisiert.")

	// Cleanup-Funktion für graceful shutdown
	defer func() {
		webui.StopAllImageCaptureProcesses(db)
		logrus.Info("MAIN: Image Capture Prozesse gestoppt.")
	}()

	select {}
}
