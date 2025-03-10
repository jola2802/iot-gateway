package webui

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

var NodeRED_URL string

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

func getNodeRedURL(c *gin.Context) {
	// check if NodeRED_URL is set
	if NodeRED_URL != "" {
		c.JSON(http.StatusOK, gin.H{"nodeRedURL": NodeRED_URL})
		logrus.Infof("NodeRED_URL: %s", NodeRED_URL)
		return
	} else {
		NodeRED_URL = os.Getenv("NODE_RED_URL")
		logrus.Infof("NodeRED_URL set from env: %s", NodeRED_URL)
		c.JSON(http.StatusOK, gin.H{"nodeRedURL": NodeRED_URL})
	}
}

// captureImage ist ein Endpunkt, der einen Bildaufnahmeprozess über OPC UA auslöst
func captureImage(c *gin.Context) {
	// 1) Parameter aus der Anfrage lesen und validieren
	params, err := parseAndValidateParams(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Parameter", "details": err.Error()})
		return
	}

	// Channels für die Kommunikation mit der Go-Routine
	resultChan := make(chan map[string]interface{})
	errorChan := make(chan map[string]interface{})

	// Starte den Prozess in einer Go-Routine
	go func() {
		// 2) OPC UA Client erstellen und verbinden
		client, err := createAndConnectOPCUAClient(params)
		if err != nil {
			errorChan <- gin.H{"error": "OPC UA Client-Fehler", "details": err.Error(), "status": http.StatusInternalServerError}
			return
		}

		ctx := context.Background()
		defer client.Close(ctx)

		// 3) Methode aufrufen (Bildaufnahme starten)
		if err := callOPCUAMethod(ctx, client, params); err != nil {
			errorChan <- gin.H{"error": "Methodenaufruf fehlgeschlagen", "details": err.Error(), "status": http.StatusInternalServerError}
			return
		}

		// 4) Warten, bis checkNodeId = true
		if err := waitForBooleanTrue(ctx, client, params.CheckNodeId); err != nil {
			errorChan <- gin.H{"error": "Timeout beim Warten auf Boolean-Wert", "details": err.Error(), "status": http.StatusRequestTimeout}
			return
		}

		// 5) Bild-String auslesen
		base64String, err := readImageString(ctx, client, params.ImageNodeId)
		if err != nil {
			errorChan <- gin.H{"error": "Bilddaten konnten nicht gelesen werden", "details": err.Error(), "status": http.StatusInternalServerError}
			return
		}

		// 6) Ack Node = true setzen
		if err := writeBoolean(ctx, client, params.AckNodeId, true); err != nil {
			logrus.Errorf("Fehler beim Schreiben des Ack-Werts: %v", err)
			// Wir setzen fort, auch wenn das Schreiben fehlschlägt
		}

		// 7) Optionaler Upload in die Datenbank
		uploadErfolgt := false
		if strings.ToLower(params.EnableUpload) == "true" {
			var uploadErr error
			uploadErfolgt, uploadErr = handleUploads(c, params, base64String)
			if uploadErr != nil {
				errorChan <- gin.H{"error": "Upload fehlgeschlagen", "details": uploadErr.Error(), "status": http.StatusInternalServerError}
				return
			}
		}

		// Erfolgreiche Antwort senden
		resultChan <- gin.H{
			"success":        true,
			"endpoint":       params.Endpoint,
			"securityMode":   params.SecurityMode,
			"securityPolicy": params.SecurityPolicy,
			"username":       params.Username,
			"image":          base64String,
			"device_id":      params.DeviceId,
			"uploaded":       uploadErfolgt,
		}
	}()

	// Auf das Ergebnis oder einen Fehler warten
	select {
	case result := <-resultChan:
		c.JSON(http.StatusOK, result)
	case errResponse := <-errorChan:
		status, ok := errResponse["status"].(int)
		if !ok {
			status = http.StatusInternalServerError
		}
		delete(errResponse, "status")
		c.JSON(status, errResponse)
	case <-time.After(30 * time.Second): // Timeout nach 30 Sekunden
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Zeitüberschreitung bei der Bildaufnahme"})
	}
}

// Struktur für die Parameter
type ImageCaptureParams struct {
	Endpoint       string            `json:"endpoint"`
	ObjectId       string            `json:"objectId"`
	MethodId       string            `json:"methodId"`
	MethodArgs     json.RawMessage   `json:"methodArgs"`
	CheckNodeId    string            `json:"checkNodeId"`
	ImageNodeId    string            `json:"imageNodeId"`
	AckNodeId      string            `json:"ackNodeId"`
	BasePath       string            `json:"basePath"`
	DeviceId       string            `json:"deviceId"`
	EnableUpload   string            `json:"enableUpload"`
	UploadUrl      string            `json:"uploadUrl"`
	SecurityMode   string            `json:"securityMode"`
	SecurityPolicy string            `json:"securityPolicy"`
	Username       string            `json:"username"`
	Password       string            `json:"password"`
	Headers        map[string]string `json:"headers"`
}

// parseAndValidateParams liest und validiert die Parameter aus der Anfrage
func parseAndValidateParams(c *gin.Context) (ImageCaptureParams, error) {
	var params ImageCaptureParams
	if err := c.ShouldBindJSON(&params); err != nil {
		return params, err
	}

	// Standardwerte setzen, wenn Parameter fehlen
	if params.Endpoint == "" {
		params.Endpoint = "opc.tcp://192.168.0.84:48010"
	}
	if params.ObjectId == "" {
		params.ObjectId = "ns=3;s=Demo.Method"
	}
	if params.MethodId == "" {
		params.MethodId = "ns=3;s=Demo.Method.DoSomethingAfter10s"
	}
	if params.CheckNodeId == "" {
		params.CheckNodeId = "ns=3;s=Demo.Dynamic.Scalar.Boolean"
	}
	if params.ImageNodeId == "" {
		params.ImageNodeId = "ns=3;s=Demo.Dynamic.Scalar.ImageGIF"
	}
	if params.AckNodeId == "" {
		params.AckNodeId = "ns=3;s=Demo.Dynamic.Scalar.Boolean"
	}
	if params.BasePath == "" {
		params.BasePath = "./data/images"
	}
	if params.SecurityMode == "" {
		params.SecurityMode = "NONE"
	}
	if params.SecurityPolicy == "" {
		params.SecurityPolicy = "NONE"
	}

	// nehme nur deviceid wenn nicht in tabelle vorhanden
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Fehler beim Verbinden mit der Datenbank: %v", err)
		return params, err
	}

	// get id from tabelle devices where address = params.Endpoint
	var id int
	err = db.QueryRow("SELECT id FROM devices WHERE address = ?", params.Endpoint).Scan(&id)
	if err != nil {
		logrus.Errorf("Fehler beim Abfragen der Datenbank: %v", err)
		return params, err
	}

	if id == 0 {
		params.DeviceId = "999999"
	} else {
		params.DeviceId = strconv.Itoa(id)
	}

	return params, nil
}

// createAndConnectOPCUAClient erstellt und verbindet einen OPC UA Client
func createAndConnectOPCUAClient(params ImageCaptureParams) (*opcua.Client, error) {
	// OPC UA Client-Optionen erstellen
	opts := []opcua.Option{}

	// Sicherheitseinstellungen konfigurieren
	var securityMode ua.MessageSecurityMode
	switch params.SecurityMode {
	case "SIGN":
		securityMode = ua.MessageSecurityModeSign
	case "SIGNANDENCRYPT":
		securityMode = ua.MessageSecurityModeSignAndEncrypt
	default:
		securityMode = ua.MessageSecurityModeNone
	}

	var securityPolicy string
	switch params.SecurityPolicy {
	case "BASIC128RSA15":
		securityPolicy = ua.SecurityPolicyURIBasic128Rsa15
	case "BASIC256":
		securityPolicy = ua.SecurityPolicyURIBasic256
	case "BASIC256SHA256":
		securityPolicy = ua.SecurityPolicyURIBasic256Sha256
	default:
		securityPolicy = ua.SecurityPolicyURINone
	}

	// Basis-Sicherheitsoptionen hinzufügen
	opts = append(opts,
		opcua.SecurityMode(securityMode),
		opcua.SecurityPolicy(securityPolicy),
	)

	// Authentifizierung konfigurieren
	if params.Username != "" && params.Password != "" {
		opts = append(opts, opcua.AuthUsername(params.Username, params.Password))
	} else {
		opts = append(opts, opcua.AuthAnonymous())
	}

	// OPC UA Client erstellen und verbinden
	ctx := context.Background()
	client, err := opcua.NewClient(params.Endpoint, opts...)
	if err != nil {
		logrus.Errorf("Fehler beim Erstellen des OPC UA Clients: %v", err)
		return nil, err
	}

	if err := client.Connect(ctx); err != nil {
		logrus.Errorf("Fehler beim Verbinden mit OPC UA Server: %v", err)
		return nil, err
	}

	return client, nil
}

// callOPCUAMethod ruft eine Methode auf dem OPC UA Server auf
func callOPCUAMethod(ctx context.Context, client *opcua.Client, params ImageCaptureParams) error {
	objectID, err := ua.ParseNodeID(params.ObjectId)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der ObjectID: %v", err)
		return err
	}

	methodID, err := ua.ParseNodeID(params.MethodId)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der MethodID: %v", err)
		return err
	}

	// Methodenargumente vorbereiten (falls vorhanden)
	var inputArguments []*ua.Variant
	if params.MethodArgs != nil {
		// JSON-String in eine Map parsen
		var argsMap map[string]interface{}
		if err := json.Unmarshal([]byte(params.MethodArgs), &argsMap); err != nil {
			logrus.Errorf("Fehler beim Parsen der Methodenargumente: %v", err)
			return fmt.Errorf("ungültiges JSON-Format für Methodenargumente: %v", err)
		}

		// Argumente in der richtigen Reihenfolge hinzufügen
		// Da OPC UA Methoden positionsbasierte Argumente erwarten, müssen wir
		// die Argumente in einer bestimmten Reihenfolge übergeben
		// Hier nehmen wir an, dass die Argumente alphabetisch sortiert werden sollen
		keys := make([]string, 0, len(argsMap))
		for k := range argsMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Für jeden Schlüssel in sortierter Reihenfolge einen Variant erstellen
		for _, key := range keys {
			value := argsMap[key]

			// Konvertiere den Wert in den passenden Datentyp
			// Hier müssen wir entscheiden, welchen Datentyp wir verwenden
			// Standardmäßig behandeln wir alles als String, aber wir könnten
			// auch versuchen, Zahlen zu erkennen

			// Versuche, numerische Werte zu erkennen
			switch v := value.(type) {
			case string:
				// Versuche, den String als Zahl zu interpretieren
				if intVal, err := strconv.Atoi(v); err == nil {
					// Es ist eine ganze Zahl
					inputArguments = append(inputArguments, ua.MustVariant(int32(intVal)))
				} else if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
					// Es ist eine Fließkommazahl
					inputArguments = append(inputArguments, ua.MustVariant(floatVal))
				} else if v == "true" || v == "false" {
					// Es ist ein Boolean
					boolVal := v == "true"
					inputArguments = append(inputArguments, ua.MustVariant(boolVal))
				} else {
					// Es ist ein normaler String
					inputArguments = append(inputArguments, ua.MustVariant(v))
				}
			case float64:
				// JSON-Zahlen werden standardmäßig als float64 geparst
				inputArguments = append(inputArguments, ua.MustVariant(v))
			case bool:
				inputArguments = append(inputArguments, ua.MustVariant(v))
			default:
				// Fallback: Konvertiere zu String
				inputArguments = append(inputArguments, ua.MustVariant(fmt.Sprintf("%v", v)))
			}
		}
	}

	req := &ua.CallMethodRequest{
		ObjectID:       objectID,
		MethodID:       methodID,
		InputArguments: inputArguments,
	}

	resp, err := client.Call(ctx, req)
	if err != nil {
		logrus.Errorf("Fehler beim Aufrufen der Methode: %v", err)
		return err
	}

	if resp.StatusCode != ua.StatusOK {
		logrus.Errorf("Methodenaufruf fehlgeschlagen mit Status: %v", resp.StatusCode)
		return fmt.Errorf("methodenaufruf fehlgeschlagen mit Status: %v", resp.StatusCode)
	}

	return nil
}

// waitForBooleanTrue wartet, bis der angegebene Knoten den Wert true hat
func waitForBooleanTrue(ctx context.Context, client *opcua.Client, nodeIDString string) error {
	checkNodeID, err := ua.ParseNodeID(nodeIDString)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der CheckNodeID: %v", err)
		return err
	}

	timeout := time.After(20 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Boolean-Wert lesen
			nodeToRead := &ua.ReadValueID{
				NodeID:      checkNodeID,
				AttributeID: ua.AttributeIDValue,
			}

			req := &ua.ReadRequest{
				NodesToRead:        []*ua.ReadValueID{nodeToRead},
				TimestampsToReturn: ua.TimestampsToReturnBoth,
			}

			resp, err := client.Read(ctx, req)
			if err != nil {
				logrus.Errorf("Fehler beim Lesen des Boolean-Werts: %v", err)
				continue
			}

			if len(resp.Results) > 0 && resp.Results[0].Status == ua.StatusOK {
				if val, ok := resp.Results[0].Value.Value().(bool); ok && val {
					// Boolean ist true, weiter zum nächsten Schritt
					return nil
				}
			}
		case <-timeout:
			logrus.Error("Timeout beim Warten auf Boolean-Wert")
			return fmt.Errorf("timeout beim Warten auf Boolean-Wert")
		}
	}
}

// readImageString liest den Bild-String aus dem angegebenen Knoten
func readImageString(ctx context.Context, client *opcua.Client, nodeIDString string) (string, error) {
	imageNodeID, err := ua.ParseNodeID(nodeIDString)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der ImageNodeID: %v", err)
		return "", err
	}

	nodeToRead := &ua.ReadValueID{
		NodeID:      imageNodeID,
		AttributeID: ua.AttributeIDValue,
	}

	req := &ua.ReadRequest{
		NodesToRead:        []*ua.ReadValueID{nodeToRead},
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}

	resp, err := client.Read(ctx, req)
	if err != nil {
		logrus.Errorf("Fehler beim Lesen des Bildes: %v", err)
		return "", err
	}

	if len(resp.Results) == 0 || resp.Results[0].Status != ua.StatusOK {
		logrus.Error("Bilddaten konnten nicht gelesen werden")
		return "", fmt.Errorf("bilddaten konnten nicht gelesen werden")
	}

	var base64String string
	// if val, ok := resp.Results[0].Value.Value().(string); ok {
	// 	base64String = val
	// } else {
	// 	logrus.Error("Bilddaten haben falsches Format")
	// 	return "", fmt.Errorf("bilddaten haben falsches Format")
	// }

	dataValue := resp.Results[0].Value.Value()
	// dataValue ist ein []byte
	// wir müssen es in einen string konvertieren
	base64String = base64.StdEncoding.EncodeToString(dataValue.([]byte))

	return base64String, nil
}

// writeBoolean schreibt einen booleschen Wert in den angegebenen Knoten
func writeBoolean(ctx context.Context, client *opcua.Client, nodeIDString string, value bool) error {
	nodeID, err := ua.ParseNodeID(nodeIDString)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der NodeID: %v", err)
		return err
	}

	writeReq := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      nodeID,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					Value: ua.MustVariant(value),
				},
			},
		},
	}

	_, err = client.Write(ctx, writeReq)
	if err != nil {
		logrus.Errorf("Fehler beim Schreiben des Werts: %v", err)
		return err
	}

	// if len(writeResp.Results) > 0 && writeResp.Results[0] != ua.StatusOK {
	// 	logrus.Errorf("Schreiben des Werts fehlgeschlagen mit Status: %v", writeResp.Results[0])
	// 	return fmt.Errorf("schreiben des Werts fehlgeschlagen mit Status: %v", writeResp.Results[0])
	// }

	return nil
}

// handleUploads verarbeitet den Upload des Bildes in die lokale und externe Datenbank
func handleUploads(c *gin.Context, params ImageCaptureParams, base64String string) (bool, error) {
	uploadErfolgt := false

	// Lokaler Upload in die Datenbank
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Fehler beim Verbinden mit der Datenbank: %v", err)
		return false, err
	}

	// Bild in die Datenbank speichern
	query := "INSERT INTO images (device, image, timestamp) VALUES (?, ?, ?)"
	if _, err := db.Exec(query, params.DeviceId, base64String, time.Now()); err != nil {
		logrus.Errorf("Fehler beim Speichern des Bildes in der Datenbank: %v", err)
		return false, err
	}
	uploadErfolgt = true

	// Maximale Anzahl von Bildern in der Datenbank begrenzen (max. 100)
	if err := limitDatabaseImages(db); err != nil {
		logrus.Errorf("Fehler beim Begrenzen der Bildanzahl: %v", err)
		// Wir setzen fort, auch wenn die Begrenzung fehlschlägt
	}

	// Wenn eine externe Upload-URL angegeben ist, auch dorthin hochladen
	if params.UploadUrl != "" {
		// Externer Upload per HTTP
		externalUploadSuccess := uploadToExternalDatabase(base64String, params.UploadUrl, params.Headers, params.DeviceId)
		uploadErfolgt = uploadErfolgt || externalUploadSuccess
	}

	return uploadErfolgt, nil
}

// limitDatabaseImages begrenzt die Anzahl der Bilder in der Datenbank auf maximal 100
func limitDatabaseImages(db *sql.DB) error {
	rows, err := db.Query("SELECT COUNT(*) FROM images")
	if err != nil {
		return err
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		if err = rows.Scan(&count); err != nil {
			return err
		}
	}

	// Wenn mehr als 100 Bilder, lösche die ältesten
	if count > 100 {
		deleteQuery := "DELETE FROM images WHERE id IN (SELECT id FROM images ORDER BY timestamp ASC LIMIT ?)"
		if _, err := db.Exec(deleteQuery, count-100); err != nil {
			return err
		}
	}

	return nil
}

// uploadToExternalDatabase lädt ein Bild per HTTP zu einer externen Datenbank hoch
func uploadToExternalDatabase(base64String, uploadUrl string, headers map[string]string, deviceId string) bool {
	// Standardheader setzen, falls keine angegeben sind
	if headers == nil {
		headers = make(map[string]string)
	}

	// Standardwert für Content-Type setzen, falls nicht angegeben
	if _, ok := headers["Content-Type"]; !ok {
		// headers["Content-Type"] = "application/octet-stream"
	}

	// HTTP-Client mit angepassten Einstellungen erstellen
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Zertifikatsüberprüfung deaktivieren (nur für Entwicklung!)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Request-Body erstellen
	body := strings.NewReader(base64String)

	// HTTP-Request erstellen
	req, err := http.NewRequest("POST", uploadUrl, body)
	if err != nil {
		logrus.Errorf("Fehler beim Erstellen des HTTP-Requests: %v", err)
		return false
	}

	// Header hinzufügen
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Geräte-ID als zusätzlichen Header hinzufügen, falls nicht bereits vorhanden
	if _, ok := headers["Device-ID"]; !ok {
		req.Header.Set("Device-ID", deviceId)
	}

	// Request senden
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("Fehler beim Senden des HTTP-Requests: %v", err)
		return false
	}
	defer resp.Body.Close()

	// Antwort überprüfen
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logrus.Errorf("HTTP-Request fehlgeschlagen mit Status: %d", resp.StatusCode)
		return false
	}
	return true
}
