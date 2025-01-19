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
	}

	err := dbConn.QueryRow("SELECT username, name, address, company FROM users LIMIT 1").Scan(&profileData.Username, &profileData.Name, &profileData.Address, &profileData.Company)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting profile data"})
		return
	}

	c.JSON(http.StatusOK, profileData)
}

// updateProfile updates the user profile
func updateProfile(c *gin.Context) {
	// Get the db instance from the gin.Context
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	dbConn := db.(*sql.DB)
	var profileData struct {
		Username        string `json:"username"`
		Name            string `json:"name"`
		Address         string `json:"address"`
		Company         string `json:"company"`
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	if err := c.ShouldBindJSON(&profileData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}
	var storedPassword string
	err := dbConn.QueryRow("SELECT password FROM users LIMIT 1").Scan(&storedPassword)
	if err != nil || storedPassword != profileData.CurrentPassword {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Current password is incorrect"})
		return
	}

	tx, err := dbConn.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error starting transaction"})
		return
	}

	_, err = tx.Exec("DELETE FROM users")
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting old profile"})
		return
	}

	if profileData.NewPassword != "" {
		_, err = tx.Exec("INSERT INTO users (username, name, address, company, password) VALUES (?, ?, ?, ?, ?)",
			profileData.Username, profileData.Name, profileData.Address, profileData.Company, profileData.NewPassword)
	} else {
		_, err = tx.Exec("INSERT INTO users (username, name, address, company, password) VALUES (?, ?, ?, ?, ?)",
			profileData.Username, profileData.Name, profileData.Address, profileData.Company, storedPassword)
	}

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting new profile"})
		return
	}

	err = tx.Commit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error committing transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully!"})
}

func changePassword(c *gin.Context) {
	// Get the db instance from the gin.Context
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	var passwordData struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	if err := c.ShouldBindJSON(&passwordData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	var storedPassword string
	err = db.QueryRow("SELECT password FROM users LIMIT 1").Scan(&storedPassword)
	if err != nil || storedPassword != passwordData.CurrentPassword {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Current password is incorrect"})
		return
	}

	_, err = db.Exec("UPDATE users SET password = ?", passwordData.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully!"})
}
