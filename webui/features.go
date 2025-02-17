package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

func startImageProcessOLD(c *gin.Context) {
	var requestData struct {
		API_URL          string `json:"api_url"`
		API_Header       string `json:"api_header"`
		DatapointId      string `json:"datapointId"`
		Device           string `json:"device"`
		MethodParentNode string `json:"methodParentNode"`
		MethodImageNode  string `json:"methodImageNode"`
		ImageNode        string `json:"imageNode"`
		CapturedNode     string `json:"capturedNode"`
		CompleteNode     string `json:"completeNode"`
		Timeout          string `json:"timeout"`
		CaptureMode      string `json:"captureMode"`
		AdditionalInput  string `json:"additionalInput"`
		Status           string `json:"status"`
		Logs             string `json:"logs"`
	}

	// Parse JSON request body into requestData
	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Log the received data
	logrus.Infof("Received image processing request: %+v", requestData)

	// TODO: Speichern der Daten in der Datenbank
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}

	// Füge das Gerät direkt in die 'devices'-Tabelle ein
	query := `
        INSERT INTO img_process (device, m_parent_node, m_image_node, image_node, captured_node, complete_node, timeout, capture_mode, trigger, status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err = db.Exec(query, requestData.Device, requestData.MethodParentNode, requestData.MethodImageNode, requestData.ImageNode,
		requestData.CapturedNode, requestData.CompleteNode, requestData.Timeout, requestData.CaptureMode, requestData.AdditionalInput, requestData.Status, "configured")
	if err != nil {
		logrus.Println("Error inserting device data into the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting device data"})
		return
	}

	// Geräteeinstellungen abrufen (Adresse, Sicherheitsrichtlinie und Modus)
	query = "SELECT address, security_policy, security_mode FROM devices WHERE name = ?"
	row := db.QueryRow(query, requestData.Device)

	var address, securityPolicy, securityMode string
	if err := row.Scan(&address, &securityPolicy, &securityMode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching device settings", "error": err.Error()})
		return
	}

	// Füge hier den Code ein, der den Endpunkt :1880/opc-image mit dem Body aufruft
	//

	// // OPC UA Client konfigurieren
	// client, err := opcua.NewClient(address)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating OPC UA client", "error": err.Error()})
	// 	return
	// }

	// logrus.Println(client)

	// filePath := filepath.Join("./files", requestData.Device)

	// // TODO: start der bildverarbeitung
	// features.StartImageCapture(client, requestData.MethodParentNode, requestData.MethodImageNode, requestData.ImageNode,
	// 	requestData.CapturedNode, requestData.CompleteNode, filePath, requestData.DatapointId, requestData.API_URL,
	// 	requestData.Timeout, requestData.CaptureMode, requestData.AdditionalInput)

	// Send a response
	c.JSON(http.StatusOK, gin.H{"message": "Image processing started successfully"})
}

func startImageProcess(c *gin.Context) {
	// processID := c.Param("id")

}

func deleteImageProcess(c *gin.Context) {
	// processID := c.Param("id")

}

func stopImageProcess(c *gin.Context) {
	// processID := c.Param("id")

}

func addImageProcess(c *gin.Context) {
	var requestData struct {
		API_URL          string `json:"api_url"`
		API_Header       string `json:"api_header"`
		DatapointId      string `json:"datapointId"`
		Device           string `json:"device"`
		MethodParentNode string `json:"methodParentNode"`
		MethodImageNode  string `json:"methodImageNode"`
		ImageNode        string `json:"imageNode"`
		CapturedNode     string `json:"capturedNode"`
		CompleteNode     string `json:"completeNode"`
		Timeout          string `json:"timeout"`
		CaptureMode      string `json:"captureMode"`
		AdditionalInput  string `json:"additionalInput"`
		Status           string `json:"status"`
		Logs             string `json:"logs"`
	}

	// Parse JSON request body into requestData
	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Log the received data
	logrus.Infof("Received image processing request: %+v", requestData)

	// 1) Datenbankverbindung herstellen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}
	defer db.Close()

	// 2) Datensatz in Tabelle img_process anlegen
	query := `
        INSERT INTO img_process (device, m_parent_node, m_image_node, image_node, captured_node, complete_node, timeout, capture_mode, trigger, status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err = db.Exec(query,
		requestData.Device,
		requestData.MethodParentNode,
		requestData.MethodImageNode,
		requestData.ImageNode,
		requestData.CapturedNode,
		requestData.CompleteNode,
		requestData.Timeout,
		requestData.CaptureMode,
		requestData.AdditionalInput,
		requestData.Status,
	)
	if err != nil {
		logrus.Println("Error inserting device data into the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting device data"})
		return
	}

	// 3) Geräteeinstellungen abrufen (z.B. address, security_policy, security_mode)
	query = "SELECT address, security_policy, security_mode FROM devices WHERE name = ?"
	row := db.QueryRow(query, requestData.Device)

	var (
		address        string
		securityPolicy string
		securityMode   string
	)
	if err := row.Scan(&address, &securityPolicy, &securityMode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching device settings", "error": err.Error()})
		return
	}

	// 4) Body für den POST-Aufruf an Node-RED bauen:
	//    Der Inhalt sollte deinem benötigten JSON entsprechen.
	postBody := map[string]interface{}{
		"endpoint":          address, // z.B. "opc.tcp://127.0.0.1:48010"
		"objectId":          requestData.MethodParentNode,
		"methodId":          requestData.MethodImageNode,
		"checkNodeId":       requestData.CapturedNode,
		"imageNodeId":       requestData.ImageNode,
		"ackNodeId":         requestData.CompleteNode,
		"basePath":          "C:/Users/jonas/Documents/iot-gateway2/files",
		"device":            requestData.Device,
		"enableUpload":      "true",
		"uploadUrl":         "http://127.0.0.1:1880/opc-upload",
		"securityModeVar":   securityMode,   // z.B. "NONE"
		"securityPolicyVar": securityPolicy, // z.B. "NONE"
		"username":          "",             // falls nicht nötig, leer
		"password":          "",
	}

	// Falls bestimmte Felder aus requestData.API_URL o.ä. gebraucht werden, füge sie hinzu.
	// z.B. "uploadUrl": requestData.API_URL, etc.

	// 5) Das Ganze nach JSON serialisieren
	jsonBytes, err := json.Marshal(postBody)
	if err != nil {
		logrus.Errorf("Error marshaling JSON for Node-RED call: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error building JSON for Node-RED"})
		return
	}

	// 6) HTTP-Request an Node-RED aufbauen
	nodeRedURL := "http://127.0.0.1:1880/opc-image" // Dein Node-RED-Endpunkt
	req, err := http.NewRequest("POST", nodeRedURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		logrus.Errorf("Error creating POST request to Node-RED: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating request to Node-RED"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// 7) Request abschicken
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("Error calling Node-RED endpoint: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error calling Node-RED endpoint"})
		return
	}
	defer resp.Body.Close()

	// 8) Response-Body von Node-RED (optional) einlesen
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Error reading response from Node-RED: %v", err)
		// trotzdem 200 oder 500 schicken, je nach Wunsch
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logrus.Errorf("Node-RED returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("Node-RED returned non-2xx status: %d", resp.StatusCode),
			"body":    string(bodyBytes),
		})
		return
	}

	// Optional: könnte man in die DB loggen oder so
	logrus.Infof("Node-RED call successful: %s", string(bodyBytes))

	// Abschließende Antwort an den Client
	c.JSON(http.StatusOK, gin.H{"message": "Image processing started successfully"})
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
	defer rows.Close()

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

	// logrus.Println(nodeList)

	// Knoten als JSON zurückgeben
	c.JSON(http.StatusOK, gin.H{"nodes": nodeList})

	// // speichere die Nodeliste in einer csv datei mit dem namen "nodelist.csv" in dem die nodes sortiert nach browsenamen gespeichert sind
	// file, err := os.Create("nodelist.csv")
	// if err != nil {
	// 	logrus.Errorf("Error creating CSV file: %v", err)
	// 	return
	// }
	// defer file.Close()

	// writer := csv.NewWriter(file)
	// defer writer.Flush()

	// // Schreibe die Header-Zeile
	// if err := writer.Write([]string{"NodeID", "NodeClass", "BrowseName", "Description", "Path"}); err != nil {
	// 	logrus.Errorf("Error writing header to CSV file: %v", err)
	// 	return
	// }

	// // Sortiere die NodeList nach BrowseName
	// sort.Slice(nodeList, func(i, j int) bool {
	// 	return nodeList[i].BrowseName < nodeList[j].BrowseName
	// })

	// // Schreibe die Nodes in die CSV-Datei
	// for _, node := range nodeList {
	// 	record := []string{
	// 		node.NodeID.String(),
	// 		node.NodeClass.String(),
	// 		node.BrowseName,
	// 		node.Description,
	// 		node.Path,
	// 	}
	// 	if err := writer.Write(record); err != nil {
	// 		logrus.Errorf("Error writing record to CSV file: %v", err)
	// 		return
	// 	}
	// }

	// logrus.Println("Node list saved to nodelist.csv")
}

func removeNonVariableNodes(nodeList []NodeDef) []NodeDef {
	for _, node := range nodeList {
		if node.NodeClass != ua.NodeClassVariable {
			nodeList = append(nodeList, node)
		}
	}
	return nodeList
}
