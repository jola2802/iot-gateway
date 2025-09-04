package webui

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"iot-gateway/logic"
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
	ID                  int                    `json:"id"`
	Name                string                 `json:"name"`
	DeviceID            int                    `json:"device_id"`
	DeviceName          string                 `json:"device_name"`
	Endpoint            string                 `json:"endpoint"`
	ObjectID            string                 `json:"object_id"`
	MethodID            string                 `json:"method_id"`
	MethodArgs          map[string]interface{} `json:"method_args"`
	CheckNodeID         string                 `json:"check_node_id"`
	ImageNodeID         string                 `json:"image_node_id"`
	AckNodeID           string                 `json:"ack_node_id"`
	EnableUpload        bool                   `json:"enable_upload"`
	UploadURL           string                 `json:"upload_url"`
	UploadHeaders       map[string]string      `json:"upload_headers"`
	TimestampHeaderName string                 `json:"timestamp_header_name"`
	EnableCyclic        bool                   `json:"enable_cyclic"`
	CyclicInterval      int                    `json:"cyclic_interval"`
	Description         string                 `json:"description"`
	Status              string                 `json:"status"`
	LastExecution       *time.Time             `json:"last_execution"`
	LastImage           string                 `json:"last_image"`
	LastUploadStatus    string                 `json:"last_upload_status"`
	LastUploadError     string                 `json:"last_upload_error"`
	UploadSuccessCount  int                    `json:"upload_success_count"`
	UploadFailureCount  int                    `json:"upload_failure_count"`
	CreatedAt           string                 `json:"created_at"`
	UpdatedAt           string                 `json:"updated_at"`
}

// ProcessManager verwaltet die laufenden Image Capture Prozesse
type ProcessManager struct {
	processes map[int]*RunningProcess
	mutex     sync.RWMutex
}

// IsProcessRunning prüft, ob ein Prozess läuft
func (pm *ProcessManager) IsProcessRunning(processID int) bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if runningProcess, exists := pm.processes[processID]; exists {
		return runningProcess.IsRunning
	}
	return false
}

// IsProcessReallyRunning prüft, ob ein Prozess wirklich läuft
// Diese Funktion wird vor kritischen Operationen aufgerufen
func (pm *ProcessManager) IsProcessReallyRunning(processID int) bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if runningProcess, exists := pm.processes[processID]; exists {
		// Zusätzliche Prüfung: Ist der Prozess nicht nur in der Map, sondern auch wirklich aktiv?
		return runningProcess.IsRunning && runningProcess.StopChan != nil
	}
	return false
}

// GetRunningProcesses gibt eine Liste aller laufenden Prozesse zurück
func (pm *ProcessManager) GetRunningProcesses() []int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var runningIDs []int
	for processID, runningProcess := range pm.processes {
		if runningProcess.IsRunning {
			runningIDs = append(runningIDs, processID)
		}
	}
	return runningIDs
}

// ValidateRunningProcesses prüft alle laufenden Prozesse und stoppt ungültige
func (pm *ProcessManager) ValidateRunningProcesses() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	var processesToStop []int

	for processID, runningProcess := range pm.processes {
		// Prüfen, ob der Prozess wirklich noch laufen sollte
		if !runningProcess.IsRunning || runningProcess.StopChan == nil {
			processesToStop = append(processesToStop, processID)
			logrus.Warnf("Prozess %d ist in einem ungültigen Zustand und wird gestoppt", processID)
		}
	}

	// Ungültige Prozesse stoppen
	for _, processID := range processesToStop {
		if runningProcess, exists := pm.processes[processID]; exists {
			// Channel sicher schließen
			select {
			case <-runningProcess.StopChan:
				// Channel bereits geschlossen
			default:
				close(runningProcess.StopChan)
			}
			delete(pm.processes, processID)
			logrus.Infof("Ungültiger Prozess %d wurde bereinigt", processID)
		}
	}
}

// StartProcessMonitor startet einen Monitor, der regelmäßig alle laufenden Prozesse überprüft
func (pm *ProcessManager) StartProcessMonitor() {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Alle 30 Sekunden prüfen
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pm.ValidateRunningProcesses()
			}
		}
	}()

	logrus.Info("Image Capture Prozess-Monitor gestartet")
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

// InitImageCaptureProcesses initialisiert alle laufenden Prozesse beim Start der Anwendung
func InitImageCaptureProcesses(db *sql.DB) {
	if db == nil {
		logrus.Errorf("Datenbankverbindung ist nil beim Initialisieren der Image Capture Prozesse")
		return
	}

	query := `
		SELECT 
			id, name, device_id, endpoint, object_id, method_id, method_args,
			check_node_id, image_node_id, ack_node_id,
			enable_upload, upload_url, upload_headers, timestamp_header_name,
			enable_cyclic, cyclic_interval, description,
			status, last_execution, last_image,
			last_upload_status, last_upload_error, upload_success_count, upload_failure_count,
			created_at, updated_at
		FROM image_capture_processes
		WHERE status = 'running'
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
		var lastExecutionStr, lastImageStr sql.NullString
		var uploadURLStr, descriptionStr, timestampHeaderNameStr sql.NullString

		err := rows.Scan(
			&process.ID, &process.Name, &process.DeviceID,
			&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
			&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
			&process.EnableUpload, &uploadURLStr, &uploadHeadersStr, &timestampHeaderNameStr,
			&process.EnableCyclic, &process.CyclicInterval, &descriptionStr,
			&process.Status, &lastExecutionStr, &lastImageStr,
			&process.LastUploadStatus, &process.LastUploadError, &process.UploadSuccessCount, &process.UploadFailureCount,
			&process.CreatedAt, &process.UpdatedAt,
		)
		if err != nil {
			logrus.Errorf("Fehler beim Scannen der Prozessdaten: %v", err)
			continue
		}

		// Parse process data using the common function
		process, err = parseProcessData(process, methodArgsStr, uploadHeadersStr, lastExecutionStr, lastImageStr, uploadURLStr, descriptionStr, timestampHeaderNameStr)
		if err != nil {
			logrus.Errorf("Fehler beim Parsen der Prozessdaten: %v", err)
			continue
		}

		// Device-Namen separat laden
		process.DeviceName = getDeviceNameByID(db, process.DeviceID)

		// Prozess starten
		if err := processManager.StartProcess(db, &process); err != nil {
			logrus.Errorf("Fehler beim Starten des Prozesses %d: %v", process.ID, err)
		} else {
			logrus.Infof("Prozess %s (ID: %d) erfolgreich wiederhergestellt", process.Name, process.ID)
		}
	}

	// Prozess-Monitor starten
	processManager.StartProcessMonitor()
}

// getImageCaptureProcesses holt alle Image Capture Prozesse aus der Datenbank
func getImageCaptureProcesses(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Datenbankverbindung fehlgeschlagen: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	query := `SELECT id, name, device_id, endpoint, object_id, method_id, method_args,
			check_node_id, image_node_id, ack_node_id,
			enable_upload, upload_url, upload_headers, timestamp_header_name,
			enable_cyclic, cyclic_interval, description,
			status, last_execution, last_image,
			last_upload_status, last_upload_error, upload_success_count, upload_failure_count,
			created_at, updated_at
		FROM image_capture_processes
	`

	rows, err := db.Query(query)
	if err != nil {
		logrus.Errorf("Fehler beim Datenbankquery: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Abrufen der Prozesse", "details": err.Error()})
		return
	}
	defer rows.Close()

	var processes []ImageCaptureProcess
	for rows.Next() {
		process, err := scanProcessFromRow(db, rows)
		if err != nil {
			logrus.Errorf("Fehler beim Scannen der Prozessdaten: %v", err)
			continue
		}

		// Aktuellen Status und Informationen aus dem ProcessManager holen
		updateProcessStatusFromManager(&process)

		processes = append(processes, process)
	}

	// logrus.Infof("API: %d Prozesse gefunden", len(processes))
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
			id, name, device_id, COALESCE(d.name, 'Unbekanntes Gerät') as device_name,
			endpoint, object_id, method_id, method_args,
			check_node_id, image_node_id, ack_node_id,
			enable_upload, upload_url, upload_headers, timestamp_header_name,
			enable_cyclic, cyclic_interval, description,
			status, last_execution, last_image,
			created_at, updated_at
		FROM image_capture_processes
		WHERE id = ?
	`

	row := db.QueryRow(query, id)
	process, err := scanProcessFromQueryRow(db, row)
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
	logrus.Infof("API: addImageCaptureProcess aufgerufen")

	var process ImageCaptureProcess
	if err := c.ShouldBindJSON(&process); err != nil {
		logrus.Errorf("Fehler beim Parsen der JSON-Daten: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Daten", "details": err.Error()})
		return
	}

	// Validierung
	if process.Name == "" {
		logrus.Errorf("Validierungsfehler: Name ist leer")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name ist erforderlich"})
		return
	}

	if process.DeviceID == 0 {
		logrus.Errorf("Validierungsfehler: DeviceID ist 0")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geräte-ID ist erforderlich"})
		return
	}

	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Datenbankverbindung fehlgeschlagen: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	// MethodArgs und UploadHeaders in JSON konvertieren
	methodArgsJSON, err := json.Marshal(process.MethodArgs)
	if err != nil {
		logrus.Errorf("Fehler beim Marshal der MethodArgs: %v", err)
		methodArgsJSON = []byte("{}")
	}

	uploadHeadersJSON, err := json.Marshal(process.UploadHeaders)
	if err != nil {
		logrus.Errorf("Fehler beim Marshal der UploadHeaders: %v", err)
		uploadHeadersJSON = []byte("{}")
	}

	now := time.Now()
	query := `
		INSERT INTO image_capture_processes (
			name, device_id, endpoint, object_id, method_id, method_args,
			check_node_id, image_node_id, ack_node_id, enable_upload, upload_url, upload_headers, timestamp_header_name,
			enable_cyclic, cyclic_interval, description, status, 
			last_upload_status, last_upload_error, upload_success_count, upload_failure_count,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	logrus.Infof("Führe INSERT-Query aus...")
	result, err := logic.SafeDBExec(db, query,
		process.Name, process.DeviceID, process.Endpoint, process.ObjectID, process.MethodID, string(methodArgsJSON),
		process.CheckNodeID, process.ImageNodeID, process.AckNodeID, process.EnableUpload, process.UploadURL, string(uploadHeadersJSON), process.TimestampHeaderName,
		process.EnableCyclic, process.CyclicInterval, process.Description, "stopped",
		"not_attempted", "", 0, 0,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		logrus.Errorf("Fehler beim INSERT: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Erstellen des Prozesses", "details": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	process.ID = int(id)
	process.Status = "stopped"
	process.CreatedAt = now.Format(time.RFC3339)
	process.UpdatedAt = now.Format(time.RFC3339)

	logrus.Infof("Prozess erfolgreich erstellt mit ID: %d", process.ID)
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
			check_node_id = ?, image_node_id = ?, ack_node_id = ?, enable_upload = ?, upload_url = ?, upload_headers = ?, timestamp_header_name = ?,
			enable_cyclic = ?, cyclic_interval = ?, description = ?, updated_at = ?
		WHERE id = ?
	`

	_, err = logic.SafeDBExec(db, query,
		process.Name, process.DeviceID, process.Endpoint, process.ObjectID, process.MethodID, string(methodArgsJSON),
		process.CheckNodeID, process.ImageNodeID, process.AckNodeID, process.EnableUpload, process.UploadURL, string(uploadHeadersJSON), process.TimestampHeaderName,
		process.EnableCyclic, process.CyclicInterval, process.Description, now.Format(time.RFC3339), id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Aktualisieren des Prozesses"})
		return
	}

	process.UpdatedAt = now.Format(time.RFC3339)
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
	_, err = logic.SafeDBExec(db, query, id)
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
			id, name, device_id,
			endpoint, object_id, method_id, method_args,
			check_node_id, image_node_id, ack_node_id,
			enable_upload, upload_url, upload_headers, timestamp_header_name,
			enable_cyclic, cyclic_interval, description,
			status, last_execution, last_image,
			created_at, updated_at
		FROM image_capture_processes
		WHERE id = ?
	`

	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr, lastImageStr sql.NullString
	var uploadURLStr, descriptionStr, timestampHeaderNameStr sql.NullString

	err = db.QueryRow(query, id).Scan(
		&process.ID, &process.Name, &process.DeviceID,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &uploadURLStr, &uploadHeadersStr, &process.TimestampHeaderName,
		&process.EnableCyclic, &process.CyclicInterval, &descriptionStr,
		&process.Status, &lastExecutionStr, &lastImageStr,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Prozess nicht gefunden"})
		return
	}

	// Parse process data using the common function
	process, err = parseProcessData(process, methodArgsStr, uploadHeadersStr, lastExecutionStr, lastImageStr, uploadURLStr, descriptionStr, timestampHeaderNameStr)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der Prozessdaten: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Parsen der Prozessdaten"})
		return
	}

	// Prozess starten
	if err := processManager.StartProcess(db, &process); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Starten des Prozesses", "details": err.Error()})
		return
	}

	// Status in der Datenbank auf "running" setzen
	now := time.Now()
	updateQuery := `UPDATE image_capture_processes SET status = ?, updated_at = ? WHERE id = ?`
	_, err = logic.SafeDBExec(db, updateQuery, "running", now, id)
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
	_, err = logic.SafeDBExec(db, updateQuery, "stopped", now, id)
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
			id, name, device_id,
			endpoint, object_id, method_id, method_args,
			check_node_id, image_node_id, ack_node_id,
			enable_upload, upload_url, upload_headers, timestamp_header_name,
			enable_cyclic, cyclic_interval, description,
			status, last_execution, last_image,
			created_at, updated_at
		FROM image_capture_processes
		WHERE id = ?
	`

	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr, lastImageStr sql.NullString
	var uploadURLStr, descriptionStr, timestampHeaderNameStr sql.NullString

	err = db.QueryRow(query, id).Scan(
		&process.ID, &process.Name, &process.DeviceID,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &uploadURLStr, &uploadHeadersStr, &timestampHeaderNameStr,
		&process.EnableCyclic, &process.CyclicInterval, &descriptionStr,
		&process.Status, &lastExecutionStr, &lastImageStr,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Prozess nicht gefunden"})
		return
	}

	// Parse process data using the common function
	process, err = parseProcessData(process, methodArgsStr, uploadHeadersStr, lastExecutionStr, lastImageStr, uploadURLStr, descriptionStr, timestampHeaderNameStr)
	if err != nil {
		logrus.Errorf("Fehler beim Parsen der Prozessdaten: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Parsen der Prozessdaten"})
		return
	}

	// Kanal für das Bild erstellen
	imageChan := make(chan *ImageCaptureResult)

	// Einmalige Ausführung in einer Go-Routine
	go func() {
		result, err := executeSingleImageCapture(db, &process, true) // true = manuelle Ausführung
		if err != nil {
			imageChan <- &ImageCaptureResult{Error: err}
		} else {
			imageChan <- result
		}
	}()

	// Auf das Ergebnis warten
	select {
	case result := <-imageChan:
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{})

	case <-time.After(20 * time.Second):
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Zeitüberschreitung beim Image Capture"})
	}
}

// ImageCaptureResult enthält das Ergebnis einer Image Capture Ausführung
type ImageCaptureResult struct {
	Image string
	Error error
}

// NodeRedImageCaptureResponse repräsentiert die Antwort der Node-RED API
type NodeRedImageCaptureResponse struct {
	Success        bool   `json:"success"`
	Endpoint       string `json:"endpoint"`
	SecurityMode   string `json:"securityMode"`
	SecurityPolicy string `json:"securityPolicy"`
	Username       string `json:"username"`
	Image          string `json:"image"`
	DeviceID       string `json:"device_id"`
	Uploaded       bool   `json:"uploaded"`
	ProcessID      int    `json:"process_id"` // Geändert von string zu int
}

func executeSingleImageCapture(db *sql.DB, process *ImageCaptureProcess, isManualExecution bool) (*ImageCaptureResult, error) {
	// Sicherheitsprüfung: Nur bei automatischen (zyklischen) Ausführungen prüfen
	if !isManualExecution && !processManager.IsProcessReallyRunning(process.ID) {
		return nil, fmt.Errorf("prozess %d ist nicht mehr aktiv - image capture abgebrochen", process.ID)
	}

	// Geräteinformationen aus der Datenbank holen
	if db == nil {
		return nil, fmt.Errorf("datenbankverbindung ist nil")
	}

	deviceQuery := `SELECT security_mode, security_policy, username, password FROM devices WHERE id = ?`
	var securityMode, securityPolicy, username, password sql.NullString
	err := db.QueryRow(deviceQuery, process.DeviceID).Scan(&securityMode, &securityPolicy, &username, &password)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Laden der Geräteinformationen: %v", err)
	}

	// MethodArgs konvertieren
	methodArgsJSON := json.RawMessage("{}")
	if process.MethodArgs != nil {
		if jsonBytes, err := json.Marshal(process.MethodArgs); err == nil {
			methodArgsJSON = json.RawMessage(jsonBytes)
		}
	}

	// Node-RED API Request Payload erstellen
	payload := map[string]interface{}{
		"OPC_ENDPOINT":          process.Endpoint,
		"M_NODE_PARENT":         process.ObjectID,
		"M_NODE_IMAGE":          process.MethodID,
		"M_ARGS":                string(methodArgsJSON),
		"NODE_CHECK":            process.CheckNodeID,
		"NODE_READ_AVAILABLE":   process.AckNodeID,
		"NODE_IMAGE":            process.ImageNodeID,
		"NODE_ACK_READ":         process.AckNodeID,
		"DEVICE_ID":             strconv.Itoa(process.DeviceID),
		"ENABLE_UPLOAD":         fmt.Sprintf("%t", process.EnableUpload),
		"UPLOAD_URL":            process.UploadURL,
		"TIMESTAMP_HEADER_NAME": process.TimestampHeaderName,
		"OPC_SEC_MODE":          securityMode.String,
		"OPC_SEC_POLICY":        securityPolicy.String,
		"OPC_USER":              username.String,
		"OPC_PW":                password.String,
		"PROCESS_ID":            process.ID,
	}

	// Upload Headers hinzufügen wenn vorhanden
	if process.UploadHeaders != nil {
		headersJSON, _ := json.Marshal(process.UploadHeaders)
		payload["HEADERS"] = string(headersJSON)
	}

	// HTTP Request an Node-RED API senden
	err = callNodeRedImageCaptureAPI(payload)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Aufruf der Node-RED API: %v", err)
	}

	if err != nil {
		return nil, fmt.Errorf("node-RED Image Capture fehlgeschlagen: %v", err)
	}

	return &ImageCaptureResult{
		Error: nil,
	}, nil
}

// callNodeRedImageCaptureAPI führt einen HTTP POST Request zur Node-RED Image Capture API aus
func callNodeRedImageCaptureAPI(payload map[string]interface{}) error {
	// Node-RED läuft standardmäßig auf Port 1880
	nodeRedURL := "http://node-red:1880/api/img-capture"

	// JSON Payload erstellen
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("fehler beim Erstellen der JSON-Payload: %v", err)
	}

	// HTTP Request erstellen
	req, err := http.NewRequest("POST", nodeRedURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("fehler beim Erstellen des HTTP-Requests: %v", err)
	}

	// Headers setzen
	req.Header.Set("Content-Type", "application/json")

	// HTTP Client mit Timeout erstellen
	client := &http.Client{
		Timeout: 30 * time.Second, // 30 Sekunden Timeout für Image Capture
	}

	// Request ausführen
	// logrus.Infof("Sende Image Capture Request an Node-RED: %s", nodeRedURL)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Fehler beim Ausführen des HTTP-Requests: %v", err)
	}
	defer resp.Body.Close()

	return nil
}

// StartProcess startet einen Image Capture Prozess
func (pm *ProcessManager) StartProcess(db *sql.DB, process *ImageCaptureProcess) error {
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
	go pm.runProcess(db, runningProcess)

	return nil
}

// StopProcess stoppt einen Image Capture Prozess
func (pm *ProcessManager) StopProcess(processID int) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if runningProcess, exists := pm.processes[processID]; exists {
		runningProcess.IsRunning = false

		// Channel sicher schließen (nur wenn noch offen)
		select {
		case <-runningProcess.StopChan:
			// Channel bereits geschlossen
		default:
			close(runningProcess.StopChan)
		}

		delete(pm.processes, processID)
		logrus.Infof("Image Capture Prozess %d erfolgreich gestoppt", processID)
	} else {
		logrus.Warnf("Image Capture Prozess %d war nicht im ProcessManager registriert", processID)
	}
}

// runProcess führt den Image Capture Prozess aus
func (pm *ProcessManager) runProcess(db *sql.DB, runningProcess *RunningProcess) {
	process := runningProcess.Process
	processID := process.ID

	logrus.Infof("Image Capture Prozess %d gestartet (Intervall: %d Sekunden)", processID, process.CyclicInterval)

	// Intervall für zyklische Ausführung bestimmen
	interval := time.Duration(process.CyclicInterval) * time.Second
	if interval < time.Second {
		interval = 30 * time.Second // Standard: 30 Sekunden
	}

	ticker := time.NewTicker(interval)
	defer func() {
		ticker.Stop()
		logrus.Infof("Image Capture Prozess %d Goroutine beendet", processID)
	}()

	for {
		// Zuerst prüfen, ob der Prozess noch laufen soll
		pm.mutex.RLock()
		if !runningProcess.IsRunning {
			pm.mutex.RUnlock()
			logrus.Infof("Image Capture Prozess %d ist nicht mehr running, stoppe", processID)
			return
		}
		pm.mutex.RUnlock()

		select {
		case <-runningProcess.StopChan:
			logrus.Infof("Image Capture Prozess %d erhielt Stop-Signal", processID)
			return

		case <-ticker.C:
			// Doppelt prüfen ob der Prozess noch laufen soll
			pm.mutex.RLock()
			if !runningProcess.IsRunning {
				pm.mutex.RUnlock()
				logrus.Infof("Image Capture Prozess %d ist nicht mehr running, stoppe", processID)
				return
			}
			pm.mutex.RUnlock()

			logrus.Infof("Image Capture Prozess %d führt Capture aus", processID)

			// Image Capture mit der gemeinsamen Funktion ausführen
			result, err := executeSingleImageCapture(db, process, false) // false = automatische Ausführung
			if err != nil {
				runningProcess.LastError = err.Error()
				logrus.Errorf("Fehler beim Image Capture für Prozess %d: %v", processID, err)
				continue
			}

			// Erfolgreiche Ausführung
			runningProcess.LastError = ""
			runningProcess.LastImage = result.Image
			logrus.Infof("Image Capture Prozess %d erfolgreich ausgeführt", processID)
		}
	}
}

// StopAllImageCaptureProcesses stoppt alle laufenden Image Capture Prozesse
func StopAllImageCaptureProcesses(db *sql.DB) {
	processManager.mutex.Lock()
	defer processManager.mutex.Unlock()

	logrus.Infof("Stoppe alle %d laufenden Image Capture Prozesse", len(processManager.processes))

	for processID, runningProcess := range processManager.processes {
		runningProcess.IsRunning = false

		// Channel sicher schließen
		select {
		case <-runningProcess.StopChan:
			// Channel bereits geschlossen
		default:
			close(runningProcess.StopChan)
		}

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
	_, err := logic.SafeDBExec(db, updateQuery, "stopped", now)
	if err != nil {
		logrus.Errorf("Fehler beim Aktualisieren der Prozessstatus: %v", err)
	} else {
		logrus.Info("Alle Image Capture Prozesse gestoppt und Status aktualisiert")
	}
}

// LogProcessStatus loggt den aktuellen Status aller laufenden Prozesse
func (pm *ProcessManager) LogProcessStatus() {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	runningCount := 0
	stoppedCount := 0

	for processID, runningProcess := range pm.processes {
		if runningProcess.IsRunning {
			runningCount++
			logrus.Infof("Prozess %d läuft (Start: %s, Letztes Bild: %s)",
				processID,
				runningProcess.StartTime.Format("15:04:05"),
				runningProcess.LastImage)
		} else {
			stoppedCount++
			logrus.Infof("Prozess %d ist gestoppt (Letzter Fehler: %s)",
				processID,
				runningProcess.LastError)
		}
	}

	// logrus.Infof("Image Capture Prozess-Status: %d laufend, %d gestoppt", runningCount, stoppedCount)
}

// GracefulShutdown stoppt alle laufenden Prozesse sauber beim Herunterfahren der Anwendung
func GracefulShutdown(db *sql.DB) {
	logrus.Info("Starte sauberes Herunterfahren der Image Capture Prozesse...")

	// Alle laufenden Prozesse stoppen
	StopAllImageCaptureProcesses(db)

	// Kurz warten, damit alle Goroutines sauber beendet werden können
	time.Sleep(2 * time.Second)

	logrus.Info("Image Capture Prozesse erfolgreich heruntergefahren")
}

// scanProcessFromRow scannt einen Prozess aus einer Datenbankzeile
func scanProcessFromRow(db *sql.DB, rows *sql.Rows) (ImageCaptureProcess, error) {
	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr, lastImageStr sql.NullString
	var uploadURLStr, descriptionStr, timestampHeaderNameStr sql.NullString

	err := rows.Scan(
		&process.ID, &process.Name, &process.DeviceID,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &uploadURLStr, &uploadHeadersStr, &timestampHeaderNameStr,
		&process.EnableCyclic, &process.CyclicInterval, &descriptionStr,
		&process.Status, &lastExecutionStr, &lastImageStr,
		&process.LastUploadStatus, &process.LastUploadError, &process.UploadSuccessCount, &process.UploadFailureCount,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		return process, err
	}

	// Device-Namen separat laden
	process.DeviceName = getDeviceNameByID(db, process.DeviceID)

	return parseProcessData(process, methodArgsStr, uploadHeadersStr, lastExecutionStr, lastImageStr, uploadURLStr, descriptionStr, timestampHeaderNameStr)
}

// scanProcessFromQueryRow scannt einen Prozess aus einer QueryRow
func scanProcessFromQueryRow(db *sql.DB, row *sql.Row) (ImageCaptureProcess, error) {
	var process ImageCaptureProcess
	var methodArgsStr, uploadHeadersStr sql.NullString
	var lastExecutionStr, lastImageStr sql.NullString
	var uploadURLStr, descriptionStr, timestampHeaderNameStr sql.NullString

	err := row.Scan(
		&process.ID, &process.Name, &process.DeviceID,
		&process.Endpoint, &process.ObjectID, &process.MethodID, &methodArgsStr,
		&process.CheckNodeID, &process.ImageNodeID, &process.AckNodeID,
		&process.EnableUpload, &uploadURLStr, &uploadHeadersStr, &timestampHeaderNameStr,
		&process.EnableCyclic, &process.CyclicInterval, &descriptionStr,
		&process.Status, &lastExecutionStr, &lastImageStr,
		&process.LastUploadStatus, &process.LastUploadError, &process.UploadSuccessCount, &process.UploadFailureCount,
		&process.CreatedAt, &process.UpdatedAt,
	)
	if err != nil {
		return process, err
	}

	// Device-Namen separat laden
	process.DeviceName = getDeviceNameByID(db, process.DeviceID)

	return parseProcessData(process, methodArgsStr, uploadHeadersStr, lastExecutionStr, lastImageStr, uploadURLStr, descriptionStr, timestampHeaderNameStr)
}

// getDeviceNameByID holt den Device-Namen anhand der ID
func getDeviceNameByID(db *sql.DB, deviceID int) string {
	if db == nil {
		return "Unbekanntes Gerät"
	}

	var deviceName sql.NullString
	query := `SELECT name FROM devices WHERE id = ?`
	err := db.QueryRow(query, deviceID).Scan(&deviceName)
	if err != nil {
		logrus.Errorf("Fehler beim Laden des Device-Namens für ID %d: %v", deviceID, err)
		return "Unbekanntes Gerät"
	}

	if deviceName.Valid {
		return deviceName.String
	}
	return "Unbekanntes Gerät"
}

// parseProcessData parst die JSON-Felder und LastExecution eines Prozesses
func parseProcessData(process ImageCaptureProcess, methodArgsStr, uploadHeadersStr sql.NullString, lastExecutionStr sql.NullString, lastImageStr sql.NullString, uploadURLStr sql.NullString, descriptionStr sql.NullString, timestampHeaderNameStr sql.NullString) (ImageCaptureProcess, error) {
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

	// LastImage verarbeiten
	if lastImageStr.Valid {
		process.LastImage = lastImageStr.String
	} else {
		process.LastImage = ""
	}

	// UploadURL verarbeiten
	if uploadURLStr.Valid {
		process.UploadURL = uploadURLStr.String
	} else {
		process.UploadURL = ""
	}

	// Description verarbeiten
	if descriptionStr.Valid {
		process.Description = descriptionStr.String
	} else {
		process.Description = ""
	}

	// TimestampHeaderName verarbeiten
	if timestampHeaderNameStr.Valid {
		process.TimestampHeaderName = timestampHeaderNameStr.String
	} else {
		process.TimestampHeaderName = ""
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

// updateProcessExecutionInfo ist nicht mehr notwendig - wird in executeSingleImageCapture behandelt
