package webui

import (
	"net/http"
	"os"
	"strings"
	"time"

	"iot-gateway/logic"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
)

// Die aktuelle IP-Adresse des Gateways
var IPAddress = "" // Standard-Wert

// Wird beim Start ausgeführt
func init() {
	loadIPAddress()
}

// Lädt die IP-Adresse aus der Umgebungsvariable
func loadIPAddress() {
	NR_URL := os.Getenv("NODE_RED_URL")
	if NR_URL != "" {
		// Extract IP address from URL e.g. https://192.168.0.84/nodered
		IPAddress = strings.Split(NR_URL, "/")[2]
		NodeRED_URL = NR_URL

		return
	}
}

// showSettingsPage shows the settings page, returning broker settings and user data.
func showSettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"IPAddress": IPAddress,
		"NR_URL":    NodeRED_URL,
	})
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
