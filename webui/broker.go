package webui

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func showBrokerPage(c *gin.Context) {
	c.HTML(http.StatusOK, "broker.html", nil)
}

// Gibt User und deren ACL-Einträge zurück
func getAllBrokerUsers(c *gin.Context) {
	db, _ := getDBConnection(c)

	// Abfrage für alle Benutzer aus der auth-Tabelle
	authQuery := `
		SELECT username, password
		FROM auth
	`
	rows, err := db.Query(authQuery)
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Username, &user.Password); err != nil {
			logrus.Error(err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// ACL-Daten für den Benutzer abrufen
		aclQuery := `
			SELECT topic, permission
			FROM acl
			WHERE username = ?
		`
		aclRows, err := db.Query(aclQuery, user.Username)
		if err != nil {
			logrus.Error(err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer aclRows.Close()

		// ACL-Einträge dem Benutzer zuweisen
		acls := []ACLEntry{}
		for aclRows.Next() {
			var acl ACLEntry
			if err := aclRows.Scan(&acl.Topic, &acl.Permission); err != nil {
				logrus.Error(err)
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			acls = append(acls, acl)
		}
		user.AclEntries = acls
		users = append(users, user)
	}

	// Ergebnis als JSON zurückgeben
	c.JSON(http.StatusOK, users)
}

// Funktion zum Erhalt eines Benutzers und dessen ACL-Einträge
func getBrokerUser(c *gin.Context) {
	// Hole Username als Parameter
	username := c.Param("username")

	// Hole Datenbankverbindung
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Hole Benutzerdaten
	authQuery := `
		SELECT username, password
		FROM auth
		WHERE username = ?
	`
	rows, err := db.Query(authQuery, username)
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Benutzerdaten in struct speichern
	var user User
	if rows.Next() {
		if err := rows.Scan(&user.Username, &user.Password); err != nil {
			logrus.Error(err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	} else {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	// Hole ACL-Einträge
	aclQuery := `
		SELECT topic, permission
		FROM acl
		WHERE username = ?
	`
	aclRows, err := db.Query(aclQuery, user.Username)
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer aclRows.Close()

	// ACL-Einträge in struct speichern
	acls := []ACLEntry{}
	for aclRows.Next() {
		var acl ACLEntry
		if err := aclRows.Scan(&acl.Topic, &acl.Permission); err != nil {
			logrus.Error(err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		acls = append(acls, acl)
	}
	user.AclEntries = acls

	// Ergebnis als JSON zurückgeben
	c.JSON(http.StatusOK, user)
}

// Funktion zum Erhalt der Broker Login-Daten für Broker Seite
func getBrokerLogin(c *gin.Context) {
	db, _ := getDBConnection(c)

	// Hole Benutzerdaten
	authQuery := `
		SELECT password
		FROM auth
		WHERE username = ?
	`
	var user User
	user.Username = "admin"

	err := db.QueryRow(authQuery, "admin").Scan(&user.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			logrus.Error("No user found with username: admin")
			c.JSON(404, gin.H{"error": "User not found"})
		} else {
			logrus.Error("SQL error: ", err)
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return
	}

	// Schreibe die URL des Brokers in die Variable brokerURL
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Error("Failed to get hostname: ", err)
		hostname = "localhost"
	}
	brokerUrl := "wss://" + hostname + ":5101/"

	// User und BrokerUrl als JSON zurückgeben
	c.JSON(http.StatusOK, gin.H{"username": user.Username, "password": user.Password, "brokerUrl": brokerUrl})
}

func addBrokerUser(c *gin.Context) {
	type User struct {
		Username string     `json:"username"`
		Password string     `json:"password"`
		ACLs     []ACLEntry `json:"acls"`
	}

	var userData User

	// JSON-Daten binden
	if err := c.ShouldBindJSON(&userData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	// Extrahiere die Datenbankverbindung aus dem Context
	db, _ := getDBConnection(c)

	var exists bool
	// Überprüfen, ob der Benutzername bereits existiert
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM auth WHERE username = ?)", userData.Username).Scan(&exists)
	if err != nil {
		logrus.Println("Error checking if username exists:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error checking if username exists"})
		return
	}

	if exists {
		// Lösche den alten Benutzer
		_, err = db.Exec("DELETE FROM auth WHERE username = ?", userData.Username)
		if err != nil {
			logrus.Println("Error deleting user:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting user"})
			return
		}
		// Löschen der ACL-Einträge
		_, err = db.Exec("DELETE FROM acl WHERE username = ?", userData.Username)
		if err != nil {
			logrus.Println("Error deleting acl entries:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting acl entries"})
			return
		}
	}

	// Füge den Benutzer in die auth-Tabelle ein
	query := `
		INSERT INTO auth (username, password, allow)
		VALUES (?, ?, 1)
	`
	_, err = db.Exec(query, userData.Username, userData.Password)
	if err != nil {
		logrus.Println("Error inserting user data into the database:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting user data"})
		return
	}
	// Inserting acl entries into acl table
	for _, entry := range userData.ACLs {
		_, err = db.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", userData.Username, entry.Topic, entry.Permission)
		if err != nil {
			logrus.Println("Error inserting acl entries into the database:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting acl entries"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "User added successfully"})
}

func deleteBrokerUser(c *gin.Context) {
	db, err := getDBConnection(c)
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Username aus der URL holen
	username := c.Param("username")

	// Prüfen ob Benutzer existiert
	var existingUser string
	err = db.QueryRow("SELECT username FROM auth WHERE username = ?", username).Scan(&existingUser)
	if err == sql.ErrNoRows {
		c.JSON(404, gin.H{"error": "Benutzer existiert nicht"})
		return
	}

	// Transaktion starten
	tx, err := db.Begin()
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Erst ACL-Einträge löschen (wegen Foreign Key)
	_, err = tx.Exec("DELETE FROM acl WHERE username = ?", username)
	if err != nil {
		tx.Rollback()
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Dann den Benutzer löschen
	_, err = tx.Exec("DELETE FROM auth WHERE username = ?", username)
	if err != nil {
		tx.Rollback()
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Transaktion bestätigen
	err = tx.Commit()
	if err != nil {
		logrus.Error(err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Erfolgreiche Antwort
	c.JSON(http.StatusOK, gin.H{
		"message":  "Benutzer erfolgreich gelöscht",
		"username": username,
	})
}
