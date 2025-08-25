package webui

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/awcullen/opcua/client"
	awcullenua "github.com/awcullen/opcua/ua"
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
	var filteredNodes []NodeDef
	for _, node := range nodeList {
		if node.NodeClass == ua.NodeClassVariable {
			filteredNodes = append(filteredNodes, node)
		}
	}
	return filteredNodes
}

// captureImage ist ein Endpunkt, der einen Bildaufnahmeprozess über OPC UA auslöst
func captureImage(c *gin.Context) {
	// 1) Parameter aus der Anfrage lesen und validieren
	params, err := parseAndValidateParams(c)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der Parameter zum Image Capture Prozess: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Parameter", "details": err.Error()})
		return
	}

	// Channels für die Kommunikation mit der Go-Routine
	resultChan := make(chan map[string]interface{})
	errorChan := make(chan map[string]interface{})

	// Starte den Prozess in einer Go-Routine
	go func() {
		// 2) OPC UA Client erstellen und verbinden
		ch, err := createAndConnectOPCUAClient(params)
		if err != nil {
			errorChan <- gin.H{"error": "OPC UA Client-Fehler", "details": err.Error(), "status": http.StatusInternalServerError}
			return
		}

		ctx := context.Background()
		defer ch.Close(ctx)

		// 3) Methode aufrufen (Bildaufnahme starten) - verwendet gopcua
		if err := callOPCUAMethod(ctx, params); err != nil {
			errorChan <- gin.H{"error": "Methodenaufruf fehlgeschlagen", "details": err.Error(), "status": http.StatusInternalServerError}
			return
		}

		// 4) Warten, bis checkNodeId = true
		if err := waitForBooleanTrue(ctx, ch, params.CheckNodeId); err != nil {
			errorChan <- gin.H{"error": "Timeout beim Warten auf Boolean-Wert", "details": err.Error(), "status": http.StatusRequestTimeout}
			return
		}

		// 5) Bild-String auslesen
		base64String, err := readImageString(ctx, ch, params.ImageNodeId)
		if err != nil {
			errorChan <- gin.H{"error": "Bilddaten konnten nicht gelesen werden", "details": err.Error(), "status": http.StatusInternalServerError}
			return
		}

		// 6) Ack Node = true setzen
		if err := writeBoolean(ctx, ch, params.AckNodeId, true); err != nil {
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
	Endpoint            string            `json:"endpoint"`
	ObjectId            string            `json:"objectId"`
	MethodId            string            `json:"methodId"`
	MethodArgs          json.RawMessage   `json:"methodArgs"`
	CheckNodeId         string            `json:"checkNodeId"`
	ImageNodeId         string            `json:"imageNodeId"`
	AckNodeId           string            `json:"ackNodeId"`
	BasePath            string            `json:"basePath"`
	DeviceId            string            `json:"deviceId"`
	EnableUpload        string            `json:"enableUpload"`
	UploadUrl           string            `json:"uploadUrl"`
	SecurityMode        string            `json:"securityMode"`
	SecurityPolicy      string            `json:"securityPolicy"`
	Username            string            `json:"username"`
	Password            string            `json:"password"`
	Headers             map[string]string `json:"headers"`
	TimestampHeaderName string            `json:"timestampHeaderName"`
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
		params.BasePath = "./data/shared/images"
	}
	if params.SecurityMode == "" {
		params.SecurityMode = "NONE"
	}
	if params.SecurityPolicy == "" {
		params.SecurityPolicy = "NONE"
	}
	if params.TimestampHeaderName == "" {
		params.TimestampHeaderName = "Timestamp"
	}

	// wenn deviceid leer dann schaue ob in tabelle vorhanden
	if params.DeviceId == "" {
		db, err := getDBConnection(c)
		if err != nil {
			logrus.Errorf("Fehler beim Verbinden mit der Datenbank: %v", err)
			return params, err
		}

		// get id from tabelle devices where address = params.Endpoint
		var id int
		err = db.QueryRow("SELECT id FROM devices WHERE address = ?", params.Endpoint).Scan(&id)
		if err != nil {
			logrus.Errorf("Device not found in database: %v", err)
			return params, err
		}

		params.DeviceId = strconv.Itoa(id)
	} else if params.DeviceId == "0" {
		// nehme nur deviceid wenn nicht in tabelle vorhanden
		params.DeviceId = "999999"
	}

	return params, nil
}

// createAndConnectOPCUAClient erstellt und verbindet einen OPC UA Client
func createAndConnectOPCUAClient(params ImageCaptureParams) (*client.Client, error) {
	// OPC UA Client-Optionen erstellen (vereinfacht für awcullen/opcua)
	opts := []client.Option{
		client.WithInsecureSkipVerify(),
	}

	// TODO: Security und Auth-Optionen müssen für awcullen/opcua anders implementiert werden
	// Die aktuelle Implementation ist vereinfacht
	logrus.Infof("Connecting to OPC UA server with simplified options (SecurityMode: %s, SecurityPolicy: %s, User: %s)",
		params.SecurityMode, params.SecurityPolicy, params.Username)

	// OPC UA Client erstellen und verbinden
	ctx := context.Background()
	ch, err := client.Dial(ctx, params.Endpoint, opts...)
	if err != nil {
		logrus.Errorf("Fehler beim Verbinden mit OPC UA Server: %v", err)
		return nil, err
	}

	return ch, nil
}

// callOPCUAMethod ruft eine Methode auf dem OPC UA Server auf (verwendet gopcua für Method Calls)
func callOPCUAMethod(ctx context.Context, params ImageCaptureParams) error {
	// Erstelle temporären gopcua Client nur für Method Call
	client, err := opcua.NewClient(params.Endpoint)
	if err != nil {
		logrus.Errorf("Fehler beim Erstellen des OPC UA Clients für Method Call: %v", err)
		return err
	}

	if err := client.Connect(ctx); err != nil {
		logrus.Errorf("Fehler beim Verbinden für Method Call: %v", err)
		return err
	}
	defer client.Close(ctx)

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

// waitForBooleanTrue wartet, bis der angegebene Knoten den Wert true hat (awcullen/opcua)
func waitForBooleanTrue(ctx context.Context, ch *client.Client, nodeIDString string) error {
	checkNodeID := awcullenua.ParseNodeID(nodeIDString)

	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Boolean-Wert lesen mit awcullen/opcua
			req := &awcullenua.ReadRequest{
				NodesToRead: []awcullenua.ReadValueID{
					{
						NodeID:      checkNodeID,
						AttributeID: awcullenua.AttributeIDValue,
					},
				},
				TimestampsToReturn: awcullenua.TimestampsToReturnBoth,
			}

			resp, err := ch.Read(ctx, req)
			if err != nil {
				logrus.Errorf("Fehler beim Lesen des Boolean-Werts: %v", err)
				continue
			}

			if len(resp.Results) > 0 && resp.Results[0].StatusCode.IsGood() {
				if val, ok := resp.Results[0].Value.(bool); ok && val {
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

// readImageString liest den Bild-String aus dem angegebenen Knoten (awcullen/opcua)
func readImageString(ctx context.Context, ch *client.Client, nodeIDString string) (string, error) {
	imageNodeID := awcullenua.ParseNodeID(nodeIDString)

	req := &awcullenua.ReadRequest{
		NodesToRead: []awcullenua.ReadValueID{
			{
				NodeID:      imageNodeID,
				AttributeID: awcullenua.AttributeIDValue,
			},
		},
		TimestampsToReturn: awcullenua.TimestampsToReturnBoth,
	}

	resp, err := ch.Read(ctx, req)
	if err != nil {
		logrus.Errorf("Fehler beim Lesen des Bildes: %v", err)
		return "", err
	}

	if len(resp.Results) == 0 || !resp.Results[0].StatusCode.IsGood() {
		logrus.Error("Bilddaten konnten nicht gelesen werden")
		return "", fmt.Errorf("bilddaten konnten nicht gelesen werden")
	}

	var base64String string
	dataValue := resp.Results[0].Value

	// Versuche verschiedene Datentypen zu handhaben
	logrus.Debugf("Bilddaten-Typ erhalten: %T, Wert-Info: %v", dataValue, dataValue)

	// Debug: Zeige mehr Details über den Wert
	if dataValue != nil {
		reflectVal := reflect.ValueOf(dataValue)
		logrus.Debugf("Reflect-Info: Kind=%v, Type=%v, Len=%v", reflectVal.Kind(), reflectVal.Type(),
			func() interface{} {
				if reflectVal.Kind() == reflect.Slice || reflectVal.Kind() == reflect.Array {
					return reflectVal.Len()
				}
				return "N/A"
			}())
	}

	switch v := dataValue.(type) {
	case []byte:
		// Rohe Bytes - direkt zu base64 konvertieren
		logrus.Debugf("Bilddaten als []byte erhalten (%d bytes)", len(v))
		base64String = base64.StdEncoding.EncodeToString(v)

	case string:
		// Prüfe ob es bereits base64 ist oder roher String
		if isValidBase64(v) {
			logrus.Debug("Bilddaten als base64-String erhalten")
			base64String = v
		} else {
			// Roher String (möglicherweise Binärdaten) - zu base64 konvertieren
			logrus.Debugf("Bilddaten als roher String erhalten (%d Zeichen), beginnt mit: %q", len(v), getSafePrefix(v, 20))
			base64String = base64.StdEncoding.EncodeToString([]byte(v))
		}

	case awcullenua.ByteString:
		// Expliziter Case für ua.ByteString
		logrus.Debugf("Bilddaten als ua.ByteString erhalten (%d bytes)", len(v))
		base64String = base64.StdEncoding.EncodeToString([]byte(v))

	case []interface{}:
		// Array von Interface{} - konvertiere zu Bytes
		logrus.Debugf("Bilddaten als []interface{} erhalten (%d Elemente)", len(v))
		bytes := make([]byte, len(v))
		for i, val := range v {
			if byteVal, ok := val.(byte); ok {
				bytes[i] = byteVal
			} else if intVal, ok := val.(int); ok {
				bytes[i] = byte(intVal)
			} else if uintVal, ok := val.(uint8); ok {
				bytes[i] = uintVal
			} else {
				// Fallback: konvertiere zu int und dann zu byte
				bytes[i] = byte(fmt.Sprintf("%v", val)[0])
			}
		}
		base64String = base64.StdEncoding.EncodeToString(bytes)

	case interface{}:
		// Generisches Interface - versuche verschiedene Konvertierungen
		logrus.Debugf("Bilddaten als interface{} erhalten: %T", v)

		// Versuche reflect um den echten Typ zu finden
		reflectVal := reflect.ValueOf(v)
		switch reflectVal.Kind() {
		case reflect.Slice, reflect.Array:
			// Slice oder Array - versuche zu Bytes zu konvertieren
			length := reflectVal.Len()
			bytes := make([]byte, length)
			for i := 0; i < length; i++ {
				elem := reflectVal.Index(i)
				if elem.CanInterface() {
					switch elemVal := elem.Interface().(type) {
					case uint8:
						bytes[i] = elemVal
					case int:
						bytes[i] = byte(elemVal)
					case int8:
						bytes[i] = byte(elemVal)
					case int16:
						bytes[i] = byte(elemVal)
					case int32:
						bytes[i] = byte(elemVal)
					case int64:
						bytes[i] = byte(elemVal)
					case uint16:
						bytes[i] = byte(elemVal)
					case uint32:
						bytes[i] = byte(elemVal)
					case uint64:
						bytes[i] = byte(elemVal)
					case float32:
						bytes[i] = byte(elemVal)
					case float64:
						bytes[i] = byte(elemVal)
					default:
						// Versuche String-Konvertierung
						if str := fmt.Sprintf("%v", elemVal); len(str) > 0 {
							bytes[i] = byte(str[0])
						} else {
							bytes[i] = byte(0)
						}
					}
				}
			}
			logrus.Debugf("Konvertiert %d Elemente zu Bytes", length)
			base64String = base64.StdEncoding.EncodeToString(bytes)
		default:
			// String-Konvertierung als letzter Ausweg
			stringValue := fmt.Sprintf("%v", v)
			logrus.Warnf("Fallback String-Konvertierung für Typ %T: %s", v, getSafePrefix(stringValue, 50))
			base64String = base64.StdEncoding.EncodeToString([]byte(stringValue))
		}

	default:
		// Letzter Fallback: Versuche String-Konvertierung
		stringValue := fmt.Sprintf("%v", dataValue)
		logrus.Warnf("Unerwarteter Datentyp für Bilddaten: %T, String-Fallback: %s", dataValue, getSafePrefix(stringValue, 50))
		base64String = base64.StdEncoding.EncodeToString([]byte(stringValue))
	}

	return base64String, nil
}

// isValidBase64 prüft, ob ein String gültiges base64 Format hat
func isValidBase64(s string) bool {
	// Prüfe basic base64 pattern
	if len(s) == 0 || len(s)%4 != 0 {
		return false
	}

	// Versuche zu dekodieren
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// getSafePrefix gibt einen sicheren String-Prefix zurück (ersetzt unprintable Zeichen)
func getSafePrefix(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}

	// Ersetze unprintable Zeichen durch ihre hex-Darstellung
	result := ""
	for _, r := range s {
		if r >= 32 && r <= 126 {
			result += string(r)
		} else {
			result += fmt.Sprintf("\\x%02x", r)
		}
	}
	return result
}

// writeBoolean schreibt einen booleschen Wert in den angegebenen Knoten (awcullen/opcua)
func writeBoolean(ctx context.Context, ch *client.Client, nodeIDString string, value bool) error {
	nodeID := awcullenua.ParseNodeID(nodeIDString)

	writeReq := &awcullenua.WriteRequest{
		NodesToWrite: []awcullenua.WriteValue{
			{
				NodeID:      nodeID,
				AttributeID: awcullenua.AttributeIDValue,
				Value: awcullenua.DataValue{
					Value: value,
				},
			},
		},
	}

	writeResp, err := ch.Write(ctx, writeReq)
	if err != nil {
		logrus.Errorf("Fehler beim Schreiben des Werts: %v", err)
		return err
	}

	if len(writeResp.Results) > 0 && !writeResp.Results[0].IsGood() {
		logrus.Errorf("Schreiben des Werts fehlgeschlagen mit Status: %v", writeResp.Results[0])
		return fmt.Errorf("schreiben des Werts fehlgeschlagen mit Status: %v", writeResp.Results[0])
	}

	return nil
}

// handleUploads verarbeitet den Upload des Bildes in die lokale und externe Datenbank
func handleUploads(c *gin.Context, params ImageCaptureParams, base64String string) (bool, error) {
	uploadErfolgt := false

	// Wenn eine externe Upload-URL angegeben ist, auch dorthin hochladen
	if params.UploadUrl != "" {
		// Externer Upload per HTTP
		externalUploadSuccess := uploadToExternalDatabase(base64String, params.UploadUrl, params.Headers, params.DeviceId, params.TimestampHeaderName)
		uploadErfolgt = uploadErfolgt || externalUploadSuccess
	}

	return uploadErfolgt, nil
}

// uploadToExternalDatabase lädt ein Bild per HTTP zu einer externen Datenbank hoch
func uploadToExternalDatabase(base64String, uploadUrl string, headers map[string]string, deviceId string, timestampHeaderName string) bool {
	// Standardheader setzen, falls keine angegeben sind
	if headers == nil {
		headers = make(map[string]string)
	}

	// Standardwert für Content-Type setzen, falls nicht angegeben
	// (momentan nicht verwendet)

	// HTTP-Client mit angepassten Einstellungen erstellen
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Zertifikatsüberprüfung deaktivieren (nur für Entwicklung!)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
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

	// Timestamp-Header hinzufügen falls konfiguriert und nicht bereits vorhanden
	if timestampHeaderName != "" {
		if _, ok := headers[timestampHeaderName]; !ok {
			req.Header.Set(timestampHeaderName, time.Now().Format("2006-01-02 15:04:05"))
		}
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
