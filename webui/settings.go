package webui

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
)

// showSettingsPage shows the settings page, returning broker settings and user data.
func showSettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", gin.H{})
}

// showLogs shows the logs page.
func grabLogs(c *gin.Context) {
	// grab logs from file
	logs, err := os.ReadFile("logs/gateway.log")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": string(logs)})
}
