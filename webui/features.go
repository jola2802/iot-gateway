package webui

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"crypto/tls"

	"github.com/gin-gonic/gin"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

var NodeRED_URL, NodeRED_URL_OPC string

func listCapturedImages(c *gin.Context) {
	// Basis-Verzeichnis für die Bilder
	baseDir := "/data/shared"

	// Slice für alle gefundenen Bilder
	var images []gin.H

	// Durchsuche rekursiv alle Unterverzeichnisse
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Überspringe Verzeichnisse
		if info.IsDir() {
			return nil
		}

		// Prüfe ob es sich um ein Bild handelt
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			// Erstelle relativen Pfad vom baseDir aus
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return err
			}

			// Füge Bildinformationen hinzu
			images = append(images, gin.H{
				"filename":  relPath, // Relativer Pfad als Dateiname
				"timestamp": info.ModTime().Format(time.RFC3339),
				"size":      info.Size(),
			})
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error reading images",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"images": images})
}

// getImageProcessStatus holt den Status des Bildverarbeitungsprozesses aus der Datenbank (Base64-Bild)
func getImageProcessStatus(c *gin.Context) {
	processID := c.Param("id")

	// Datenbankverbindung herstellen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}

	// Status aus der Datenbank holen
	var status, statusData, device string
	query := "SELECT status, status_data, device FROM img_process WHERE id = ?"
	err = db.QueryRow(query, processID).Scan(&status, &statusData, &device)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "Process not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching status", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status, "statusData": statusData, "device": device})
}

func deleteImageProcess(c *gin.Context) {
	processID := c.Param("id")

	// Stoppe den Bildverarbeitung Worker
	stopImgProcess(processID)

	// Datenbankverbindung herstellen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}

	// Datensatz aus der Tabelle img_process löschen
	query := "DELETE FROM img_process WHERE id = ?"
	_, err = db.Exec(query, processID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting image process", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image process deleted successfully"})
}

func listImgCapProcesses(c *gin.Context) {
	// Datenbankverbindung holen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}

	// Holen Sie die Liste der Bildverarbeitungsprozesse aus der Datenbank
	query := "SELECT * FROM img_process"
	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching image processing processes", "error": err.Error()})
		return
	}

	// Spaltennamen ermitteln
	columns, err := rows.Columns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching columns", "error": err.Error()})
		return
	}

	var processes []map[string]interface{}

	for rows.Next() {
		// Slice für die Werte und Pointer für Scan
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// Datensätze in Values scannen
		if err := rows.Scan(valuePtrs...); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error scanning image processing process", "error": err.Error()})
			return
		}

		// Map für den aktuellen Prozess erstellen
		process := make(map[string]interface{})
		for i, col := range columns {
			process[col] = values[i]
		}
		processes = append(processes, process)
	}

	// Prüfen auf Fehler beim Iterieren
	if err := rows.Err(); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"processes": []map[string]interface{}{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error iterating rows", "error": err.Error()})
		return
	}

	logrus.Printf("Found %v image processing processes", len(processes))

	// Sende die Prozessliste als JSON
	c.JSON(http.StatusOK, gin.H{"processes": processes})
}

// Function to handle fetching the latest image
func latestImage(c *gin.Context) {
	deviceName := c.Param("deviceName")
	// Pfad zum Ordner, in dem sich die Bilder befinden
	imageDirectory := filepath.Join("./files", deviceName)
	logrus.Println(imageDirectory)

	// Lese den Inhalt des Verzeichnisses
	files, err := os.ReadDir(imageDirectory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error reading directory", "error": err.Error()})
		return
	}

	// Liste der Bilddateien erstellen (Filter für Bildformate wie .jpg, .jpeg, .png)
	var imageFiles []os.DirEntry
	for _, file := range files {
		if !file.IsDir() && (filepath.Ext(file.Name()) == ".jpg" || filepath.Ext(file.Name()) == ".jpeg" || filepath.Ext(file.Name()) == ".png") {
			imageFiles = append(imageFiles, file)
		}
	}

	// Überprüfen, ob es Bilddateien gibt
	if len(imageFiles) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "No images found"})
		return
	}

	// Sortiere die Dateien nach ihrem Änderungsdatum (neueste zuerst)
	sort.Slice(imageFiles, func(i, j int) bool {
		fileInfoI, errI := imageFiles[i].Info()
		fileInfoJ, errJ := imageFiles[j].Info()
		if errI != nil || errJ != nil {
			return false
		}
		return fileInfoI.ModTime().After(fileInfoJ.ModTime())
	})

	// Das neueste Bild ist das erste in der sortierten Liste
	latestFile := imageFiles[0]
	latestFilePath := filepath.Join(imageDirectory, latestFile.Name())

	// Hole das Änderungsdatum des Bildes
	fileInfo, err := latestFile.Info()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting file info", "error": err.Error()})
		return
	}
	creationTime := fileInfo.ModTime().Format(time.RFC3339)

	// Sende den Pfad und das Erstellungsdatum des neuesten Bildes an den Client
	c.Header("X-Creation-Time", creationTime)
	c.File(latestFilePath)
}

type NodeDef struct {
	NodeID      *ua.NodeID
	NodeClass   ua.NodeClass
	BrowseName  string
	Description string
	Path        string
}

// join fügt den aktuellen Pfad mit dem neuen Knoten zusammen
func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

// browse durchläuft die Knoten eines OPC UA Servers rekursiv
func browse(ctx context.Context, n *opcua.Node, path string, level int) ([]NodeDef, error) {
	if level > 10 {
		return nil, nil
	}

	attrs, err := n.Attributes(ctx, ua.AttributeIDNodeClass, ua.AttributeIDBrowseName, ua.AttributeIDDescription)
	if err != nil {
		return nil, err
	}

	var def = NodeDef{
		NodeID: n.ID,
	}

	if attrs[0].Status == ua.StatusOK {
		def.NodeClass = ua.NodeClass(attrs[0].Value.Int())
	}

	if attrs[1].Status == ua.StatusOK {
		def.BrowseName = attrs[1].Value.String()
	}

	if attrs[2].Status == ua.StatusOK {
		def.Description = attrs[2].Value.String()
	}

	def.Path = join(path, def.BrowseName)

	var nodes []NodeDef
	if def.NodeClass == ua.NodeClassVariable {
		nodes = append(nodes, def)
	}

	browseChildren := func(refType uint32) error {
		refs, err := n.ReferencedNodes(ctx, refType, ua.BrowseDirectionForward, ua.NodeClassAll, true)
		if err != nil {
			return err
		}
		for _, rn := range refs {
			children, err := browse(ctx, rn, def.Path, level+1)
			if err != nil {
				return err
			}
			nodes = append(nodes, children...)
		}
		return nil
	}

	if err := browseChildren(id.HasComponent); err != nil {
		return nil, err
	}
	if err := browseChildren(id.Organizes); err != nil {
		return nil, err
	}
	if err := browseChildren(id.HasProperty); err != nil {
		return nil, err
	}
	return nodes, nil
}

// browseNodes ist der Endpunkt, der die Knoten eines Geräts durchsucht und als JSON zurückgibt
func browseNodes(c *gin.Context) {
	// Hol den Gerätenamen aus der URL
	deviceID := c.Param("deviceID")

	// Datenbankverbindung holen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}

	// Geräteeinstellungen abrufen (Adresse, Sicherheitsrichtlinie und Modus)
	query := "SELECT address, security_policy, security_mode FROM devices WHERE id = ?"
	row := db.QueryRow(query, deviceID)

	var address, securityPolicy, securityMode string
	if err := row.Scan(&address, &securityPolicy, &securityMode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching device settings", "error": err.Error()})
		return
	}

	// OPC UA Client konfigurieren
	client, err := opcua.NewClient(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating OPC UA client", "error": err.Error()})
		return
	}
	ctx := context.Background()

	// Verbinde mit dem OPC UA Server
	if err := client.Connect(ctx); err != nil {
		logrus.Errorf("Failed to connect to OPC UA server: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to OPC UA server", "error": err.Error()})
		return
	}
	defer client.Close(ctx)

	// Root-Knoten durchsuchen
	id, err := ua.ParseNodeID("ns=0;i=84") // Standard-Root-Knoten
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Invalid node ID", "error": err.Error()})
		return
	}

	// Durchsuche die Knoten rekursiv
	nodeList, err := browse(ctx, client.Node(id), "", 0)
	if err != nil {
		logrus.Errorf("Error browsing OPC UA server: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error browsing nodes", "error": err.Error()})
		return
	}

	// Enferne alle Nodes die nicht vom Typ Variable sind
	nodeList = removeNonVariableNodes(nodeList)

	// Knoten als JSON zurückgeben
	c.JSON(http.StatusOK, gin.H{"nodes": nodeList})
}

func removeNonVariableNodes(nodeList []NodeDef) []NodeDef {
	for _, node := range nodeList {
		if node.NodeClass != ua.NodeClassVariable {
			nodeList = append(nodeList, node)
		}
	}
	return nodeList
}

// HeaderEntry repräsentiert einen einzelnen Header-Eintrag
type HeaderEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Argument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func saveImageProcess(c *gin.Context) {
	var requestData struct {
		Device               string        `json:"device"`
		MethodParentNode     string        `json:"methodParentNode"`
		MethodImageNode      string        `json:"methodImageNode"`
		MethodArguments      []Argument    `json:"methodArguments"`
		ImageNode            string        `json:"imageNode"`
		CaptureCompletedNode string        `json:"captureCompletedNode"`
		ReadCompletedNode    string        `json:"readCompletedNode"`
		Timeout              int           `json:"timeout"`
		CaptureMode          string        `json:"captureMode"`
		Interval             int           `json:"interval"`
		TriggerNode          string        `json:"triggerNode"`
		RestUri              string        `json:"restUri"`
		Headers              []HeaderEntry `json:"headers"`
	}

	// Parse JSON request body
	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Headers in JSON-String umwandeln
	headersJSON, err := json.Marshal(requestData.Headers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing headers"})
		return
	}

	// entweder triggerNode oder interval, das was nicht leer ist soll dann in additionalInput gespeichert werden
	var additionalInput string
	if requestData.TriggerNode != "" {
		additionalInput = requestData.TriggerNode
	} else if requestData.Interval != 0 {
		additionalInput = strconv.Itoa(requestData.Interval)
	}

	if requestData.Timeout == 0 {
		requestData.Timeout = 30
	}

	// Datenbankverbindung herstellen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}

	logrus.Infof("Method Arguments: %v", requestData.MethodArguments)
	methodArgumentsJSON, err := json.Marshal(requestData.MethodArguments)

	// Datensatz in Tabelle img_process anlegen
	query := `
        INSERT INTO img_process (device, m_parent_node, m_image_node, method_arguments, image_node, captured_node, complete_node, timeout, capture_mode, trigger, rest_uri, headers, status, status_data)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	result, err := db.Exec(query,
		requestData.Device,
		requestData.MethodParentNode,
		requestData.MethodImageNode,
		methodArgumentsJSON,
		requestData.ImageNode,
		requestData.CaptureCompletedNode,
		requestData.ReadCompletedNode,
		requestData.Timeout,
		requestData.CaptureMode,
		additionalInput,
		requestData.RestUri,
		string(headersJSON), // Headers als JSON-String speichern
		"configured",
		"e",
	)
	if err != nil {
		logrus.Println("Error inserting device data into the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting device data"})
		return
	}

	// Get the ID of the inserted record
	id, err := result.LastInsertId()
	if err != nil {
		logrus.Println("Error getting last insert ID:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting process ID"})
		return
	}

	// Starte die Bildverarbeitung
	startImageProcessWorker(db, requestData.Device, strconv.Itoa(int(id)), NodeRED_URL_OPC, requestData.Interval)

	c.JSON(http.StatusOK, gin.H{
		"message": "Image process saved successfully",
		"id":      id,
	})
}

// imageProcessWorkers speichert pro route_id die jeweilige Stop-Funktion
var imageProcessWorkers = make(map[string]func())

// Typdefinitionen für die aus der DB geladenen Daten
type ImageProcessData struct {
	ID              int
	Device          string
	MParentNode     string
	MImageNode      string
	MethodArguments string
	ImageNode       string
	CapturedNode    string
	Timeout         string
	CaptureMode     string
	Trigger         string
	CompleteNode    string
	RestURI         string
	Headers         string
	Status          string
	StatusData      string
}

type DeviceSettings struct {
	Address        string
	SecurityPolicy string
	SecurityMode   string
	Username       sql.NullString
	Password       sql.NullString
}

type NodeRedResponse struct {
	Success        bool   `json:"success"`
	Endpoint       string `json:"endpoint"`
	SecurityMode   string `json:"securityMode"`
	SecurityPolicy string `json:"securityPolicy"`
	Username       string `json:"username"`
	SavedFilePath  string `json:"savedFilePath"`
	Uploaded       bool   `json:"uploaded"`
	Image          struct {
		Type string `json:"type"`
		Data []byte `json:"data"`
	} `json:"image"`
}

// fetchImageProcessData liest die Bildprozessdaten aus der Datenbank anhand der device-ID.
func fetchImageProcessData(db *sql.DB, deviceID string) (*ImageProcessData, error) {
	query := "SELECT id, device, m_parent_node, m_image_node, method_arguments, image_node, captured_node, timeout, capture_mode, trigger, complete_node, rest_uri, headers, status, status_data FROM img_process WHERE device = ?"
	row := db.QueryRow(query, deviceID)

	var data ImageProcessData
	err := row.Scan(&data.ID, &data.Device, &data.MParentNode, &data.MImageNode, &data.MethodArguments, &data.ImageNode,
		&data.CapturedNode, &data.Timeout, &data.CaptureMode, &data.Trigger, &data.CompleteNode,
		&data.RestURI, &data.Headers, &data.Status, &data.StatusData)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// fetchDeviceSettings holt die Geräteeinstellungen anhand der device-ID (als int) aus der Datenbank.
func fetchDeviceSettings(db *sql.DB, deviceID int) (*DeviceSettings, error) {
	query := "SELECT address, security_policy, security_mode, username, password FROM devices WHERE id = ?"
	row := db.QueryRow(query, deviceID)

	var settings DeviceSettings
	err := row.Scan(&settings.Address, &settings.SecurityPolicy, &settings.SecurityMode, &settings.Username, &settings.Password)
	if err != nil {
		return nil, err
	}
	return &settings, nil

}

// buildPostBody erstellt das Request-Body-Objekt für den Node-RED Aufruf.
func buildPostBody(imgData *ImageProcessData, devSettings *DeviceSettings) map[string]interface{} {
	upload := "false"
	if imgData.RestURI != "" {
		upload = "true"
	}
	return map[string]interface{}{
		"endpoint":          devSettings.Address,
		"objectId":          imgData.MParentNode,
		"methodId":          imgData.MImageNode,
		"methodArguments":   imgData.MethodArguments,
		"checkNodeId":       imgData.CapturedNode,
		"imageNodeId":       imgData.ImageNode,
		"ackNodeId":         imgData.CompleteNode,
		"basePath":          "/data/images",
		"device":            imgData.Device,
		"enableUpload":      upload,
		"uploadUrl":         imgData.RestURI,
		"securityModeVar":   devSettings.SecurityMode,
		"securityPolicyVar": devSettings.SecurityPolicy,
		"username":          devSettings.Username,
		"password":          devSettings.Password,
	}
}

// callNodeRED sendet den HTTP-POST Request an Node-RED und gibt die Response-Daten zurück.
func callNodeRED(noderedURL string, postBody map[string]interface{}, timeoutSeconds int) ([]byte, error) {
	jsonBytes, err := json.Marshal(postBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", noderedURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Client mit deaktivierter SSL-Verifizierung
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &httpError{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	}

	return bodyBytes, nil
}

// parseNodeRedResponse wandelt die Response von Node-RED in das entsprechende Struct um.
func parseNodeRedResponse(bodyBytes []byte) (*NodeRedResponse, error) {
	var response NodeRedResponse
	err := json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// updateImageProcessStatus speichert den Status und die Bilddaten (als Base64) in der Datenbank.
func updateImageProcessStatus(db *sql.DB, processID int, statusData string) error {
	query := "UPDATE img_process SET status = ?, status_data = ? WHERE id = ?"

	timezone, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		logrus.Errorf("Error loading timezone: %v", err)
		return err
	}
	timestamp := time.Now().In(timezone).Format("2006-01-02 15:04:05")

	_, err = db.Exec(query, timestamp, statusData, processID)
	return err
}

// httpError definiert einen Fehler, falls der HTTP-Status nicht im 2xx-Bereich liegt.
type httpError struct {
	StatusCode int
	Body       string
}

func (he *httpError) Error() string {
	return "HTTP error: status code " + strconv.Itoa(he.StatusCode) + ", body: " + he.Body
}

// runImageProcess fasst den kompletten Ablauf zusammen.
func runImageProcess(db *sql.DB, deviceID string, noderedURL string) {
	// Hole Bildprozess-Daten
	imgData, err := fetchImageProcessData(db, deviceID)
	if err != nil {
		logrus.Errorf("Error fetching image process data: %v", err)
		return
	}

	// Konvertiere deviceID in int für den Geräteabruf
	deviceIDInt, err := strconv.Atoi(deviceID)
	if err != nil {
		logrus.Errorf("Error converting deviceID to int: %v", err)
		return
	}

	// Hole Geräteeinstellungen
	devSettings, err := fetchDeviceSettings(db, deviceIDInt)
	if err != nil {
		logrus.Errorf("Error fetching device settings: %v", err)
	}

	// Erstelle Body für den Node-RED Request
	postBody := buildPostBody(imgData, devSettings)

	// Timeout als Integer parsen
	timeoutInt, err := strconv.Atoi(imgData.Timeout)
	if err != nil {
		logrus.Errorf("Error converting timeout to int: %v", err)
		timeoutInt = 10 // Fallback-Wert
	}

	// Sende den Request an Node-RED
	bodyBytes, err := callNodeRED(noderedURL, postBody, timeoutInt)
	if err != nil {
		logrus.Errorf("Error calling Node-RED endpoint: %v", err)
		return
	}

	// Parse die Node-RED Antwort
	nodeRedResp, err := parseNodeRedResponse(bodyBytes)
	if err != nil {
		logrus.Errorf("Error parsing Node-RED response: %v", err)
		return
	}

	// Wenn Bilddaten vorhanden, in Base64 konvertieren
	var statusData string
	if len(nodeRedResp.Image.Data) > 0 {
		statusData = base64.StdEncoding.EncodeToString(nodeRedResp.Image.Data)
	} else {
		statusData = string(bodyBytes)
	}

	// Update der Datenbank mit dem neuen Status
	err = updateImageProcessStatus(db, imgData.ID, statusData)
	if err != nil {
		logrus.Errorf("Error saving status to database: %v", err)
	}

	// logrus.Infof("Node-RED call successful.")
}

func StartAllImageProcessWorkers(db *sql.DB, noderedURL string) {
	NodeRED_URL = noderedURL
	NodeRED_URL_OPC = noderedURL + "/opc-image"

	// query := "SELECT id, device, trigger FROM img_process"
	// rows, err := db.Query(query)
	// if err != nil {
	// 	logrus.Errorf("Error querying image processes: %v", err)
	// 	return
	// }

	// for rows.Next() {
	// 	var id, trigger int
	// 	var device string
	// 	err := rows.Scan(&id, &device, &trigger)
	// 	if err != nil {
	// 		logrus.Errorf("Error scanning image process: %v", err)
	// 		continue
	// 	}
	// 	startImageProcessWorker(db, device, strconv.Itoa(id), NodeRED_URL_OPC, trigger)
	// }
	// rows.Close()
}

func startImageProcessWorker(db *sql.DB, deviceID string, routeID string, noderedURL string, intervalSeconds int) func() {
	stopChan := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()
		// Initiale Ausführung
		runImageProcess(db, deviceID, noderedURL)
		for {
			select {
			case <-ticker.C:
				runImageProcess(db, deviceID, noderedURL)
			case <-stopChan:
				logrus.Infof("Image process worker stopped for route %s", routeID)
				return
			}
		}
	}()
	stopFunc := func() {
		close(stopChan)
	}
	// Speichere die Stop-Funktion in der globalen Map unter dem Schlüssel routeID
	imageProcessWorkers[routeID] = stopFunc
	return stopFunc
}

// stopImgProcess beendet den laufenden Bildverarbeitungsprozess für die übergebene routeID.
func stopImgProcess(routeID string) {
	if stopFunc, exists := imageProcessWorkers[routeID]; exists {
		stopFunc()
		delete(imageProcessWorkers, routeID)
		logrus.Infof("Stopped image process worker for route %s", routeID)
	} else {
		logrus.Warnf("No image process worker found for route %s", routeID)
	}
}

func getNodeRedURL(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"nodeRedURL": NodeRED_URL})
}
