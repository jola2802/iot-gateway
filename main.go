package main

import (
	"os"

	"github.com/sirupsen/logrus"

	dataforwarding "iot-gateway/data-forwarding"
	logic "iot-gateway/logic"
	mqtt_broker "iot-gateway/mqtt_broker"
	webui "iot-gateway/webui"
)

var (
	dbPath         = "./iot_gateway.db"
	noderedURL     = os.Getenv("NODE_RED_URL")
	influxdbURL    = os.Getenv("INFLUXDB_URL")
	influxdbToken  = os.Getenv("INFLUXDB_TOKEN")
	influxdbOrg    = os.Getenv("INFLUXDB_ORG")
	influxdbBucket = os.Getenv("INFLUXDB_BUCKET")
)

func main() {
	// Initialisiere die SQLite-Datenbank mit dem übergebenen Pfad
	db, _ := logic.InitDB(dbPath)
	defer db.Close()

	// Set influxdb config
	dataforwarding.SetInfluxDBConfig(influxdbURL, influxdbToken, influxdbOrg, influxdbBucket)

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

	go dataforwarding.StartDataForwarding(db, server)
	defer dataforwarding.StopDataForwarding()

	webui.StartAllImageProcessWorkers(db, noderedURL)
	// go webui.StartAllImageProcessWorkers(db, noderedURL)

	select {}
}
