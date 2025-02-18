package webui

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
)

// showProfilePage shows the profile page
func showProfilePage(c *gin.Context) {
	c.HTML(http.StatusOK, "profile.html", nil)
}

func getProfile(c *gin.Context) {
	// Get the db instance from the gin.Context
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	dbConn := db.(*sql.DB)
	var profileData struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Address  string `json:"address"`
		Company  string `json:"company"`
		Email    string `json:"email"`
	}

	err := dbConn.QueryRow("SELECT username, name, address, company, email FROM users LIMIT 1").Scan(&profileData.Username, &profileData.Name, &profileData.Address, &profileData.Company, &profileData.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting profile data"})
		return
	}

	c.JSON(http.StatusOK, profileData)
}

// updateProfile updates the user profile
func updateProfile(c *gin.Context) {
	// Datenbankverbindung aus dem Kontext holen
	dbConn, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Datenbank-Verbindung nicht gefunden"})
		return
	}

	// Profil-Daten-Datentyp ohne Passwort-Felder definieren
	var profileData struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Address  string `json:"address"`
		Company  string `json:"company"`
		Email    string `json:"email"`
	}

	// JSON-Daten aus der Anfrage binden
	if err := c.ShouldBindJSON(&profileData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ungültige Anfrage"})
		return
	}

	// UPDATE-Abfrage: Nur die angegebenen Felder aktualisieren, Passwort bleibt unverändert.
	_, err = dbConn.Exec(
		"UPDATE users SET username = ?, name = ?, address = ?, company = ?, email = ?",
		profileData.Username, profileData.Name, profileData.Address, profileData.Company, profileData.Email,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Fehler beim Aktualisieren des Profils"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profil erfolgreich aktualisiert!"})
}

func changePassword(c *gin.Context) {
	// Get the db instance from the gin.Context
	db, _ := getDBConnection(c)

	var passwordData struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	if err := c.ShouldBindJSON(&passwordData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	var storedPassword string
	err := db.QueryRow("SELECT password FROM users LIMIT 1").Scan(&storedPassword)
	if err != nil || storedPassword != passwordData.CurrentPassword {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Current password is incorrect"})
		return
	}

	_, err = db.Exec("UPDATE users SET password = ?", passwordData.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating password"})
		return
	}
	c.Redirect(http.StatusSeeOther, "/logout")
	c.Abort()

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully!"})
}
