package dataforwarding

// Struktur für eine DataRoute
type DataRoute struct {
	ID              int      `json:"id"`
	DestinationType string   `json:"destinationType"` // REST, File, MQTT (intern), External MQTT
	DataFormat      string   `json:"dataFormat"`
	Interval        string   `json:"interval"`
	Devices         []string `json:"devices"`
	DestinationURL  string   `json:"destinationUrl,omitempty"`
	Headers         []Header `json:"headers,omitempty"`
	FilePath        string   `json:"filePath,omitempty"`
	Status          string   `json:"status,omitempty"`
}

// InfluxConfig speichert die InfluxDB-Konfiguration, die aus der SQLite-Tabelle influxdb geladen wird.
type InfluxConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// DeviceData speichert die Daten der Geräte, die aus der SQLite-Tabelle device geladen werden.
type DeviceData struct {
	DeviceName  string
	DeviceId    string
	Datapoint   string
	DatapointId string
	Value       interface{}
	Timestamp   string
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DataPoint struct {
	DatapointId string `json:"DatapointId"`
	Value       string `json:"Value"`
	Timestamp   string `json:"Timestamp"`
}

// DataReading enthält das Format für das JSON-Objekt, das gesendet wird
type DataReading struct {
	DatapointId string `json:"DatapointId"`
	Value       string `json:"Value"`
	Timestamp   string `json:"Timestamp"`
}
