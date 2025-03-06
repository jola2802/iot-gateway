package logic

import (
	"database/sql"
	"os"

	_ "github.com/glebarez/go-sqlite"
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
			company TEXT,
			email TEXT
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
			device_id INT NOT NULL,
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL,
			datatype VARCHAR(100) NOT NULL,
			address VARCHAR(20) NOT NULL
		);
	`

	createOPCUADatanodesTable = `
		CREATE TABLE IF NOT EXISTS opcua_datanodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INT NOT NULL,
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL,
			node_identifier VARCHAR(100) NOT NULL
		);
	`

	createImagesTable = `
		CREATE TABLE IF NOT EXISTS images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device TEXT NOT NULL,
			image TEXT NOT NULL,
			timestamp TEXT NOT NULL
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
		createDevicesTable,
		createS7DatapointsTable,
		createOPCUADatanodesTable,
		createImagesTable,
	}

	// Tabellen erstellen
	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return nil, err
		}
	}

	// Check if there are any users in the database
	var countUsers int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&countUsers)

	// If no users are found, create a default admin user
	if countUsers == 0 {
		db.Exec(`
            INSERT INTO users (username, password, name, address, company, email)
            VALUES (?, ?, ?, ?, ?, ?)
        `, "admin", "password", "Admin", "Admin Street", "Admin Corp.", "admin@admin.com")
	}

	return db, nil
}
