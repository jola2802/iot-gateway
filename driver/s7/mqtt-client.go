package s7

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

// DataPoint repräsentiert einen Datenpunkt für die MQTT-Nachricht
type DataPoint struct {
	DeviceName string      `json:"deviceName"`
	NodeId     string      `json:"nodeId"`
	Value      interface{} `json:"value"`
}

var mqttClient mqtt.Client

// InitMqttClient initialisiert den MQTT-Client
func initMqttClient(mqttConfig MqttConfig) {
	if mqttClient != nil && mqttClient.IsConnected() {
		logrus.Infof("S7: MQTT client already connected")
		return
	}

	opts := mqtt.NewClientOptions().
		AddBroker(mqttConfig.Broker).
		SetUsername(mqttConfig.Username).
		SetPassword(mqttConfig.Password).
		// SetKeepAlive(120 * time.Second).
		SetPingTimeout(10 * time.Second).
		SetAutoReconnect(true)

	mqttClient = mqtt.NewClient(opts)
	token := mqttClient.Connect()
	token.Wait()
	if token.Error() != nil {
		logrus.Errorf("S7: Failed to connect to MQTT broker: %v", token.Error())
	} else {
		// logrus.Info("S7: Connected to MQTT broker")
	}
}

// StartMqttDataUpdateListener startet den MQTT-Listener für Datenpunkt-Updates
func StartMqttDataUpdateListener(topic string, db *sql.DB) {
	if mqttClient == nil {
		logrus.Errorf("S7: MQTT client for Update not initialized")
		return
	}

	// Verwende Closure, um die DB-Verbindung in der Funktion zu kapseln
	messageHandler := func(client mqtt.Client, message mqtt.Message) {
		handleMqttDataUpdate(client, message, db)
	}

	if token := mqttClient.Subscribe(topic, 1, messageHandler); token.Wait() && token.Error() != nil {
		logrus.Errorf("S7: Failed to subscribe to topic: %v", token.Error())
	} else {
		// logrus.Info("S7: MQTT data update listener started")
	}
}

// handleMqttDataUpdate verarbeitet MQTT-Nachrichten zum Aktualisieren von Datenpunkten und schickt diese an die S7-Steuerung
func handleMqttDataUpdate(client mqtt.Client, message mqtt.Message, db *sql.DB) {
	// MQTT-Nachricht in DataPoint-Struktur umwandeln
	var dataPoint DataPoint
	if err := json.Unmarshal(message.Payload(), &dataPoint); err != nil {
		logrus.Errorf("S7: Failed to unmarshal MQTT message: %v", err)
		return
	}

	// Abrufen der Gerätekonfiguration
	deviceConfig, err := GetConfigByName(db, dataPoint.DeviceName)
	if err != nil {
		logrus.Errorf("S7: Could not find device config for %s: %v", dataPoint.DeviceName, err)
		return
	}

	// Aktualisieren des S7-Datenpunkts
	if err := UpdateDataPoint(deviceConfig, dataPoint.NodeId, dataPoint.Value); err != nil {
		logrus.Errorf("S7: Failed to update S7 value for %s at %s: %v", dataPoint.DeviceName, dataPoint.NodeId, err)
		return
	}

	logrus.Infof("S7: Successfully updated S7 value for %s at %s", dataPoint.DeviceName, dataPoint.NodeId)
}

// PubData veröffentlicht die Daten auf dem MQTT-Broker
func PubData(data []map[string]interface{}, deviceName string) error {
	if mqttClient == nil {
		logrus.Errorf("S7: MQTT client not initialized")
		return nil
	}

	for _, dp := range data {
		name, ok := dp["name"].(string)
		if !ok {
			logrus.Errorf("S7: Invalid datapoint name")
			return nil
		}
		value, ok := dp["value"]
		if !ok {
			logrus.Errorf("S7: Invalid datapoint value")
			return nil
		}

		payload, err := json.Marshal(value)
		if err != nil {
			logrus.Errorf("S7: Failed to marshal data for datapoint %s: %v", name, err)
			return nil
		}

		topic := fmt.Sprintf("data/s7/%s/%s", deviceName, name)
		token := mqttClient.Publish(topic, 1, false, payload)
		token.Wait()
		if token.Error() != nil {
			logrus.Errorf("S7: Failed to publish data for datapoint %s: %v", name, token.Error())
			return nil
		}
	}
	return nil
}
