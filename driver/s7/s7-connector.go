package s7

import (
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

// Erstellt einen S7-Client und stellt die Verbindung zur S7-Steuerung her
func createS7Client(device opcua.DeviceConfig) (s7.Client, *s7.TCPClientHandler, error) {
	handler, err := newS7Handler(device.Address, device.Rack, device.Slot)
	if err != nil {
		logrus.Errorf("S7: could not connect to PLC %s: %v", device.Name, err)
		return nil, nil, err
	}

	client := s7.NewClient(handler)
	if client == nil {
		handler.Close()
		logrus.Errorf("S7: could not create client for PLC %s", device.Name)
		return nil, nil, fmt.Errorf("could not create S7 client")
	}

	return client, handler, nil
}

// Liest die Daten von einer bestehenden S7-Verbindung
func fetchS7Data(client s7.Client, device opcua.DeviceConfig) ([]map[string]interface{}, error) {
	results, err := readData(client, device)
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
func newS7Handler(address string, rack int, slot int) (*s7.TCPClientHandler, error) {
	handler := s7.NewTCPClientHandler(address, rack, slot)
	handler.Timeout = 10 * time.Second

	if err := handler.Connect(); err != nil {
		// logrus.Errorf("S7: Failed to connect to PLC %s: %v", deviceName, err)
		return nil, err
	}

	return handler, nil
}

// parseAddress converts variable addresses into individual byte and bit values as well as the corresponding variable types.
//
// Parameters:
//   - address: A string representing the address of the variable.
//
// Returns:
//   - ParsedAddress: A struct containing the parsed address information.
//   - error: An error if the address format is invalid.
func parseAddress(address string, datatype string) (ParsedAddress, error) {
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

		// Unterstütze sowohl DB5.0.0 oder DB5.0 Format
		if strings.Contains(address, ".") {
			// Neues Format: DB5.0.0 oder DB5.0 usw.
			parts := strings.SplitN(address, ".", 2)
			if len(parts) != 2 {
				return parsed, fmt.Errorf("invalid DB address format")
			}

			// DB-Nummer extrahieren
			parsed.DBNum, err = strconv.Atoi(parts[0])
			if err != nil {
				return parsed, fmt.Errorf("invalid DB number: %v", err)
			}

			// Adresstyp und Offset extrahieren
			if strings.Contains(parts[1], ".") {
				addrParts := strings.SplitN(parts[1], ".", 2)
				if len(addrParts) != 2 {
					return parsed, fmt.Errorf("invalid DB address format")
				}

				// Bit Offset extrahieren
				parsed.BitAddr, err = strconv.Atoi(addrParts[1])
				if err != nil {
					return parsed, fmt.Errorf("invalid bit address: %v", err)
				}

				// Datentyp bestimmen
				parsed.DataType = datatype

				return parsed, nil
			} else {
				parsed.ByteAddr, err = strconv.Atoi(parts[1])
				if err != nil {
					return parsed, fmt.Errorf("invalid byte address: %v", err)
				}
				parsed.BitAddr = -1

				// Datentyp bestimmen
				parsed.DataType = datatype

				return parsed, nil
			}

		} else {
			// Altes Format: DB5.DBX0.1
			parts := strings.SplitN(address, ".", 2)
			if len(parts) != 2 {
				return parsed, fmt.Errorf("invalid DB address format")
			}
			parsed.DBNum, err = strconv.Atoi(parts[0])
			if err != nil {
				return parsed, fmt.Errorf("invalid DB number: %v", err)
			}
			parsed.ByteAddr, _ = strconv.Atoi(parts[1])

			// Datentyp bestimmen
			parsed.DataType = datatype

			return parsed, nil
		}
	default:
		return parsed, fmt.Errorf("unknown variable type")
	}

	parsed.DataType = datatype

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

	// Handle bit part only for BOOL type and if a bit address is provided
	if datatype == "BOOL" && len(parts) == 2 {
		parsed.BitAddr, err = strconv.Atoi(parts[1])
		if err != nil {
			return parsed, fmt.Errorf("invalid bit address: %v", err)
		}
		// Ensure bit address is in the valid range (0-7 for S7 systems)
		if parsed.BitAddr < 0 || parsed.BitAddr > 7 {
			return parsed, fmt.Errorf("bit address out of range: %d", parsed.BitAddr)
		}
	} else {
		// No bit address for other data types
		parsed.BitAddr = -1 // Set to -1 if no bit address is provided
	}
	return parsed, nil
}

// readData reads all variable endpoints according to the conversion.
//
// Parameters:
//   - client: An s7.Client instance to communicate with the PLC.
//   - device: An opcua.DeviceConfig struct containing the configuration for the PLC connection.
//
// Returns:
//   - A slice of maps containing the read data, or an error if reading data from the PLC fails.
func readData(client s7.Client, device opcua.DeviceConfig) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, len(device.Datapoint))
	for i, dp := range device.Datapoint {
		parsedAddr, err := parseAddress(dp.Address, dp.Datatype)
		if err != nil {
			return nil, fmt.Errorf("S7: failed to parse address %s: %v", dp.Address, err)
		}

		var value any

		switch parsedAddr.Type {
		case Input:
			value, err = readInputValue(client, parsedAddr, dp.Datatype)
		case Output:
			value, err = readOutputValue(client, parsedAddr, dp.Datatype)
		case Merker:
			value, err = readMerkerValue(client, parsedAddr, dp.Datatype)
		case DataBlock:
			value, err = readDBValue(client, parsedAddr, dp.Datatype)
		default:
			return nil, fmt.Errorf("S7: unsupported variable type %v", parsedAddr.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("S7: failed to read data from address %s: %v", dp.Address, err)
		}

		results[i] = map[string]interface{}{
			"id":    dp.ID,
			"name":  dp.Name,
			"value": value,
		}
	}

	return results, nil
}

// Zentrale Funktion zur Konvertierung der Bytes in den entsprechenden Datentyp
func convertBufferToType(buffer []byte, datatype string, bitAddr int) (interface{}, error) {
	switch strings.ToUpper(datatype) {
	case "BOOL":
		if bitAddr >= 0 {
			return (buffer[0] >> bitAddr) & 1, nil
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
	case "STRING":
		// Entferne Nullbytes am Ende
		return strings.TrimRight(string(buffer), "\x00"), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", datatype)
	}
}

// readInputValue Funktion
func readInputValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	size, err := getDataTypeSize(addr.DataType)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, size)
	if err := client.AGReadEB(addr.ByteAddr, size, buffer); err != nil {
		return nil, err
	}

	return convertBufferToType(buffer, datatype, addr.BitAddr)
}

// writeInputValue writes a value to an input on an S7 client.
//
// Parameters:
//   - client: An s7.Client instance to communicate with the PLC.
//   - addr: A ParsedAddress struct containing the parsed address information.
//   - value: The value to be written.
//
// Returns:
//   - An error if writing the value fails.
func writeInputValue(client s7.Client, addr ParsedAddress, value interface{}) error {
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

// readOutputValue Funktion
func readOutputValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	size, err := getDataTypeSize(addr.DataType)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, size)
	if err := client.AGReadAB(addr.ByteAddr, size, buffer); err != nil {
		return nil, err
	}

	return convertBufferToType(buffer, datatype, addr.BitAddr)
}

// writeOutputValue writes a value to an output on an S7 client.
//
// Parameters:
//   - client: An s7.Client instance to communicate with the PLC.
//   - addr: A ParsedAddress struct containing the parsed address information.
//   - value: The value to be written.
//
// Returns:
//   - An error if writing the value fails.
func writeOutputValue(client s7.Client, addr ParsedAddress, value interface{}) error {
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

// readMerkerValue Funktion
func readMerkerValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	size, err := getDataTypeSize(addr.DataType)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, size)
	if err := client.AGReadMB(addr.ByteAddr, size, buffer); err != nil {
		return nil, err
	}

	return convertBufferToType(buffer, datatype, addr.BitAddr)
}

// writeMerkerValue writes a value to a marker on an S7 client.
//
// Parameters:
//   - client: An s7.Client instance to communicate with the PLC.
//   - addr: A ParsedAddress struct containing the parsed address information.
//   - value: The value to be written.
//
// Returns:
//   - An error if writing the value fails.
func writeMerkerValue(client s7.Client, addr ParsedAddress, value interface{}) error {
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

// readDBValue Funktion
func readDBValue(client s7.Client, addr ParsedAddress, datatype string) (interface{}, error) {
	size, err := getDataTypeSize(addr.DataType)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, size)
	if err := client.AGReadDB(addr.DBNum, addr.ByteAddr, size, buffer); err != nil {
		return nil, err
	}

	return convertBufferToType(buffer, datatype, addr.BitAddr)
}

// writeDBValue writes a value to a data block on an S7 client.
//
// Parameters:
//   - client: An s7.Client instance to communicate with the PLC.
//   - addr: A ParsedAddress struct containing the parsed address information.
//   - value: The value to be written.
//
// Returns:
//   - An error if writing the value fails.
func writeDBValue(client s7.Client, addr ParsedAddress, value interface{}) error {
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

// updateDataPoint updates an S7 data point when it receives a corresponding MQTT message.
//
// Parameters:
//   - device: A pointer to a DeviceConfig struct containing the configuration for the PLC connection.
//   - address: A string representing the address of the variable.
//   - value: The value to be written.
//
// Returns:
//   - An error if updating the data point fails.
func updateDataPoint(device *DeviceConfig, address string, value interface{}) error {
	handler, err := newS7Handler(device.Address, device.Rack, device.Slot)
	if err != nil {
		return fmt.Errorf("S7: could not connect to PLC %s: %v", device.Name, err)
	}
	defer handler.Close()

	client := s7.NewClient(handler)
	if client == nil {
		return fmt.Errorf("S7: could not create client for PLC %s", device.Name)
	}

	// suche nach datatype dort wo address steht
	datatype := ""
	for _, dp := range device.Datapoint {
		if dp.Address == address {
			datatype = dp.DataType
			break
		}
	}

	parsedAddr, err := parseAddress(address, datatype)
	if err != nil {
		return fmt.Errorf("S7: failed to parse address %s: %v", address, err)
	}

	switch parsedAddr.Type {
	case Input:
		return writeInputValue(client, parsedAddr, value)
	case Output:
		return writeOutputValue(client, parsedAddr, value)
	case Merker:
		return writeMerkerValue(client, parsedAddr, value)
	case DataBlock:
		return writeDBValue(client, parsedAddr, value)
	default:
		return fmt.Errorf("S7: unknown variable type for address %s", address)
	}
}

// Zentrale Funktion zur Bestimmung der Größe und Validierung des Datentyps
func getDataTypeSize(dataType string) (int, error) {
	switch strings.ToUpper(dataType) {
	case "BOOL":
		return 1, nil
	case "BYTE":
		return 1, nil
	case "WORD", "INT":
		return 2, nil
	case "DWORD", "DINT", "REAL":
		return 4, nil
	case "STRING":
		return 256, nil
	default:
		return 0, fmt.Errorf("unsupported data type: %s", dataType)
	}
}
