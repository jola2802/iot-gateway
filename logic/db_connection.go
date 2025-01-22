package logic

import (
	"database/sql"
	"os"

	_ "github.com/glebarez/go-sqlite"
	"github.com/sirupsen/logrus"
)

// SQL-Queries als Konstanten definieren
const (
	createAuthTable = `
		CREATE TABLE IF NOT EXISTS auth (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			allow BOOLEAN NOT NULL
		);
	`

	createACLTable = `
		CREATE TABLE IF NOT EXISTS acl (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			topic TEXT NOT NULL,
			permission INTEGER NOT NULL,
			FOREIGN KEY(username) REFERENCES auth(username)
		);
	`

	createUsersTable = `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			name TEXT,
			address TEXT,
			company TEXT
		);
	`

	createBrokerSettingsTable = `
		CREATE TABLE IF NOT EXISTS broker_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT NOT NULL,
			username TEXT,
			password TEXT
		);
	`

	createDevicesTable = `
		CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type VARCHAR(50) NOT NULL,
			name VARCHAR(100) NOT NULL,
			address TEXT NOT NULL,
			acquisition_time INT NOT NULL,
			status TEXT NOT NULL,
			rack INT,					 -- Optional
			slot INT,				     -- Optional
			security_mode TEXT,          -- Optional
			security_policy TEXT,        -- Optional
			certificate TEXT,            -- Optional für Zertifikat-basierte Authentifizierung
			key TEXT,                    -- Optional für Zertifikat-basierte Authentifizierung
			username TEXT,               -- Optional für Username-basierte Authentifizierung
			password TEXT                -- Optional für Passwort-basierte Authentifizierung
		);
	`

	createS7DatapointsTable = `
		CREATE TABLE IF NOT EXISTS s7_datapoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INT REFERENCES devices(id),
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL,
			datatype VARCHAR(100) NOT NULL,
			address VARCHAR(20) NOT NULL
		);
	`

	createOPCUADatanodesTable = `
		CREATE TABLE IF NOT EXISTS opcua_datanodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INT REFERENCES devices(id),
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL,
			node_identifier VARCHAR(100) NOT NULL
		);
	`

	createMQTTDatapointsTable = `
		CREATE TABLE IF NOT EXISTS opcua_datanodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INT REFERENCES devices(id),
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL
		);
	`

	createdataRoutesTable = `
		CREATE TABLE IF NOT EXISTS data_routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			destination_type TEXT NOT NULL,
			data_format TEXT NOT NULL,
			interval INTEGER NOT NULL,
			devices TEXT NOT NULL,
			destination_url TEXT,
			headers TEXT, -- Verwende TEXT für JSON-Daten
			file_path TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`

	createdeviceDataTable = `
		CREATE TABLE IF NOT EXISTS device_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_name TEXT NOT NULL,
			deviceId TEXT,
			datapoint TEXT NOT NULL,
			datapointId TEXT,
			value TEXT NOT NULL, -- Verwende TEXT für JSON-Daten
			timestamp TEXT NOT NULL
		);
	`

	createImageProcessTable = `
		CREATE TABLE IF NOT EXISTS img_process (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device TEXT NOT NULL,
			m_parent_node TEXT NOT NULL,
			m_image_node TEXT NOT NULL,
			image_node TEXT NOT NULL,
			captured_node TEXT NOT NULL,
			timeout TEXT NOT NULL,
			capture_mode TEXT NOT NULL,
			trigger TEXT NOT NULL,
			complete_node TEXT NOT NULL,
			enableUpload BOOLEAN NOT NULL,
			uploadUrl TEXT NOT NULL,
			basePath TEXT NOT NULL,
			logs TEXT DEFAULT NULL,
			status TEXT DEFAULT NULL
		);
	`
)

// InitDB initialisiert die SQLite-Datenbank mit einem übergebenen Pfad
func InitDB(dbPath string) (*sql.DB, error) {
	// Überprüfen, ob die Datenbankdatei existiert
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Datenbankdatei erstellen
		file, err := os.Create(dbPath)
		if err != nil {
			return nil, err
		}
		file.Close()
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// SQL-Tabellen in einer Liste speichern
	tables := []string{
		createAuthTable,
		createACLTable,
		createUsersTable,
		createBrokerSettingsTable,
		createDevicesTable,
		createS7DatapointsTable,
		createOPCUADatanodesTable,
		createMQTTDatapointsTable,
		createdataRoutesTable,
		createdeviceDataTable,
		createImageProcessTable,
	}

	// Tabellen erstellen
	for _, table := range tables {
		if err := execQuery(db, table); err != nil {
			return nil, err
		}
	}

	// Check if there are any users in the database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)

	// If no users are found, create a default admin user
	if count == 0 {
		_, err = db.Exec(`
            INSERT INTO users (username, password, name, address, company)
            VALUES (?, ?, ?, ?, ?)
        `, "admin", "password", "Admin", "Admin Street", "Admin Corp.")
	}

	// Check if there are any broker settings in the database
	err = db.QueryRow("SELECT COUNT(*) FROM broker_settings").Scan(&count)

	// If no broker settings are found, create a default setting
	if count == 0 {
		_, err = db.Exec(`
            INSERT INTO broker_settings (address, username, password)
            VALUES (?, ?, ?)
        `, "ws://127.0.0.1:5001", "admin", "abc+1247")
	}

	return db, nil
}

// execQuery führt eine SQL-Abfrage aus und behandelt Fehler
func execQuery(db *sql.DB, query string) error {
	_, err := db.Exec(query)
	if err != nil {
		logrus.Errorf("Failed to execute query: %v\nQuery: %s", err, query)
		return err
	}
	return nil
}
