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
	brokerUrl := "wss://" + hostname + ":5101/ws"

	// User und BrokerUrl als JSON zurückgeben
	c.JSON(http.StatusOK, gin.H{"username": user.Username, "password": user.Password, "brokerUrl": brokerUrl})
}
