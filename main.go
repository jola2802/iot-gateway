package main

import (
	"log"
	"time"

	"github.com/sirupsen/logrus"

	logic "iot-gateway/logic"
	mqtt_broker "iot-gateway/mqtt_broker"
	webui "iot-gateway/webui"
)

var (
	dbPath      = "./iot_gateway.db"
	influxDBURL = "http://127.0.0.1:8086"
)

func main() {
	// Initialisiere die SQLite-Datenbank mit dem übergebenen Pfad
	db, err := logic.InitDB(dbPath)
	if err != nil {
		logrus.Error("MAIN: Error initializing database")
		log.Fatalf("MAIN: Error initializing database: %v\n", err)
		return
	}
	defer db.Close()

	// Start MQTT-Broker
	server := mqtt_broker.StartBroker(db)
	logrus.Info("MAIN: Broker started.")

	time.Sleep(3 * time.Second)

	// Web-UI
	go webui.Main(db, server)
	// defer webui.Stop()
	logrus.Info("MAIN: Web-UI-server started.")

	// Start Driver
	go logic.StartAllDrivers(db, server)

	select {}
}
