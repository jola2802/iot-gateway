package opcua

import (
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

// ConvData konvertiert die OPC-UA-Daten in ein MQTT-kompatibles Format (konvertiert die nodes aus der config-datei in eine Map von Strings)
func convData(client *opcua.Client, data []*ua.DataValue, nodes []DataNode) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for i, value := range data {
		if value == nil {
			logrus.Warnf("OPC-UA: No value for node %s, skipping conversion.", nodes[i].ID)
			continue
		}
		// result[nodes[i].ID] = value.Value.Value()
		result["["+nodes[i].ID+"] "+nodes[i].Name] = value.Value.Value()
	}
	return result, nil
}
