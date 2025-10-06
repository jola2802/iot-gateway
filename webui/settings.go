package webui

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"iot-gateway/logic"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/sirupsen/logrus"
)

// showSettingsPage shows the settings page, returning broker settings and user data.
func showSettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", gin.H{})
}

// showLogs shows the logs page.
func grabLogs(c *gin.Context) {
	// Cache-Header setzen, um sicherzustellen, dass wir immer die aktuellsten Daten bekommen
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, post-check=0, pre-check=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// Logs aus dem Speicher holen
	logs := logic.GetLogs()

	// Wenn keine Logs vorhanden sind, gib eine leere Antwort zurück
	if len(logs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"logs": "",
			"info": map[string]interface{}{
				"message":   "No Logs available",
				"timestamp": time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Verbinde die Logs zu einem String
	logsString := strings.Join(logs, "")

	// Sende die Logs zurück
	c.JSON(http.StatusOK, gin.H{
		"logs": logsString,
		"info": map[string]interface{}{
			"count":     len(logs),
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// clearLogs löscht alle Logs
func clearLogs(c *gin.Context) {
	// Logs löschen
	logic.ClearLogs()

	// Erfolg melden
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Logs were deleted",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// SystemSetting Struktur für Einstellungen
type SystemSetting struct {
	ID          int    `json:"id"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsEncrypted bool   `json:"isEncrypted"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// getSystemSettings holt alle Systemeinstellungen
func getSystemSettings(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Error getting database connection: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	rows, err := db.Query(`
		SELECT id, setting_key, setting_value, setting_type, description, category, is_encrypted, created_at, updated_at
		FROM system_settings
		ORDER BY category, setting_key
	`)
	if err != nil {
		logrus.Errorf("Error querying system settings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query settings"})
		return
	}
	// defer rows.Close()

	var settings []SystemSetting
	for rows.Next() {
		var setting SystemSetting
		err := rows.Scan(
			&setting.ID,
			&setting.Key,
			&setting.Value,
			&setting.Type,
			&setting.Description,
			&setting.Category,
			&setting.IsEncrypted,
			&setting.CreatedAt,
			&setting.UpdatedAt,
		)
		if err != nil {
			logrus.Errorf("Error scanning setting: %v", err)
			continue
		}
		settings = append(settings, setting)
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// updateSystemSetting aktualisiert eine Systemeinstellung
func updateSystemSetting(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Error getting database connection: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var request struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		logrus.Errorf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Validierung basierend auf dem Setting-Typ
	if err := validateSettingValue(db, request.Key, request.Value); err != nil {
		logrus.Errorf("Validation error for setting %s: %v", request.Key, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// Update der Einstellung
	result, err := db.Exec(`
		UPDATE system_settings 
		SET setting_value = ?, updated_at = ?
		WHERE setting_key = ?
	`, request.Value, now, request.Key)

	if err != nil {
		logrus.Errorf("Error updating setting %s: %v", request.Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
		return
	}

	// Prüfe, ob eine Zeile aktualisiert wurde
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logrus.Errorf("Error getting rows affected for setting %s: %v", request.Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
		return
	}

	if rowsAffected == 0 {
		logrus.Errorf("Setting %s not found in database", request.Key)
		c.JSON(http.StatusNotFound, gin.H{"error": "Setting not found"})
		return
	}

	logrus.Infof("Setting \"%s\" updated to: \"%s\"", request.Key, request.Value)
	c.JSON(http.StatusOK, gin.H{"message": "Setting updated successfully"})
}

// validateSettingValue validiert den Wert einer Einstellung
func validateSettingValue(db *sql.DB, key, value string) error {
	// Hole den Setting-Typ aus der Datenbank
	var settingType string
	err := db.QueryRow("SELECT setting_type FROM system_settings WHERE setting_key = ?", key).Scan(&settingType)
	if err != nil {
		return fmt.Errorf("unknown setting key: %s", key)
	}

	switch settingType {
	case "integer":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("value must be an integer")
		}
	case "boolean":
		if value != "true" && value != "false" {
			return fmt.Errorf("value must be 'true' or 'false'")
		}
	case "string":
		// String-Validierungen für spezielle Keys
		switch key {
		case "node_red_url", "influxdb_url":
			if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
				return fmt.Errorf("URL must start with http:// or https://")
			}
		case "log_level":
			validLevels := []string{"debug", "info", "warn", "error"}
			if !contains(validLevels, value) {
				return fmt.Errorf("log level must be one of: %s", strings.Join(validLevels, ", "))
			}
		}
	}

	return nil
}

// contains prüft, ob ein String in einem Slice enthalten ist
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getSystemSetting holt eine spezifische Systemeinstellung
func getSystemSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Setting key is required"})
		return
	}

	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Error getting database connection: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var setting SystemSetting
	err = db.QueryRow(`
		SELECT id, setting_key, setting_value, setting_type, description, category, is_encrypted, created_at, updated_at
		FROM system_settings
		WHERE setting_key = ?
	`, key).Scan(
		&setting.ID,
		&setting.Key,
		&setting.Value,
		&setting.Type,
		&setting.Description,
		&setting.Category,
		&setting.IsEncrypted,
		&setting.CreatedAt,
		&setting.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Setting not found"})
		} else {
			logrus.Errorf("Error querying setting %s: %v", key, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query setting"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"setting": setting})
}

// resetSystemSettings setzt alle Einstellungen auf Standardwerte zurück
func resetSystemSettings(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Errorf("Error getting database connection: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Lösche alle bestehenden Einstellungen
	_, err = db.Exec("DELETE FROM system_settings")
	if err != nil {
		logrus.Errorf("Error deleting settings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset settings"})
		return
	}

	// Füge Standard-Einstellungen wieder hinzu
	now := time.Now().Format("2006-01-02 15:04:05")

	defaultSettings := []struct {
		key, value, settingType, description, category string
	}{
		{"node_red_url", "http://node-red:1880", "string", "Node-RED Web-Interface URL", "integration"},
		{"influxdb_url", "http://influxdb:8086", "string", "InfluxDB Server URL", "integration"},
	}

	for _, setting := range defaultSettings {
		_, err = db.Exec(`
			INSERT INTO system_settings (setting_key, setting_value, setting_type, description, category, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, setting.key, setting.value, setting.settingType, setting.description, setting.category, now, now)
		if err != nil {
			logrus.Errorf("Error inserting default setting %s: %v", setting.key, err)
		}
	}

	logrus.Info("System settings reset to defaults")
	c.JSON(http.StatusOK, gin.H{"message": "Settings reset to defaults successfully"})
}

// GetSystemSetting holt eine spezifische Einstellung aus der Datenbank
func GetSystemSetting(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT setting_value FROM system_settings WHERE setting_key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("setting %s not found", key)
		}
		return "", err
	}
	return value, nil
}
