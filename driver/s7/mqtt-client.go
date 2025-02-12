package s7

import (
	"encoding/json"
	"fmt"

	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

// DataPoint repräsentiert einen Datenpunkt für die MQTT-Nachricht
type DataPoint struct {
	DeviceName string      `json:"deviceName"`
	NodeId     string      `json:"nodeId"`
	Value      interface{} `json:"value"`
}

// PubData veröffentlicht die Daten auf dem MQTT-Broker
func pubData(data []map[string]interface{}, deviceName string, server *MQTT.Server) error {

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
		err = server.Publish(topic, []byte(payload), false, 2)
		if err != nil {
			logrus.Errorf("S7: Failed to publish data for datapoint %s: %v", name, err)
			return nil
		}
	}
	return nil
}
