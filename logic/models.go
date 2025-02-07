package logic

import "sync"

// <---------------------------------------->
// Models for driver_manager.go und S7 und OPC-UA
// <---------------------------------------->

// type DeviceConfig struct {
// 	Type            string      `json:"type"`
// 	Name            string      `json:"name"`
// 	Address         string      `json:"address"`
// 	SecurityMode    string      `json:"securityMode,omitempty"`   // Only for OPC UA
// 	SecurityPolicy  string      `json:"securityPolicy,omitempty"` // Only for OPC UA
// 	Datapoints      []Datapoint `json:"datapoints,omitempty"`     // Only for S7
// 	DataNodes       []string    `json:"dataNodes,omitempty"`      // Only for OPC UA
// 	AcquisitionTime int         `json:"acquisitionTime"`
// 	Certificate     string      `json:"certificate,omitempty"` // Only for OPC UA
// 	Username        string      `json:"username,omitempty"`    // Only for OPC UA
// 	Password        string      `json:"password,omitempty"`    // Only for OPC UA
// 	Rack            int         `json:"rack,omitempty"`        // Only for S7
// 	Slot            int         `json:"slot,omitempty"`        // Only for S7
// }

// type Config struct {
// 	Devices    []DeviceConfig `json:"devices"`
// 	MqttBroker struct {
// 		Broker   string `json:"broker"`
// 		Username string `json:"username"`
// 		Password string `json:"password"`
// 	}
// }

// type MqttConfig struct {
// 	Broker     string `json:"broker"`
// 	Username   string `json:"username"`
// 	Password   string `json:"password"`
// 	CACert     string `json:"caCert,omitempty"`
// 	ClientCert string `json:"clientCert,omitempty"`
// 	ClientKey  string `json:"clientKey,omitempty"`
// }

// type DeviceCommand struct {
// 	Topic   string
// 	Command string
// }

// Gerätestatus enthält individuellen Mutex und Stop-Kanal
type DeviceState struct {
	mu       sync.RWMutex
	running  bool
	status   string
	stopChan chan struct{}
}

// // Datapoint defines a single datapoint to be read from the PLC
// type Datapoint struct {
// 	Name     string `json:"name"`
// 	Datatype string `json:"datatype"`
// 	Address  string `json:"address"`
// }

// <---------------------------------------->
// Models for user_manager.go
// <---------------------------------------->

// Auth repräsentiert die Authentifizierungsinformationen eines Benutzers
type Auth struct {
	Username string
	Password string
	Allow    bool
}

// Filters speichert MQTT-Zugriffsebenen für Topics
type Filters map[string]int

// ACL repräsentiert die Zugriffssteuerliste eines Benutzers
type ACL struct {
	Username string
	Filters  Filters
}

// Command repräsentiert ein MQTT-User-Management-Kommando
type Command struct {
	Action   string         `json:"action"`
	Username string         `json:"username"`
	Password string         `json:"password"`
	Allow    bool           `json:"allow"`
	Filters  map[string]int `json:"filters,omitempty"`
}

// <---------------------------------------->
// Models for xxx
// <---------------------------------------->
