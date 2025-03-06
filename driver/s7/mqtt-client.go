package s7

import (
	"encoding/json"
	"fmt"

	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/sirupsen/logrus"
)

// PubData veröffentlicht die Daten auf dem MQTT-Broker
func pubData(data []map[string]interface{}, deviceName string, server *MQTT.Server) error {

	for _, dp := range data {
		// name muss aus [DatapointId]_[DatapointName] bestehen
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

		// name muss aus [DatapointId]_DatapointName bestehen
		id := dp["id"].(string)
		name = fmt.Sprintf("[%s] %s", id, name)

		topic := fmt.Sprintf("data/s7/%s/%s", deviceName, name)
		err = server.Publish(topic, []byte(payload), false, 2)
		if err != nil {
			logrus.Errorf("S7: Failed to publish data for datapoint %s: %v", name, err)
			return nil
		}
	}
	return nil
}
