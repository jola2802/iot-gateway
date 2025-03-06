// USER MANAGEMENT FÜR MQTT-BROKER
package logic

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/glebarez/go-sqlite" // Import für SQLite
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
			log.Printf("Created user %s with password %s", username, password)
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

// WebUIAccessManagement verwaltet den Web-UI-Benutzer in der Datenbank
func WebUIAccessManagement(db *sql.DB) error {
	username1 := "webui-admin"
	password := genRandomPW()
	filters := map[string]int{
		"#":                           1, // Lesezugriff auf alle Topics
		"data/#":                      3, // Lese- und Schreibzugriff auf Daten-Topics
		"iot-gateway/driver/states/#": 3, // Lese- und Schreibzugriff auf Treiberzustände
		"iot-gateway/config/#":        3, // Lese- und Schreibzugriff auf Konfigurationen
	}

	err := addUser(db, username1, password, true, filters)
	if err != nil {
		deleteUser(db, username1)
		err = addUser(db, username1, password, true, filters)
		if err != nil {
			return fmt.Errorf("failed to create user for webui-admin: %v", err)
		}
	}

	username2 := "webui"
	password = genRandomPW()
	filters = map[string]int{
		"#": 3, // Voller Zugriff auf alle Topics
	}

	err = addUser(db, username2, password, true, filters)
	if err != nil {
		deleteUser(db, username2)
		err = addUser(db, username2, password, true, filters)
		if err != nil {
			return fmt.Errorf("failed to create user for webui: %v", err)
		}
	}

	return nil
}

// WebUIAccessManagement verwaltet den Web-UI-Benutzer in der Datenbank
func AddAdminUser(db *sql.DB) error {
	username := "admin"
	// username2 := "webui-user"
	password := "abc+1247"
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
