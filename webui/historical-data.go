package webui

import (
	"fmt"
	"net/http"
	"time"

	dataforwarding "iot-gateway/data-forwarding"

	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/sirupsen/logrus"
)

type DataPoint struct {
	X time.Time `json:"x"`
	Y float64   `json:"y"`
}

var influxConfig *dataforwarding.InfluxConfig

func showHistoricalDataPage(c *gin.Context) {
	c.HTML(http.StatusOK, "historical-data.html", nil)
}

func queryDataHandler(c *gin.Context) {
	// Verbindung zur SQLite-Datenbank herstellen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to SQLite database"})
		return
	}

	if influxConfig == nil {
		influxConfig, err = dataforwarding.GetInfluxConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch InfluxDB configuration"})
			return
		}
	}

	// Anfrage-Parameter auslesen
	var requestData struct {
		Start       string `json:"start" binding:"required"`
		Duration    string `json:"duration" binding:"required"`
		Measurement string `json:"measurement" binding:"required"`
	}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	logrus.Infof("Request data: %+v", requestData)

	// Verbindung zur InfluxDB herstellen
	logrus.Infof("Verbindung zu InfluxDB unter %s wird hergestellt", influxConfig.URL)
	client := influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	defer client.Close()

	queryAPI := client.QueryAPI(influxConfig.Org)

	location, err := time.LoadLocation("Europe/Berlin")
	logrus.Info(location)

	if err != nil {
		logrus.Errorf("Failed to load location: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load location"})
		return
	}

	// Startzeit mit dem erwarteten Format (ISO-8601 mit T) parsen
	const inputFormat = "2006-01-02T15:04" // Format des Eingabewerts
	startTime, err := time.ParseInLocation(inputFormat, requestData.Start, location)
	if err != nil {
		logrus.Errorf("Failed to parse start time: %s. Ensure it is in the correct format.", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start time format. Expected format (e.g., 2025-01-26T15:50)."})
		return
	}

	duration, err := time.ParseDuration(requestData.Duration + "m")
	if err != nil {
		logrus.Errorf("Failed to parse duration: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid duration format"})
		return
	}

	// Korrekte Berechnung der stopTime: startTime + duration
	stopTime := startTime.Add(duration)

	// Flux-Query erstellen
	query := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["_measurement"] == "%s")
		|> filter(fn: (r) => r["_field"] == "value")
	`, influxConfig.Bucket, startTime.UTC().Format(time.RFC3339), stopTime.UTC().Format(time.RFC3339), requestData.Measurement)

	// Query ausführen
	result, err := queryAPI.Query(c, query)
	if err != nil {
		logrus.Errorf("Failed to execute query: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query data from InfluxDB"})
		return
	}

	// Ergebnisse verarbeiten
	var data []map[string]interface{}
	for result.Next() {
		record := result.Record()
		data = append(data, map[string]interface{}{
			"x": record.Time(),
			"y": record.Value(),
		})
	}

	// Fehler während der Verarbeitung prüfen
	if result.Err() != nil {
		logrus.Errorf("Error processing query results: %s", result.Err().Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Err().Error()})
		return
	}

	// Daten als JSON zurückgeben
	c.JSON(http.StatusOK, data)
}

// Funktion, die Measurements aus der InfluxDB abruft
func getMeasurements(c *gin.Context) {
	var request struct {
		DeviceID string `json:"deviceId"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Verbindung zur SQLite-Datenbank, um InfluxDB-Details zu holen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to SQLite database"})
		return
	}

	if influxConfig == nil {
		influxConfig, err = dataforwarding.GetInfluxConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch InfluxDB configuration"})
			return
		}
	}

	// Verbindung zur InfluxDB
	client := influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	defer client.Close()

	queryAPI := client.QueryAPI(influxConfig.Org)

	// Flux-Query: Hole alle Measurements für die angegebene deviceId
	query := fmt.Sprintf(`
	from(bucket: "%s")
		|> range(start: -30d)  // Zeitfenster der letzten 30 Tage
		|> filter(fn: (r) => r["deviceId"] == "%s")
		|> group(columns: ["_measurement"])
		|> distinct(column: "_measurement")
		|> keep(columns: ["_measurement"])
	`, influxConfig.Bucket, request.DeviceID)

	// Query ausführen
	result, err := queryAPI.Query(c, query)
	if err != nil {
		logrus.Errorf("Error while querying measurements: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query measurements from InfluxDB"})
		return
	}

	// Measurements sammeln
	var measurements []string

	// Gefilterte Measurements sammeln
	for result.Next() {
		measurement := result.Record().ValueByKey("_measurement").(string)
		measurements = append(measurements, measurement)
	}

	// Fehler beim Querying prüfen
	if result.Err() != nil {
		logrus.Errorf("Error processing filtered query results: %s", result.Err().Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Err().Error()})
		return
	}

	// Measurements zurückgeben
	c.JSON(http.StatusOK, gin.H{"measurements": measurements})
}
