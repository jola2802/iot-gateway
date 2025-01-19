package webui

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Device struct {
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
		Datatype    string `json:"datatype"`
		Address     string `json:"address"`
	} `json:"datapoint,omitempty"`
	DataNodes []struct {
		DatapointId    string `json:"datapointId"`
		Name           string `json:"name"`
		NodeIdentifier string `json:"nodeIdentifier"`
	} `json:"dataNodes,omitempty"`
	Rack     sql.NullString `json:"rack,omitempty"`
	Slot     sql.NullString `json:"slot,omitempty"`
	Password string         `json:"password,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Im Produktivcode sollte diese Funktion verbessert werden
	},
}

var defaultTimeout = 20 * time.Minute

// showDevicesPage shows the devices page
func showDevicesPage(c *gin.Context) {
	// c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	// if err := tmpl.ExecuteTemplate(c.Writer, "devices.html", nil); err != nil {
	// 	c.String(http.StatusInternalServerError, "Error rendering template: %v", err)
	// }
	c.HTML(http.StatusOK, "devices.html", nil)
}

func listDevices(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
	}

	var devices []string
	rows, err := db.Query("SELECT name FROM devices where type = 'opcua'")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var device string
		err := rows.Scan(&device)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		devices = append(devices, device)
	}
	c.JSON(200, gin.H{"devices": devices})
	logrus.Println(devices)
}

// WebSocket-Endpunkt für den Gerätestatus und Steuerungsnachrichten
func deviceStatusWebSocket(c *gin.Context) {
	// WebSocket-Verbindung herstellen
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("Error upgrading to WebSocket: %v", err)
		return
	}
	defer gracefulShutdown(conn)

	// Hole die Datenbankverbindung aus dem Context
	db, exists := c.Get("db")
	if !exists {
		logrus.Println("Error retrieving database connection from context")
		gracefulShutdown(conn)
		return
	}

	// Hole einen MQTT-Client aus dem Pool
	mqttClient := getPooledMQTTClient(mqttClientPool, db.(*sql.DB))
	defer releaseMQTTClient(mqttClientPool, mqttClient) // Gib den Client nach der Nutzung zurück

	// Channel für den Empfang von Gerätestatusinformationen
	deviceStatuses := make(chan Device)

	// MQTT-Abonnement neu einrichten
	subscribeToDeviceStatus(mqttClient, "iot-gateway/driver/states/#", deviceStatuses, db.(*sql.DB))
	defer mqttClient.Unsubscribe("iot-gateway/driver/states/#")

	// Starte eine Goroutine, um eingehende WebSocket-Nachrichten zu behandeln (Steuerungsnachrichten vom Client)
	go func() {
		for {
			var controlData struct {
				Action     string `json:"action"`
				DeviceName string `json:"deviceName"`
				DeviceType string `json:"deviceType"`
			}

			// Lies die Steuerungsnachricht vom WebSocket-Client
			err := conn.ReadJSON(&controlData)
			if err != nil {
				logrus.Warnf("Error reading control message from WebSocket: %v", err)
				gracefulShutdown(conn)
				return
			}

			// Verarbeite die Steuerungsnachricht
			logrus.Infof("Received control message: %v", controlData)
			err = sendMQTTControlMessage(controlData.DeviceType, controlData.DeviceName, controlData.Action, db.(*sql.DB))
			if err != nil {
				logrus.Errorf("Error sending MQTT control message: %v", err)
				conn.WriteJSON(gin.H{"message": "Error sending MQTT control message"})
				continue
			}

			// Sende Erfolgsmeldung an den Client
			conn.WriteJSON(gin.H{"message": "Control message processed successfully"})
		}
	}()

	// Behandle Gerätestatus und sende sie über WebSocket
	for device := range deviceStatuses {
		if err := conn.WriteJSON(device); err != nil {
			logrus.Errorf("Error sending device status: %v", err)
			gracefulShutdown(conn)
			return
		}
	}
}

func gracefulShutdown(conn *websocket.Conn) {
	if err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		logrus.Errorf("Error closing WebSocket: %v", err)
	}
	conn.Close()
}

// Reusable function to handle WebSocket communication
func handleWebSocketMessages(conn *websocket.Conn, dataChan chan Device) {
	for device := range dataChan {
		// JSON Nachricht senden
		if err := conn.WriteJSON(device); err != nil {
			logrus.Errorf("Error sending device status: %v", err)
			return
		}
	}
}

// WebSocket-Endpunkt für die Gerätepunkte
func deviceDataWebSocket(c *gin.Context) {
	for {
		// WebSocket-Verbindung herstellen
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logrus.Errorf("Error upgrading to WebSocket: %v", err)
			return
		}
		defer gracefulShutdown(conn)

		// Starte eine Goroutine, um Verbindungsabbrüche zu überwachen
		go func() {
			defer gracefulShutdown(conn)
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					logrus.Warnf("WebSocket disconnected: %v", err)
					gracefulShutdown(conn)
					return
				}
			}
		}()

		// Hole die Datenbankverbindung aus dem Context
		db, exists := c.Get("db")
		if !exists {
			logrus.Println("Error retrieving database connection from context")
			return
		}

		// Hole einen MQTT-Client aus dem Pool
		mqttClient := getPooledMQTTClient(mqttClientPool, db.(*sql.DB))
		defer releaseMQTTClient(mqttClientPool, mqttClient) // Gib den Client nach der Nutzung zurück

		// Channel für den Empfang von Datenpunkten
		deviceDataPoints := make(chan DeviceData)

		// MQTT-Abonnement neu einrichten
		subscribeToDeviceDataPoints(mqttClient, "data/#", deviceDataPoints)

		// WebSocket-Nachrichten behandeln
		for {
			select {
			case dataPoint := <-deviceDataPoints:
				// Sende die Gerätepunkte an den Client
				if err := conn.WriteJSON(dataPoint); err != nil {
					logrus.Errorf("Error sending data point: %v", err)
					return
				}
			case <-time.After(defaultTimeout):
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}
}

func subscribeToDeviceStatus(client mqtt.Client, topic string, deviceStatuses chan Device, db *sql.DB) {
	// Abonniere auf das entsprechende Gerätetopic
	token := client.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		// Teile das Topic in den Gerätetyp und die Geräte-ID
		parts := strings.Split(msg.Topic(), "/")
		if len(parts) < 5 {
			logrus.Errorf("Invalid topic: %s", msg.Topic())
			return
		}
		deviceType := parts[3]
		deviceID := parts[4] // Dies ist die Geräte-ID, nicht der Name

		// Gerätedaten aus der Datenbank abrufen, um den tatsächlichen Namen zu bekommen
		var deviceName string
		err := db.QueryRow("SELECT name FROM devices WHERE id = ?", deviceID).Scan(&deviceName)
		if err != nil {
			deviceName = deviceID
		}

		// Status ist die Nachricht, die von MQTT gesendet wurde
		status := string(msg.Payload())

		// Erstelle ein Device-Objekt und sende es zurück
		device := Device{
			DeviceType: deviceType,
			DeviceName: deviceName, // Verwende den tatsächlichen Namen des Geräts
			Status:     status,
		}

		// Sende das Gerät in den Channel
		deviceStatuses <- device
	})

	token.Wait()
	if token.Error() != nil {
		log.Printf("Error subscribing to topic %s: %v", topic, token.Error())
	}
}

func getDeviceStatus(c *gin.Context) {
	deviceName := c.Param("deviceName")

	// Extrahiere die Datenbankverbindung aus dem Context
	db, exists := c.Get("db")
	if !exists {
		logrus.Println("Error retrieving database connection from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving database connection"})
		return
	}

	// Gerätedaten aus der Datenbank abrufen
	var device Device
	query := `
		SELECT name, type, address, aquisition_time, security_mode, security_policy, rack, slot
		FROM devices WHERE name = ?
	`
	err := db.(*sql.DB).QueryRow(query, deviceName).Scan(
		&device.DeviceName, &device.DeviceType, &device.Address, &device.AcquisitionTime,
		&device.SecurityMode, &device.SecurityPolicy, &device.Rack, &device.Slot)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "Device not found"})
		} else {
			logrus.Println("Error querying device data from database:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error querying device data"})
		}
		return
	}

	// Falls es OPC-UA-Knoten gibt, müssen diese ebenfalls geladen werden
	if device.DeviceType == "opcua" {
		nodeQuery := `SELECT datapointId, name, node_identifier FROM opcua_datanodes WHERE device_id = (SELECT id FROM devices WHERE name = ?)`
		rows, err := db.(*sql.DB).Query(nodeQuery, deviceName)
		if err != nil {
			logrus.Println("Error querying OPC-UA nodes from database:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error querying OPC-UA nodes"})
			return
		}
		defer rows.Close()

		// Füge die OPC-UA-Knoten zur Geräteliste hinzu
		for rows.Next() {
			var datapointId, name, nodeIdentifier sql.NullString
			if err := rows.Scan(&datapointId, &name, &nodeIdentifier); err != nil {
				logrus.Println("Error scanning OPC-UA node data:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error scanning OPC-UA node data"})
				return
			}
			device.DataNodes = append(device.DataNodes, struct {
				DatapointId    string `json:"datapointId"`
				Name           string `json:"name"`
				NodeIdentifier string `json:"nodeIdentifier"`
			}{
				DatapointId:    datapointId.String,    // Setze hier den DatapointId-Wert falls vorhanden
				Name:           name.String,           // Setze hier den Namen falls vorhanden
				NodeIdentifier: nodeIdentifier.String, // Verwende den String-Wert von nodeIdentifier
			})
		}
	} else if device.DeviceType == "s7" {
		// S7-Datenpunkte abrufen
		datapointQuery := `SELECT datapointId, name, datatype, address FROM s7_datapoints WHERE device_id = (SELECT id FROM devices WHERE name = ?)`
		rows, err := db.(*sql.DB).Query(datapointQuery, deviceName)
		if err != nil {
			logrus.Println("Error querying S7 datapoints from database:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error querying S7 datapoints"})
			return
		}
		defer rows.Close()

		// Füge die S7-Datapoints zur Geräteliste hinzu
		for rows.Next() {
			var dpDatapointId, dpName, dpDatatype, dpAddress sql.NullString
			if err := rows.Scan(&dpDatapointId, &dpName, &dpDatatype, &dpAddress); err != nil {
				logrus.Println("Error scanning S7 data points:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error scanning S7 data points"})
				return
			}

			// Füge die Datenpunkte zur Geräteliste hinzu
			device.DataPoint = append(device.DataPoint, struct {
				DatapointId string `json:"datapointId"`
				Name        string `json:"name"`
				Datatype    string `json:"datatype"`
				Address     string `json:"address"`
			}{
				DatapointId: dpDatapointId.String,
				Name:        dpName.String,
				Datatype:    dpDatatype.String,
				Address:     dpAddress.String,
			})
		}
	} else if device.DeviceType == "mqtt" {
		// aus der Datenbank Tabelle auth das password des jeweiligen gerätes durch den namen holen und zurückgeben
		query := `SELECT password FROM auth WHERE username = ?`
		var mqttPassword string
		err := db.(*sql.DB).QueryRow(query, deviceName).Scan(&mqttPassword)
		if err != nil {
			logrus.Println("Error querying MQTT password from database:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error querying MQTT password"})
			return
		}
		device.Password = mqttPassword
	}

	// Formatiere die Daten für die Rückgabe
	c.JSON(http.StatusOK, gin.H{
		"deviceName":      device.DeviceName,
		"deviceType":      device.DeviceType,
		"address":         device.Address,
		"acquisitionTime": device.AcquisitionTime,
		"securityMode":    device.SecurityMode,
		"securityPolicy":  device.SecurityPolicy,
		"datapoints":      device.DataPoint,
		"dataNodes":       device.DataNodes,
		"rack":            device.Rack,
		"slot":            device.Slot,
		"password":        device.Password,
	})
}

type DeviceData struct {
	DeviceName string `json:"deviceName"`
	DeviceType string `json:"deviceType"`
	Topic      string `json:"topic"`
	Value      string `json:"value"`
}

// Funktion, um die Datenpunkte über MQTT zu erhalten
func subscribeToDeviceDataPoints(client mqtt.Client, topic string, deviceDataPoints chan DeviceData) {
	// Abonniere auf das entsprechende Gerätetopic
	token := client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {

		// Teile das Topic in die Komponenten, um Gerätedetails zu extrahieren
		parts := strings.Split(msg.Topic(), "/")
		if len(parts) < 2 {
			log.Printf("Invalid topic: %s", msg.Topic())
			return
		}
		deviceType := parts[1]
		deviceName := parts[2]

		// Der empfangene Wert (Payload) als Datenpunkt
		value := string(msg.Payload())

		// Erstelle ein DeviceData-Objekt und sende es zurück
		dataPoint := DeviceData{
			DeviceName: deviceName,
			DeviceType: deviceType,
			Topic:      msg.Topic(),
			Value:      value,
		}

		// Sende den Datenpunkt in den Channel
		deviceDataPoints <- dataPoint
	})

	token.Wait()
	if token.Error() != nil {
		log.Printf("Error subscribing to topic %s: %v", topic, token.Error())
	}
}

func addDevice(c *gin.Context) {
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
			Name     string `json:"name"`
			Datatype string `json:"datatype"`
			Address  string `json:"address"`
		} `json:"datapoints,omitempty"`
		DataNodes []string `json:"dataNodes,omitempty"`
		Rack      string   `json:"rack,omitempty"`
		Slot      string   `json:"slot,omitempty"`
		Password  string   `json:"password"`
	}
	var deviceData Device

	// JSON-Daten binden
	if err := c.ShouldBindJSON(&deviceData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	// Extrahiere die Datenbankverbindung aus dem Context
	db, exists := c.Get("db")
	if !exists {
		logrus.Println("Error retrieving database connection from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving database connection"})
		return
	}

	// Überprüfen, ob der Gerätename bereits existiert
	err := db.(*sql.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM devices WHERE name = ?)", deviceData.DeviceName).Scan(&exists)
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
		INSERT INTO devices (type, name, address, aquisition_time, security_mode, security_policy, rack, slot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = db.(*sql.DB).Exec(query, deviceData.DeviceType, deviceData.DeviceName, deviceData.Address,
		deviceData.AcquisitionTime, deviceData.SecurityMode, deviceData.SecurityPolicy, deviceData.Rack, deviceData.Slot)
	if err != nil {
		logrus.Println("Error inserting device data into the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting device data"})
		return
	}

	// Hole einen MQTT-Client aus dem Pool
	mqttClient := getPooledMQTTClient(mqttClientPool, db.(*sql.DB))
	defer releaseMQTTClient(mqttClientPool, mqttClient) // Gib den Client nach der Nutzung zurück

	// Erstelle das MQTT-Topic
	topic := fmt.Sprintf("iot-gateway/driver/states/%s/%s", deviceData.DeviceType, deviceData.DeviceName)
	token := mqttClient.Publish(topic, 0, false, "2 (initializing)")
	token.Wait()

	// RestartGateway(db)

	c.JSON(http.StatusOK, gin.H{"message": "Device added successfully"})
}

func deleteDevice(c *gin.Context) {
	deviceName := c.Param("deviceName")

	// Extrahiere die Datenbankverbindung aus dem Context
	db, exists := c.Get("db")
	if !exists {
		logrus.Println("Error retrieving database connection from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving database connection"})
		return
	}

	// Gerätetyp aus der Datenbank abrufen
	var deviceType string
	query := ` SELECT type FROM devices WHERE name = ?`
	err := db.(*sql.DB).QueryRow(query, deviceName).Scan(&deviceType)
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
	query = `DELETE FROM devices WHERE name = ?`
	_, err = db.(*sql.DB).Exec(query, deviceName)
	if err != nil {
		logrus.Println("Error deleting device from the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting device"})
		return
	}

	// Hole einen MQTT-Client aus dem Pool
	mqttClient := getPooledMQTTClient(mqttClientPool, db.(*sql.DB))
	defer releaseMQTTClient(mqttClientPool, mqttClient) // Gib den Client nach der Nutzung zurück

	// Erstelle das MQTT-Topic
	topic := fmt.Sprintf("iot-gateway/driver/states/%s/%s", deviceType, deviceName)
	token := mqttClient.Publish(topic, 0, false, "9 (deleted)")
	token.Wait()

	RestartGateway(db)

	// Erfolgreiche Löschbestätigung senden
	c.JSON(http.StatusOK, gin.H{"message": "Device deleted successfully"})
}

func updateDeviceStatus(c *gin.Context) {
	deviceName := c.Param("deviceName")

	var controlData struct {
		DeviceName string `json:"deviceName"`
		DeviceType string `json:"deviceType"`
		Action     string `json:"action"`
	}

	if err := c.ShouldBindJSON(&controlData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	// Hole die Datenbankverbindung aus dem Context
	db, exists := c.Get("db")
	if !exists {
		logrus.Println("Error retrieving database connection from context")
		return
	}

	err := sendMQTTControlMessage(controlData.DeviceType, deviceName, controlData.Action, db.(*sql.DB))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error sending MQTT message"})
		return
	}

	logrus.Println("send MQTT message successfully to device: ", deviceName, "with action: ", controlData.Action)

	c.JSON(http.StatusOK, gin.H{"message": "MQTT message sent successfully"})
}

func updateDevice(c *gin.Context) {
	deviceName := c.Param("deviceName")

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
		DataNodes []struct {
			DatapointId    string `json:"datapointId"`    // DatapointId für OPC UA hinzugefügt
			Name           string `json:"name"`           // Name des Datapoints
			NodeIdentifier string `json:"nodeIdentifier"` // Node Identifier hinzugefügt
		} `json:"dataNodes,omitempty"`
		Rack     string `json:"rack,omitempty"`
		Slot     string `json:"slot,omitempty"`
		Password string `json:"password,omitempty"`
	}

	var updatedDevice Device
	if err := c.ShouldBindJSON(&updatedDevice); err != nil {
		logrus.Fatalf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	logrus.Infof("Recieved data:  %+v", updatedDevice)

	// Extrahiere die Datenbankverbindung aus dem Context
	db, exists := c.Get("db")
	if !exists {
		logrus.Println("Error retrieving database connection from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving database connection"})
		return
	}

	// Aktualisiere die allgemeinen Gerätedaten
	query := `UPDATE devices SET address = ?, aquisition_time = ? WHERE name = ?`
	_, err := db.(*sql.DB).Exec(query, updatedDevice.Address, updatedDevice.AcquisitionTime, deviceName)
	if err != nil {
		logrus.Println("Error updating device data:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating device data"})
		return
	}

	// Gerätetyp-spezifische Logik für S7
	if updatedDevice.DeviceType == "s7" {
		// Aktualisiere die S7-spezifischen Felder
		query = `UPDATE devices SET rack = ?, slot = ? WHERE name = ?`
		_, err := db.(*sql.DB).Exec(query, updatedDevice.Rack, updatedDevice.Slot, deviceName)
		if err != nil {
			logrus.Println("Error updating S7-specific fields:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating S7-specific fields"})
			return
		}

		// Aktualisiere die S7-Datapoints
		_, err = db.(*sql.DB).Exec(`DELETE FROM s7_datapoints WHERE device_id = (SELECT id FROM devices WHERE name = ?)`, deviceName)
		if err != nil {
			logrus.Println("Error clearing old datapoints:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error clearing old datapoints"})
			return
		}

		for _, dp := range updatedDevice.DataPoints {
			// Automatische Generierung der DatapointId, falls leer
			if dp.DatapointId == "" {
				// Hole die device_id
				var deviceId int
				err := db.(*sql.DB).QueryRow(`SELECT id FROM devices WHERE name = ?`, deviceName).Scan(&deviceId)
				if err != nil {
					logrus.Println("Error retrieving device_id:", err)
					c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving device_id"})
					return
				}
				var nextId int
				err = db.(*sql.DB).QueryRow(`SELECT COALESCE(MAX(CAST(id AS INTEGER)), 0) + 1 FROM s7_datapoints WHERE device_id = ?`, deviceId).Scan(&nextId)
				if err != nil {
					logrus.Println("Error finding next datapoint ID:", err)
					c.JSON(http.StatusInternalServerError, gin.H{"message": "Error finding next datapoint ID"})
					return
				}
				dp.DatapointId = fmt.Sprintf("1%.3d%.4d", deviceId, nextId)
				logrus.Error(dp.DatapointId)
			}

			_, err = db.(*sql.DB).Exec(`INSERT INTO s7_datapoints (device_id, datapointId, name, datatype, address) VALUES ((SELECT id FROM devices WHERE name = ?), ?, ?, ?, ?)`, deviceName, dp.DatapointId, dp.Name, dp.Datatype, dp.Address)
			if err != nil {
				logrus.Println("Error inserting new datapoints:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting new datapoints"})
				return
			}
		}
	}

	// Gerätetyp-spezifische Logik für OPC UA
	if updatedDevice.DeviceType == "opcua" {
		// Aktualisiere die OPC-UA-spezifischen Felder
		query = `UPDATE devices SET security_mode = ?, security_policy = ? WHERE name = ?`
		_, err := db.(*sql.DB).Exec(query, updatedDevice.SecurityMode, updatedDevice.SecurityPolicy, deviceName)
		if err != nil {
			logrus.Println("Error updating OPC-UA-specific fields:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating OPC-UA-specific fields"})
			return
		}

		// Lösche alte OPC-UA DataNodes
		_, err = db.(*sql.DB).Exec(`DELETE FROM opcua_datanodes WHERE device_id = (SELECT id FROM devices WHERE name = ?)`, deviceName)
		if err != nil {
			logrus.Println("Error clearing old OPC-UA nodes:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error clearing old OPC-UA nodes"})
			return
		}

		// Füge die neuen OPC-UA DataNodes ein
		for _, node := range updatedDevice.DataNodes {
			// Automatische Generierung der DatapointId, falls leer
			if node.DatapointId == "" {
				// Hole die device_id
				var deviceId int
				err := db.(*sql.DB).QueryRow(`SELECT id FROM devices WHERE name = ?`, deviceName).Scan(&deviceId)
				if err != nil {
					logrus.Println("Error retrieving device_id:", err)
					c.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving device_id"})
					return
				}
				var nextId int
				err = db.(*sql.DB).QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(datapointId, -3) AS INTEGER)), 0) + 1 FROM opcua_datanodes WHERE device_id = ?`, deviceId).Scan(&nextId)
				if err != nil {
					logrus.Println("Error finding next datapoint ID:", err)
					c.JSON(http.StatusInternalServerError, gin.H{"message": "Error finding next datapoint ID"})
					return
				}
				node.DatapointId = fmt.Sprintf("1%03d%03d", deviceId, nextId)
			}
			query = `
				INSERT INTO opcua_datanodes (device_id, datapointId, name, node_identifier)
				VALUES ((SELECT id FROM devices WHERE name = ?), ?, ?, ?)`
			_, err := db.(*sql.DB).Exec(query, deviceName, node.DatapointId, node.Name, node.NodeIdentifier)
			if err != nil {
				logrus.Println("Error inserting new OPC-UA nodes:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting new OPC-UA nodes"})
				return
			}
		}
	}

	if updatedDevice.DeviceType == "mqtt" {
		// Extrahiere die Datenbankverbindung aus dem Context
		dbConn := db.(*sql.DB)

		// Schritt 1: Überprüfen, ob der Benutzer bereits existiert
		var existingUser bool
		err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM auth WHERE username = ?)", updatedDevice.DeviceName).Scan(&existingUser)
		if err != nil {
			logrus.Println("Error checking if user exists:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error checking if user exists"})
			return
		}

		if existingUser {
			// Schritt 2: Benutzer existiert, Passwort aktualisieren
			_, err = dbConn.Exec("UPDATE auth SET password = ? WHERE username = ?", updatedDevice.Password, updatedDevice.DeviceName)
			if err != nil {
				logrus.Println("Error updating password:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating password"})
				return
			}
		} else {
			// Schritt 3: Benutzer existiert nicht, neuen Benutzer erstellen
			_, err = dbConn.Exec("INSERT INTO auth (username, password, allow) VALUES (?, ?, ?)", updatedDevice.DeviceName, updatedDevice.Password, 1)
			if err != nil {
				logrus.Println("Error adding new MQTT user:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error adding new MQTT user"})
				return
			}
		}

		// Schritt 4: Zugriff auf das MQTT-Topic für diesen Benutzer sicherstellen
		deviceTopic := fmt.Sprintf("data/mqtt/%s/#", updatedDevice.DeviceName)

		// Lösche vorhandene ACL-Einträge für diesen Benutzer und dieses Topic
		_, err = dbConn.Exec("DELETE FROM acl WHERE username = ? ", updatedDevice.DeviceName)
		if err != nil {
			logrus.Println("Error deleting existing ACL entries:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting existing ACL entries"})
			return
		}

		// Schritt 5: Füge neuen ACL-Eintrag für diesen Benutzer hinzu
		_, err1 := dbConn.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", updatedDevice.DeviceName, "#", 0)
		_, err2 := dbConn.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", updatedDevice.DeviceName, deviceTopic, 3)
		if err1 != nil || err2 != nil {
			logrus.Println("Error adding ACL entry:", err1, err2)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error adding ACL entry"})
			return
		}

		RestartGateway(db)

		logrus.Infof("MQTT device and user updated successfully for %s", updatedDevice.DeviceName)
	}

	// Erfolgreiche Antwort senden
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func sendMQTTControlMessage(deviceType, deviceName, action string, db *sql.DB) error {
	// Hole einen MQTT-Client aus dem Pool
	mqttClient := getPooledMQTTClient(mqttClientPool, db)
	defer releaseMQTTClient(mqttClientPool, mqttClient) // Gib den Client nach der Nutzung zurück

	// Erstelle das MQTT-Topic
	topic := fmt.Sprintf("iot-gateway/driver/states/%s/%s", deviceType, deviceName)

	// Führe eine Retry-Schleife für den Fall eines Fehlers ein
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		token := mqttClient.Publish(topic, 1, false, action)
		token.Wait()
		if token.Error() == nil {
			return nil
		}
		log.Printf("Error sending MQTT message, retrying (%d/%d): %v", i+1, maxRetries, token.Error())
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("failed to send MQTT message after %d attempts", maxRetries)
}
