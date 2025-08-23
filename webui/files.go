package webui

import (
	"archive/zip"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ImageRequest repräsentiert den JSON-Payload, der vom Client gesendet wird.
type ImageRequest struct {
	Image     string `json:"image"`
	Device    string `json:"device"`
	ProcessID string `json:"process_id"`
}

func saveImage(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var req ImageRequest

	// Parse den JSON-Requestbody in das Struct
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	deviceID, err := strconv.Atoi(req.Device)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID: " + err.Error()})
		return
	}
	saveImageToDB(db, req.Image, time.Now(), deviceID, req.ProcessID)

	// Sende eine Erfolgsmeldung zurück.
	c.JSON(http.StatusOK, gin.H{"message": "Image saved"})
}

func saveImageToDB(db *sql.DB, image string, timestamp time.Time, deviceID int, processID string) {
	// Speichere das Bild (als Blob) und den Timestamp in der Datenbank.
	query := "INSERT INTO images (device, image, timestamp, process_id) VALUES (?, ?, ?, ?)"
	if _, err := db.Exec(query, deviceID, image, timestamp, processID); err != nil {
		return
	}

	// max 1000 Bilder in der Datenbank speichern
	rows, err := db.Query("SELECT COUNT(*) FROM images")
	if err != nil {
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			return
		}
	}

	// Lösche das älteste Bild, wenn die Anzahl der Bilder NUM_IMAGES_DB überschreitet
	maxImages, err := strconv.Atoi(os.Getenv("NUM_IMAGES_DB"))
	if err != nil {
		return
	}
	if count >= maxImages {
		db.Exec("DELETE FROM images WHERE id = (SELECT id FROM images ORDER BY timestamp ASC LIMIT 1)")
	}
}

type Image struct {
	ID         int    `json:"id"`
	Device     string `json:"device"`
	DeviceName string `json:"device_name"`
	ProcessID  string `json:"process_id"`
	Image      string `json:"image"`
	Timestamp  string `json:"timestamp"`
}

func getImages(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	rows, err := db.Query("SELECT * FROM images")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query images"})
		return
	}

	defer rows.Close()

	images := []Image{}

	for rows.Next() {
		var img Image
		if err := rows.Scan(&img.ID, &img.Device, &img.ProcessID, &img.Image, &img.Timestamp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan image"})
			return
		}
		img.DeviceName = getDeviceName(img.Device, db)
		images = append(images, img)
	}

	// Delete all images older than 3 months and 2 minutes
	_, err = db.Exec("DELETE FROM images WHERE timestamp < ?", time.Now().AddDate(0, -3, 0).Add(2*time.Minute))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete images"})
		return
	}

	c.JSON(http.StatusOK, images)
}

// downloadImagesAsZip erstellt eine ZIP-Datei mit allen Bildern aus der Datenbank
func downloadImagesAsZip(c *gin.Context) {
	// Verbindung zur Datenbank herstellen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankverbindung fehlgeschlagen"})
		return
	}

	// Bilder aus der Datenbank abrufen
	rows, err := db.Query("SELECT * FROM images")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Abfragen der Bilder"})
		return
	}
	defer rows.Close()

	// Temporäre ZIP-Datei erstellen
	tempFile, err := os.CreateTemp("", "images-*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Erstellen der temporären Datei"})
		return
	}
	defer os.Remove(tempFile.Name()) // Temporäre Datei nach dem Senden löschen
	defer tempFile.Close()

	// Neuen ZIP-Writer erstellen
	zipWriter := zip.NewWriter(tempFile)
	defer zipWriter.Close()

	// Bilder durchlaufen und zur ZIP-Datei hinzufügen
	var imageCount int = 0
	for rows.Next() {
		var img Image
		if err := rows.Scan(&img.ID, &img.Device, &img.Image, &img.Timestamp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Lesen des Bildes"})
			return
		}

		// Base64-String dekodieren
		// Das Format ist normalerweise "data:image/png;base64,ACTUAL_DATA"
		base64Data := img.Image
		if strings.HasPrefix(base64Data, "data:") {
			// Extrahiere den tatsächlichen Base64-Teil nach dem Komma
			parts := strings.Split(base64Data, ",")
			if len(parts) > 1 {
				base64Data = parts[1]
			}
		}

		imageData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			fmt.Printf("Fehler beim Dekodieren des Bildes %d: %v\n", img.ID, err)
			continue // Überspringe fehlerhafte Bilder
		}

		// Zeitstempel parsen für den Dateinamen
		timestamp, err := time.Parse(time.RFC3339, img.Timestamp)
		if err != nil {
			timestamp = time.Now() // Fallback auf aktuelle Zeit
		}

		// Eindeutigen Dateinamen erstellen
		fileName := fmt.Sprintf("%s_%s_%d.png",
			img.Device,
			timestamp.Format("2006-01-02_15-04-05"),
			img.ID)

		// Datei zur ZIP hinzufügen
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			continue // Überspringe bei Fehler
		}

		_, err = fileWriter.Write(imageData)
		if err != nil {
			continue // Überspringe bei Fehler
		}

		imageCount++
	}

	// ZIP-Writer schließen, um sicherzustellen, dass alle Daten geschrieben wurden
	zipWriter.Close()

	// Wenn keine Bilder hinzugefügt wurden
	if imageCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Keine Bilder zum Herunterladen gefunden"})
		return
	}

	// Datei zum Anfang zurücksetzen
	tempFile.Seek(0, 0)

	// Aktuelles Datum für den Dateinamen
	currentDate := time.Now().Format("2006-01-02")

	// ZIP-Datei an den Client senden
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=images_%s.zip", currentDate))
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	c.File(tempFile.Name())
}
