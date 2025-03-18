// USER MANAGEMENT FÜR MQTT-BROKER
package logic

import (
	"database/sql"
	"fmt"

	_ "github.com/glebarez/go-sqlite" // Import für SQLite
	"github.com/sirupsen/logrus"
)

// AddUser fügt einen neuen Benutzer zur Datenbank hinzu
func addUser(db *sql.DB, username, password string, allow bool, filters Filters) error {
	// Check if the user already exists
	exists, err := userExists(db, username)
	if err != nil {
		return err
	}
	if exists {
		// return fmt.Errorf("user %s already exists", username)
		return nil
	}

	// Füge den neuen Benutzer zur Authentifizierungstabelle hinzu
	_, err = db.Exec("INSERT INTO auth (username, password, allow) VALUES (?, ?, ?)", username, password, allow)
	if err != nil {
		return err
	}

	// Füge die ACL für den Benutzer hinzu
	for topic, permission := range filters {
		_, err := db.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", username, topic, permission)
		if err != nil {
			logrus.Printf("Created user %s with password %s", username, password)
			return err
		}
	}
	return nil
}

// deleteUser löscht einen Benutzer aus der Datenbank
func deleteUser(db *sql.DB, username string) error {
	// Lösche den Benutzer aus der Authentifizierungstabelle
	_, err := db.Exec("DELETE FROM auth WHERE username = ?", username)
	if err != nil {
		return err
	}

	// Lösche die ACL-Einträge des Benutzers
	_, err = db.Exec("DELETE FROM acl WHERE username = ?", username)
	if err != nil {
		return err
	}

	return nil
}

func userExists(db *sql.DB, username string) (bool, error) {
	var userCount int
	err := db.QueryRow("SELECT COUNT(*) FROM auth WHERE username = ?", username).Scan(&userCount)
	if err != nil {
		return false, err
	}
	return userCount > 0, nil
}

// WebAccessManagement verwaltet den Web-UI-Benutzer in der Datenbank
func NodeREDAccessManagement(db *sql.DB) error {
	username := "nodered"
	password := genRandomPW()
	filters := map[string]int{
		"#": 3, // Lese- und Schreibzugriff auf alle Topics
	}

	err := addUser(db, username, password, true, filters)
	if err != nil {
		deleteUser(db, username)
		err = addUser(db, username, password, true, filters)
		if err != nil {
			return fmt.Errorf("failed to create user for nodered: %v", err)
		}
	}

	return nil
}

// WebAccessManagement verwaltet den Web-UI-Benutzer in der Datenbank
func WebAccessManagement(db *sql.DB) error {
	username := "web"
	password := genRandomPW()
	filters := map[string]int{
		"data/#": 3, // Lese- und Schreibzugriff auf Daten-Topics
		"#":      1, // Lesezugriff auf alle Topics
	}

	err := addUser(db, username, password, true, filters)
	if err != nil {
		deleteUser(db, username)
		err = addUser(db, username, password, true, filters)
		if err != nil {
			return fmt.Errorf("failed to create user for webui: %v", err)
		}
	}

	return nil
}

// ExternalDriverAccessManagement verwaltet den Zugriff auf die Treiber-Topics
func ExternalDriverAccessManagement(db *sql.DB) error {
	username := "driver"
	password := genRandomPW()
	filters := map[string]int{
		"#":               1,
		"data/#":          3, // Lese- und Schreibzugriff auf Daten-Topics
		"driver/states/#": 3, // Schreibzugriff auf Treiberzustände
	}

	err := addUser(db, username, password, true, filters)
	if err != nil {
		deleteUser(db, username)
		err = addUser(db, username, password, true, filters)
		if err != nil {
			return fmt.Errorf("failed to create user for driver: %v", err)
		}
	}

	return nil
}

// AddAdminUser fügt einen neuen Admin-Benutzer zur Datenbank hinzu
func AddAdminUser(db *sql.DB) error {
	username := "admin"
	password := "password"
	filters := map[string]int{
		"#": 3, // voller Zugriff auf alle Topics
	}

	err := addUser(db, username, password, true, filters)
	if err != nil {
		deleteUser(db, username)
		err = addUser(db, username, password, true, filters)
		if err != nil {
			return fmt.Errorf("failed to create user for admin: %v", err)
		}
	}

	return nil
}
