package webui

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
)

var cachedBrokerSettings *BrokerSettings

type BrokerSettings struct {
	Address  string
	Username string
	Password string
}

// User defines a user with their associated ACL entries.
type User struct {
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	AclEntries []ACLEntry `json:"aclEntries"`
}

// ACLEntry defines the ACL topic and its associated permission for a user.
type ACLEntry struct {
	Topic      string `json:"topic"`
	Permission int    `json:"permission"` // Als int definiert
}

// Settings repräsentiert die Einstellungen der Anwendung
type Settings struct {
	DockerIP          string `json:"docker_ip"`
	UseCustomServices bool   `json:"use_custom_services"`
	NodeRedURL        string `json:"nodered_url,omitempty"`
	InfluxDBURL       string `json:"influxdb_url,omitempty"`
	UseExternalBroker bool   `json:"use_external_broker"`
	BrokerURL         string `json:"broker_url,omitempty"`
	BrokerPort        string `json:"broker_port,omitempty"`
	BrokerUsername    string `json:"broker_username,omitempty"`
	BrokerPassword    string `json:"broker_password,omitempty"`
}

func getSettings(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Datenbankverbindung nicht gefunden"})
		return
	}

	dbConn := db.(*sql.DB)

	var settings Settings
	err := dbConn.QueryRow(`
		SELECT docker_ip, use_custom_services, nodered_url, influxdb_url, 
		       use_external_broker, broker_url, broker_port, broker_username, broker_password 
		FROM settings WHERE id = 1
	`).Scan(
		&settings.DockerIP, &settings.UseCustomServices, &settings.NodeRedURL, &settings.InfluxDBURL,
		&settings.UseExternalBroker, &settings.BrokerURL, &settings.BrokerPort, &settings.BrokerUsername, &settings.BrokerPassword,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Wenn keine Einstellungen gefunden wurden, sende leere Einstellungen
			c.JSON(http.StatusOK, Settings{})
			return
		}
		log.Printf("Fehler beim Lesen der Einstellungen: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Fehler beim Lesen der Einstellungen"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func updateSettings(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Datenbankverbindung nicht gefunden"})
		return
	}

	dbConn := db.(*sql.DB)

	var settings Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ungültige Einstellungsdaten"})
		return
	}

	// Aktualisiere oder füge neue Einstellungen ein
	_, err := dbConn.Exec(`
		INSERT OR REPLACE INTO settings (
			id, docker_ip, use_custom_services, nodered_url, influxdb_url,
			use_external_broker, broker_url, broker_port, broker_username, broker_password
		) VALUES (
			1, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`,
		settings.DockerIP, settings.UseCustomServices, settings.NodeRedURL, settings.InfluxDBURL,
		settings.UseExternalBroker, settings.BrokerURL, settings.BrokerPort, settings.BrokerUsername, settings.BrokerPassword,
	)

	if err != nil {
		log.Printf("Fehler beim Speichern der Einstellungen: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Fehler beim Speichern der Einstellungen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Einstellungen erfolgreich gespeichert"})
}

// showSettingsPage shows the settings page, returning broker settings and user data.
func showSettingsPage(c *gin.Context) {
	// Get the db instance from the gin.Context
	dbConn, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch broker settings
	address, err := fetchBrokerSettings(dbConn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching broker settings"})
		return
	}

	// Fetch 'webui-admin' user's password
	username, password, err := fetchUserCredentials(dbConn, "webui-admin")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching password for user 'webui-admin'"})
		return
	}

	// Fetch all users and their ACL entries
	users, err := fetchUsersAndACLs(dbConn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching users and ACLs"})
		return
	}

	// Render response based on the Accept header
	respondWithSettingsPage(c, address, username, password, users)
}

// fetchBrokerSettings fetches the broker address from the database.
func fetchBrokerSettings(db *sql.DB) (string, error) {
	var address string
	err := db.QueryRow("SELECT address FROM broker_settings WHERE id = 1").Scan(&address)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return address, nil
}

// fetchUserCredentials fetches the password of a given username from the auth table.
func fetchUserCredentials(db *sql.DB, username string) (string, string, error) {
	var password string
	err := db.QueryRow("SELECT password FROM auth WHERE username = ?", username).Scan(&password)
	if err != nil {
		return "", "", err
	}
	return username, password, nil
}

// fetchUsersAndACLs fetches users and their associated ACL entries from the database.
func fetchUsersAndACLs(db *sql.DB) ([]User, error) {
	// Fetch users
	userMap, err := fetchUsers(db)
	if err != nil {
		return nil, err
	}

	// Fetch ACL entries for users
	err = fetchACLs(db, userMap)
	if err != nil {
		return nil, err
	}

	// Convert the user map to a slice
	var users []User
	for _, user := range userMap {
		users = append(users, *user)
	}
	return users, nil
}

// fetchUsers retrieves users from the auth table and returns a map of users.
func fetchUsers(db *sql.DB) (map[string]*User, error) {
	rows, err := db.Query("SELECT username, password FROM auth")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userMap := make(map[string]*User)

	// Populate users
	for rows.Next() {
		var username, password string
		if err := rows.Scan(&username, &password); err != nil {
			return nil, err
		}

		// Add user to the map
		userMap[username] = &User{
			Username:   username,
			Password:   password,
			AclEntries: []ACLEntry{},
		}
	}
	return userMap, nil
}

// fetchACLs retrieves ACL entries from the acl table and adds them to users in the map.
func fetchACLs(db *sql.DB, userMap map[string]*User) error {
	rows, err := db.Query("SELECT username, topic, permission FROM acl")
	if err != nil {
		return err
	}
	defer rows.Close()

	// Populate ACL entries for users
	for rows.Next() {
		var username, topic string
		var permission int
		if err := rows.Scan(&username, &topic, &permission); err != nil {
			return err
		}

		// Add ACL entry to the appropriate user
		if user, exists := userMap[username]; exists {
			user.AclEntries = append(user.AclEntries, ACLEntry{
				Topic:      topic,
				Permission: permission,
			})
		}
	}
	return nil
}

// respondWithSettingsPage sends either a JSON or HTML response based on the Accept header.
func respondWithSettingsPage(c *gin.Context, address, username, password string, users []User) {
	acceptHeader := c.Request.Header.Get("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"address":  address,
			"username": username,
			"password": password,
			"users":    users,
		})
	} else {
		c.HTML(http.StatusOK, "settings.html", gin.H{
			"address":  address,
			"username": username,
			"password": password,
			"users":    users,
		})
		// HTML-Antwort mit dem eingebetteten Template zurückgeben
		// c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		// err := tmpl.ExecuteTemplate(c.Writer, "settings.html", gin.H{
		// 	"address":  address,
		// 	"username": username,
		// 	"password": password,
		// 	"users":    users,
		// })
		// if err != nil {
		// 	c.String(http.StatusInternalServerError, "Error rendering template: %v", err)
		// }
	}
}

// updateBrokerSettings updates the broker settings in the database based on the user's input.
func updateBrokerSettings(c *gin.Context) {
	// Get the db instance from the gin.Context
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	dbConn := db.(*sql.DB)

	var settingsData struct {
		Address  string `json:"address"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&settingsData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	_, err := dbConn.Exec("REPLACE INTO broker_settings (id, address, username, password) VALUES (?, ?, ?, ?)",
		1, settingsData.Address, settingsData.Username, settingsData.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating broker settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Broker settings updated successfully!"})
}

// getBrokerSettings fetches the broker settings from the database.
func getBrokerSettings(db *sql.DB) (BrokerSettings, error) {
	var settings BrokerSettings
	// Broker address will still come from broker_settings
	err := db.QueryRow("SELECT address FROM broker_settings WHERE id = 1").Scan(&settings.Address)
	if err != nil {
		return settings, fmt.Errorf("error fetching broker address: %v", err)
	}

	// Fetch the password for the username 'webui' from the auth table
	settings.Username = "webui"
	err = db.QueryRow("SELECT password FROM auth WHERE username = ?", settings.Username).Scan(&settings.Password)
	if err != nil {
		return settings, fmt.Errorf("error fetching password for user 'webui': %v", err)
	}

	return settings, nil
}

// getCachedBrokerSettings returns the broker settings, either from the cache or by querying the database.
// It caches the settings to avoid frequent database queries.
func getCachedBrokerSettings(db *sql.DB) (BrokerSettings, error) {
	if cachedBrokerSettings != nil {
		return *cachedBrokerSettings, nil
	}
	settings, err := getBrokerSettings(db)
	if err != nil {
		return settings, err
	}
	cachedBrokerSettings = &settings // Cache the settings
	return settings, nil
}

// deleteUserHandler löscht einen Benutzer und seine ACL-Einträge.
func deleteUserHandler(c *gin.Context) {
	username := c.Param("username")

	dbConn, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	// Lösche den Benutzer aus der auth-Tabelle
	_, err = dbConn.Exec("DELETE FROM auth WHERE username = ?", username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting user"})
		return
	}

	// Lösche die ACL-Einträge des Benutzers
	_, err = dbConn.Exec("DELETE FROM acl WHERE username = ?", username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting ACL entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// updateUserHandler bearbeitet einen Benutzer und aktualisiert Passwort und ACLs.
func updateUserHandler(c *gin.Context) {
	username := c.Param("username")

	var userData struct {
		Password   string     `json:"password"`
		AclEntries []ACLEntry `json:"aclEntries"`
	}

	if err := c.ShouldBindJSON(&userData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	dbConn, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	// Aktualisiere Passwort des Benutzers
	_, err = dbConn.Exec("UPDATE auth SET password = ? WHERE username = ?", userData.Password, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating password"})
		return
	}

	// Lösche vorhandene ACL-Einträge
	_, err = dbConn.Exec("DELETE FROM acl WHERE username = ?", username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting ACL entries"})
		return
	}

	// Füge neue ACL-Einträge hinzu
	for _, entry := range userData.AclEntries {
		_, err = dbConn.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)", username, entry.Topic, entry.Permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting ACL entries"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

func createUserHandler(c *gin.Context) {
	// Get the db instance from the gin.Context
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	dbConn := db.(*sql.DB)
	log.Println("Database connection established")

	// Parse the incoming JSON request
	var userPayload User
	if err := c.ShouldBindJSON(&userPayload); err != nil {
		log.Printf("Invalid request data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request data"})
		return
	}

	// Check if username, password, and ACL entries are provided
	if userPayload.Username == "" || userPayload.Password == "" || len(userPayload.AclEntries) == 0 {
		log.Println("Missing required fields: username, password or ACL entries are empty")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing required fields"})
		return
	}

	// Check if user already exists
	var existingUser bool
	err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM auth WHERE username = ?)", userPayload.Username).Scan(&existingUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error checking if user exists"})
		return
	}
	if existingUser {
		c.JSON(http.StatusConflict, gin.H{"message": "User already exists"})
		return
	}

	// Insert new user into the auth table
	_, err = dbConn.Exec("INSERT INTO auth (username, password, allow) VALUES (?, ?, ?)", userPayload.Username, userPayload.Password, 1)
	if err != nil {
		log.Printf("Error adding user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error adding user"})
		return
	}

	// Insert ACL entries for the new user
	for _, aclEntry := range userPayload.AclEntries {
		_, err = dbConn.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)",
			userPayload.Username, aclEntry.Topic, aclEntry.Permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error adding ACL entries"})
			return
		}
	}

	// Success response
	c.JSON(http.StatusOK, gin.H{"message": "User added successfully"})
}

// InitSettingsTable erstellt die settings-Tabelle, falls sie nicht existiert
func InitSettingsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY,
			docker_ip TEXT,
			use_custom_services BOOLEAN DEFAULT FALSE,
			nodered_url TEXT,
			influxdb_url TEXT,
			use_external_broker BOOLEAN DEFAULT FALSE,
			broker_url TEXT,
			broker_port TEXT,
			broker_username TEXT,
			broker_password TEXT
		)
	`)
	return err
}
