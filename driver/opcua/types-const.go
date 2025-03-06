package opcua

import "github.com/gopcua/opcua"

// opcua-connector.go types

var opcuaClients = make(map[string]*opcua.Client) // Map to store OPC-UA clients by device name

// logic.go types

// Konfigurationsstruktur
type Config struct {
	Devices []DeviceConfig `json:"devices"`
	// MqttBroker MqttConfig     `json:"mqttBroker"`
}

// MQTT-Konfigurationsstruktur
type MqttConfig struct {
	Broker     string `json:"broker"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	CACert     string `json:"caCert,omitempty"`
	ClientCert string `json:"clientCert,omitempty"`
	ClientKey  string `json:"clientKey,omitempty"`
}

// Gerätekonfigurationsstruktur
type DeviceConfig struct {
	ID              string      `json:"id"`
	Type            string      `json:"type"`
	Name            string      `json:"name"`
	Address         string      `json:"address"`
	SecurityMode    string      `json:"securityMode,omitempty"`   // Only for OPC UA
	SecurityPolicy  string      `json:"securityPolicy,omitempty"` // Only for OPC UA
	Datapoint       []Datapoint `json:"datapoints,omitempty"`     // Only for S7
	DataNode        []DataNode  `json:"dataNodes,omitempty"`      // Only for OPC UA
	AcquisitionTime int         `json:"acquisitionTime"`
	CertFile        string      `json:"certificate,omitempty"` // Only for OPC UA
	KeyFile         string      `json:"key,omitempty"`         // Only for OPC UA
	Username        string      `json:"username,omitempty"`    // Only for OPC UA
	Password        string      `json:"password,omitempty"`    // Only for OPC UA
	Rack            int         `json:"rack,omitempty"`        // Only for S7
	Slot            int         `json:"slot,omitempty"`        // Only for S7
}

type Datapoint struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Datatype string `json:"datatype"`
	Address  string `json:"address"`
}

type DataNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Node string `json:"node"`
}
