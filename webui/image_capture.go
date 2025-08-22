package webui

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// showImageCapturePage zeigt die Image Capture Prozessverwaltungsseite an
func showImageCapturePage(c *gin.Context) {
	c.HTML(http.StatusOK, "image-capture.html", gin.H{
		"title": "Image Capture Prozessverwaltung",
	})
}

// ImageCaptureProcess repräsentiert einen Image Capture Prozess
type ImageCaptureProcess struct {
	ID             int                    `json:"id"`
	Name           string                 `json:"name"`
	DeviceID       int                    `json:"device_id"`
	DeviceName     string                 `json:"device_name"`
	Endpoint       string                 `json:"endpoint"`
	ObjectID       string                 `json:"object_id"`
	MethodID       string                 `json:"method_id"`
	MethodArgs     map[string]interface{} `json:"method_args"`
	CheckNodeID    string                 `json:"check_node_id"`
	ImageNodeID    string                 `json:"image_node_id"`
	AckNodeID      string                 `json:"ack_node_id"`
	EnableUpload   bool                   `json:"enable_upload"`
	UploadURL      string                 `json:"upload_url"`
	UploadHeaders  map[string]string      `json:"upload_headers"`
	EnableCyclic   bool                   `json:"enable_cyclic"`
	CyclicInterval int                    `json:"cyclic_interval"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"`
	LastExecution  *time.Time             `json:"last_execution"`
	LastImage      string                 `json:"last_image"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// ProcessManager verwaltet die laufenden Image Capture Prozesse
type ProcessManager struct {
	processes map[int]*RunningProcess
	mutex     sync.RWMutex
}

// RunningProcess repräsentiert einen laufenden Prozess
type RunningProcess struct {
	Process   *ImageCaptureProcess
	StopChan  chan bool
	IsRunning bool
	LastError string
	LastImage string
	StartTime time.Time
}

var processManager = &ProcessManager{
	processes: make(map[int]*RunningProcess),
}

// Globale Datenbankvariable für Image Capture Prozesse
var globalDB *sql.DB

// InitImageCaptureProcesses initialisiert alle laufenden Prozesse beim Start der Anwendung
func InitImageCaptureProcesses(db *sql.DB) {
	if db == nil {
		logrus.Errorf("Datenbankverbindung ist nil beim Initialisieren der Image Capture Prozesse")
		return
	}

	// Globale Datenbankvariable setzen
	globalDB = db

	query := `
		SELECT 
			icp.id, icp.name, icp.device_id, d.name as device_name,
			icp.endpoint, icp.object_id, icp.method_id, icp.method_args,
			icp.check_node_id, icp.image_node_id, icp.ack_node_id,
			icp.enable_upload, icp.upload_url, icp.upload_headers,
			icp.enable_cyclic, icp.cyclic_interval, icp.description,
			icp.status, icp.last_execution, icp.last_image,
			icp.created_at, icp.updated_at
		FROM image_capture_processes icp
		LEFT JOIN devices d ON icp.device_id = d.id
		WHERE icp.status = 'running'
	`

	rows, err := db.Query(query)
	if err != nil {
		logrus.Errorf("Fehler beim Abrufen der laufenden Prozesse: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var process ImageCaptureProcess
		var methodArgsStr, uploadHeadersStr sql.NullString
		var lastExecutionStr sql.NullString

		err := rows.Scan(
			&process.ID, &process.Name, &process.DeviceID, &process.DeviceName,
			&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
			&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
			&process.EnableUpload, &process.UploadURL, &uploadHeadersStr,
			&process.EnableCyclic, &process.CyclicInterval, &process.Description,
			&process.Status, &lastExecutionStr, &process.LastImage,
			&process.CreatedAt, &process.UpdatedAt,
		)
		if err != nil {
			logrus.Errorf("Fehler beim Scannen der Prozessdaten: %v", err)
			continue
		}

		// MethodArgs parsen
		if methodArgsStr.Valid && methodArgsStr.String != "" {
			if err := json.Unmarshal([]byte(methodArgsStr.String), &process.MethodArgs); err != nil {
				logrus.Errorf("Fehler beim Parsen der MethodArgs: %v", err)
			}
		}

		// UploadHeaders parsen
		if uploadHeadersStr.Valid && uploadHeadersStr.String != "" {
			if err := json.Unmarshal([]byte(uploadHeadersStr.String), &process.UploadHeaders); err != nil {
				logrus.Errorf("Fehler beim Parsen der UploadHeaders: %v", err)
			}
		}

		// Prozess starten
		if err := processManager.StartProcess(&process); err != nil {
			logrus.Errorf("Fehler beim Starten des Prozesses %d: %v", process.ID, err)
		} else {
			logrus.Infof("Prozess %s (ID: %d) erfolgreich wiederhergestellt", process.Name, process.ID)
		}
	}
}

// getImageCaptureProcesses holt alle Image Capture Prozesse aus der Datenbank
func getImageCaptureProcesses(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	query := `
		SELECT 
			icp.id, icp.name, icp.device_id, d.name as device_name,
			icp.endpoint, icp.object_id, icp.method_id, icp.method_args,
			icp.check_node_id, icp.image_node_id, icp.ack_node_id,
			icp.enable_upload, icp.upload_url, icp.upload_headers,
			icp.enable_cyclic, icp.cyclic_interval, icp.description,
			icp.status, icp.last_execution, icp.last_image,
			icp.created_at, icp.updated_at
		FROM image_capture_processes icp
		LEFT JOIN devices d ON icp.device_id = d.id
		ORDER BY icp.created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Abrufen der Prozesse"})
		return
	}
	defer rows.Close()

	var processes []ImageCaptureProcess
	for rows.Next() {
		process, err := scanProcessFromRow(rows)
		if err != nil {
			logrus.Errorf("Fehler beim Scannen der Prozessdaten: %v", err)
			continue
		}

		// Aktuellen Status und Informationen aus dem ProcessManager holen
		updateProcessStatusFromManager(&process)

		processes = append(processes, process)
	}

	c.JSON(http.StatusOK, gin.H{"processes": processes})
}

// getImageCaptureProcess holt einen einzelnen Image Capture Prozess
func getImageCaptureProcess(c *gin.Context) {
	processID := c.Param("id")
	if processID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prozess-ID erforderlich"})
		return
	}

	id, err := strconv.Atoi(processID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Prozess-ID"})
		return
	}

	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	query := `
		SELECT 
			icp.id, icp.name, icp.device_id, d.name as device_name,
			icp.endpoint, icp.object_id, icp.method_id, icp.method_args,
			icp.check_node_id, icp.image_node_id, icp.ack_node_id,
			icp.enable_upload, icp.upload_url, icp.upload_headers,
			icp.enable_cyclic, icp.cyclic_interval, icp.description,
			icp.status, icp.last_execution, icp.last_image,
			icp.created_at, icp.updated_at
		FROM image_capture_processes icp
		LEFT JOIN devices d ON icp.device_id = d.id
		WHERE icp.id = ?
	`

	row := db.QueryRow(query, id)
	process, err := scanProcessFromQueryRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Prozess nicht gefunden"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Abrufen des Prozesses"})
		}
		return
	}

	// Aktuellen Status aus dem ProcessManager holen
	updateProcessStatusFromManager(&process)

	c.JSON(http.StatusOK, gin.H{"process": process})
}

// addImageCaptureProcess erstellt einen neuen Image Capture Prozess
func addImageCaptureProcess(c *gin.Context) {
	var process ImageCaptureProcess
	if err := c.ShouldBindJSON(&process); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Daten", "details": err.Error()})
		return
	}

	// Validierung
	if process.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name ist erforderlich"})
		return
	}

	if process.DeviceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geräte-ID ist erforderlich"})
		return
	}

	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	// MethodArgs und UploadHeaders in JSON konvertieren
	methodArgsJSON, _ := json.Marshal(process.MethodArgs)
	uploadHeadersJSON, _ := json.Marshal(process.UploadHeaders)

	now := time.Now()
	query := `
		INSERT INTO image_capture_processes (
			name, device_id, endpoint, object_id, method_id, method_args,
			check_node_id, image_node_id, ack_node_id, enable_upload, upload_url, upload_headers,
			enable_cyclic, cyclic_interval, description, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query,
		process.Name, process.DeviceID, process.Endpoint, process.ObjectID, process.MethodID, string(methodArgsJSON),
		process.CheckNodeID, process.ImageNodeID, process.AckNodeID, process.EnableUpload, process.UploadURL, string(uploadHeadersJSON),
		process.EnableCyclic, process.CyclicInterval, process.Description, "stopped", now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Erstellen des Prozesses"})
		return
	}

	id, _ := result.LastInsertId()
	process.ID = int(id)
	process.Status = "stopped"
	process.CreatedAt = now
	process.UpdatedAt = now

	c.JSON(http.StatusCreated, gin.H{"process": process, "message": "Prozess erfolgreich erstellt"})
}

// updateImageCaptureProcess aktualisiert einen bestehenden Image Capture Prozess
func updateImageCaptureProcess(c *gin.Context) {
	processID := c.Param("id")
	if processID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prozess-ID erforderlich"})
		return
	}

	id, err := strconv.Atoi(processID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Prozess-ID"})
		return
	}

	var process ImageCaptureProcess
	if err := c.ShouldBindJSON(&process); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Daten", "details": err.Error()})
		return
	}

	process.ID = id

	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	// MethodArgs und UploadHeaders in JSON konvertieren
	methodArgsJSON, _ := json.Marshal(process.MethodArgs)
	uploadHeadersJSON, _ := json.Marshal(process.UploadHeaders)

	now := time.Now()
	query := `
		UPDATE image_capture_processes SET
			name = ?, device_id = ?, endpoint = ?, object_id = ?, method_id = ?, method_args = ?,
			check_node_id = ?, image_node_id = ?, ack_node_id = ?, enable_upload = ?, upload_url = ?, upload_headers = ?,
			enable_cyclic = ?, cyclic_interval = ?, description = ?, updated_at = ?
		WHERE id = ?
	`

	_, err = db.Exec(query,
		process.Name, process.DeviceID, process.Endpoint, process.ObjectID, process.MethodID, string(methodArgsJSON),
		process.CheckNodeID, process.ImageNodeID, process.AckNodeID, process.EnableUpload, process.UploadURL, string(uploadHeadersJSON),
		process.EnableCyclic, process.CyclicInterval, process.Description, now, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Aktualisieren des Prozesses"})
		return
	}

	process.UpdatedAt = now
	c.JSON(http.StatusOK, gin.H{"process": process, "message": "Prozess erfolgreich aktualisiert"})
}

// deleteImageCaptureProcess löscht einen Image Capture Prozess
func deleteImageCaptureProcess(c *gin.Context) {
	processID := c.Param("id")
	if processID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prozess-ID erforderlich"})
		return
	}

	id, err := strconv.Atoi(processID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Prozess-ID"})
		return
	}

	// Prozess stoppen, falls er läuft
	processManager.StopProcess(id)

	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	query := "DELETE FROM image_capture_processes WHERE id = ?"
	_, err = db.Exec(query, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Löschen des Prozesses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Prozess erfolgreich gelöscht"})
}

// startImageCaptureProcess startet einen Image Capture Prozess
func startImageCaptureProcess(c *gin.Context) {
	processID := c.Param("id")
	if processID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prozess-ID erforderlich"})
		return
	}

	id, err := strconv.Atoi(processID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Prozess-ID"})
		return
	}

	// Prozess aus der Datenbank holen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	query := `
		SELECT 
			icp.id, icp.name, icp.device_id, d.name as device_name,
			icp.endpoint, icp.object_id, icp.method_id, icp.method_args,
			icp.check_node_id, icp.image_node_id, icp.ack_node_id,
			icp.enable_upload, icp.upload_url, icp.upload_headers,
			icp.enable_cyclic, icp.cyclic_interval, icp.description,
			icp.status, icp.last_execution, icp.last_image,
			icp.created_at, icp.updated_at
		FROM image_capture_processes icp
		LEFT JOIN devices d ON icp.device_id = d.id
		WHERE icp.id = ?
	`

	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr sql.NullString

	err = db.QueryRow(query, id).Scan(
		&process.ID, &process.Name, &process.DeviceID, &process.DeviceName,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &process.UploadURL, &uploadHeadersStr,
		&process.EnableCyclic, &process.CyclicInterval, &process.Description,
		&process.Status, &lastExecutionStr, &process.LastImage,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Prozess nicht gefunden"})
		return
	}

	// MethodArgs und UploadHeaders parsen
	if methodArgsStr.Valid && methodArgsStr.String != "" {
		if err := json.Unmarshal([]byte(methodArgsStr.String), &process.MethodArgs); err != nil {
			logrus.Errorf("Fehler beim Parsen der MethodArgs: %v", err)
		}
	}

	if uploadHeadersStr.Valid && uploadHeadersStr.String != "" {
		if err := json.Unmarshal([]byte(uploadHeadersStr.String), &process.UploadHeaders); err != nil {
			logrus.Errorf("Fehler beim Parsen der UploadHeaders: %v", err)
		}
	}

	// Prozess starten
	if err := processManager.StartProcess(&process); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Starten des Prozesses", "details": err.Error()})
		return
	}

	// Status in der Datenbank auf "running" setzen
	now := time.Now()
	updateQuery := `UPDATE image_capture_processes SET status = ?, updated_at = ? WHERE id = ?`
	_, err = db.Exec(updateQuery, "running", now, id)
	if err != nil {
		logrus.Errorf("Fehler beim Aktualisieren des Prozessstatus: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Prozess erfolgreich gestartet"})
}

// stopImageCaptureProcess stoppt einen Image Capture Prozess
func stopImageCaptureProcess(c *gin.Context) {
	processID := c.Param("id")
	if processID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prozess-ID erforderlich"})
		return
	}

	id, err := strconv.Atoi(processID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Prozess-ID"})
		return
	}

	processManager.StopProcess(id)

	// Status in der Datenbank auf "stopped" setzen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	now := time.Now()
	updateQuery := `UPDATE image_capture_processes SET status = ?, updated_at = ? WHERE id = ?`
	_, err = db.Exec(updateQuery, "stopped", now, id)
	if err != nil {
		logrus.Errorf("Fehler beim Aktualisieren des Prozessstatus: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Prozess erfolgreich gestoppt"})
}

// executeImageCaptureProcess führt einen einmaligen Image Capture aus
func executeImageCaptureProcess(c *gin.Context) {
	processID := c.Param("id")
	if processID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prozess-ID erforderlich"})
		return
	}

	id, err := strconv.Atoi(processID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Prozess-ID"})
		return
	}

	// Prozess aus der Datenbank holen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	query := `
		SELECT 
			icp.id, icp.name, icp.device_id, d.name as device_name,
			icp.endpoint, icp.object_id, icp.method_id, icp.method_args,
			icp.check_node_id, icp.image_node_id, icp.ack_node_id,
			icp.enable_upload, icp.upload_url, icp.upload_headers,
			icp.enable_cyclic, icp.cyclic_interval, icp.description,
			icp.status, icp.last_execution, icp.last_image,
			icp.created_at, icp.updated_at
		FROM image_capture_processes icp
		LEFT JOIN devices d ON icp.device_id = d.id
		WHERE icp.id = ?
	`

	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr sql.NullString

	err = db.QueryRow(query, id).Scan(
		&process.ID, &process.Name, &process.DeviceID, &process.DeviceName,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &process.UploadURL, &uploadHeadersStr,
		&process.EnableCyclic, &process.CyclicInterval, &process.Description,
		&process.Status, &lastExecutionStr, &process.LastImage,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Prozess nicht gefunden"})
		return
	}

	// MethodArgs und UploadHeaders parsen
	if methodArgsStr.Valid && methodArgsStr.String != "" {
		if err := json.Unmarshal([]byte(methodArgsStr.String), &process.MethodArgs); err != nil {
			logrus.Errorf("Fehler beim Parsen der MethodArgs: %v", err)
		}
	}

	if uploadHeadersStr.Valid && uploadHeadersStr.String != "" {
		if err := json.Unmarshal([]byte(uploadHeadersStr.String), &process.UploadHeaders); err != nil {
			logrus.Errorf("Fehler beim Parsen der UploadHeaders: %v", err)
		}
	}

	// Einmalige Ausführung in einer Go-Routine
	go func() {
		// Geräteinformationen aus der Datenbank holen
		deviceQuery := `SELECT security_mode, security_policy, username, password FROM devices WHERE id = ?`
		var securityMode, securityPolicy, username, password sql.NullString
		db.QueryRow(deviceQuery, process.DeviceID).Scan(&securityMode, &securityPolicy, &username, &password)

		params := ImageCaptureParams{
			Endpoint:       process.Endpoint,
			ObjectId:       process.ObjectID,
			MethodId:       process.MethodID,
			MethodArgs:     json.RawMessage(methodArgsStr.String),
			CheckNodeId:    process.CheckNodeID,
			ImageNodeId:    process.ImageNodeID,
			AckNodeId:      process.AckNodeID,
			DeviceId:       strconv.Itoa(process.DeviceID),
			EnableUpload:   fmt.Sprintf("%t", process.EnableUpload),
			UploadUrl:      process.UploadURL,
			SecurityMode:   securityMode.String,
			SecurityPolicy: securityPolicy.String,
			Username:       username.String,
			Password:       password.String,
			Headers:        process.UploadHeaders,
		}

		// Image Capture ausführen
		client, err := createAndConnectOPCUAClient(params)
		if err != nil {
			logrus.Errorf("Fehler beim Erstellen des OPC UA Clients: %v", err)
			return
		}

		ctx := context.Background()
		defer client.Close(ctx)

		// Methode aufrufen
		if err := callOPCUAMethod(ctx, client, params); err != nil {
			logrus.Errorf("Fehler beim Methodenaufruf: %v", err)
			return
		}

		// Warten auf Boolean-Wert
		if err := waitForBooleanTrue(ctx, client, params.CheckNodeId); err != nil {
			logrus.Errorf("Fehler beim Warten auf Boolean-Wert: %v", err)
			return
		}

		// Bild lesen
		base64String, err := readImageString(ctx, client, params.ImageNodeId)
		if err != nil {
			logrus.Errorf("Fehler beim Lesen des Bildes: %v", err)
			return
		}

		// Ack setzen
		if err := writeBoolean(ctx, client, params.AckNodeId, true); err != nil {
			logrus.Errorf("Fehler beim Setzen des Ack-Werts: %v", err)
		}

		// Upload verarbeiten
		if process.EnableUpload {
			handleUploads(c, params, base64String)
		}

		// Datenbank aktualisieren
		now := time.Now()
		updateQuery := `
			UPDATE image_capture_processes SET
				last_execution = ?, last_image = ?, updated_at = ?
			WHERE id = ?
		`
		db.Exec(updateQuery, now.Format(time.RFC3339), base64String, now, process.ID)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Image Capture gestartet"})
}

// StartProcess startet einen Image Capture Prozess
func (pm *ProcessManager) StartProcess(process *ImageCaptureProcess) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Prüfen, ob der Prozess bereits läuft
	if _, exists := pm.processes[process.ID]; exists {
		return fmt.Errorf("prozess läuft bereits")
	}

	// Neuen laufenden Prozess erstellen
	runningProcess := &RunningProcess{
		Process:   process,
		StopChan:  make(chan bool),
		IsRunning: true,
		StartTime: time.Now(),
	}

	pm.processes[process.ID] = runningProcess

	// Prozess in einer Go-Routine starten
	go pm.runProcess(runningProcess)

	return nil
}

// StopProcess stoppt einen Image Capture Prozess
func (pm *ProcessManager) StopProcess(processID int) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if runningProcess, exists := pm.processes[processID]; exists {
		runningProcess.IsRunning = false
		close(runningProcess.StopChan)
		delete(pm.processes, processID)
	}
}

// runProcess führt den Image Capture Prozess aus
func (pm *ProcessManager) runProcess(runningProcess *RunningProcess) {
	process := runningProcess.Process

	// Intervall für zyklische Ausführung bestimmen
	interval := time.Duration(process.CyclicInterval) * time.Second
	if interval < time.Second {
		interval = 30 * time.Second // Standard: 30 Sekunden
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !runningProcess.IsRunning {
				return
			}

			// MethodArgs konvertieren
			methodArgsJSON := json.RawMessage("{}")
			if process.MethodArgs != nil {
				if jsonBytes, err := json.Marshal(process.MethodArgs); err == nil {
					methodArgsJSON = json.RawMessage(jsonBytes)
				}
			}

			// Geräteinformationen aus der Datenbank holen
			if globalDB == nil {
				runningProcess.LastError = "Datenbankverbindung fehlgeschlagen"
				logrus.Errorf("Globale Datenbankverbindung ist nil")
				continue
			}

			deviceQuery := `SELECT security_mode, security_policy, username, password FROM devices WHERE id = ?`
			var securityMode, securityPolicy, username, password sql.NullString
			globalDB.QueryRow(deviceQuery, process.DeviceID).Scan(&securityMode, &securityPolicy, &username, &password)

			// Image Capture ausführen
			params := ImageCaptureParams{
				Endpoint:       process.Endpoint,
				ObjectId:       process.ObjectID,
				MethodId:       process.MethodID,
				MethodArgs:     methodArgsJSON,
				CheckNodeId:    process.CheckNodeID,
				ImageNodeId:    process.ImageNodeID,
				AckNodeId:      process.AckNodeID,
				DeviceId:       strconv.Itoa(process.DeviceID),
				EnableUpload:   fmt.Sprintf("%t", process.EnableUpload),
				UploadUrl:      process.UploadURL,
				SecurityMode:   securityMode.String,
				SecurityPolicy: securityPolicy.String,
				Username:       username.String,
				Password:       password.String,
				Headers:        process.UploadHeaders,
			}

			// Image Capture ausführen
			client, err := createAndConnectOPCUAClient(params)
			if err != nil {
				runningProcess.LastError = err.Error()
				logrus.Errorf("Fehler beim Erstellen des OPC UA Clients: %v", err)
				continue
			}

			ctx := context.Background()
			defer client.Close(ctx)

			// Methode aufrufen
			if err := callOPCUAMethod(ctx, client, params); err != nil {
				runningProcess.LastError = err.Error()
				logrus.Errorf("Fehler beim Methodenaufruf: %v", err)
				continue
			}

			// Warten auf Boolean-Wert
			if err := waitForBooleanTrue(ctx, client, params.CheckNodeId); err != nil {
				runningProcess.LastError = err.Error()
				logrus.Errorf("Fehler beim Warten auf Boolean-Wert: %v", err)
				continue
			}

			// Bild lesen
			base64String, err := readImageString(ctx, client, params.ImageNodeId)
			if err != nil {
				runningProcess.LastError = err.Error()
				logrus.Errorf("Fehler beim Lesen des Bildes: %v", err)
				continue
			}

			// Ack setzen
			if err := writeBoolean(ctx, client, params.AckNodeId, true); err != nil {
				logrus.Errorf("Fehler beim Setzen des Ack-Werts: %v", err)
			}

			// Erfolgreiche Ausführung
			runningProcess.LastError = ""
			runningProcess.LastImage = base64String

			// Datenbank aktualisieren
			updateProcessExecutionInfo(process.ID, base64String)

		case <-runningProcess.StopChan:
			return
		}
	}
}

// StopAllImageCaptureProcesses stoppt alle laufenden Image Capture Prozesse
func StopAllImageCaptureProcesses(db *sql.DB) {
	processManager.mutex.Lock()
	defer processManager.mutex.Unlock()

	for processID, runningProcess := range processManager.processes {
		runningProcess.IsRunning = false
		close(runningProcess.StopChan)
		logrus.Infof("Prozess %d gestoppt", processID)
	}

	// Map leeren
	processManager.processes = make(map[int]*RunningProcess)

	// Status aller Prozesse in der Datenbank auf "stopped" setzen
	if db == nil {
		logrus.Errorf("Datenbankverbindung ist nil beim Stoppen der Prozesse")
		return
	}

	now := time.Now()
	updateQuery := `UPDATE image_capture_processes SET status = ?, updated_at = ? WHERE status = 'running'`
	_, err := db.Exec(updateQuery, "stopped", now)
	if err != nil {
		logrus.Errorf("Fehler beim Aktualisieren der Prozessstatus: %v", err)
	} else {
		logrus.Info("Alle Image Capture Prozesse gestoppt und Status aktualisiert")
	}
}

// scanProcessFromRow scannt einen Prozess aus einer Datenbankzeile
func scanProcessFromRow(rows *sql.Rows) (ImageCaptureProcess, error) {
	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr sql.NullString

	err := rows.Scan(
		&process.ID, &process.Name, &process.DeviceID, &process.DeviceName,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &process.UploadURL, &uploadHeadersStr,
		&process.EnableCyclic, &process.CyclicInterval, &process.Description,
		&process.Status, &lastExecutionStr, &process.LastImage,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		return process, err
	}

	return parseProcessData(process, methodArgsStr, uploadHeadersStr, lastExecutionStr)
}

// scanProcessFromQueryRow scannt einen Prozess aus einer QueryRow
func scanProcessFromQueryRow(row *sql.Row) (ImageCaptureProcess, error) {
	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr sql.NullString

	err := row.Scan(
		&process.ID, &process.Name, &process.DeviceID, &process.DeviceName,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &process.UploadURL, &uploadHeadersStr,
		&process.EnableCyclic, &process.CyclicInterval, &process.Description,
		&process.Status, &lastExecutionStr, &process.LastImage,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		return process, err
	}

	return parseProcessData(process, methodArgsStr, uploadHeadersStr, lastExecutionStr)
}

// parseProcessData parst die JSON-Felder und LastExecution eines Prozesses
func parseProcessData(process ImageCaptureProcess, methodArgsStr, uploadHeadersStr sql.NullString, lastExecutionStr sql.NullString) (ImageCaptureProcess, error) {
	// MethodArgs parsen
	if methodArgsStr.Valid && methodArgsStr.String != "" {
		if err := json.Unmarshal([]byte(methodArgsStr.String), &process.MethodArgs); err != nil {
			logrus.Errorf("Fehler beim Parsen der MethodArgs für Prozess %d: %v", process.ID, err)
			process.MethodArgs = make(map[string]interface{})
		}
	} else {
		process.MethodArgs = make(map[string]interface{})
	}

	// UploadHeaders parsen
	if uploadHeadersStr.Valid && uploadHeadersStr.String != "" {
		if err := json.Unmarshal([]byte(uploadHeadersStr.String), &process.UploadHeaders); err != nil {
			logrus.Errorf("Fehler beim Parsen der UploadHeaders für Prozess %d: %v", process.ID, err)
			process.UploadHeaders = make(map[string]string)
		}
	} else {
		process.UploadHeaders = make(map[string]string)
	}

	// LastExecution parsen
	if lastExecutionStr.Valid && lastExecutionStr.String != "" {
		if t, err := time.Parse(time.RFC3339, lastExecutionStr.String); err == nil {
			process.LastExecution = &t
		} else {
			logrus.Errorf("Fehler beim Parsen der LastExecution für Prozess %d: %v", process.ID, err)
		}
	}

	return process, nil
}

// updateProcessStatusFromManager aktualisiert den Prozessstatus basierend auf dem ProcessManager
func updateProcessStatusFromManager(process *ImageCaptureProcess) {
	processManager.mutex.RLock()
	defer processManager.mutex.RUnlock()

	if runningProcess, exists := processManager.processes[process.ID]; exists {
		// Prozess läuft im ProcessManager
		if runningProcess.IsRunning {
			if runningProcess.LastError != "" {
				process.Status = "error"
			} else {
				process.Status = "running"
			}
		} else {
			process.Status = "stopped"
		}

		// Aktuelles letztes Bild aus dem RunningProcess übernehmen
		if runningProcess.LastImage != "" {
			process.LastImage = runningProcess.LastImage
		}
	} else {
		// Prozess läuft nicht im ProcessManager, aber könnte in der DB als "running" stehen
		// Das passiert z.B. nach einem Neustart wenn der ProcessManager noch nicht initialisiert ist
		if process.Status == "running" {
			// Falls in DB als "running" markiert, aber nicht im ProcessManager -> als gestoppt betrachten
			process.Status = "stopped"
		}
	}
}

// getProcessByID holt einen Prozess nach ID aus der Datenbank
func getProcessByID(c *gin.Context, id int) (ImageCaptureProcess, error) {
	db, err := getDBConnection(c)
	if err != nil {
		return ImageCaptureProcess{}, err
	}

	query := `
		SELECT 
			icp.id, icp.name, icp.device_id, d.name as device_name,
			icp.endpoint, icp.object_id, icp.method_id, icp.method_args,
			icp.check_node_id, icp.image_node_id, icp.ack_node_id,
			icp.enable_upload, icp.upload_url, icp.upload_headers,
			icp.enable_cyclic, icp.cyclic_interval, icp.description,
			icp.status, icp.last_execution, icp.last_image,
			icp.created_at, icp.updated_at
		FROM image_capture_processes icp
		LEFT JOIN devices d ON icp.device_id = d.id
		WHERE icp.id = ?
	`

	row := db.QueryRow(query, id)
	return scanProcessFromQueryRow(row)
}

// updateProcessExecutionInfo aktualisiert die Ausführungsinformationen eines Prozesses
func updateProcessExecutionInfo(processID int, base64Image string) {
	if globalDB == nil {
		logrus.Errorf("globale Datenbankverbindung ist nil beim Aktualisieren der Prozessdaten")
		return
	}

	now := time.Now()
	updateQuery := `
		UPDATE image_capture_processes SET
			last_execution = ?, last_image = ?, updated_at = ?
		WHERE id = ?
	`

	if _, err := globalDB.Exec(updateQuery, now.Format(time.RFC3339), base64Image, now, processID); err != nil {
		logrus.Errorf("fehler beim Aktualisieren der Prozessdaten: %v", err)
	}
}
