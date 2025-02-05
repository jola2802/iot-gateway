package opcua

import (
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

// ConvData konvertiert die OPC-UA-Daten in ein MQTT-kompatibles Format (konvertiert die nodes aus der config-datei in eine Map von Strings)
func ConvData(client *opcua.Client, data []*ua.DataValue, nodes []DataNode) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for i, value := range data {
		if value == nil {
			logrus.Warnf("OPC-UA: No value for node %s, skipping conversion.", nodes[i])
			continue
		}
		// Verwenden Sie direkt den in der Konfiguration definierten Namen, anstatt den Namen erneut vom Server abzurufen.
		result[nodes[i].Name] = value.Value.Value()
	}
	return result, nil
}
