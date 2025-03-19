package s7

import s7 "github.com/robinson/gos7"

//
// logic.go types
//
// Gerätekonfigurationsstruktur
type DeviceConfig struct {
	Type            string               `json:"type"`
	Name            string               `json:"name"`
	Address         string               `json:"address"`
	AcquisitionTime int                  `json:"acquisitionTime"`
	Rack            int                  `json:"rack"`
	Slot            int                  `json:"slot"`
	Datapoint       []Datapoint          `json:"datapoints"`
	PLCClient       *s7.Client           `json:"-"`
	PLCHandler      *s7.TCPClientHandler `json:"-"`
	ID              string               `json:"id"` // ID des Geräts

}

// Datapoint defines a single datapoint to be read from the PLC
type Datapoint struct {
	Name     string `json:"name"`
	DataType string `json:"datatype"`
	Address  string `json:"address"`
}

//
// s7-connector.go types
//
const (
	Input VariableType = iota
	Output
	Merker
	DataBlock
)

type VariableType int

type ParsedAddress struct {
	Type     VariableType
	DBNum    int
	ByteAddr int
	BitAddr  int
	DataType string
}
