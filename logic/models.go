package logic

import "sync"

// <---------------------------------------->
// Models for driver_manager.go und S7 und OPC-UA
// <---------------------------------------->

// Gerätestatus enthält individuellen Mutex und Stop-Kanal
type DeviceState struct {
	mu       sync.RWMutex
	running  bool
	status   string
	stopChan chan struct{}
}

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
