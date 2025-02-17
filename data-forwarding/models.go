package dataforwarding

// Struktur für eine DataRoute
type DataRoute struct {
	ID              int      `json:"id"`
	DestinationType string   `json:"destinationType"` // REST, File, MQTT (intern), External MQTT
	DataFormat      string   `json:"dataFormat"`
	Interval        int      `json:"interval"`
	Devices         []string `json:"devices"`
	DestinationURL  string   `json:"destinationUrl,omitempty"`
	Headers         []Header `json:"headers,omitempty"`
	FilePath        string   `json:"filePath,omitempty"`
	Status          string   `json:"status,omitempty"`
}
