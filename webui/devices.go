package webui

import (
	"database/sql"
	"fmt"
	"iot-gateway/logic"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/gorilla/websocket"
	MQTT "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/rand"
)

type Device struct {
	ID              int            `json:"id"`
	DeviceType      string         `json:"deviceType"`
	DeviceName      string         `json:"deviceName"`
	Status          string         `json:"status"`
	Value           string         `json:"value"`
	Connected       bool           `json:"connected"`
	Address         string         `json:"address,omitempty"`
	AcquisitionTime int            `json:"acquisitionTime,omitempty"`
	SecurityMode    sql.NullString `json:"securityMode,omitempty"`
	SecurityPolicy  sql.NullString `json:"securityPolicy,omitempty"`
	DataPoint       []struct {
		DatapointId string `json:"datapointId"`
		Name        string `json:"name"`
		Datatype    string `json:"datatype,omitempty"`
		Address     string `json:"address"`
	} `json:"datapoint,omitempty"`
	Rack     sql.NullString `json:"rack,omitempty"`
	Slot     sql.NullString `json:"slot,omitempty"`
	Username sql.NullString `json:"username,omitempty"`
	Password sql.NullString `json:"password,omitempty"`
}

type Datapoint struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type AggregatedData struct {
	DeviceID   string      `json:"device_id"`
	Datapoints []Datapoint `json:"datapoints"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Im Produktivcode sollte diese Funktion verbessert werden
	},
}

// showDevicesPage shows the devices page
func showDevicesPage(c *gin.Context) {
	c.HTML(http.StatusOK, "devices.html", nil)
}

// getDevices returns a list of devices
func getDevices(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var devices []Device
	rows, err := db.Query("SELECT id, name, type, address, acquisition_time, status FROM devices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var device Device
		err := rows.Scan(&device.ID, &device.DeviceName, &device.DeviceType, &device.Address, &device.AcquisitionTime, &device.Status)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Add the device to the list
		devices = append(devices, device)
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// getDevice returns a device by id
func getDevice(c *gin.Context) {
	device_id := c.Param("device_id")

	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var device Device
	device.ID, err = strconv.Atoi(device_id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID"})
		return
	}
	query := `SELECT name, type, address, acquisition_time, rack, slot, security_mode, security_policy, username, password FROM devices WHERE id = ?`
	err = db.QueryRow(query, device.ID).Scan(&device.DeviceName, &device.DeviceType, &device.Address, &device.AcquisitionTime, &device.Rack, &device.Slot, &device.SecurityMode, &device.SecurityPolicy, &device.Username, &device.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		logrus.Info(err)
		return
	}

	// Fetch datapoints for the device
	if device.DeviceType == "opc-ua" {
		query = `SELECT datapointId, name, node_identifier FROM opcua_datanodes WHERE device_id = ?`
	} else if device.DeviceType == "s7" {
		query = `SELECT datapointId, name, datatype, address FROM s7_datapoints WHERE device_id = ?`
	} else if device.DeviceType == "mqtt" {
		// do nothing
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported device type"})
		return
	}

	rows, err := db.Query(query, device.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching data points"})
		logrus.Error("Error querying data points:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var node struct {
			DatapointId string `json:"datapointId"`
			Name        string `json:"name"`
			Datatype    string `json:"datatype,omitempty"`
			Address     string `json:"address"`
		}
		if device.DeviceType == "opc-ua" {
			if err := rows.Scan(&node.DatapointId, &node.Name, &node.Address); err != nil {
			}
			if err := rows.Scan(&node.DatapointId, &node.Name, &node.Address); err != nil {
				logrus.Error("Error scanning OPC-UA node:", err)
				continue
			}
			device.DataPoint = append(device.DataPoint, node)
		} else if device.DeviceType == "s7" {
			if err := rows.Scan(&node.DatapointId, &node.Name, &node.Datatype, &node.Address); err != nil {
				logrus.Error("Error scanning S7 point:", err)
				continue
			}
			device.DataPoint = append(device.DataPoint, node)
		} else if device.DeviceType == "mqtt" {
			// do nothing
		}
	}

	c.JSON(http.StatusOK, gin.H{"device": device})
}

// Funktion zum Abrufen der aktuellen Gerätestatus aus der Datenbank
func getDeviceStatuses(db *sql.DB) map[string]string {
	deviceStatus := make(map[string]string)

	rows, err := db.Query("SELECT id, type, status FROM devices")
	if err != nil {
		logrus.Errorf("Error querying device statuses: %v", err)
		return deviceStatus
	}
	defer rows.Close()

	for rows.Next() {
		var id, deviceType, status string
		if err := rows.Scan(&id, &deviceType, &status); err != nil {
			logrus.Errorf("Error scanning device status: %v", err)
			continue
		}
		key := fmt.Sprintf("%s_%s", deviceType, id)
		deviceStatus[key] = status
	}

	return deviceStatus
}

// deviceDataWebSocket übernimmt die Funktionalität des Node-RED Nodes zur Aggregation
// und Weitergabe der MQTT-Daten (Topic: "data/#") über eine WebSocket-Verbindung.
func deviceDataWebSocket(c *gin.Context) {
	// Token-Überprüfung
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token is required"})
		return
	}

	// WebSocket-Verbindung
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("Error upgrading to WebSocket: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebSocket upgrade failed"})
		return
	}
	defer conn.Close()

	// Datenbankverbindung
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Error getting database connection: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Maps mit Mutex-Schutz
	aggregated := make(map[string]*AggregatedData)
	deviceStatus := make(map[string]string)
	lastSentStatus := make(map[string]string) // Neue Map für den letzten gesendeten Status
	var aggMutex sync.Mutex

	// Verbesserte Status-Update-Verarbeitung
	callbackFn := func(cl *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
		topic := pk.TopicName
		payloadStr := string(pk.Payload)

		if strings.HasPrefix(topic, "driver/states/") {
			parts := strings.Split(topic, "/")
			if len(parts) >= 4 {
				deviceType := parts[2]
				deviceID := parts[3]

				if deviceType == "" || deviceID == "" {
					logrus.Warn("Invalid device type or ID in topic:", topic)
					return
				}

				key := fmt.Sprintf("%s_%s", deviceType, deviceID)

				aggMutex.Lock()
				// Nur aktualisieren wenn sich der Status wirklich geändert hat
				if deviceStatus[key] != payloadStr {
					deviceStatus[key] = payloadStr
				}
				aggMutex.Unlock()
				return
			}
		}

		// Normale Datenpunkt-Verarbeitung
		parts := strings.Split(topic, "/")
		if len(parts) < 3 {
			logrus.Warn("Ungültiges Topic-Format: " + topic)
			return
		}
		deviceID, measurement := parts[2], parts[3]

		var value string
		if num, err := strconv.ParseFloat(payloadStr, 64); err == nil {
			value = fmt.Sprintf("%.2f", num)
		} else {
			value = payloadStr
		}

		updateAggregation(aggregated, &aggMutex, deviceID, measurement, value)
	}

	// Verbesserte Ticker-Behandlung
	dataTicker := time.NewTicker(500 * time.Millisecond)
	statusTicker := time.NewTicker(5 * time.Second)
	defer func() {
		dataTicker.Stop()
		statusTicker.Stop()
	}()

	// Initialer Status mit Fehlerbehandlung
	aggMutex.Lock()
	deviceStatus = getDeviceStatuses(db)
	aggMutex.Unlock()

	// MQTT-Server aus dem Context holen
	server := c.MustGet("server").(*MQTT.Server)

	// Abonniere die MQTT-Topics
	if err := server.Subscribe("data/#", rand.Intn(100), callbackFn); err != nil {
		logrus.Errorf("Error subscribing to topic data/#: %v", err)
		return
	}

	if err := server.Subscribe("driver/states/#", rand.Intn(100), callbackFn); err != nil {
		logrus.Errorf("Error subscribing to topic driver/states/#: %v", err)
		return
	}

	for {
		select {
		case <-dataTicker.C:
			aggMutex.Lock()
			outputArray := buildOutput(&aggregated, deviceStatus)

			// Prüfe ob sich die Status geändert haben
			statusChanged := false
			for key, status := range deviceStatus {
				if lastSentStatus[key] != status {
					statusChanged = true
					lastSentStatus[key] = status
				}
			}

			// Nur senden wenn es Datenpunkte gibt oder sich Status geändert haben
			if len(outputArray) > 0 || statusChanged {
				if err := conn.WriteJSON(outputArray); err != nil {
					logrus.Errorf("Error sending data: %v", err)
					aggMutex.Unlock()
					return
				}
			}
			aggMutex.Unlock()

		case <-statusTicker.C:
			// Aktualisiere die Gerätestatus aus der Datenbank
			aggMutex.Lock()
			newStatus := getDeviceStatuses(db)

			// Prüfe ob sich die Status wirklich geändert haben
			statusChanged := false
			for key, status := range newStatus {
				if deviceStatus[key] != status {
					statusChanged = true
					deviceStatus[key] = status
					lastSentStatus[key] = status
				}
			}

			// Nur senden wenn sich Status geändert haben
			if statusChanged {
				outputArray := buildOutput(&aggregated, deviceStatus)
				if err := conn.WriteJSON(outputArray); err != nil {
					logrus.Errorf("Error sending status update: %v", err)
					aggMutex.Unlock()
					return
				}
			}
			aggMutex.Unlock()
		}
	}
}

// updateAggregation aktualisiert die Aggregation für einen bestimmten Gerät und Messung
func updateAggregation(aggregated map[string]*AggregatedData, aggMutex *sync.Mutex, deviceID, measurement, value string) {
	aggMutex.Lock()
	defer aggMutex.Unlock()

	agg, exists := aggregated[deviceID]
	if !exists {
		agg = &AggregatedData{
			DeviceID:   deviceID,
			Datapoints: []Datapoint{},
		}
		aggregated[deviceID] = agg
	}

	// Falls bereits ein Datapoint für das jeweilige Measurement existiert, aktualisieren
	updated := false
	for i, dp := range agg.Datapoints {
		if dp.ID == measurement {
			agg.Datapoints[i].Value = value
			updated = true
			break
		}
	}
	if !updated {
		newDP := Datapoint{
			ID:    measurement,
			Name:  measurement,
			Value: value,
		}
		agg.Datapoints = append(agg.Datapoints, newDP)
	}
}

// buildOutput erstellt das Ausgabearray aus dem Aggregationsspeicher und leert diesen anschließend.
func buildOutput(aggregated *map[string]*AggregatedData, deviceStatus map[string]string) []interface{} {
	var outputArray []interface{}

	// Erstelle eine Map für bereits verarbeitete Geräte
	processedDevices := make(map[string]bool)

	// Füge die aggregierten Datenpunkte hinzu
	for _, agg := range *aggregated {
		// Sortiere die Datapoints alphabetisch nach ihrer ID
		sort.Slice(agg.Datapoints, func(i, j int) bool {
			return agg.Datapoints[i].ID < agg.Datapoints[j].ID
		})

		// Füge den Status hinzu, falls vorhanden
		deviceKey := fmt.Sprintf("opc-ua_%s", agg.DeviceID) // Versuche zuerst OPC UA
		if status, exists := deviceStatus[deviceKey]; exists {
			outputArray = append(outputArray, struct {
				DeviceID   string      `json:"device_id"`
				Datapoints []Datapoint `json:"datapoints"`
				Status     string      `json:"status"`
			}{
				DeviceID:   agg.DeviceID,
				Datapoints: agg.Datapoints,
				Status:     status,
			})
			processedDevices[deviceKey] = true
			continue
		}

		deviceKey = fmt.Sprintf("s7_%s", agg.DeviceID) // Versuche dann S7
		if status, exists := deviceStatus[deviceKey]; exists {
			outputArray = append(outputArray, struct {
				DeviceID   string      `json:"device_id"`
				Datapoints []Datapoint `json:"datapoints"`
				Status     string      `json:"status"`
			}{
				DeviceID:   agg.DeviceID,
				Datapoints: agg.Datapoints,
				Status:     status,
			})
			processedDevices[deviceKey] = true
			continue
		}

		// Wenn kein Status gefunden wurde, sende ohne Status
		outputArray = append(outputArray, *agg)
	}

	// Füge alle verbleibenden Gerätestatus hinzu (Geräte ohne Datenpunkte)
	for deviceKey, status := range deviceStatus {
		if !processedDevices[deviceKey] {
			parts := strings.Split(deviceKey, "_")
			if len(parts) == 2 {
				outputArray = append(outputArray, struct {
					DeviceID   string      `json:"device_id"`
					Datapoints []Datapoint `json:"datapoints"`
					Status     string      `json:"status"`
				}{
					DeviceID:   parts[1],
					Datapoints: []Datapoint{},
					Status:     status,
				})
			}
		}
	}

	// Leere den Aggregationsspeicher für den nächsten Intervall
	*aggregated = make(map[string]*AggregatedData)

	return outputArray
}

// gracefulShutdown schließt die WebSocket-Verbindung ordnungsgemäß
func gracefulShutdown(conn *websocket.Conn) {
	if err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		logrus.Infof("Error closing WebSocket: %v", err)
	}
	conn.Close()
}

// restartDevice startet den Treiber für ein Gerät neu
func restartDevice(c *gin.Context) {
	deviceID := c.Param("device_id")
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logic.RestartDevice(db, deviceID)
	c.JSON(http.StatusOK, gin.H{"message": "Device restarted successfully"})
}

// addDevice fügt ein neues Gerät hinzu
func addDevice(c *gin.Context) {
	type Device struct {
		DeviceType      string `json:"deviceType"`
		DeviceName      string `json:"deviceName"`
		Status          string `json:"status"`
		Address         string `json:"address,omitempty"`
		AcquisitionTime int    `json:"acquisitionTime,omitempty"`
		SecurityMode    string `json:"securityMode,omitempty"`
		SecurityPolicy  string `json:"securityPolicy,omitempty"`
		DataPoints      []struct {
			Name     string `json:"name"`
			Datatype string `json:"datatype"`
			Address  string `json:"address"`
		} `json:"datapoints,omitempty"`
		DataNodes []string `json:"dataNodes,omitempty"`
		Rack      string   `json:"rack,omitempty"`
		Slot      string   `json:"slot,omitempty"`
		Username  string   `json:"username"`
		Password  string   `json:"password"`
	}
	var deviceData Device

	// JSON-Daten binden
	if err := c.ShouldBindJSON(&deviceData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	// Extrahiere die Datenbankverbindung aus dem Context
	db, _ := getDBConnection(c)

	var exists bool
	// Überprüfen, ob der Gerätename bereits existiert
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM devices WHERE name = ?)", deviceData.DeviceName).Scan(&exists)
	if err != nil {
		logrus.Println("Error checking if device name exists:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error checking if device name exists"})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{"message": "Device name already exists"})
		return
	}

	// Füge das Gerät direkt in die 'devices'-Tabelle ein
	query := `
		INSERT INTO devices (type, name, address, acquisition_time, security_mode, security_policy, rack, slot, username, password, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = db.Exec(query, deviceData.DeviceType, deviceData.DeviceName, deviceData.Address,
		deviceData.AcquisitionTime, deviceData.SecurityMode, deviceData.SecurityPolicy, deviceData.Rack, deviceData.Slot, deviceData.Username, deviceData.Password, logic.Initializing)
	if err != nil {
		logrus.Println("Error inserting device data into the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting device data"})
		return
	}

	// Hole die device id für das neue Gerät
	var deviceID int
	err = db.QueryRow("SELECT id FROM devices WHERE name = ?", deviceData.DeviceName).Scan(&deviceID)
	if err != nil {
		logrus.Println("Error retrieving device ID:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving device ID"})
		return
	}

	// Erstelle das MQTT-Topic
	topic := fmt.Sprintf("driver/states/%s/%d", deviceData.DeviceType, deviceID)
	server.Publish(topic, []byte("2 (initializing)"), true, 2)

	// Restarte den Treiber für das neue Gerät
	// übergeben der device id
	c.Set("device_id", deviceID)
	RestartDriver(c)

	c.JSON(http.StatusOK, gin.H{"message": "Device added successfully"})
}

func deleteDevice(c *gin.Context) {
	device_id := c.Param("device_id")

	db, _ := getDBConnection(c)
	server, _ := getMQTTServer(c)

	// Gerätetyp aus der Datenbank abrufen
	var deviceType string
	query := ` SELECT type FROM devices WHERE id = ?`
	err := db.QueryRow(query, device_id).Scan(&deviceType)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "Device not found"})
		} else {
			logrus.Println("Error querying device data from database:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error querying device data"})
		}
		return
	}

	// Lösche das Gerät direkt aus der 'devices'-Tabelle
	query = `DELETE FROM devices WHERE id = ?`
	_, err = db.Exec(query, device_id)
	if err != nil {
		logrus.Println("Error deleting device from the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting device"})
		return
	}

	logic.StopDriver(device_id)

	// Erstelle das MQTT-Topic
	payload := ""
	topic := fmt.Sprintf("driver/states/%s/%s", deviceType, device_id)
	server.Publish(topic, []byte(payload), false, 2)

	// RestartGateway(c)

	// Erfolgreiche Löschbestätigung senden
	c.JSON(http.StatusOK, gin.H{"message": "Device deleted successfully"})
}

func updateDevice(c *gin.Context) {
	device_id := c.Param("device_id")

	logrus.Infof("Updating device with id: %s", device_id)

	type Device struct {
		DeviceType      string `json:"deviceType"`
		DeviceName      string `json:"deviceName"`
		Status          string `json:"status"`
		Value           string `json:"value"`
		Connected       bool   `json:"connected"`
		Address         string `json:"address,omitempty"`
		AcquisitionTime int    `json:"acquisitionTime,omitempty"`
		SecurityMode    string `json:"securityMode,omitempty"`
		SecurityPolicy  string `json:"securityPolicy,omitempty"`
		DataPoints      []struct {
			DatapointId string `json:"datapointId"`
			Name        string `json:"name"`
			Datatype    string `json:"datatype"`
			Address     string `json:"address"`
		} `json:"datapoints,omitempty"`
		Rack     string `json:"rack,omitempty"`
		Slot     string `json:"slot,omitempty"`
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
	}

	var updatedDevice Device
	if err := c.ShouldBindJSON(&updatedDevice); err != nil {
		logrus.Errorf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	logrus.Infof("Recieved data:  %+v", updatedDevice)

	// Extrahiere die Datenbankverbindung aus dem Context
	db, _ := getDBConnection(c)

	// Aktualisiere die allgemeinen Gerätedaten
	query := `UPDATE devices SET address = ?, acquisition_time = ? WHERE id = ?`
	_, err := db.Exec(query, updatedDevice.Address, updatedDevice.AcquisitionTime, device_id)
	if err != nil {
		logrus.Errorf("Error updating device data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating device data"})
		return
	}

	logrus.Infof("Device data updated successfully for %s", updatedDevice.DeviceName)

	// Gerätetyp-spezifische Logik für S7
	if updatedDevice.DeviceType == "s7" {
		// Aktualisiere die S7-spezifischen Felder
		query = `UPDATE devices SET rack = ?, slot = ? WHERE id = ?`
		_, err := db.Exec(query, updatedDevice.Rack, updatedDevice.Slot, device_id)
		if err != nil {
			logrus.Errorf("Error updating S7-specific fields: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating S7-specific fields"})
			return
		}

		// Aktualisiere die S7-Datapoints
		_, err = db.Exec(`DELETE FROM s7_datapoints WHERE device_id = (SELECT id FROM devices WHERE id = ?)`, device_id)
		if err != nil {
			logrus.Errorf("Error clearing old datapoints: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error clearing old datapoints"})
			return
		}

		for _, dp := range updatedDevice.DataPoints {
			// Automatische Generierung der DatapointId, falls leer
			if dp.DatapointId == "" && dp.Address != "" && dp.Name != "" {
				// Hole die device_id
				var deviceId int
				err := db.QueryRow(`SELECT id FROM devices WHERE id = ?`, device_id).Scan(&deviceId)
				if err != nil {
					logrus.Errorf("Error retrieving device_id: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving device_id"})
					return
				}
				var nextId int
				err = db.QueryRow(`SELECT COALESCE(MAX(CAST(id AS INTEGER)), 0) + 1 FROM s7_datapoints WHERE device_id = ?`, deviceId).Scan(&nextId)
				if err != nil {
					logrus.Errorf("Error finding next datapoint ID: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"message": "Error finding next datapoint ID"})
					return
				}
				dp.DatapointId = fmt.Sprintf("1%.3d%.4d", deviceId, nextId)
				logrus.Error(dp.DatapointId)
			}

			if dp.DatapointId == "" || dp.Name == "" || dp.Address == "" {
				logrus.Errorf("Skipping empty datapoint: %+v", dp)
				continue
			}

			_, err = db.Exec(`INSERT INTO s7_datapoints (device_id, datapointId, name, datatype, address) VALUES ((SELECT id FROM devices WHERE id = ?), ?, ?, ?, ?)`, device_id, dp.DatapointId, dp.Name, dp.Datatype, dp.Address)
			if err != nil {
				logrus.Errorf("Error inserting new datapoints: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting new datapoints"})
				return
			}
		}

		logrus.Infof("S7 device and datapoints updated successfully for %s", updatedDevice.DeviceName)
	}

	// Gerätetyp-spezifische Logik für OPC UA
	if updatedDevice.DeviceType == "opc-ua" {
		// Aktualisiere die OPC-UA-spezifischen Felder
		query = `UPDATE devices SET security_mode = ?, security_policy = ?, username = ?, password = ? WHERE id = ?`
		_, err := db.Exec(query, updatedDevice.SecurityMode, updatedDevice.SecurityPolicy, updatedDevice.Username, updatedDevice.Password, device_id)
		if err != nil {
			logrus.Errorf("Error updating OPC-UA-specific fields: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating OPC-UA-specific fields"})
			return
		}

		// Lösche alte OPC-UA DataNodes
		_, err = db.Exec(`DELETE FROM opcua_datanodes WHERE device_id = (SELECT id FROM devices WHERE id = ?)`, device_id)
		if err != nil {
			logrus.Errorf("Error clearing old OPC-UA nodes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error clearing old OPC-UA nodes"})
			return
		}

		// Device_id to INT-Value
		dev_id, _ := strconv.Atoi(device_id)

		// Füge die neuen OPC-UA DataNodes ein
		for _, node := range updatedDevice.DataPoints {
			// Automatische Generierung der DatapointId, falls leer
			if node.DatapointId == "" && node.Address != "" && node.Name != "" {
				var nextId int
				err = db.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(datapointId, -3) AS INTEGER)), 0) + 1 FROM opcua_datanodes WHERE device_id = ?`, device_id).Scan(&nextId)
				if err != nil {
					logrus.Errorf("Error finding next datapoint ID: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"message": "Error finding next datapoint ID"})
					return
				}
				node.DatapointId = fmt.Sprintf("1%03d%03d", dev_id, nextId)
			}

			if node.DatapointId == "" || node.Name == "" || node.Address == "" {
				logrus.Errorf("Skipping empty OPC-UA node: %+v", node)
				continue
			}

			query = `
				INSERT INTO opcua_datanodes (device_id, datapointId, name, node_identifier)
				VALUES ( ?, ?, ?, ?)`
			_, err = db.Exec(query, dev_id, node.DatapointId, node.Name, node.Address)
			if err != nil {
				logrus.Errorf("Error inserting new OPC-UA nodes: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting new OPC-UA nodes"})
				return
			}
		}

		logrus.Infof("OPC-UA device and nodes updated successfully for %s", updatedDevice.DeviceName)
	}

	if updatedDevice.DeviceType == "mqtt" {

		// Schritt 1: Überprüfen, ob der Benutzer bereits existiert
		var existingUser bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM auth WHERE username = ?)", updatedDevice.DeviceName).Scan(&existingUser)
		if err != nil {
			logrus.Errorf("Error checking if user exists: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error checking if user exists"})
			return
		}

		if existingUser {
			// Schritt 2: Benutzer existiert, Passwort aktualisieren
			_, err = db.Exec("UPDATE auth SET password = ? WHERE username = ?", updatedDevice.Password, updatedDevice.DeviceName)
			if err != nil {
				logrus.Errorf("Error updating password: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating password"})
				return
			}
		} else {
			// Schritt 3: Benutzer existiert nicht, neuen Benutzer erstellen
			_, err = db.Exec("INSERT INTO auth (username, password, allow) VALUES (?, ?, ?)", updatedDevice.DeviceName, updatedDevice.Password, 1)
			if err != nil {
				logrus.Errorf("Error adding new MQTT user: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error adding new MQTT user"})
				return
			}
		}

		// Schritt 4: Zugriff auf das MQTT-Topic für diesen Benutzer sicherstellen
		deviceTopic := fmt.Sprintf("data/mqtt/%s/#", updatedDevice.DeviceName)

		// Lösche vorhandene ACL-Einträge für diesen Benutzer und dieses Topic
		_, err = db.Exec("DELETE FROM acl WHERE username = ? ", updatedDevice.DeviceName)
		if err != nil {
			logrus.Errorf("Error deleting existing ACL entries: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting existing ACL entries"})
			return
		}

		// Schritt 5: Füge neuen ACL-Eintrag für diesen Benutzer hinzu
		_, err1 := db.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", updatedDevice.DeviceName, "#", 0)
		_, err2 := db.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", updatedDevice.DeviceName, deviceTopic, 3)
		if err1 != nil || err2 != nil {
			logrus.Errorf("Error adding ACL entry: %v, %v", err1, err2)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error adding ACL entry"})
			return
		}

		logrus.Infof("MQTT device and user updated successfully for %s", updatedDevice.DeviceName)
	}

	logrus.Infof("Device updated successfully for %s", updatedDevice.DeviceName)

	c.Set("device_id", device_id)
	RestartDriver(c)

	// Erfolgreiche Antwort senden
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
