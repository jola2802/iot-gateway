package webui

import (
	"database/sql"
	"encoding/json"
	dataforwarding "iot-gateway/data-forwarding"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type Route struct {
	ID              int      `json:"id"`
	DestinationType string   `json:"destinationType"`
	DataFormat      string   `json:"dataFormat"`
	Interval        int      `json:"interval"`
	Headers         []string `json:"headers"`
	DestinationURL  string   `json:"destination_url"`
	Devices         []string `json:"devices"` // Geräte hinzufügen
	FilePath        string   `json:"filePath"`
	CreatedAt       string   `json:"createdAt"`
}

func getRoutes(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// SQL-Abfrage, um alle Routen aus der Tabelle 'data_routes' zu laden
	query := `
		SELECT id, destination_type, data_format, destination_url, file_path, interval, devices, destination_url, headers, file_path, created_at
		FROM data_routes
	`
	rows, err := db.Query(query)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var routes []Route

	// Speichere die Routen in einer Liste
	for rows.Next() {
		var route Route
		var devices, headers sql.NullString // devices und headers als JSON-Strings

		// Daten aus der Datenbank in die Struktur lesen
		err := rows.Scan(&route.ID, &route.DestinationType, &route.DataFormat, &route.DestinationURL, &route.FilePath, &route.Interval, &devices, &route.DestinationURL, &headers, &route.FilePath, &route.CreatedAt)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Verarbeite die devices (NULL oder JSON oder CSV)
		if devices.Valid {
			if err := json.Unmarshal([]byte(devices.String), &route.Devices); err != nil {
				// Falls kein JSON, versuche sie als kommaseparierten String zu verarbeiten
				route.Devices = strings.Split(devices.String, ",")
			}
		} else {
			route.Devices = []string{} // Leere Liste, wenn devices NULL ist
		}

		// Verarbeite die headers (NULL oder JSON oder CSV)
		if headers.Valid {
			if err := json.Unmarshal([]byte(headers.String), &route.Headers); err != nil {
				// Falls kein JSON, versuche sie als kommaseparierten String zu verarbeiten
				route.Headers = strings.Split(headers.String, ",")
			}
		} else {
			route.Headers = []string{} // Leere Liste, wenn headers NULL ist
		}

		// Füge die Route zur Liste hinzu
		routes = append(routes, route)
	}
	// logrus.Info(routes)
	// Gebe die Routen als JSON zurück
	c.JSON(200, routes)
}

func getRoutesById(c *gin.Context) {
	routeIdStr := c.Param("routeId")
	routeId, err := strconv.Atoi(routeIdStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid route ID"})
		return
	}
	// logrus.Info("got route id: ", routeId)
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// SQL-Abfrage, um alle Routen aus der Tabelle 'data_routes' zu laden
	query := `
		SELECT destination_type, data_format, destination_url, file_path, interval, devices, headers
		FROM data_routes 
		WHERE id = ?
	`
	var route Route
	route.ID = routeId
	var devices, headers sql.NullString // devices und headers als JSON-Strings

	err = db.QueryRow(query, routeId).Scan(&route.DestinationType, &route.DataFormat, &route.DestinationURL, &route.FilePath, &route.Interval, &devices, &headers)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Verarbeite die devices und headers wie in deiner getRoutes-Funktion
	if devices.Valid {
		if err := json.Unmarshal([]byte(devices.String), &route.Devices); err != nil {
			route.Devices = strings.Split(devices.String, ",")
		}
	} else {
		route.Devices = []string{}
	}

	if headers.Valid {
		if err := json.Unmarshal([]byte(headers.String), &route.Headers); err != nil {
			route.Headers = strings.Split(headers.String, ",")
		}
	} else {
		route.Headers = []string{}
	}

	// logrus.Info(route)

	c.JSON(200, route)
}

// Speichern einer neuen Data Route in der Datenbank
func SaveRouteConfig(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var newRoute dataforwarding.DataRoute

	// JSON-Daten aus der Anfrage binden
	if err := c.ShouldBindJSON(&newRoute); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Check if the route ID already exists
	var existingRouteID int
	err = db.QueryRow("SELECT id FROM data_routes WHERE id = ?", newRoute.ID).Scan(&existingRouteID)

	if err != nil && err != sql.ErrNoRows {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if existingRouteID > 0 {
		// Route exists, update it
		query := `UPDATE data_routes SET destination_type = ?, data_format = ?, interval = ?, devices = ?, destination_url = ?, headers = ?, file_path = ?
				  WHERE id = ?`
		_, err = db.Exec(query, newRoute.DestinationType, newRoute.DataFormat, newRoute.Interval, pq.Array(newRoute.Devices),
			newRoute.DestinationURL, pq.Array(newRoute.Headers), newRoute.FilePath, newRoute.ID)

		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "Data route updated successfully.", "route_id": newRoute.ID})
	} else {
		// Route does not exist, insert it
		query := `INSERT INTO data_routes (destination_type, data_format, interval, devices, destination_url, headers, file_path)
				  VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`
		var routeID int
		err = db.QueryRow(query, newRoute.DestinationType, newRoute.DataFormat, newRoute.Interval, pq.Array(newRoute.Devices),
			newRoute.DestinationURL, pq.Array(newRoute.Headers), newRoute.FilePath).Scan(&routeID)

		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "Data route added successfully.", "route_id": routeID})
	}
}

func deleteRoute(c *gin.Context) {
	routeId := c.Param("routeId")

	db, err := getDBConnection(c)

	// SQL-Abfrage für das Löschen der Route
	query := "DELETE FROM data_routes WHERE id = ?"
	result, err := db.Exec(query, routeId)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Prüfen, ob eine Zeile betroffen war
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting rows affected"})
		return
	}

	// Prüfen, ob eine Route wirklich gelöscht wurde
	if rowsAffected == 0 {
		c.JSON(404, gin.H{"error": "Route not found"})
		return
	}

	// Erfolgreiche Löschung melden
	c.JSON(200, gin.H{"message": "Route deleted successfully"})
}

func getlistDevices(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
	}

	var devices []string
	rows, err := db.Query("SELECT name FROM devices")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var device string
		err := rows.Scan(&device)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		devices = append(devices, device)
	}
	c.JSON(200, gin.H{"devices": devices})
	// logrus.Println(devices)
}
