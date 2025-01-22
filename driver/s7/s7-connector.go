package s7

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"iot-gateway/driver/opcua"
	"math"
	"strconv"
	"strings"
	"time"

	s7 "github.com/robinson/gos7"
	"github.com/sirupsen/logrus"
)

// InitClient initializes a connection to an S7 PLC using the provided device configuration,
// creates a new S7 client, reads data from the PLC, and returns the results.
//
// Parameters:
//   - device: An opcua.DeviceConfig struct containing the configuration for the PLC connection.
//
// Returns:
//   - A slice of maps containing the read data, or an error if the connection or data reading fails.
//
// Errors:
//   - Returns an error if the S7 handler cannot be created or connected to the PLC.
//   - Returns an error if the client cannot be created.
//   - Returns an error if reading data from the PLC fails.
func InitClient(device opcua.DeviceConfig) ([]map[string]interface{}, error) {
	handler, err := NewS7Handler(device.Name, device.Address, device.Rack, device.Slot)
	if err != nil {
		logrus.Errorf("S7: could not connect to PLC %s: %v", device.Name, err)
		return nil, err
	}
	defer handler.Close()

	// Initialisierung des Clients
	client := s7.NewClient(handler)

	if client == nil {
		handler.Close()
		logrus.Errorf("S7: could not create client for PLC %s", device.Name)
		return nil, nil
	}

	// Lesen aller Variablenendpunkte entsprechend der Umwandlung
	results, err := ReadData(client, device)
	if err != nil {
		logrus.Errorf("S7: failed to read data for PLC %s: %v", device.Name, err)
		return nil, err
	}

	return results, nil
}

// NewS7Handler creates a new TCP client handler for an S7 PLC device.
// It initializes the handler with the provided address, rack, and slot,
// and sets the connection and idle timeouts. The function attempts to
// connect to the PLC and returns the handler if successful, or an error
// if the connection fails.
//
// Parameters:
//   - deviceName: A string representing the name of the device.
//   - address: A string representing the IP address of the PLC.
//   - rack: An integer representing the rack number of the PLC.
//   - slot: An integer representing the slot number of the PLC.
//
// Returns:
//   - *s7.TCPClientHandler: A pointer to the initialized TCP client handler.
//   - error: An error if the connection to the PLC fails, otherwise nil.
func NewS7Handler(deviceName string, address string, rack int, slot int) (*s7.TCPClientHandler, error) {
	handler := s7.NewTCPClientHandler(address, rack, slot)
	handler.Timeout = 200 * time.Second
	handler.IdleTimeout = 500 * time.Second

	if err := handler.Connect(); err != nil {
		// logrus.Errorf("S7: Failed to connect to PLC %s: %v", deviceName, err)
		return nil, err
	}

	return handler, nil
}

type ParsedAddress struct {
	Type     VariableType
	DBNum    int
	ByteAddr int
	BitAddr  int
	DataType string
}

type VariableType int

const (
	Input VariableType = iota
	Output
	Merker
	DataBlock
)

// Wandelt die Variablenadressen um in einzelne Byte und Bit-Werte sowie die entsprechenden Variablentypen
func ParseAddress(address string) (ParsedAddress, error) {
	var parsed ParsedAddress
	var err error

	// Detect the variable type (Input, Output, Merker, DataBlock)
	switch {
	case strings.HasPrefix(address, "I"):
		parsed.Type = Input
		address = strings.TrimPrefix(address, "I")
	case strings.HasPrefix(address, "Q"):
		parsed.Type = Output
		address = strings.TrimPrefix(address, "Q")
	case strings.HasPrefix(address, "M"):
		parsed.Type = Merker
		address = strings.TrimPrefix(address, "M")
	case strings.HasPrefix(address, "DB"):
		parsed.Type = DataBlock
		address = strings.TrimPrefix(address, "DB")
		parts := strings.SplitN(address, ".", 2)
		if len(parts) != 2 {
			return parsed, fmt.Errorf("invalid DB address format")
		}
		parsed.DBNum, err = strconv.Atoi(parts[0])
		if err != nil {
			return parsed, fmt.Errorf("invalid DB number: %v", err)
		}
		address = parts[1]
	default:
		return parsed, fmt.Errorf("unknown variable type")
	}

	// Determine the data type based on the address suffix
	if strings.HasPrefix(address, "W") {
		parsed.DataType = "Word"
		address = strings.TrimLeft(address, "W")
	} else if strings.HasPrefix(address, "D") {
		parsed.DataType = "DWord"
		address = strings.TrimLeft(address, "D")
	} else {
		parsed.DataType = "Byte" // Default to BYTE if no suffix is found
	}

	// Now split the address by "." to separate byte and bit part (if present)
	parts := strings.Split(address, ".")
	if len(parts) > 2 {
		return parsed, fmt.Errorf("invalid address format")
	}

	// Parse the byte part
	parsed.ByteAddr, err = strconv.Atoi(parts[0])
	if err != nil {
		return parsed, fmt.Errorf("invalid byte address: %v", err)
	}

	// Handle bit part only for BYTE type and if a bit address is provided
	if parsed.DataType == "Byte" && len(parts) == 2 {
		parsed.BitAddr, err = strconv.Atoi(parts[1])
		if err != nil {
			return parsed, fmt.Errorf("invalid bit address: %v", err)
		}
		// Ensure bit address is in the valid range (0-7 for S7 systems)
		if parsed.BitAddr < 0 || parsed.BitAddr > 7 {
			return parsed, fmt.Errorf("bit address out of range: %d", parsed.BitAddr)
		}
	} else {
		// No bit address for WORD, DWORD or BYTE without bit specification
		parsed.BitAddr = -1 // Set to -1 if no bit address is provided
	}
	return parsed, nil
}

// ReadData liest die Daten von den S7-Geräten
func ReadData(client s7.Client, device opcua.DeviceConfig) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, len(device.Datapoint))
	for i, dp := range device.Datapoint {
		parsedAddr, err := ParseAddress(dp.Address)
		if err != nil {
			return nil, fmt.Errorf("S7: failed to parse address %s: %v", dp.Address, err)
		}

		var value interface{}

		switch parsedAddr.Type {
		case Input:
			value, err = ReadInputValue(client, parsedAddr, dp.Datatype)
		case Output:
			value, err = ReadOutputValue(client, parsedAddr, dp.Datatype)
		case Merker:
			value, err = ReadMerkerValue(client, parsedAddr, dp.Datatype)
		case DataBlock:
			value, err = ReadDBValue(client, parsedAddr, dp.Datatype)
		default:
			return nil, fmt.Errorf("S7: unsupported variable type %v", parsedAddr.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("S7: failed to read data from address %s: %v", dp.Address, err)
		}

		results[i] = map[string]interface{}{
			"name":  dp.Name,
			"value": value,
		}
	}

	return results, nil
}

func ReadInputValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	var size int
	switch addr.DataType {
	case "Byte":
		size = 1
	case "Word":
		size = 2
	case "DWord":
		size = 4
	}
	buffer := make([]byte, size)
	err := client.AGReadEB(addr.ByteAddr, size, buffer)
	if err != nil {
		return nil, err
	}
	if addr.BitAddr >= 0 {
		return (buffer[0] >> addr.BitAddr) & 1, nil
	}
	// Rückgabe der Daten entsprechend des Datentyps
	switch datatype {
	case "BOOL":
		if addr.BitAddr >= 0 {
			return (buffer[0] >> addr.BitAddr) & 1, nil
		}
		return nil, fmt.Errorf("invalid bit address for BOOL type")
	case "BYTE":
		return buffer[0], nil
	case "INT":
		return int16(binary.BigEndian.Uint16(buffer)), nil
	case "DINT":
		return int16(binary.BigEndian.Uint16(buffer)), nil
	case "REAL":
		bits := binary.BigEndian.Uint32(buffer)
		return math.Float32frombits(bits), nil
	case "WORD":
		return binary.BigEndian.Uint16(buffer), nil
	case "DWORD":
		return binary.BigEndian.Uint32(buffer), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", datatype)
	}
}

func WriteInputValue(client s7.Client, addr ParsedAddress, value interface{}) error {
	buffer := make([]byte, 4)
	err := client.AGReadEB(addr.ByteAddr, 1, buffer)
	if err != nil {
		return err
	}

	if addr.BitAddr > 0 {
		if bitValue, ok := value.(int); ok {
			if bitValue == 1 {
				buffer[0] |= (1 << addr.BitAddr)
			} else {
				buffer[0] &^= (1 << addr.BitAddr)
			}
		} else {
			return fmt.Errorf("invalid bit value type")
		}
	} else {
		if byteValue, ok := value.(byte); ok {
			buffer[0] = byteValue
		} else {
			return fmt.Errorf("invalid byte value type")
		}
	}

	return client.AGWriteEB(addr.ByteAddr, 1, buffer)
}

func ReadOutputValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	var size int
	switch addr.DataType {
	case "Byte":
		size = 1
	case "Word":
		size = 2
	case "DWord":
		size = 4
	}
	buffer := make([]byte, size)

	err := client.AGReadAB(addr.ByteAddr, size, buffer)
	if err != nil {
		return nil, err
	}
	// Rückgabe der Daten entsprechend des Datentyps
	switch datatype {
	case "BOOL":
		if addr.BitAddr >= 0 {
			return (buffer[0] >> addr.BitAddr) & 1, nil
		}
		return nil, fmt.Errorf("invalid bit address for BOOL type")
	case "BYTE":
		return buffer[0], nil
	case "INT":
		// INT ist ein int16, konvertiert mit BigEndian
		return int16(binary.BigEndian.Uint16(buffer)), nil
	case "DINT":
		// INT ist ein int16, konvertiert mit BigEndian
		return int16(binary.BigEndian.Uint16(buffer)), nil
	case "REAL":
		// REAL ist eine 32-Bit-Gleitkommazahl (float32)
		bits := binary.BigEndian.Uint32(buffer)
		return math.Float32frombits(bits), nil
	case "WORD":
		// WORD ist ein uint16, konvertiert mit BigEndian
		return binary.BigEndian.Uint16(buffer), nil
	case "DWORD":
		// DWORD ist ein uint32, konvertiert mit BigEndian
		return binary.BigEndian.Uint32(buffer), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", datatype)
	}
}

func WriteOutputValue(client s7.Client, addr ParsedAddress, value interface{}) error {
	var size int
	switch addr.DataType {
	case "Byte":
		size = 1
	case "Word":
		size = 2
	case "DWord":
		size = 4
	}
	buffer := make([]byte, size)

	if addr.BitAddr > 0 {
		if bitValue, ok := value.(int); ok {
			if bitValue == 1 {
				buffer[0] |= (1 << addr.BitAddr)
			} else {
				buffer[0] &^= (1 << addr.BitAddr)
			}
		} else {
			return fmt.Errorf("invalid bit value type")
		}
	} else {
		switch v := value.(type) {
		case byte:
			buffer[0] = v
		case int16:
			binary.BigEndian.PutUint16(buffer, uint16(v))
		case int32:
			binary.BigEndian.PutUint32(buffer, uint32(v))
		case float32:
			binary.BigEndian.PutUint32(buffer, math.Float32bits(v))
		default:
			return fmt.Errorf("invalid value type for write")
		}
	}

	return client.AGWriteAB(addr.ByteAddr, size, buffer)
}

func ReadMerkerValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	var size int
	switch addr.DataType {
	case "Byte":
		size = 1
	case "Word":
		size = 2
	case "DWord":
		size = 4
	}
	buffer := make([]byte, size)

	err := client.AGReadMB(addr.ByteAddr, size, buffer)
	if err != nil {
		return nil, err
	}

	// Rückgabe der Daten basierend auf dem Datentyp
	switch datatype {
	case "BOOL":
		if addr.BitAddr >= 0 {
			return (buffer[0] >> addr.BitAddr) & 1, nil
		}
		return nil, fmt.Errorf("invalid bit address for BOOL type")
	case "BYTE":
		return buffer[0], nil
	case "INT":
		return int16(binary.BigEndian.Uint16(buffer)), nil
	case "DINT":
		return int32(binary.BigEndian.Uint32(buffer)), nil
	case "REAL":
		bits := binary.BigEndian.Uint32(buffer)
		return math.Float32frombits(bits), nil
	case "WORD":
		return binary.BigEndian.Uint16(buffer), nil
	case "DWORD":
		return binary.BigEndian.Uint32(buffer), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", datatype)
	}
}

func WriteMerkerValue(client s7.Client, addr ParsedAddress, value interface{}) error {
	var size int
	switch addr.DataType {
	case "Byte":
		size = 1
	case "Word":
		size = 2
	case "DWord":
		size = 4
	}
	buffer := make([]byte, size)

	if addr.BitAddr > 0 {
		if bitValue, ok := value.(int); ok {
			if bitValue == 1 {
				buffer[0] |= (1 << addr.BitAddr)
			} else {
				buffer[0] &^= (1 << addr.BitAddr)
			}
		} else {
			return fmt.Errorf("invalid bit value type")
		}
	} else {
		switch v := value.(type) {
		case byte:
			buffer[0] = v
		case int16:
			binary.BigEndian.PutUint16(buffer, uint16(v))
		case int32:
			binary.BigEndian.PutUint32(buffer, uint32(v))
		case float32:
			binary.BigEndian.PutUint32(buffer, math.Float32bits(v))
		default:
			return fmt.Errorf("invalid value type for write")
		}
	}

	return client.AGWriteMB(addr.ByteAddr, size, buffer)
}

func ReadDBValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	var size int
	switch addr.DataType {
	case "Byte":
		size = 1
	case "Word":
		size = 2
	case "DWord":
		size = 4
	}
	buffer := make([]byte, size)

	// Lese die Daten aus dem Datenbaustein (DB)
	err := client.AGReadDB(addr.DBNum, addr.ByteAddr, size, buffer)
	if err != nil {
		return nil, err
	}

	// Rückgabe der Daten basierend auf dem Datentyp
	switch datatype {
	case "BOOL":
		if addr.BitAddr >= 0 {
			return (buffer[0] >> addr.BitAddr) & 1, nil
		}
		return nil, fmt.Errorf("invalid bit address for BOOL type")
	case "BYTE":
		return buffer[0], nil
	case "INT":
		return int16(binary.BigEndian.Uint16(buffer)), nil
	case "DINT":
		return int32(binary.BigEndian.Uint32(buffer)), nil
	case "REAL":
		bits := binary.BigEndian.Uint32(buffer)
		return math.Float32frombits(bits), nil
	case "WORD":
		return binary.BigEndian.Uint16(buffer), nil
	case "DWORD":
		return binary.BigEndian.Uint32(buffer), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", datatype)
	}
}

func WriteDBValue(client s7.Client, addr ParsedAddress, value interface{}) error {
	var size int
	switch addr.DataType {
	case "Byte":
		size = 1
	case "Word":
		size = 2
	case "DWord":
		size = 4
	}
	buffer := make([]byte, size)

	if addr.BitAddr > 0 {
		if bitValue, ok := value.(int); ok {
			if bitValue == 1 {
				buffer[0] |= (1 << addr.BitAddr)
			} else {
				buffer[0] &^= (1 << addr.BitAddr)
			}
		} else {
			return fmt.Errorf("invalid bit value type")
		}
	} else {
		switch v := value.(type) {
		case byte:
			buffer[0] = v
		case int16:
			binary.BigEndian.PutUint16(buffer, uint16(v))
		case int32:
			binary.BigEndian.PutUint32(buffer, uint32(v))
		case float32:
			binary.BigEndian.PutUint32(buffer, math.Float32bits(v))
		default:
			return fmt.Errorf("invalid value type for write")
		}
	}

	return client.AGWriteDB(addr.DBNum, addr.ByteAddr, size, buffer)
}

// GetConfigByName gibt die Gerätekonfiguration für einen Gerätenamen zurück
func GetConfigByName(db *sql.DB, name string) (*DeviceConfig, error) {
	var device DeviceConfig

	// SQL-Abfrage, um das Gerät mit dem Namen aus der Tabelle devices zu holen
	deviceQuery := `
		SELECT id, type, name, address,  acquisition_time, rack, slot
		FROM devices
		WHERE name = ?
		LIMIT 1
	`

	// Gerätedaten aus der Datenbank holen
	err := db.QueryRow(deviceQuery, name).Scan(&device.ID, &device.Type, &device.Name, &device.Address, &device.AcquisitionTime, &device.Rack, &device.Slot)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("S7: device config for %s not found", name)
		}
		logrus.Errorf("S7: could not query device config for %s: %v", name, err)
		return nil, err
	}

	// SQL-Abfrage, um die Datenpunkte des Geräts aus der Tabelle s7_datapoints zu holen
	datapointQuery := `
		SELECT name, datatype, address
		FROM s7_datapoints
		WHERE device_id = ?
	`

	// Datapoints laden
	rows, err := db.Query(datapointQuery, device.ID)
	if err != nil {
		logrus.Errorf("S7: could not query datapoints for device %s: %v", name, err)
		return nil, err
	}
	defer rows.Close()

	// Datapoints zur Gerätkonfiguration hinzufügen
	for rows.Next() {
		var dp Datapoint
		err := rows.Scan(&dp.Name, &dp.DataType, &dp.Address)
		if err != nil {
			logrus.Errorf("S7: failed to scan datapoint for device %s: %v", name, err)
			return nil, err
		}
		device.Datapoint = append(device.Datapoint, dp)
	}

	if err := rows.Err(); err != nil {
		logrus.Errorf("S7: error reading datapoints for device %s: %v", name, err)
		return nil, err
	}

	return &device, nil
}

// UpdateDataPoint aktualisiert einen S7-Datenpunkt (wenn er eine entsprechende MQTT-Nachricht erhalten hat)
func UpdateDataPoint(device *DeviceConfig, address string, value interface{}) error {
	handler, err := NewS7Handler(device.Name, device.Address, device.Rack, device.Slot)
	if err != nil {
		return fmt.Errorf("S7: could not connect to PLC %s: %v", device.Name, err)
	}
	defer handler.Close()

	client := s7.NewClient(handler)
	if client == nil {
		return fmt.Errorf("S7: could not create client for PLC %s", device.Name)
	}

	parsedAddr, err := ParseAddress(address)
	if err != nil {
		return fmt.Errorf("S7: failed to parse address %s: %v", address, err)
	}

	switch parsedAddr.Type {
	case Input:
		return WriteInputValue(client, parsedAddr, value)
	case Output:
		return WriteOutputValue(client, parsedAddr, value)
	case Merker:
		return WriteMerkerValue(client, parsedAddr, value)
	case DataBlock:
		return WriteDBValue(client, parsedAddr, value)
	default:
		return fmt.Errorf("S7: unknown variable type for address %s", address)
	}
}
