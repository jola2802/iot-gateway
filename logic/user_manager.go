// USER MANAGEMENT FÜR MQTT-BROKER
package logic

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/glebarez/go-sqlite" // Import für SQLite
	"github.com/sirupsen/logrus"
)

func userExists(db *sql.DB, username string) (bool, error) {
	var userCount int
	err := db.QueryRow("SELECT COUNT(*) FROM auth WHERE username = ?", username).Scan(&userCount)
	if err != nil {
		return false, err
	}
	return userCount > 0, nil
}

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

// changeUser ändert die Informationen eines Benutzers in der Datenbank
func changeUser(db *sql.DB, username, newPassword string, newAllow bool, newFilters Filters) error {
	// Aktualisiere die Authentifizierungsdaten des Benutzers
	_, err := db.Exec("UPDATE auth SET password = ?, allow = ? WHERE username = ?", newPassword, newAllow, username)
	if err != nil {
		return err
	}

	// Lösche die bestehenden ACLs und füge die neuen ein
	_, err = db.Exec("DELETE FROM acl WHERE username = ?", username)
	if err != nil {
		return err
	}

	// Füge die neuen ACL-Einträge hinzu
	for topic, permission := range newFilters {
		_, err := db.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", username, topic, permission)
		if err != nil {
			return err
		}
	}

	return nil
}

// DriverAccessManagement verwaltet die Treiberbenutzer und einzelnen Geräte in der Datenbank
func DriverAccessManagement(db *sql.DB) error {
	devices, err := LoadDevices(db)
	if err != nil {
		logrus.Errorf("User Manager: failed to load devices from config: %v", err)
		return err
	}

	for _, device := range devices {
		// Extrahiere die relevanten Felder aus der Map
		username, ok := device["name"].(string)
		if !ok {
			logrus.Error("User Manager: device has no name or name is not a string")
			continue // Bei fehlendem Namen fortfahren, aber keinen Benutzer anlegen
		}

		deviceType, ok := device["type"].(string)
		if !ok {
			logrus.Error("User Manager: device has no type or type is not a string")
			continue // Bei fehlendem Typ fortfahren, aber keinen Benutzer anlegen
		}

		// Generiere zufälliges Passwort für das jeweilige Gerät
		password := genRandomPW()

		// Erstelle Filter für MQTT-Berechtigungen
		filters := map[string]int{
			fmt.Sprintf("data/%s/%s", deviceType, username): 3, // Lese- und Schreibzugriff auf Treiber-spezifische Topics
			"iot-gateway/#": 0, // Kein Zugriff auf andere iot-gateway-Topics
		}

		err := addUser(db, username, password, true, filters)
		if err != nil {
			deleteUser(db, username) // Lösche den Benutzer, falls er existiert
			err = addUser(db, username, password, true, filters)
			if err != nil {
				logrus.Errorf("User Manager: failed to create user for %s: %v", username, err)
				continue // Bei Fehlschlag fortfahren
			}
		}

		// log.Printf("User Manager: successfully created user for device %s with password %s", username, password)
	}
	return nil
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
