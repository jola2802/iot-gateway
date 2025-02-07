package opcua

import (
	"encoding/json"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	opcua "github.com/gopcua/opcua"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

var mqttClient mqtt.Client

// AddOpcuaClient adds an OPC-UA client to the map of clients.
//
// Args:
//
//   - deviceID (string): The id of the device.
//   - client (*opcua.Client): The OPC-UA client.
//
// Returns:
//
//	None
//
// Example:
//
//	client := opcua.NewClient("opc.tcp://localhost:4840")
//	addOpcuaClient("1", client)
func addOpcuaClient(deviceID string, client *opcua.Client) {
	opcuaClients[deviceID] = client
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
func pubData(data map[string]interface{}, deviceName string, deviceId string, server *MQTT.Server) error {
	for name, value := range data {
		payload, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("OPC-UA: Failed to marshal data for node-name %s: %v", name, err)
		}

		topic := fmt.Sprintf("data/opcua/%s/%s", deviceId, name)
		err = server.Publish(topic, []byte(payload), false, 1)
		if err != nil {
			return fmt.Errorf("OPC-UA: Failed to publish data for node-name %s: %v", name, err)
		}

	}

	return nil
}
