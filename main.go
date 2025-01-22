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
	// topicUserMqttListener     = "iot-gateway/commands/user/#"
	// topicCommandsMqttListener = "iot-gateway/driver/states/#"
	dbPath      = "./iot_gateway.db"
	influxDBURL = "http://localhost:8086"
	// cacheDuration             = 5 * time.Minute
	nodeRedURL = ""
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

	nodeRedURL = "http://localhost:7777"

	// Start MQTT-Broker
	mqtt_broker.StartBroker(db)
	logrus.Info("MAIN: Broker started.")

	// User Management für den MQTT-Broker + Listening for changes
	// logic.ManageUser(topicUserMqttListener, db)

	// MQTT Commands Listener starten
	// logic.StartMqttListener(topicCommandsMqttListener, db)
	// defer logic.StopMqttListener()

	// time.Sleep(3 * time.Second)

	// Wait until nodeRedURL is not empty
	for nodeRedURL == "" {
		time.Sleep(100 * time.Millisecond)
	}
	// Web-UI
	go webui.Main(db, nodeRedURL)
	// defer webui.Stop()
	logrus.Info("MAIN: Web-UI-server started.")

	// Starten der Treiber

	// time.Sleep(500 * time.Millisecond)

	// DRIVER
	// Initial all driver start if the configuration exists
	// logic.StartAllDrivers(db)

	// Zwischenspeicherung aller in den letzten empfangen Werte und start der jeweiligen Data Routes zum Forwarding
	// go dataforwarding.CacheMqttData(db, cacheDuration)
	// defer dataforwarding.StopCache(db)

	select {}
}
