package main

import (
	"github.com/sirupsen/logrus"

	dataforwarding "iot-gateway/data-forwarding"
	logic "iot-gateway/logic"
	mqtt_broker "iot-gateway/mqtt_broker"
	webui "iot-gateway/webui"
)

var (
	dbPath = "./iot_gateway.db"
)

func main() {
	// Initialisiere die SQLite-Datenbank mit dem übergebenen Pfad
	db, _ := logic.InitDB(dbPath)
	defer db.Close()

	// Start MQTT-Broker
	server := mqtt_broker.StartBroker(db)
	logrus.Info("MAIN: Broker started.")

	// InfluxDB-Writer
	go dataforwarding.StartInfluxDBWriter(db, server)
	defer dataforwarding.StopInfluxDBWriter()

	// Web-UI
	go webui.Main(db, server)
	defer webui.StopWebUI()
	logrus.Info("MAIN: Web-UI-server started.")

	// Start Driver
	logic.StartAllDrivers(db, server)

	go dataforwarding.StartDataForwarding(db, server)
	defer dataforwarding.StopDataForwarding()

	select {}
}
