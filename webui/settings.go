package webui

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	MQTT "github.com/mochi-mqtt/server/v2"
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

// getDBConnection retrieves the database connection from the gin.Context.
func getDBConnection(c *gin.Context) (*sql.DB, error) {
	db, exists := c.Get("db")
	if !exists {
		return nil, errors.New("database connection not found")
	}
	return db.(*sql.DB), nil
}

// getDBConnection retrieves the database connection from the gin.Context.
func getMQTTServer(c *gin.Context) (*MQTT.Server, error) {
	server, exists := c.Get("server")
	if !exists {
		return nil, errors.New("database connection not found")
	}
	return server.(*MQTT.Server), nil
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

// updateBrokerUser aktualisiert das Passwort und die ACL-Einträge für einen Benutzer
func updateBrokerUser(c *gin.Context) {
	// Holen Sie die DB-Instanz aus dem Context
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database connection not found"})
		return
	}

	dbConn := db.(*sql.DB)

	// Struktur für die erhaltenen JSON-Daten
	var userData struct {
		Username   string     `json:"username"`
		Password   string     `json:"password"`
		AclEntries []ACLEntry `json:"aclEntries"`
	}

	// JSON Body in die userData Struktur binden
	if err := c.ShouldBindJSON(&userData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request data"})
		return
	}

	// Überprüfen, ob der Benutzer existiert
	var existsInAuth bool
	err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM auth WHERE username = ?)", userData.Username).Scan(&existsInAuth)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error checking user existence"})
		return
	}

	if !existsInAuth {
		c.JSON(http.StatusBadRequest, gin.H{"message": "User does not exist"})
		return
	}

	// Schritt 1: Aktualisieren des Passworts in der auth-Tabelle
	_, err = dbConn.Exec("UPDATE auth SET password = ? WHERE username = ?", userData.Password, userData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating password"})
		return
	}

	// Schritt 2: Löschen der alten ACL-Einträge für den Benutzer
	_, err = dbConn.Exec("DELETE FROM acl WHERE username = ?", userData.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting old ACL entries"})
		return
	}

	// Schritt 3: Hinzufügen der neuen ACL-Einträge
	for _, entry := range userData.AclEntries {
		_, err = dbConn.Exec("INSERT INTO acl (username, topic, permission) VALUES (?, ?, ?)",
			userData.Username, entry.Topic, entry.Permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error inserting ACL entries"})
			return
		}
	}

	// Erfolgsmeldung zurückgeben
	c.JSON(http.StatusOK, gin.H{"message": "User and ACL entries updated successfully"})
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
