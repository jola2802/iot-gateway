package opcua

import (
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

// ConvData konvertiert die OPC-UA-Daten in ein MQTT-kompatibles Format (konvertiert die nodes aus der config-datei in eine Map von Strings)
func ConvData(client *opcua.Client, data []*ua.DataValue, selectedNodes []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for i, value := range data {
		if value == nil {
			logrus.Warnf("OPC-UA: No value for node %s, skipping conversion.", selectedNodes[i])
			continue
		}
		nodeName, err := GetNodeName(client, selectedNodes[i])
		if err != nil {
			logrus.Errorf("OPC-UA: failed to get node name: %v", err)
			return nil, err
		}
		result[nodeName] = value.Value.Value()
	}
	return result, nil
}
