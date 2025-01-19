package opcua

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	opcua "github.com/gopcua/opcua"
	"github.com/sirupsen/logrus"
)

var mqttClient mqtt.Client
var opcuaClients = make(map[string]*opcua.Client) // Map to store OPC-UA clients by device name

// InitMqttClient initialisiert den MQTT-Client
//
// Args:
//
//   - mqttConfig(MqttConfig): The configuration for the MQTT client.
//
// Returns:
//
//	None
//
// Example:
//
//		mqttConfig := MqttConfig{
//	      Broker:   "tcp://localhost:1883",
//	      Username: "username",
//	      Password: "password",
//	  }
//	  initMqttClient(mqttConfig)
func initMqttClient(mqttConfig MqttConfig) {
	opts := mqtt.NewClientOptions().
		AddBroker(mqttConfig.Broker).
		SetUsername(mqttConfig.Username).
		SetPassword(mqttConfig.Password).
		SetPingTimeout(10 * time.Second).
		SetAutoReconnect(true)

	mqttClient = mqtt.NewClient(opts)
	token := mqttClient.Connect()
	token.Wait()
	if token.Error() != nil {
		logrus.Warnf("OPC-UA: Failed to connect to MQTT broker: %v", token.Error())
	} else {
		//fmt.Println("OPC-UA: Connected to MQTT broker")
	}
}

// AddOpcuaClient adds an OPC-UA client to the map of clients.
//
// Args:
//
//   - devicename (string): The name of the device.
//   - client (*opcua.Client): The OPC-UA client.
//
// Returns:
//
//	None
//
// Example:
//
//	client := opcua.NewClient("opc.tcp://localhost:4840")
//	addOpcuaClient("device1", client)
func addOpcuaClient(devicename string, client *opcua.Client) {
	opcuaClients[devicename] = client
}

// StartMqttDataUpdateListener starts the MQTT listener for data point updates.
//
// Args:
//
//   - topic (string): The topic to subscribe to.
func startMqttDataUpdateListener(topic string) {
	if mqttClient == nil {
		logrus.Errorf("OPC-UA: MQTT client not initialized")
		return
	}

	if token := mqttClient.Subscribe(topic, 1, handleMqttDataUpdate); token.Wait() && token.Error() != nil {
		logrus.Errorf("OPC-UA: Failed to subscribe to topic %v: %v", topic, token.Error())
	} else {
		//logrus.Info("OPC-UA: MQTT data update listener started and subscribed topic : %w", topic)
	}
}

// HandleMqttDataUpdate handles MQTT messages for updating data points.
//
// Args:
//
//   - client (mqtt.Client): The MQTT client.
//   - message (mqtt.Message): The MQTT message.
//     This function is called automatically when an MQTT message is received.
func handleMqttDataUpdate(client mqtt.Client, message mqtt.Message) {
	var update struct {
		NodeID     string      `json:"nodeId"`
		Value      interface{} `json:"value"`
		DeviceName string      `json:"deviceName"` // Add DeviceName to the MQTT message payload
	}

	if err := json.Unmarshal(message.Payload(), &update); err != nil {
		logrus.Warnf("OPC-UA: Failed to unmarshal MQTT message: %v", err)
		return
	}

	opcuaClient, exists := opcuaClients[update.DeviceName]
	if !exists {
		logrus.Errorf("OPC-UA: Client for device '%s' not found", update.DeviceName)
		return
	}

	if err := UpdateDataNode(opcuaClient, update.NodeID, update.Value); err != nil {
		logrus.Warnf("OPC-UA: Failed to update data node: %v", err)
	}
}

// PubData publishes data to the MQTT broker.
//
// Args:
//
//   - data (map[string]interface{}): The data to publish.
//   - deviceName (string): The name of the device.
//
// Returns:
//
//	error: An error if the data could not be published.
//
// Example:
//
//	data := map[string]interface{}{
//	    "temperature": 25.0,
//	    "humidity":    50.0,
//	}
//	err := pubData(data, "device1")
//	if err != nil {
//	    fmt.Println(err)
//	}
func pubData(data map[string]interface{}, deviceName string) error {
	if mqttClient == nil {
		return fmt.Errorf("OPC-UA: MQTT client not initialized")
	}

	for name, value := range data {
		payload, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("OPC-UA: Failed to marshal data for node-name %s: %v", name, err)
		}

		topic := fmt.Sprintf("data/opcua/%s/%s", deviceName, name)
		token := mqttClient.Publish(topic, 1, false, payload)
		token.Wait()
		if token.Error() != nil {
			return fmt.Errorf("OPC-UA: Failed to publish data for node-name %s: %v", name, token.Error())
		}
	}

	return nil
}
