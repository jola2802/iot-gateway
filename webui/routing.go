package webui

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// showRoutingPage shows the routing page
func showRoutingPage(c *gin.Context) {
	c.HTML(http.StatusOK, "data-forwarding.html", nil)
}

func getlistDevices(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
	}

	var devices []string
	rows, err := db.Query("SELECT name, id FROM devices")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var device string
		var id int
		err := rows.Scan(&device, &id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		deviceInfo := fmt.Sprintf("%d - %s", id, device)
		devices = append(devices, deviceInfo)
	}
	c.JSON(200, gin.H{"devices": devices})
}
